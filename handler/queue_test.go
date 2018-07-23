package handler

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/godog/gherkin"
)

var (
	resourceQueue = &resourceQueueMock{make(map[string][]byte), make(map[string]int), ""}
)

type resourceQueueMock struct {
	published map[string][]byte
	count     map[string]int
	listen    string
}

func (mgr *resourceQueueMock) Ready() error { return nil }
func (mgr *resourceQueueMock) Close() error { return nil }

func (c *resourceQueueMock) Listen(target string) error {
	c.listen = target
	return nil
}
func (c *resourceQueueMock) Count(target string) (int, error) { return c.count[target], nil }
func (c *resourceQueueMock) Publish(target string, payload []byte) error {
	c.published[target] = payload
	return nil
}
func (c *resourceQueueMock) Consume(target string) []byte { return []byte(`{"awesome":"message"}`) }

func TestPublishMessageToTargetWithPayload(t *testing.T) {
	if err := h.publishMessageToTargetWithPayload("queue-resource", "abc", &gherkin.DocString{Content: `{}`}); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(resourceQueue.published["abc"], []byte(`{}`)) {
		t.Errorf("expecting published message to be %s, got %s", `{}`, resourceQueue.published["abc"])
	}
}

func TestListenMessageFromTarget(t *testing.T) {
	if err := h.listenMessageFromTarget("queue-resource", "hdhuwk"); err != nil {
		t.Error(err)
	}

	if o := resourceQueue.listen; o != "hdhuwk" {
		t.Errorf("expecting queue to listen to %s, got %s", "hdhuwk", o)
	}
}
func TestMessageFromTargetCountShouldBe(t *testing.T) {
	if err := h.messageFromTargetCountShouldBe("queue-resource", "hdhuwk", 1); err == nil {
		t.Error("expecting error, got nil")
	}

	resourceQueue.count["hdhuwk"] = 134

	if err := h.messageFromTargetCountShouldBe("queue-resource", "hdhuwk", 134); err != nil {
		t.Error(err)
	}
}

func TestMessageFromTargetShouldLookLike(t *testing.T) {
	err := h.messageFromTargetShouldLookLike("queue-resource", "hdhuwk", &gherkin.DocString{Content: `{"awesome":"message"}`})
	if err != nil {
		t.Error(err)
	}

	err = h.messageFromTargetShouldLookLike("queue-resource", "hdhuwk", &gherkin.DocString{Content: `{"awesome":200}`})
	if err == nil {
		t.Error("expecting error, got nil")
	}
}
