package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type Kafka struct {
	name      string
	config    config.Resource
	container *container.Manager

	admin    sarama.ClusterAdmin
	producer sarama.SyncProducer
	consumer sarama.Consumer
	registry *schemaRegistry // nil unless options.schema_registry is set

	pendingHeaders []sarama.RecordHeader // applied to the next published message

	messages     map[string][]*sarama.ConsumerMessage
	claimed      map[string]int // topic -> messages already matched by a receive step
	messagesMu   sync.RWMutex
	lastMessage  *sarama.ConsumerMessage
	consuming    map[string]bool
	consumingMu  sync.RWMutex
	stopChannels map[string]chan struct{}
}

func NewKafka(name string, cfg config.Resource, cm *container.Manager) (*Kafka, error) {
	return &Kafka{
		name:         name,
		config:       cfg,
		container:    cm,
		messages:     make(map[string][]*sarama.ConsumerMessage),
		consuming:    make(map[string]bool),
		stopChannels: make(map[string]chan struct{}),
	}, nil
}

func (r *Kafka) Name() string { return r.name }

func (r *Kafka) Init(ctx context.Context) error {
	brokers, err := r.getBrokers(ctx)
	if err != nil {
		return fmt.Errorf("getting brokers: %w", err)
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_0_0_0
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Consumer.Return.Errors = true
	cfg.Admin.Timeout = 30 * time.Second

	admin, err := sarama.NewClusterAdmin(brokers, cfg)
	if err != nil {
		return fmt.Errorf("creating admin client: %w", err)
	}
	r.admin = admin

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return fmt.Errorf("creating producer: %w", err)
	}
	r.producer = producer

	consumer, err := sarama.NewConsumer(brokers, cfg)
	if err != nil {
		return fmt.Errorf("creating consumer: %w", err)
	}
	r.consumer = consumer

	registryURL, err := r.schemaRegistryURL(ctx)
	if err != nil {
		return err
	}
	if registryURL != "" {
		r.registry = newSchemaRegistry(registryURL)
	}

	return nil
}

// schemaRegistryURL resolves options.schema_registry to a base URL: either
// `url` as given, or the host and mapped port of `container` (port 8081
// unless `port` says otherwise). It returns "" when no registry is configured.
func (r *Kafka) schemaRegistryURL(ctx context.Context) (string, error) {
	raw, ok := r.config.Options["schema_registry"]
	if !ok || raw == nil {
		return "", nil
	}
	opts, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("options.schema_registry must be a map with `url` or `container`")
	}
	if u, ok := opts["url"].(string); ok && u != "" {
		return u, nil
	}
	name, _ := opts["container"].(string)
	if name == "" {
		return "", fmt.Errorf("options.schema_registry needs `url` or `container`")
	}
	port := "8081"
	if p, ok := opts["port"]; ok {
		port = fmt.Sprintf("%v", p)
	}
	host, err := r.container.GetHost(ctx, name)
	if err != nil {
		return "", fmt.Errorf("getting schema registry host: %w", err)
	}
	mapped, err := r.container.GetPort(ctx, name, port+"/tcp")
	if err != nil {
		return "", fmt.Errorf("getting schema registry port: %w", err)
	}
	return fmt.Sprintf("http://%s:%s", host, mapped), nil
}

// valueSubject is the Schema Registry subject for a topic's values, following
// the default TopicNameStrategy (`<topic>-value`) unless
// options.schema_registry.subjects maps the topic to another subject.
func (r *Kafka) valueSubject(topic string) string {
	if opts, ok := r.config.Options["schema_registry"].(map[string]any); ok {
		if subjects, ok := opts["subjects"].(map[string]any); ok {
			if s, ok := subjects[topic].(string); ok && s != "" {
				return s
			}
		}
	}
	return topic + "-value"
}

func (r *Kafka) requireRegistry() error {
	if r.registry == nil {
		return fmt.Errorf("resource %q has no schema registry; set options.schema_registry.url or .container", r.name)
	}
	return nil
}

