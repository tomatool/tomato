package kafka

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/tomatool/tomato/internal/resource"
)

// Consumer groups. tomato can join one, to consume the way a service does, and
// assert on any group as the broker sees it: which members it has and what they
// are assigned. A service whose listeners cannot authenticate shows up here as
// a group with no members, whatever its own health checks say.

func newKafkaConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_0_0_0
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Consumer.Return.Errors = true
	cfg.Admin.Timeout = 30 * time.Second
	return cfg
}

// kafkaGroupMember is a consumer group tomato joined.
type kafkaGroupMember struct {
	group  sarama.ConsumerGroup
	cancel context.CancelFunc
	done   chan struct{}
}

func (r *Kafka) consumerGroupSteps() []resource.StepDef {
	return []resource.StepDef{
		{
			Group:       "Consumer Groups",
			Pattern:     `^"{resource}" consumes from "([^"]*)" as consumer group "([^"]*)"$`,
			Description: "Joins a consumer group on a topic; its messages are received like any other",
			Example:     `"{resource}" consumes from "events" as consumer group "billing"`,
			Handler:     r.consumeAsGroup,
		},
		{
			Group:       "Consumer Groups",
			Pattern:     `^"{resource}" consumer group "([^"]*)" is consuming "([^"]*)" within "([^"]*)"$`,
			Description: "Waits until a member of the consumer group is assigned the topic, as the broker sees it",
			Example:     `"{resource}" consumer group "billing" is consuming "events" within "30s"`,
			Handler:     r.groupShouldBeConsuming,
		},
		{
			Group:       "Consumer Groups",
			Pattern:     `^"{resource}" consumer group "([^"]*)" has no members$`,
			Description: "Asserts the consumer group has no members, as the broker sees it",
			Example:     `"{resource}" consumer group "billing" has no members`,
			Handler:     r.groupShouldHaveNoMembers,
		},
	}
}

// consumeAsGroup joins groupID on topic and returns once the group has its
// first assignment, so a following step sees the member.
func (r *Kafka) consumeAsGroup(topic, groupID string) error {
	r.groupsMu.Lock()
	defer r.groupsMu.Unlock()

	key := groupID + "\x00" + topic
	if _, joined := r.groups[key]; joined {
		return nil
	}

	cfg := newKafkaConfig()
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	group, err := sarama.NewConsumerGroup(r.brokers, groupID, cfg)
	if err != nil {
		return fmt.Errorf("joining consumer group %q: %w", groupID, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	handler := &kafkaGroupHandler{kafka: r, topic: topic, joined: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ctx.Err() == nil {
			if err := group.Consume(ctx, []string{topic}, handler); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	select {
	case <-handler.joined:
	case <-time.After(30 * time.Second):
		cancel()
		_ = group.Close()
		<-done
		return fmt.Errorf("consumer group %q got no assignment on %q within 30s", groupID, topic)
	}

	r.groups[key] = &kafkaGroupMember{group: group, cancel: cancel, done: done}
	return nil
}

// kafkaGroupHandler adds what the group consumes to the resource's received
// messages, so the receive and assertion steps see it.
type kafkaGroupHandler struct {
	kafka      *Kafka
	topic      string
	joined     chan struct{}
	joinedOnce sync.Once
}

func (h *kafkaGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.joinedOnce.Do(func() { close(h.joined) })
	return nil
}

func (h *kafkaGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *kafkaGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.kafka.messagesMu.Lock()
		h.kafka.messages[h.topic] = append(h.kafka.messages[h.topic], msg)
		h.kafka.lastMessage = msg
		h.kafka.messagesMu.Unlock()
		sess.MarkMessage(msg, "")
	}
	return nil
}

// stopConsumerGroups leaves every group tomato joined, which removes its
// members from the broker's view of the group.
func (r *Kafka) stopConsumerGroups() {
	r.groupsMu.Lock()
	defer r.groupsMu.Unlock()
	for key, m := range r.groups {
		m.cancel()
		_ = m.group.Close()
		select {
		case <-m.done:
		case <-time.After(10 * time.Second):
		}
		delete(r.groups, key)
	}
}

// groupMember is one member of a consumer group as the broker describes it.
type groupMember struct {
	clientID string
	host     string
	topics   []string
}

// describeGroup returns a group's state and members. A group that does not
// exist is described as Dead with no members.
func (r *Kafka) describeGroup(groupID string) (string, []groupMember, error) {
	descriptions, err := r.admin.DescribeConsumerGroups([]string{groupID})
	if err != nil {
		return "", nil, fmt.Errorf("describing consumer group %q: %w", groupID, err)
	}
	if len(descriptions) == 0 {
		return "Dead", nil, nil
	}
	d := descriptions[0]
	if d.Err != sarama.ErrNoError {
		return "", nil, fmt.Errorf("describing consumer group %q: %w", groupID, d.Err)
	}
	members := make([]groupMember, 0, len(d.Members))
	for _, m := range d.Members {
		member := groupMember{clientID: m.ClientId, host: m.ClientHost}
		if assignment, err := m.GetMemberAssignment(); err == nil && assignment != nil {
			for topic := range assignment.Topics {
				member.topics = append(member.topics, topic)
			}
			sort.Strings(member.topics)
		}
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].clientID < members[j].clientID })
	return d.State, members, nil
}

func (r *Kafka) groupShouldBeConsuming(groupID, topic, timeout string) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	deadline := time.Now().Add(d)
	for {
		state, members, err := r.describeGroup(groupID)
		var last string
		if err != nil {
			last = err.Error()
		} else {
			for _, m := range members {
				for _, t := range m.topics {
					if t == topic {
						return nil
					}
				}
			}
			last = fmt.Sprintf("state %s, members: %s", state, describeMembers(members))
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("no member of consumer group %q was assigned %q within %s (%s)", groupID, topic, timeout, last)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (r *Kafka) groupShouldHaveNoMembers(groupID string) error {
	state, members, err := r.describeGroup(groupID)
	if err != nil {
		return err
	}
	if len(members) > 0 {
		return fmt.Errorf("consumer group %q has %d member(s) (state %s): %s", groupID, len(members), state, describeMembers(members))
	}
	return nil
}

func describeMembers(members []groupMember) string {
	if len(members) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(members))
	for _, m := range members {
		parts = append(parts, fmt.Sprintf("%s@%s [%s]", m.clientID, m.host, strings.Join(m.topics, ",")))
	}
	return strings.Join(parts, "; ")
}
