package handler

import (
	"encoding/json"
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/resource/queue"
	"github.com/alileza/gebet/util/cmp"
)

func (h *Handler) publishMessageToTargetWithPayload(name, target string, payload *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	mqClient := queue.Cast(r)

	return mqClient.Publish(target, []byte(payload.Content))
}

func (h *Handler) listenMessageFromTarget(name, target string) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	mqClient := queue.Cast(r)

	return mqClient.Listen(target)
}

func (h *Handler) messageFromTargetCountShouldBe(name, target string, count int) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	mqClient := queue.Cast(r)

	return mqClient.Count(target, count)
}

func (h *Handler) messageFromTargetShouldLookLike(name, target string, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	mqClient := queue.Cast(r)

	b := mqClient.Consume(target)
	if b == nil {
		return fmt.Errorf("no message to consume `%s`", target)
	}

	consumedMessage := make(map[string]interface{})
	if err := json.Unmarshal(b, &consumedMessage); err != nil {
		return err
	}

	expectedMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(body.Content), &expectedMessage); err != nil {
		return err
	}

	if err := cmp.Map(expectedMessage, consumedMessage); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(b), err.Error())
	}

	if err := cmp.Map(consumedMessage, expectedMessage); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(b), err.Error())
	}

	return nil
}
