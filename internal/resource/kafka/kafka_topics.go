package kafka

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// A broker accepts a new topic before it serves it: KRaft answers CreateTopics
// as soon as the controller has recorded the topic, and the broker takes over
// the partitions a moment later. A consumer that starts in between asks for the
// partition's offsets and is told "not the leader for some partition". So tomato
// waits, after every topic it creates, until each partition answers that same
// offset lookup.

// topicReadyTimeout bounds the wait, like the admin client's own timeout.
const topicReadyTimeout = 30 * time.Second

// topicPollInterval is how often the wait checks; a variable for tests.
var topicPollInterval = 100 * time.Millisecond

// topicMetadata is the part of sarama.Client the wait uses.
type topicMetadata interface {
	RefreshMetadata(topics ...string) error
	Partitions(topic string) ([]int32, error)
	GetOffset(topic string, partitionID int32, time int64) (int64, error)
}

// waitTopicReady waits until the topic has at least minPartitions partitions
// (0 for any number) and each one answers an offset lookup.
func waitTopicReady(client topicMetadata, topic string, minPartitions int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := topicReady(client, topic, minPartitions)
		if err == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("topic %s is not ready after %s: %w", topic, timeout, err)
		}
		time.Sleep(topicPollInterval)
	}
}

func topicReady(client topicMetadata, topic string, minPartitions int) error {
	if err := client.RefreshMetadata(topic); err != nil {
		return err
	}
	partitions, err := client.Partitions(topic)
	if err != nil {
		return err
	}
	if len(partitions) == 0 || len(partitions) < minPartitions {
		return fmt.Errorf("the broker knows %d of its partitions", len(partitions))
	}
	for _, p := range partitions {
		if _, err := client.GetOffset(topic, p, sarama.OffsetNewest); err != nil {
			return fmt.Errorf("partition %d: %w", p, err)
		}
	}
	return nil
}
