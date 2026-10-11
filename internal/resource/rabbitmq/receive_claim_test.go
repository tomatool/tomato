package rabbitmq

import (
	"testing"
	"time"

	"github.com/cucumber/godog"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tomatool/tomato/internal/resource"
)

// A message that arrived before the receive step ran must still satisfy it.
// That ordering is exactly a fanout (subscriber-2's message lands while the
// step for subscriber-1 is waiting) and was the source of flaky receives.

func TestRabbitMQReceiveClaimsMessagesThatArrivedEarlier(t *testing.T) {
	r, _ := New("mq", resource.DummyConfig(), nil)
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
	r, _ := New("mq", resource.DummyConfig(), nil)
	r.consuming["a"], r.consuming["b"] = true, true
	r.messages["a"] = []*amqp.Delivery{{Body: []byte("for a")}}
	r.messages["b"] = []*amqp.Delivery{{Body: []byte("for b")}}
	r.lastMessage = r.messages["b"][0] // the global last message is b's

	if err := r.shouldReceiveMessage("a", "200ms", &godog.DocString{Content: "for a"}); err != nil {
		t.Errorf("queue a's own message should be compared, got %v", err)
	}
}

// "has N messages" must tolerate a delivery still in flight: the step
// before it may only have waited for the first of several messages.

func TestRabbitMQMessageCountWaitsForInFlightDelivery(t *testing.T) {
	r, _ := New("mq", resource.DummyConfig(), nil)
	r.messages["q"] = []*amqp.Delivery{{Body: []byte("message 1")}}
	go func() {
		time.Sleep(100 * time.Millisecond)
		r.messagesMu.Lock()
		r.messages["q"] = append(r.messages["q"], &amqp.Delivery{Body: []byte("message 2")})
		r.messagesMu.Unlock()
	}()
	if err := r.queueShouldHaveMessages("q", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.queueShouldHaveMessages("q", 3); err == nil {
		t.Error("a count that never arrives should still fail")
	}
}
