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

	return mqClient.Count(target, count)
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

	return cmp.JSON(consumedMessage, []byte(body.Content), false)
}
