package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// RabbitMQ provides RabbitMQ message broker testing capabilities
type RabbitMQ struct {
	name      string
	config    config.Resource
	container *container.Manager

	conn    *amqp.Connection
	channel *amqp.Channel

	// Message storage
	messages     map[string][]*amqp.Delivery // queue -> messages
	messagesMu   sync.RWMutex
	lastMessage  *amqp.Delivery
	consuming    map[string]bool
	consumingMu  sync.RWMutex
	stopChannels map[string]chan struct{}

	// Track declared resources for reset
	declaredQueues    []string
	declaredExchanges []string
}

// NewRabbitMQ creates a new RabbitMQ handler
func NewRabbitMQ(name string, cfg config.Resource, cm *container.Manager) (*RabbitMQ, error) {
	return &RabbitMQ{
		name:              name,
		config:            cfg,
		container:         cm,
		messages:          make(map[string][]*amqp.Delivery),
		consuming:         make(map[string]bool),
		stopChannels:      make(map[string]chan struct{}),
		declaredQueues:    make([]string, 0),
		declaredExchanges: make([]string, 0),
	}, nil
}

func (r *RabbitMQ) Name() string { return r.name }

func (r *RabbitMQ) Init(ctx context.Context) error {
	url, err := r.getConnectionURL(ctx)
	if err != nil {
		return fmt.Errorf("getting connection URL: %w", err)
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("connecting to RabbitMQ: %w", err)
	}
	r.conn = conn

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("opening channel: %w", err)
	}
	r.channel = ch

	// Pre-declare resources from config
	if err := r.declareConfiguredResources(); err != nil {
		return fmt.Errorf("declaring configured resources: %w", err)
	}

	return nil
}

func (r *RabbitMQ) getConnectionURL(ctx context.Context) (string, error) {
	// Check for direct URL in config
	if r.config.URL != "" {
		return r.config.URL, nil
	}

	// Get from container
	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return "", fmt.Errorf("getting container host: %w", err)
	}

	port, err := r.container.GetPort(ctx, r.config.Container, "5672/tcp")
	if err != nil {
		return "", fmt.Errorf("getting container port: %w", err)
	}

	user := "guest"
	pass := "guest"
	vhost := "/"

	if u, ok := r.config.Options["user"].(string); ok {
		user = u
	}
	if p, ok := r.config.Options["password"].(string); ok {
		pass = p
	}
	if v, ok := r.config.Options["vhost"].(string); ok {
		vhost = v
	}

	return fmt.Sprintf("amqp://%s:%s@%s:%s%s", user, pass, host, port, vhost), nil
}

func (r *RabbitMQ) declareConfiguredResources() error {
	// Declare exchanges from config
	if exchanges, ok := r.config.Options["exchanges"].([]interface{}); ok {
		for _, e := range exchanges {
			if ex, ok := e.(map[string]interface{}); ok {
				name := fmt.Sprintf("%v", ex["name"])
				exType := "direct"
				if t, ok := ex["type"].(string); ok {
					exType = t
				}
				durable := false
				if d, ok := ex["durable"].(bool); ok {
					durable = d
				}
				if err := r.channel.ExchangeDeclare(name, exType, durable, false, false, false, nil); err != nil {
					return fmt.Errorf("declaring exchange %s: %w", name, err)
				}
				r.declaredExchanges = append(r.declaredExchanges, name)
			}
		}
	}

	// Declare queues from config
	if queues, ok := r.config.Options["queues"].([]interface{}); ok {
		for _, q := range queues {
			if qu, ok := q.(map[string]interface{}); ok {
				name := fmt.Sprintf("%v", qu["name"])
				durable := false
				if d, ok := qu["durable"].(bool); ok {
					durable = d
				}
				autoDelete := false
				if ad, ok := qu["auto_delete"].(bool); ok {
					autoDelete = ad
				}
				if _, err := r.channel.QueueDeclare(name, durable, autoDelete, false, false, nil); err != nil {
					return fmt.Errorf("declaring queue %s: %w", name, err)
				}
				r.declaredQueues = append(r.declaredQueues, name)
			}
		}
	}

	// Create bindings from config
	if bindings, ok := r.config.Options["bindings"].([]interface{}); ok {
		for _, b := range bindings {
			if bind, ok := b.(map[string]interface{}); ok {
				queue := fmt.Sprintf("%v", bind["queue"])
				exchange := fmt.Sprintf("%v", bind["exchange"])
				routingKey := ""
				if rk, ok := bind["routing_key"].(string); ok {
					routingKey = rk
				}
				if err := r.channel.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
					return fmt.Errorf("binding queue %s to exchange %s: %w", queue, exchange, err)
				}
			}
		}
	}

	return nil
}