func (r *Kafka) getBrokers(ctx context.Context) ([]string, error) {
	if len(r.config.Brokers) > 0 {
		return r.config.Brokers, nil
	}

	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("getting container host: %w", err)
	}
	port, err := r.container.GetPort(ctx, r.config.Container, "9092/tcp")
	if err != nil {
		return nil, fmt.Errorf("getting container port: %w", err)
	}

	return []string{fmt.Sprintf("%s:%s", host, port)}, nil
}

func (r *Kafka) Ready(ctx context.Context) error {
	if _, err := r.admin.ListTopics(); err != nil {
		return err
	}
	if r.registry != nil {
		if err := r.registry.ready(ctx); err != nil {
			return fmt.Errorf("schema registry not ready: %w", err)
		}
	}
	return nil
}

func (r *Kafka) Reset(ctx context.Context) error {
	r.stopAllConsumers()

	r.messagesMu.Lock()
	r.messages = make(map[string][]*sarama.ConsumerMessage)
	r.claimed = make(map[string]int)
	r.lastMessage = nil
	r.messagesMu.Unlock()

	if r.registry != nil {
		r.registry.reset()
	}
	r.pendingHeaders = nil

	topics := r.getTopicsToReset()
	if len(topics) == 0 {
		return nil
	}

	strategy := "delete_recreate"
	if s, ok := r.config.Options["reset_strategy"].(string); ok {
		strategy = s
	}

	switch strategy {
	case "delete_recreate":
		return r.deleteAndRecreatTopics(topics)
	case "none":
		return nil
	default:
		return r.deleteAndRecreatTopics(topics)
	}
}

func (r *Kafka) getTopicsToReset() []string {
	if topics, ok := r.config.Options["topics"].([]interface{}); ok {
		result := make([]string, len(topics))
		for i, t := range topics {
			result[i] = fmt.Sprintf("%v", t)
		}
		return result
	}
	return nil
}

func (r *Kafka) deleteAndRecreatTopics(topics []string) error {
	partitions := 1
	replicationFactor := 1
	if p, ok := r.config.Options["partitions"].(int); ok {
		partitions = p
	}
	if rf, ok := r.config.Options["replication_factor"].(int); ok {
		replicationFactor = rf
	}

	for _, topic := range topics {
		if err := r.admin.DeleteTopic(topic); err != nil {
			if !strings.Contains(err.Error(), "Unknown Topic") && !strings.Contains(err.Error(), "does not exist") {
				log.Warn().Err(err).Str("topic", topic).Msg("error deleting topic (may not exist)")
			}
		}
	}

	time.Sleep(500 * time.Millisecond)

	for _, topic := range topics {
		detail := &sarama.TopicDetail{
			NumPartitions:     int32(partitions),
			ReplicationFactor: int16(replicationFactor),
		}
		if err := r.admin.CreateTopic(topic, detail, false); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("creating topic %s: %w", topic, err)
			}
		}
	}

	return nil
}

func (r *Kafka) stopAllConsumers() {
	r.consumingMu.Lock()
	defer r.consumingMu.Unlock()

	for topic, stopCh := range r.stopChannels {
		close(stopCh)
		delete(r.stopChannels, topic)
		r.consuming[topic] = false
	}
}

