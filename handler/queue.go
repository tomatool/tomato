package handler

import (
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/queue"
	"github.com/alileza/tomato/util/cmp"
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

	messageCount, err := mqClient.Count(target)
	if err != nil {
		return err
	}

	if messageCount != count {
		return fmt.Errorf("queue: mismatch count for target `%s`, expecting=%d got=%d", target, count, messageCount)
	}

	return nil
}

func (h *Handler) messageFromTargetShouldLookLike(name, target string, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	mqClient := queue.Cast(r)

	consumedMessage := mqClient.Consume(target)
	if consumedMessage == nil {
		return fmt.Errorf("no message to consume `%s`", target)
	}

	return cmp.JSON([]byte(body.Content), consumedMessage, false)
}