func (r *RabbitMQ) Ready(ctx context.Context) error {
	if r.channel == nil {
		return fmt.Errorf("channel not initialized")
	}
	// Try a simple operation to verify connection
	_, err := r.channel.QueueInspect("amq.rabbitmq.reply-to")
	// Ignore "not found" errors - we just want to verify connection works
	if err != nil && !strings.Contains(err.Error(), "NOT_FOUND") {
		return err
	}
	return nil
}

func (r *RabbitMQ) Reset(ctx context.Context) error {
	r.stopAllConsumers()

	r.messagesMu.Lock()
	r.messages = make(map[string][]*amqp.Delivery)
	r.lastMessage = nil
	r.messagesMu.Unlock()

	strategy := "purge"
	if s, ok := r.config.Options["reset_strategy"].(string); ok {
		strategy = s
	}

	switch strategy {
	case "purge":
		return r.purgeQueues()
	case "delete_recreate":
		return r.deleteAndRecreate()
	case "none":
		return nil
	default:
		return r.purgeQueues()
	}
}

func (r *RabbitMQ) purgeQueues() error {
	for _, queue := range r.declaredQueues {
		if _, err := r.channel.QueuePurge(queue, false); err != nil {
			log.Warn().Err(err).Str("queue", queue).Msg("error purging queue")
		}
	}
	return nil
}

func (r *RabbitMQ) deleteAndRecreate() error {
	// Delete queues
	for _, queue := range r.declaredQueues {
		if _, err := r.channel.QueueDelete(queue, false, false, false); err != nil {
			log.Warn().Err(err).Str("queue", queue).Msg("error deleting queue")
		}
	}

	// Delete exchanges
	for _, exchange := range r.declaredExchanges {
		if err := r.channel.ExchangeDelete(exchange, false, false); err != nil {
			log.Warn().Err(err).Str("exchange", exchange).Msg("error deleting exchange")
		}
	}

	// Recreate
	r.declaredQueues = make([]string, 0)
	r.declaredExchanges = make([]string, 0)

	return r.declareConfiguredResources()
}

func (r *RabbitMQ) stopAllConsumers() {
	r.consumingMu.Lock()
	defer r.consumingMu.Unlock()

	for queue, stopCh := range r.stopChannels {
		close(stopCh)
		delete(r.stopChannels, queue)
		r.consuming[queue] = false
	}
}