func (r *Kafka) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the Kafka handler
func (r *Kafka) Steps() StepCategory {
	return StepCategory{
		Name:        "Kafka",
		Description: "Steps for interacting with Apache Kafka message broker",
		Steps: []StepDef{
			// Topic Management
			{
				Group:       "Topic Management",
				Pattern:     `^"{resource}" topic "([^"]*)" exists$`,
				Description: "Asserts a Kafka topic exists",
				Example:     `"{resource}" topic "events" exists`,
				Handler:     r.topicExists,
			},
			{
				Group:       "Topic Management",
				Pattern:     `^"{resource}" creates topic "([^"]*)"$`,
				Description: "Creates a Kafka topic with 1 partition",
				Example:     `"{resource}" creates topic "events"`,
				Handler:     r.createTopic,
			},
			{
				Group:       "Topic Management",
				Pattern:     `^"{resource}" creates topic "([^"]*)" with "(\d+)" partitions$`,
				Description: "Creates a Kafka topic with specified partitions",
				Example:     `"{resource}" creates topic "events" with "3" partitions`,
				Handler:     r.createTopicWithPartitions,
			},

			// Publishing
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" message header "([^"]*)" is "([^"]*)"$`,
				Description: "Sets a header on the next message published (any publish step)",
				Example:     `"{resource}" message header "trace-id" is "abc-123"`,
				Handler:     r.setMessageHeader,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes to "([^"]*)":$`,
				Description: "Publishes a message to a topic",
				Example:     "\"{resource}\" publishes to \"events\":\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.publishMessage,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes to "([^"]*)" with key "([^"]*)":$`,
				Description: "Publishes a message with a key to a topic",
				Example:     "\"{resource}\" publishes to \"events\" with key \"user-123\":\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.publishMessageWithKey,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes json to "([^"]*)":$`,
				Description: "Publishes a JSON message to a topic",
				Example:     "\"{resource}\" publishes json to \"events\":\n  \"\"\"\n  {\"type\": \"user_created\"}\n  \"\"\"",
				Handler:     r.publishJSON,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes json to "([^"]*)" with key "([^"]*)":$`,
				Description: "Publishes a JSON message with a key",
				Example:     "\"{resource}\" publishes json to \"events\" with key \"user-123\":\n  \"\"\"\n  {\"type\": \"user_created\"}\n  \"\"\"",
				Handler:     r.publishJSONWithKey,
			},
			{
				Group:       "Publishing",
				Pattern:     `^"{resource}" publishes messages to "([^"]*)":$`,
				Description: "Publishes multiple messages from a table",
				Example:     "\"{resource}\" publishes messages to \"events\":\n  | key      | value           |\n  | user-1   | {\"id\": 1}     |",
				Handler:     r.publishMessages,
			},

			// Avro / Schema Registry
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" registers schema for subject "([^"]*)":$`,
				Description: "Registers an Avro schema under a subject",
				Example:     "\"{resource}\" registers schema for subject \"orders-value\":\n  \"\"\"\n  {\"type\": \"record\", \"name\": \"Order\", \"fields\": [{\"name\": \"id\", \"type\": \"string\"}]}\n  \"\"\"",
				Handler:     r.registerSchema,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" registers schema for subject "([^"]*)" from file "([^"]*)"$`,
				Description: "Registers an Avro schema (.avsc) from a file",
				Example:     `"{resource}" registers schema for subject "orders-value" from file "schemas/order.avsc"`,
				Handler:     r.registerSchemaFromFile,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" publishes avro to "([^"]*)":$`,
				Description: "Publishes JSON as Avro, using the latest schema of the topic's value subject",
				Example:     "\"{resource}\" publishes avro to \"orders\":\n  \"\"\"\n  {\"id\": \"order-1\"}\n  \"\"\"",
				Handler:     r.publishAvro,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" publishes avro to "([^"]*)" with key "([^"]*)":$`,
				Description: "Publishes JSON as Avro with a string key",
				Example:     "\"{resource}\" publishes avro to \"orders\" with key \"order-1\":\n  \"\"\"\n  {\"id\": \"order-1\"}\n  \"\"\"",
				Handler:     r.publishAvroWithKey,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" receives avro from "([^"]*)" within "([^"]*)":$`,
				Description: "Waits for an Avro message whose JSON form contains the given fields",
				Example:     "\"{resource}\" receives avro from \"orders\" within \"10s\":\n  \"\"\"\n  {\"id\": \"order-1\"}\n  \"\"\"",
				Handler:     r.shouldReceiveAvro,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" last message avro matches:$`,
				Description: "Asserts the last message, decoded from Avro, equals the JSON exactly",
				Example:     "\"{resource}\" last message avro matches:\n  \"\"\"\n  {\"id\": \"order-1\", \"note\": null}\n  \"\"\"",
				Handler:     r.lastMessageAvroShouldMatch,
			},
			{
				Group:       "Avro and Schema Registry",
				Pattern:     `^"{resource}" last message avro contains:$`,
				Description: "Asserts the last message, decoded from Avro, contains the JSON fields",
				Example:     "\"{resource}\" last message avro contains:\n  \"\"\"\n  {\"id\": \"order-1\"}\n  \"\"\"",
				Handler:     r.lastMessageAvroShouldContain,
			},

			// Consuming
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" consumes from "([^"]*)"$`,
				Description: "Starts consuming messages from a topic",
				Example:     `"{resource}" consumes from "events"`,
				Handler:     r.startConsuming,
			},
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" receives from "([^"]*)" within "([^"]*)"$`,
				Description: "Waits for a message from a topic within timeout",
				Example:     `"{resource}" receives from "events" within "5s"`,
				Handler:     r.consumeMessage,
			},
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" receives from "([^"]*)" within "([^"]*)":$`,
				Description: "Asserts a specific message is received within timeout",
				Example:     "\"{resource}\" receives from \"events\" within \"5s\":\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.shouldReceiveMessage,
			},
			{
				Group:       "Consuming",
				Pattern:     `^"{resource}" receives from "([^"]*)" with key "([^"]*)" within "([^"]*)"$`,
				Description: "Asserts a message with specific key is received",
				Example:     `"{resource}" receives from "events" with key "user-123" within "5s"`,
				Handler:     r.shouldReceiveMessageWithKey,
			},

			// Assertions
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" topic "([^"]*)" has "(\d+)" messages$`,
				Description: "Asserts topic has exactly N messages consumed",
				Example:     `"{resource}" topic "events" has "3" messages`,
				Handler:     r.topicShouldHaveMessages,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" topic "([^"]*)" is empty$`,
				Description: "Asserts no messages have been consumed from topic",
				Example:     `"{resource}" topic "events" is empty`,
				Handler:     r.topicShouldBeEmpty,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message contains:$`,
				Description: "Asserts the last consumed message contains content",
				Example:     "\"{resource}\" last message contains:\n  \"\"\"\n  user_created\n  \"\"\"",
				Handler:     r.lastMessageShouldContain,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message has key "([^"]*)"$`,
				Description: "Asserts the last consumed message has specific key",
				Example:     `"{resource}" last message has key "user-123"`,
				Handler:     r.lastMessageShouldHaveKey,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message has header "([^"]*)" with value "([^"]*)"$`,
				Description: "Asserts the last message has a header with value",
				Example:     `"{resource}" last message has header "content-type" with value "application/json"`,
				Handler:     r.lastMessageShouldHaveHeader,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" receives messages from "([^"]*)" in order:$`,
				Description: "Asserts messages are received in specified order",
				Example:     "\"{resource}\" receives messages from \"events\" in order:\n  | key    | value  |\n  | key1   | msg1   |",
				Handler:     r.shouldReceiveMessagesInOrder,
			},
		},
	}
}

func (r *Kafka) topicExists(topic string) error {
	topics, err := r.admin.ListTopics()
	if err != nil {
		return err
	}
	if _, exists := topics[topic]; !exists {
		return fmt.Errorf("topic %q does not exist", topic)
	}
	return nil
}

func (r *Kafka) createTopic(topic string) error {
	return r.createTopicWithPartitions(topic, 1)
}

func (r *Kafka) createTopicWithPartitions(topic string, partitions int) error {
	detail := &sarama.TopicDetail{
		NumPartitions:     int32(partitions),
		ReplicationFactor: 1,
	}
	err := r.admin.CreateTopic(topic, detail, false)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

func (r *Kafka) publishMessage(topic string, doc *godog.DocString) error {
	return r.publishMessageWithKey(topic, "", doc)
}

func (r *Kafka) publishMessageWithKey(topic, key string, doc *godog.DocString) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(doc.Content),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}
	_, _, err := r.send(msg)
	return err
}

func (r *Kafka) publishJSON(topic string, doc *godog.DocString) error {
	return r.publishJSONWithKey(topic, "", doc)
}

func (r *Kafka) publishJSONWithKey(topic, key string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(doc.Content),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}
	_, _, err := r.send(msg)
	return err
}

func (r *Kafka) publishMessages(topic string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}

	headers := table.Rows[0].Cells
	keyIdx := -1
	valueIdx := -1
	for i, cell := range headers {
		switch strings.ToLower(cell.Value) {
		case "key":
			keyIdx = i
		case "value", "message", "payload":
			valueIdx = i
		}
	}

	if valueIdx == -1 {
		return fmt.Errorf("table must have a 'value' or 'message' column")
	}

	for _, row := range table.Rows[1:] {
		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(row.Cells[valueIdx].Value),
		}
		if keyIdx >= 0 && keyIdx < len(row.Cells) {
			msg.Key = sarama.StringEncoder(row.Cells[keyIdx].Value)
		}
		if _, _, err := r.send(msg); err != nil {
			return fmt.Errorf("sending message: %w", err)
		}
	}

	return nil
}

