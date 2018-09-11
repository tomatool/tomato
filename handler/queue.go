package handler

import (
	"errors"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/compare"
)

func (h *Handler) publishMessage(resourceName, target string, payload *gherkin.DocString) error {
	r, err := h.resource.GetQueue(resourceName)
	if err != nil {
		return err
	}

	return r.Publish(target, []byte(payload.Content))
}

func (h *Handler) listenMessage(resourceName, target string) error {
	r, err := h.resource.GetQueue(resourceName)
	if err != nil {
		return err
	}

	return r.Listen(target)
}

func (h *Handler) countMessage(resourceName, target string, expectedCount int) error {
	r, err := h.resource.GetQueue(resourceName)
	if err != nil {
		return err
	}

	messages, err := r.Fetch(target)
	if err != nil {
		return err
	}

	if len(messages) != expectedCount {
		return errors.New("m")
	}

	return nil
}

func (h *Handler) messageCompare(resourceName, target string, expectedMessage *gherkin.DocString) error {
	r, err := h.resource.GetQueue(resourceName)
	if err != nil {
		return err
	}

	messages, err := r.Fetch(target)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		return errors.New("no message on queue")
	}

	for _, msg := range messages {
		if err := compare.JSON(
			[]byte(expectedMessage.Content),
			msg,
			false,
		); err == nil {
			return nil
		}
	}

	return errors.New("couldn't find : " + expectedMessage.Content)
}
