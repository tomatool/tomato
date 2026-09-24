package handler

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cucumber/godog"
	amqp "github.com/rabbitmq/amqp091-go"
)

// A message that arrived before the receive step ran must still satisfy it.
// That ordering is exactly a fanout (subscriber-2's message lands while the
// step for subscriber-1 is waiting) and was the source of flaky receives.

func TestRabbitMQReceiveClaimsMessagesThatArrivedEarlier(t *testing.T) {
	r, _ := NewRabbitMQ("mq", DummyConfig(), nil)
	r.consuming["q1"], r.consuming["q2"] = true, true
	r.messages["q1"] = []*amqp.Delivery{{Body: []byte("broadcast")}}
	r.messages["q2"] = []*amqp.Delivery{{Body: []byte("broadcast")}}

	for _, q := range []string{"q1", "q2"} {
		if err := r.shouldReceiveMessage(q, "200ms", &godog.DocString{Content: "broadcast"}); err != nil {
			t.Fatalf("receive on %s: %v", q, err)
		}
	}
	// Each message is matched once: a second receive waits for a new one.
	start := time.Now()
	if err := r.receiveMessage("q1", "150ms"); err == nil {
		t.Error("a second receive on q1 should wait for a new message and time out")
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Error("receive returned before its timeout")
	}
}

func TestRabbitMQReceiveComparesThatQueuesMessage(t *testing.T) {
	r, _ := NewRabbitMQ("mq", DummyConfig(), nil)
	r.consuming["a"], r.consuming["b"] = true, true
	r.messages["a"] = []*amqp.Delivery{{Body: []byte("for a")}}
	r.messages["b"] = []*amqp.Delivery{{Body: []byte("for b")}}
	r.lastMessage = r.messages["b"][0] // the global last message is b's

	if err := r.shouldReceiveMessage("a", "200ms", &godog.DocString{Content: "for a"}); err != nil {
		t.Errorf("queue a's own message should be compared, got %v", err)
	}
}

func TestKafkaReceiveClaimsMessagesThatArrivedEarlier(t *testing.T) {
	k, _ := NewKafka("events", DummyConfig(), nil)
	k.consuming["t"] = true
	k.messages["t"] = []*sarama.ConsumerMessage{{Value: []byte("one")}, {Value: []byte("two")}}

	for _, want := range []string{"one", "two"} {
		if err := k.shouldReceiveMessage("t", "200ms", &godog.DocString{Content: want}); err != nil {
			t.Fatalf("receive %q: %v", want, err)
		}
	}
	if err := k.consumeMessage("t", "100ms"); err == nil {
		t.Error("with both messages matched, a third receive should time out")
	}
}