func (r *Kafka) startConsuming(topic string) error {
	r.consumingMu.Lock()
	if r.consuming[topic] {
		r.consumingMu.Unlock()
		return nil
	}
	r.consuming[topic] = true
	stopCh := make(chan struct{})
	r.stopChannels[topic] = stopCh
	r.consumingMu.Unlock()

	partitions, err := r.consumer.Partitions(topic)
	if err != nil {
		return fmt.Errorf("getting partitions: %w", err)
	}

	for _, partition := range partitions {
		pc, err := r.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return fmt.Errorf("consuming partition %d: %w", partition, err)
		}

		go func(pc sarama.PartitionConsumer) {
			defer pc.Close()
			for {
				select {
				case msg := <-pc.Messages():
					r.messagesMu.Lock()
					r.messages[topic] = append(r.messages[topic], msg)
					r.lastMessage = msg
					r.messagesMu.Unlock()
				case <-stopCh:
					return
				}
			}
		}(pc)
	}

	return nil
}

func (r *Kafka) consumeMessage(topic, timeout string) error {
	_, err := r.nextMessage(topic, timeout)
	return err
}

// nextMessage waits for the next message on topic that no earlier receive
// step has matched, and makes it the last message. Waiting for "a message
// newer than when this step started" missed messages that arrived before the
// step ran, which made receive steps flaky.
func (r *Kafka) nextMessage(topic, timeout string) (*sarama.ConsumerMessage, error) {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}
	if err := r.startConsuming(topic); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(duration)
	for {
		r.messagesMu.Lock()
		if r.claimed == nil {
			r.claimed = make(map[string]int)
		}
		if next := r.claimed[topic]; next < len(r.messages[topic]) {
			msg := r.messages[topic][next]
			r.claimed[topic] = next + 1
			r.lastMessage = msg
			r.messagesMu.Unlock()
			return msg, nil
		}
		r.messagesMu.Unlock()
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("no message received within %s", timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *Kafka) shouldReceiveMessage(topic, timeout string, doc *godog.DocString) error {
	msg, err := r.nextMessage(topic, timeout)
	if err != nil {
		return err
	}

	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(msg.Value))

	if actual != expected {
		return fmt.Errorf("message mismatch:\nexpected: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *Kafka) shouldReceiveMessageWithKey(topic, key, timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if err := r.startConsuming(topic); err != nil {
		return err
	}

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		r.messagesMu.RLock()
		for _, msg := range r.messages[topic] {
			if string(msg.Key) == key {
				r.lastMessage = msg
				r.messagesMu.RUnlock()
				return nil
			}
		}
		r.messagesMu.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("no message with key %q received within %s", key, timeout)
}

func (r *Kafka) getMessageCount(topic string) int {
	r.messagesMu.RLock()
	defer r.messagesMu.RUnlock()
	return len(r.messages[topic])
}

func (r *Kafka) topicShouldHaveMessages(topic string, expected int) error {
	count := r.getMessageCount(topic)
	if count != expected {
		return fmt.Errorf("topic %q: expected %d messages, got %d", topic, expected, count)
	}
	return nil
}

func (r *Kafka) topicShouldBeEmpty(topic string) error {
	return r.topicShouldHaveMessages(topic, 0)
}

func (r *Kafka) lastMessageShouldContain(doc *godog.DocString) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	expected := strings.TrimSpace(doc.Content)
	actual := string(lastMsg.Value)

	if !strings.Contains(actual, expected) {
		return fmt.Errorf("message does not contain expected content:\nexpected to contain: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *Kafka) lastMessageShouldHaveKey(key string) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	if string(lastMsg.Key) != key {
		return fmt.Errorf("expected key %q, got %q", key, string(lastMsg.Key))
	}

	return nil
}

func (r *Kafka) lastMessageShouldHaveHeader(headerKey, headerValue string) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	for _, h := range lastMsg.Headers {
		if string(h.Key) == headerKey {
			if string(h.Value) == headerValue {
				return nil
			}
			return fmt.Errorf("header %q: expected %q, got %q", headerKey, headerValue, string(h.Value))
		}
	}

	return fmt.Errorf("header %q not found", headerKey)
}