func (r *RabbitMQ) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the RabbitMQ handler
func (r *RabbitMQ) Steps() StepCategory {
	return StepCategory{
		Name:        "RabbitMQ",
		Description: "Steps for interacting with RabbitMQ message broker",
		Steps: []StepDef{
			// Queue Management
			{
				Group:       "Queue Management",
				Pattern:     `^"{resource}" declares queue "([^"]*)"$`,
				Description: "Declares a queue with default settings",
				Example:     `"{resource}" declares queue "orders"`,
				Handler:     r.declareQueue,
			},
			{
				Group:       "Queue Management",
				Pattern:     `^"{resource}" declares durable queue "([^"]*)"$`,
				Description: "Declares a durable queue",
				Example:     `"{resource}" declares durable queue "orders"`,
				Handler:     r.declareDurableQueue,
			},
			{
				Group:       "Queue Management",
				Pattern:     `^"{resource}" queue "([^"]*)" exists$`,
				Description: "Asserts a queue exists",
				Example:     `"{resource}" queue "orders" exists`,
				Handler:     r.queueExists,
			},
			{
				Group:       "Queue Management",
				Pattern:     `^"{resource}" purges queue "([^"]*)"$`,
				Description: "Purges all messages from a queue",
				Example:     `"{resource}" purges queue "orders"`,
				Handler:     r.purgeQueue,
			},

			// Exchange Management
			{
				Group:       "Exchange Management",
				Pattern:     `^"{resource}" declares exchange "([^"]*)" of type "([^"]*)"$`,
				Description: "Declares an exchange (direct, fanout, topic, headers)",
				Example:     `"{resource}" declares exchange "events" of type "topic"`,
				Handler:     r.declareExchange,
			},
			{
				Group:       "Exchange Management",
				Pattern:     `^"{resource}" declares durable exchange "([^"]*)" of type "([^"]*)"$`,
				Description: "Declares a durable exchange",
				Example:     `"{resource}" declares durable exchange "events" of type "topic"`,
				Handler:     r.declareDurableExchange,
			},
			{
				Group:       "Exchange Management",
				Pattern:     `^"{resource}" exchange "([^"]*)" exists$`,
				Description: "Asserts an exchange exists",
				Example:     `"{resource}" exchange "events" exists`,
				Handler:     r.exchangeExists,
			},

			// Bindings
			{
				Group:       "Bindings",
				Pattern:     `^"{resource}" binds queue "([^"]*)" to exchange "([^"]*)"$`,
				Description: "Binds a queue to an exchange with empty routing key",
				Example:     `"{resource}" binds queue "orders" to exchange "events"`,
				Handler:     r.bindQueue,
			},
			{
				Group:       "Bindings",
				Pattern:     `^"{resource}" binds queue "([^"]*)" to exchange "([^"]*)" with routing key "([^"]*)"$`,
				Description: "Binds a queue to an exchange with a routing key",
				Example:     `"{resource}" binds queue "orders" to exchange "events" with routing key "order.*"`,
				Handler:     r.bindQueueWithKey,
			},

			// Publishing - Queue
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes to queue "([^"]*)":$`,
				Description: "Publishes a message directly to a queue",
				Example:     "\"{resource}\" publishes to queue \"orders\":\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.publishToQueue,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes json to queue "([^"]*)":$`,
				Description: "Publishes a JSON message directly to a queue",
				Example:     "\"{resource}\" publishes json to queue \"orders\":\n  \"\"\"\n  {\"order_id\": 123}\n  \"\"\"",
				Handler:     r.publishJSONToQueue,
			},

			// Publishing - Exchange
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes to exchange "([^"]*)" with routing key "([^"]*)":$`,
				Description: "Publishes a message to an exchange with routing key",
				Example:     "\"{resource}\" publishes to exchange \"events\" with routing key \"order.created\":\n  \"\"\"\n  Order created\n  \"\"\"",
				Handler:     r.publishToExchange,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes json to exchange "([^"]*)" with routing key "([^"]*)":$`,
				Description: "Publishes a JSON message to an exchange with routing key",
				Example:     "\"{resource}\" publishes json to exchange \"events\" with routing key \"order.created\":\n  \"\"\"\n  {\"event\": \"order.created\"}\n  \"\"\"",
				Handler:     r.publishJSONToExchange,
			},

			// Publishing - Batch
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes messages to queue "([^"]*)":$`,
				Description: "Publishes multiple messages from a table",
				Example:     "\"{resource}\" publishes messages to queue \"orders\":\n  | routing_key | message |\n  | order.1     | msg1    |",
				Handler:     r.publishMessages,
			},

			// Consuming
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" consumes from queue "([^"]*)"$`,
				Description: "Starts consuming messages from a queue",
				Example:     `"{resource}" consumes from queue "orders"`,
				Handler:     r.startConsuming,
			},
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" receives from queue "([^"]*)" within "([^"]*)"$`,
				Description: "Waits for a message from a queue within timeout",
				Example:     `"{resource}" receives from queue "orders" within "5s"`,
				Handler:     r.receiveMessage,
			},
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" receives from queue "([^"]*)" within "([^"]*)":$`,
				Description: "Asserts a specific message is received within timeout",
				Example:     "\"{resource}\" receives from queue \"orders\" within \"5s\":\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.shouldReceiveMessage,
			},

			// Assertions
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" queue "([^"]*)" has "(\d+)" messages$`,
				Description: "Asserts queue has exactly N messages consumed",
				Example:     `"{resource}" queue "orders" has "3" messages`,
				Handler:     r.queueShouldHaveMessages,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" queue "([^"]*)" is empty$`,
				Description: "Asserts no messages have been consumed from queue",
				Example:     `"{resource}" queue "orders" is empty`,
				Handler:     r.queueShouldBeEmpty,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message contains:$`,
				Description: "Asserts the last consumed message contains content",
				Example:     "\"{resource}\" last message contains:\n  \"\"\"\n  order_id\n  \"\"\"",
				Handler:     r.lastMessageShouldContain,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message has routing key "([^"]*)"$`,
				Description: "Asserts the last consumed message has specific routing key",
				Example:     `"{resource}" last message has routing key "order.created"`,
				Handler:     r.lastMessageShouldHaveRoutingKey,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message has header "([^"]*)" with value "([^"]*)"$`,
				Description: "Asserts the last message has a header with value",
				Example:     `"{resource}" last message has header "content-type" with value "application/json"`,
				Handler:     r.lastMessageShouldHaveHeader,
			},
		},
	}
}

// Queue Management

func (r *RabbitMQ) declareQueue(name string) error {
	_, err := r.channel.QueueDeclare(name, false, false, false, false, nil)
	if err != nil {
		return err
	}
	r.trackQueue(name)
	return nil
}

func (r *RabbitMQ) declareDurableQueue(name string) error {
	_, err := r.channel.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		return err
	}
	r.trackQueue(name)
	return nil
}

func (r *RabbitMQ) trackQueue(name string) {
	for _, q := range r.declaredQueues {
		if q == name {
			return
		}
	}
	r.declaredQueues = append(r.declaredQueues, name)
}

func (r *RabbitMQ) queueExists(name string) error {
	_, err := r.channel.QueueInspect(name)
	if err != nil {
		return fmt.Errorf("queue %q does not exist: %w", name, err)
	}
	return nil
}

func (r *RabbitMQ) purgeQueue(name string) error {
	_, err := r.channel.QueuePurge(name, false)
	return err
}

// Exchange Management

func (r *RabbitMQ) declareExchange(name, exchangeType string) error {
	err := r.channel.ExchangeDeclare(name, exchangeType, false, false, false, false, nil)
	if err != nil {
		return err
	}
	r.trackExchange(name)
	return nil
}

func (r *RabbitMQ) declareDurableExchange(name, exchangeType string) error {
	err := r.channel.ExchangeDeclare(name, exchangeType, true, false, false, false, nil)
	if err != nil {
		return err
	}
	r.trackExchange(name)
	return nil
}

func (r *RabbitMQ) trackExchange(name string) {
	for _, e := range r.declaredExchanges {
		if e == name {
			return
		}
	}
	r.declaredExchanges = append(r.declaredExchanges, name)
}

func (r *RabbitMQ) exchangeExists(name string) error {
	// Try passive declare - fails if exchange doesn't exist
	err := r.channel.ExchangeDeclarePassive(name, "", false, false, false, false, nil)
	if err != nil {
		// Need to reopen channel after failed passive declare
		ch, chErr := r.conn.Channel()
		if chErr != nil {
			return fmt.Errorf("exchange %q does not exist (and failed to reopen channel): %w", name, err)
		}
		r.channel = ch
		return fmt.Errorf("exchange %q does not exist", name)
	}
	return nil
}

// Bindings

func (r *RabbitMQ) bindQueue(queue, exchange string) error {
	return r.channel.QueueBind(queue, "", exchange, false, nil)
}

func (r *RabbitMQ) bindQueueWithKey(queue, exchange, routingKey string) error {
	return r.channel.QueueBind(queue, routingKey, exchange, false, nil)
}

// Publishing

func (r *RabbitMQ) publishToQueue(queue string, doc *godog.DocString) error {
	return r.channel.PublishWithContext(
		context.Background(),
		"",    // default exchange
		queue, // routing key = queue name
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(doc.Content),
		},
	)
}

func (r *RabbitMQ) publishJSONToQueue(queue string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return r.channel.PublishWithContext(
		context.Background(),
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(doc.Content),
		},
	)
}

func (r *RabbitMQ) publishToExchange(exchange, routingKey string, doc *godog.DocString) error {
	return r.channel.PublishWithContext(
		context.Background(),
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(doc.Content),
		},
	)
}

func (r *RabbitMQ) publishJSONToExchange(exchange, routingKey string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return r.channel.PublishWithContext(
		context.Background(),
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(doc.Content),
		},
	)
}

func (r *RabbitMQ) publishMessages(queue string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}

	headers := table.Rows[0].Cells
	messageIdx := -1
	routingKeyIdx := -1

	for i, cell := range headers {
		switch strings.ToLower(cell.Value) {
		case "message", "value", "body", "payload":
			messageIdx = i
		case "routing_key", "routingkey", "key":
			routingKeyIdx = i
		}
	}

	if messageIdx == -1 {
		return fmt.Errorf("table must have a 'message' or 'value' column")
	}

	for _, row := range table.Rows[1:] {
		routingKey := queue
		if routingKeyIdx >= 0 && routingKeyIdx < len(row.Cells) {
			routingKey = row.Cells[routingKeyIdx].Value
		}

		err := r.channel.PublishWithContext(
			context.Background(),
			"",
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(row.Cells[messageIdx].Value),
			},
		)
		if err != nil {
			return fmt.Errorf("publishing message: %w", err)
		}
	}

	return nil
}

// Consuming

func (r *RabbitMQ) startConsuming(queue string) error {
	r.consumingMu.Lock()
	if r.consuming[queue] {
		r.consumingMu.Unlock()
		return nil
	}
	r.consuming[queue] = true
	stopCh := make(chan struct{})
	r.stopChannels[queue] = stopCh
	r.consumingMu.Unlock()

	msgs, err := r.channel.Consume(
		queue,
		"",    // consumer tag
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("starting consumer: %w", err)
	}

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				r.messagesMu.Lock()
				r.messages[queue] = append(r.messages[queue], &msg)
				r.lastMessage = &msg
				r.messagesMu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) receiveMessage(queue, timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if err := r.startConsuming(queue); err != nil {
		return err
	}

	deadline := time.Now().Add(duration)
	initialCount := r.getMessageCount(queue)

	for time.Now().Before(deadline) {
		if r.getMessageCount(queue) > initialCount {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("no message received within %s", timeout)
}

func (r *RabbitMQ) shouldReceiveMessage(queue, timeout string, doc *godog.DocString) error {
	if err := r.receiveMessage(queue, timeout); err != nil {
		return err
	}

	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(lastMsg.Body))

	if actual != expected {
		return fmt.Errorf("message mismatch:\nexpected: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *RabbitMQ) getMessageCount(queue string) int {
	r.messagesMu.RLock()
	defer r.messagesMu.RUnlock()
	return len(r.messages[queue])
}

// Assertions

func (r *RabbitMQ) queueShouldHaveMessages(queue string, expected int) error {
	count := r.getMessageCount(queue)
	if count != expected {
		return fmt.Errorf("queue %q: expected %d messages, got %d", queue, expected, count)
	}
	return nil
}

func (r *RabbitMQ) queueShouldBeEmpty(queue string) error {
	return r.queueShouldHaveMessages(queue, 0)
}

func (r *RabbitMQ) lastMessageShouldContain(doc *godog.DocString) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	expected := strings.TrimSpace(doc.Content)
	actual := string(lastMsg.Body)

	if !strings.Contains(actual, expected) {
		return fmt.Errorf("message does not contain expected content:\nexpected to contain: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *RabbitMQ) lastMessageShouldHaveRoutingKey(routingKey string) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	if lastMsg.RoutingKey != routingKey {
		return fmt.Errorf("expected routing key %q, got %q", routingKey, lastMsg.RoutingKey)
	}

	return nil
}

func (r *RabbitMQ) lastMessageShouldHaveHeader(headerKey, headerValue string) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	if lastMsg.Headers == nil {
		return fmt.Errorf("message has no headers")
	}

	val, ok := lastMsg.Headers[headerKey]
	if !ok {
		return fmt.Errorf("header %q not found", headerKey)
	}

	if fmt.Sprintf("%v", val) != headerValue {
		return fmt.Errorf("header %q: expected %q, got %q", headerKey, headerValue, val)
	}

	return nil
}

func (r *RabbitMQ) Cleanup(ctx context.Context) error {
	r.stopAllConsumers()

	var errs []error
	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

var _ Handler = (*RabbitMQ)(nil)
