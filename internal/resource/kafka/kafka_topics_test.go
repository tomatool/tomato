package kafka

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// fakeTopicBroker answers like a broker that has just accepted a topic: it
// does not know the topic for the first unknownFor metadata refreshes, then
// says "not the leader" for the first notLeaderFor offset lookups.
type fakeTopicBroker struct {
	partitions   []int32
	unknownFor   int
	notLeaderFor int
	refreshes    int
	lookups      int
}

func (b *fakeTopicBroker) RefreshMetadata(...string) error {
	b.refreshes++
	return nil
}

func (b *fakeTopicBroker) Partitions(string) ([]int32, error) {
	if b.refreshes <= b.unknownFor {
		return nil, sarama.ErrUnknownTopicOrPartition
	}
	return b.partitions, nil
}

func (b *fakeTopicBroker) GetOffset(string, int32, int64) (int64, error) {
	b.lookups++
	if b.lookups <= b.notLeaderFor {
		return -1, sarama.ErrNotLeaderForPartition
	}
	return 0, nil
}

func fastTopicPolls(t *testing.T) {
	t.Helper()
	orig := topicPollInterval
	topicPollInterval = time.Millisecond
	t.Cleanup(func() { topicPollInterval = orig })
}

// The failure main's CI hit: the first offset lookups on a new topic's
// partition are refused. tomato used to start consuming right then.
func TestWaitTopicReady_WaitsOutNotLeaderForPartition(t *testing.T) {
	fastTopicPolls(t)
	b := &fakeTopicBroker{partitions: []int32{0}, unknownFor: 2, notLeaderFor: 3}
	if err := waitTopicReady(b, "avro-many", 1, time.Second); err != nil {
		t.Fatalf("waitTopicReady: %v", err)
	}
	if b.lookups != 4 {
		t.Errorf("%d offset lookups, want 3 refused and 1 answered", b.lookups)
	}
}

func TestWaitTopicReady_WaitsForEveryPartition(t *testing.T) {
	fastTopicPolls(t)
	b := &fakeTopicBroker{partitions: []int32{0, 1}}
	if err := waitTopicReady(b, "t", 3, 20*time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), "2 of its partitions") {
		t.Errorf("error %v, want the missing partition reported", err)
	}

	b = &fakeTopicBroker{partitions: []int32{0, 1, 2}}
	if err := waitTopicReady(b, "t", 3, time.Second); err != nil {
		t.Errorf("waitTopicReady: %v", err)
	}
	if b.lookups != 3 {
		t.Errorf("%d offset lookups, want one per partition", b.lookups)
	}
}

func TestWaitTopicReady_GivesUpWithTheLastError(t *testing.T) {
	fastTopicPolls(t)
	b := &fakeTopicBroker{partitions: []int32{0}, notLeaderFor: 1 << 30}
	err := waitTopicReady(b, "stuck", 1, 20*time.Millisecond)
	if !errors.Is(err, sarama.ErrNotLeaderForPartition) || !strings.Contains(err.Error(), "topic stuck is not ready after 20ms") {
		t.Errorf("error %v, want the timeout with the broker's last answer", err)
	}
}

// startConsuming marks a topic as consumed before it has a consumer. When it
// failed, the mark stayed, and every later "consumes from" was a silent no-op.
func TestAbortConsuming_LetsALaterStepTryAgain(t *testing.T) {
	r := &Kafka{consuming: map[string]bool{"t": true}, stopChannels: map[string]chan struct{}{}}
	stopCh := make(chan struct{})
	r.stopChannels["t"] = stopCh

	r.abortConsuming("t", stopCh)
	if r.consuming["t"] || r.stopChannels["t"] != nil {
		t.Errorf("still marked consuming: %v, %v", r.consuming, r.stopChannels)
	}
	select {
	case <-stopCh:
	default:
		t.Error("partitions already started were not stopped")
	}
}