func (r *Kafka) shouldReceiveMessagesInOrder(topic string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}

	headers := table.Rows[0].Cells
	keyIdx := -1
	valueIdx := -1
	for i, cell := range headers {
		switch strings.ToLower(cell.Value) {
		case "key":
			keyIdx = i
		case "value", "message", "payload":
			valueIdx = i
		}
	}

	r.messagesMu.RLock()
	messages := r.messages[topic]
	r.messagesMu.RUnlock()

	expectedRows := table.Rows[1:]
	if len(messages) < len(expectedRows) {
		return fmt.Errorf("expected at least %d messages, got %d", len(expectedRows), len(messages))
	}

	for i, row := range expectedRows {
		msg := messages[i]

		if keyIdx >= 0 && keyIdx < len(row.Cells) {
			expectedKey := row.Cells[keyIdx].Value
			if string(msg.Key) != expectedKey {
				return fmt.Errorf("message %d: expected key %q, got %q", i+1, expectedKey, string(msg.Key))
			}
		}

		if valueIdx >= 0 && valueIdx < len(row.Cells) {
			expectedValue := row.Cells[valueIdx].Value
			if !strings.Contains(string(msg.Value), expectedValue) {
				return fmt.Errorf("message %d: expected value containing %q, got %q", i+1, expectedValue, string(msg.Value))
			}
		}
	}

	return nil
}

