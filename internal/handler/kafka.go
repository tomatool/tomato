package handler

import (
	"context"
	"encoding/json"
	"fmt"
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

	messages     map[string][]*sarama.ConsumerMessage
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
	_, err := r.admin.ListTopics()
	return err
}

func (r *Kafka) Reset(ctx context.Context) error {
	r.stopAllConsumers()

	r.messagesMu.Lock()
	r.messages = make(map[string][]*sarama.ConsumerMessage)
	r.lastMessage = nil
	r.messagesMu.Unlock()

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
	ctx.Step(fmt.Sprintf(`^kafka topic "([^"]*)" exists on "%s"$`, r.name), r.topicExists)
	ctx.Step(fmt.Sprintf(`^I create kafka topic "([^"]*)" on "%s"$`, r.name), r.createTopic)
	ctx.Step(fmt.Sprintf(`^I create kafka topic "([^"]*)" on "%s" with "(\d+)" partitions$`, r.name), r.createTopicWithPartitions)

	ctx.Step(fmt.Sprintf(`^I publish message to "%s" topic "([^"]*)":$`, r.name), r.publishMessage)
	ctx.Step(fmt.Sprintf(`^I publish message to "%s" topic "([^"]*)" with key "([^"]*)":$`, r.name), r.publishMessageWithKey)
	ctx.Step(fmt.Sprintf(`^I publish JSON to "%s" topic "([^"]*)":$`, r.name), r.publishJSON)
	ctx.Step(fmt.Sprintf(`^I publish JSON to "%s" topic "([^"]*)" with key "([^"]*)":$`, r.name), r.publishJSONWithKey)
	ctx.Step(fmt.Sprintf(`^I publish messages to "%s" topic "([^"]*)":$`, r.name), r.publishMessages)

	ctx.Step(fmt.Sprintf(`^I start consuming from "%s" topic "([^"]*)"$`, r.name), r.startConsuming)
	ctx.Step(fmt.Sprintf(`^I consume message from "%s" topic "([^"]*)" within "([^"]*)"$`, r.name), r.consumeMessage)
	ctx.Step(fmt.Sprintf(`^I should receive message from "%s" topic "([^"]*)" within "([^"]*)":$`, r.name), r.shouldReceiveMessage)
	ctx.Step(fmt.Sprintf(`^I should receive message from "%s" topic "([^"]*)" with key "([^"]*)" within "([^"]*)"$`, r.name), r.shouldReceiveMessageWithKey)

	ctx.Step(fmt.Sprintf(`^"%s" topic "([^"]*)" should have "(\d+)" messages$`, r.name), r.topicShouldHaveMessages)
	ctx.Step(fmt.Sprintf(`^"%s" topic "([^"]*)" should be empty$`, r.name), r.topicShouldBeEmpty)
	ctx.Step(fmt.Sprintf(`^the last message from "%s" should contain:$`, r.name), r.lastMessageShouldContain)
	ctx.Step(fmt.Sprintf(`^the last message from "%s" should have key "([^"]*)"$`, r.name), r.lastMessageShouldHaveKey)
	ctx.Step(fmt.Sprintf(`^the last message from "%s" should have header "([^"]*)" with value "([^"]*)"$`, r.name), r.lastMessageShouldHaveHeader)

	ctx.Step(fmt.Sprintf(`^I should receive messages from "%s" topic "([^"]*)" in order:$`, r.name), r.shouldReceiveMessagesInOrder)
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
	_, _, err := r.producer.SendMessage(msg)
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
	_, _, err := r.producer.SendMessage(msg)
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
		if _, _, err := r.producer.SendMessage(msg); err != nil {
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
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if err := r.startConsuming(topic); err != nil {
		return err
	}

	deadline := time.Now().Add(duration)
	initialCount := r.getMessageCount(topic)

	for time.Now().Before(deadline) {
		if r.getMessageCount(topic) > initialCount {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("no message received within %s", timeout)
}

func (r *Kafka) shouldReceiveMessage(topic, timeout string, doc *godog.DocString) error {
	if err := r.consumeMessage(topic, timeout); err != nil {
		return err
	}

	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(lastMsg.Value))

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

	_, _, err := r.producer.SendMessage(msg)
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
