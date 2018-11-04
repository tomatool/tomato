package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/compare"
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
		return fmt.Errorf("expecting message count to be %d, got %d\n", expectedCount, len(messages))
	}

	return nil
}

func (h *Handler) compareMessageEquals(resourceName, target string, expectedMessage *gherkin.DocString) error {
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

	var comparison compare.Comparison
	for _, msg := range messages {
		comparison, err = compare.JSON(msg, []byte(expectedMessage.Content), true)
		if err != nil {
			return err
		} else if comparison.ShouldFailStep() {
			return comparison
		}
	}
	return nil
}

func (h *Handler) compareMessageContains(resourceName, target string, expectedMessage *gherkin.DocString) error {
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

	expected := make(map[string]interface{})
	if err := json.Unmarshal([]byte(expectedMessage.Content), &expected); err != nil {
		return err
	}

	var consumedMessage []string
	for _, msg := range messages {

		actual := make(map[string]interface{})
		if err := json.Unmarshal(msg, &actual); err != nil {
			return err
		}

		err := compare.Value(actual, expected)
		if err == nil {
			return nil
		}

		consumedMessage = append(consumedMessage, string(msg)+"\n"+err.Error())
	}

	return fmt.Errorf("expecting message : %+v\nconsumed messages : %+v", expectedMessage.Content, strings.Join(consumedMessage, "\n"))
}