func (r *Kafka) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}

	for k, v := range headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	_, _, err := r.send(msg)
	return err
}

func (r *Kafka) Cleanup(ctx context.Context) error {
	r.stopAllConsumers()

	var errs []error
	if r.producer != nil {
		if err := r.producer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.consumer != nil {
		if err := r.consumer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.admin != nil {
		if err := r.admin.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

var _ Handler = (*Kafka)(nil)
var _ MessagePublisher = (*Kafka)(nil)

// --- Avro / Schema Registry ---

func (r *Kafka) registerSchema(subject string, doc *godog.DocString) error {
	if err := r.requireRegistry(); err != nil {
		return err
	}
	_, err := r.registry.register(context.Background(), subject, doc.Content)
	return err
}

func (r *Kafka) registerSchemaFromFile(subject, path string) error {
	if err := r.requireRegistry(); err != nil {
		return err
	}
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading schema file: %w", err)
	}
	_, err = r.registry.register(context.Background(), subject, string(schema))
	return err
}

func (r *Kafka) publishAvro(topic string, doc *godog.DocString) error {
	return r.publishAvroWithKey(topic, "", doc)
}

func (r *Kafka) publishAvroWithKey(topic, key string, doc *godog.DocString) error {
	if err := r.requireRegistry(); err != nil {
		return err
	}
	value, err := r.registry.encode(context.Background(), r.valueSubject(topic), ReplaceVariables(doc.Content))
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}
	_, _, err = r.send(msg)
	return err
}

func (r *Kafka) shouldReceiveAvro(topic, timeout string, doc *godog.DocString) error {
	if err := r.requireRegistry(); err != nil {
		return err
	}
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	var expected any
	if err := json.Unmarshal([]byte(ReplaceVariables(doc.Content)), &expected); err != nil {
		return fmt.Errorf("expected JSON is invalid: %w", err)
	}
	if err := r.startConsuming(topic); err != nil {
		return err
	}

	ctx := context.Background()
	deadline := time.Now().Add(duration)
	var lastMismatch error
	for {
		r.messagesMu.RLock()
		msgs := append([]*sarama.ConsumerMessage(nil), r.messages[topic]...)
		r.messagesMu.RUnlock()

		for _, msg := range msgs {
			actual, err := r.decodeAvroJSON(ctx, msg)
			if err != nil {
				lastMismatch = err
				continue
			}
			if err := CompareJSON(expected, actual, "", true); err != nil {
				lastMismatch = err
				continue
			}
			r.messagesMu.Lock()
			r.lastMessage = msg
			r.messagesMu.Unlock()
			return nil
		}

		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastMismatch != nil {
		return fmt.Errorf("no matching Avro message on %q within %s; last mismatch: %w", topic, timeout, lastMismatch)
	}
	return fmt.Errorf("no message received on %q within %s", topic, timeout)
}

func (r *Kafka) lastMessageAvroShouldMatch(doc *godog.DocString) error {
	return r.compareLastAvro(doc, false)
}

func (r *Kafka) lastMessageAvroShouldContain(doc *godog.DocString) error {
	return r.compareLastAvro(doc, true)
}

func (r *Kafka) compareLastAvro(doc *godog.DocString, partial bool) error {
	if err := r.requireRegistry(); err != nil {
		return err
	}
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()
	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	var expected any
	if err := json.Unmarshal([]byte(ReplaceVariables(doc.Content)), &expected); err != nil {
		return fmt.Errorf("expected JSON is invalid: %w", err)
	}
	actual, err := r.decodeAvroJSON(context.Background(), lastMsg)
	if err != nil {
		return err
	}
	return CompareJSON(expected, actual, "", partial)
}

// decodeAvroJSON decodes a wire-format Avro message value into generic JSON.
func (r *Kafka) decodeAvroJSON(ctx context.Context, msg *sarama.ConsumerMessage) (any, error) {
	text, err := r.registry.decode(ctx, msg.Value)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, fmt.Errorf("decoded Avro is not valid JSON: %w", err)
	}
	return v, nil
}

func (r *Kafka) setMessageHeader(key, value string) error {
	r.pendingHeaders = append(r.pendingHeaders, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(ReplaceVariables(value)),
	})
	return nil
}

// send publishes msg with any headers set by "message header ... is ...",
// then clears them so they apply to exactly one publish step.
func (r *Kafka) send(msg *sarama.ProducerMessage) (int32, int64, error) {
	if len(r.pendingHeaders) > 0 {
		msg.Headers = append(msg.Headers, r.pendingHeaders...)
	}
	partition, offset, err := r.producer.SendMessage(msg)
	if err == nil {
		r.pendingHeaders = nil
	}
	return partition, offset, err
}
