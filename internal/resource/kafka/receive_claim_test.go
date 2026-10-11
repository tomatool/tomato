package kafka

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/resource"
)

// A message that arrived before the receive step ran must still satisfy it.
// That ordering is exactly a fanout (subscriber-2's message lands while the
// step for subscriber-1 is waiting) and was the source of flaky receives.

func TestKafkaReceiveClaimsMessagesThatArrivedEarlier(t *testing.T) {
	k, _ := New("events", resource.DummyConfig(), nil)
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

// "has N messages" must tolerate a delivery still in flight: the step
// before it may only have waited for the first of several messages.

func TestKafkaMessageCountWaitsForInFlightMessage(t *testing.T) {
	r, _ := New("events", resource.DummyConfig(), nil)
	r.messages["t"] = []*sarama.ConsumerMessage{{Value: []byte("message 1")}}
	go func() {
		time.Sleep(100 * time.Millisecond)
		r.messagesMu.Lock()
		r.messages["t"] = append(r.messages["t"], &sarama.ConsumerMessage{Value: []byte("message 2")})
		r.messagesMu.Unlock()
	}()
	if err := r.topicShouldHaveMessages("t", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.topicShouldHaveMessages("t", 3); err == nil {
		t.Error("a count that never arrives should still fail")
	}
}
