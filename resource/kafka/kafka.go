package rabbitmq

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/stub"
)

const (
	DefaultWaitDuration = time.Millisecond * 50
	resourceName        = "kafka"
)

type Kafka struct {
	mtx             sync.Mutex
	datasource      string
	conn            *amqp.Connection
	consumedMessage map[string][][]byte
	waitDuration    time.Duration
	stubs           *stub.Stubs
}

// New creates and validates the resource params for the connection initialized
// in Open()
func New(cfg *config.Resource) (*Kafka, error) {
	params := cfg.Options
	datasource, ok := params["datasource"]
	if !ok {
		return nil, errors.Errorf("%s: datasource is required", resourceName)
	}

	waitDuration, err := time.ParseDuration(params["wait_duration"])
	if err != nil {
		waitDuration = DefaultWaitDuration
	}

	path, ok := cfg.Options["stubs_path"]
	stubs := &stub.Stubs{}
	if ok {
		var err error
		stubs, err = stub.RetrieveFiles(path)
		if err != nil {
			return nil, err
		}
	}

	return &Kafka{
		datasource:      datasource,
		waitDuration:    waitDuration,
		consumedMessage: make(map[string][][]byte),
		stubs:           stubs,
	}, nil
}

// Open attempts a dial connection to the AMQP server
func (c *Kafka) Open() error {
	var err error
	c.conn, err = amqp.Dial(c.datasource)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to connect on initial dial", resourceName)
	}

	return nil
}

// Ready attempts to publish a message to a test queue, if no error is returned the
// server is deemed ready
func (c *Kafka) Ready() error {
	if err := c.Publish("test:test", []byte("")); err != nil {
		return errors.Wrapf(err, "%s: ready check failed", resourceName)
	}
	return nil
}

// Reset empties the consumedMessage map and cleans up the queues
func (c *Kafka) Reset() error {

	failedTargets := make(map[string][]string)
	for target := range c.consumedMessage {
		exchange, key := c.target(target)
		ch, err := c.conn.Channel()
		if err != nil {
			msg := fmt.Sprintf("reset connection attempt failed: %s", err.Error())
			failedTargets[msg] = append(failedTargets[msg], target)
			continue
		}
		if _, err := ch.QueueDelete(c.queueName(exchange, key), false, false, false); err != nil {
			msg := fmt.Sprintf("queue deletion attempt failed: %s", err.Error())
			failedTargets[msg] = append(failedTargets[msg], target)
		}
	}
	c.consumedMessage = make(map[string][][]byte)

	if len(failedTargets) > 0 {
		var errMsg string
		for msg, targets := range failedTargets {
			errMsg += msg + strings.Join(targets, ",")
		}
		return errors.Errorf("%s: %s", resourceName, errMsg)
	}
	return nil
}

// Close cleans up the underlying connection
func (c *Kafka) Close() error {
	if err := c.conn.Close(); err != nil {
		return errors.Wrapf(err, "%s: failed to close connection", resourceName)
	}
	return nil
}

// Listen creates a consumer on the passed target, creates the queue, and puts the messages
// into consumedMessage map for later access
func (c *Kafka) Listen(target string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return errors.Wrapf(err, "%s: listen attempt failed", resourceName)
	}

	exchange, key := c.target(target)

	if err := exchangeDeclare(ch, exchange); err != nil {
		return errors.Wrapf(err, "%s: failed to declare exchange `%s`", resourceName, exchange)
	}

	q, err := queueDeclare(ch, c.queueName(exchange, key))
	if err != nil {
		return errors.Wrapf(err, "%s: failed to declare queue `%s`", resourceName, c.queueName(exchange, key))
	}

	if err := queueBind(ch, q.Name, exchange, key); err != nil {
		return errors.Wrapf(err, "%s: failed to bind to queue `%s`", resourceName, q.Name)
	}

	if _, err := ch.QueuePurge(q.Name, false); err != nil {
		return errors.Wrapf(err, "%s: failed to purge to queue `%s`", resourceName, q.Name)
	}
	c.consumedMessage[target] = make([][]byte, 0)

	msgs, err := consume(ch, q.Name)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to consume messages from queue `%s`", resourceName, q.Name)
	}

	go func(msgs <-chan amqp.Delivery, target string) {
		for d := range msgs {
			c.consume(target, d.Body)
		}
	}(msgs, target)

	return nil
}

// PublishFromFile attempts to publish a message read from a file to the passed target
func (c *Kafka) PublishFromFile(target string, fileName string) error {
	payload, err := c.stubs.Get(fileName)
	if err != nil {
		return err
	}
	return c.Publish(target, payload)
}

// Publish attempts to publish a message to the passed target
func (c *Kafka) Publish(target string, payload []byte) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return errors.Wrapf(err, "%s: failed to connect to target `%s`", resourceName, target)
	}
	defer ch.Close()

	exchange, key := c.target(target)
	if err := exchangeDeclare(ch, exchange); err != nil {
		return errors.Wrapf(err, "%s: failed to declare exchange `%s`", resourceName, exchange)
	}

	err = ch.Publish(
		exchange,
		key,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Transient,
			ContentType:  "application/json",
			Body:         payload,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to publish message to target `%s`", resourceName, target)
	}
	time.Sleep(c.waitDuration)
	return nil
}

// Fetch attempts to retrieve a message from the passed target
func (c *Kafka) Fetch(target string) ([][]byte, error) {
	time.Sleep(c.waitDuration)
	c.mtx.Lock()
	defer c.mtx.Unlock()

	msgs, ok := c.consumedMessage[target]
	if !ok {
		return nil, errors.Errorf("%s: queue does not exist, please listen to it `%s` %+v", resourceName, target, c.consumedMessage)
	}

	return msgs, nil
}

func (c *Kafka) target(target string) (string, string) {
	result := strings.Split(target, ":")
	if len(result) < 2 {
		return result[0], ""
	}
	return result[0], result[1]
}

func (c *Kafka) queueName(exchange, key string) string {
	return fmt.Sprintf("%s.%s.tmp.queue", exchange, key)
}

func (c *Kafka) clear(target string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.consumedMessage[target] = [][]byte{}
}

func (c *Kafka) consume(target string, payload []byte) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.consumedMessage[target] = append(c.consumedMessage[target], payload)
}

func exchangeDeclare(ch *amqp.Channel, name string) error {
	return ch.ExchangeDeclare(
		name,    // name of the exchange
		"topic", // type
		true,    // durable
		false,   // delete when complete
		false,   // internal
		false,   // noWait
		nil,     // arguments
	)
}

func queueDeclare(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,  // name, leave empty to generate a unique name
		true,  // durable
		false, // delete when usused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
}

func queueBind(ch *amqp.Channel, queue, exchange, key string) error {
	return ch.QueueBind(
		queue,    // name of the queue
		key,      // bindingKey
		exchange, // sourceExchange
		false,    // noWait
		nil,      // arguments
	)
}

func consume(ch *amqp.Channel, queue string) (<-chan amqp.Delivery, error) {
	return ch.Consume(
		queue,
		"",
		true,
		false,
		false,
		false,
		nil)
}
