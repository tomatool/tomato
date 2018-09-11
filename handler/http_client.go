package handler

import (
	"errors"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/compare"
)

func (h *Handler) sendRequest(resourceName, target string) error {
	return h.sendRequestWithBody(resourceName, target, nil)
}

func (h *Handler) sendRequestWithBody(resourceName, target string, content *gherkin.DocString) error {
	r, err := h.resource.GetHTTPClient(resourceName)
	if err != nil {
		return err
	}

	tt := strings.Split(target, " ")

	var requestBody []byte
	if content != nil {
		requestBody = []byte(content.Content)
	}
	return r.Request(tt[0], tt[1], requestBody)
}

func (h *Handler) checkResponseCode(resourceName string, expectedCode int) error {
	r, err := h.resource.GetHTTPClient(resourceName)
	if err != nil {
		return err
	}
	code, _, err := r.Response()
	if err != nil {
		return err
	}
	if code != expectedCode {
		return errors.New("invalid code")
	}

	return nil
}

func (h *Handler) checkResponseBody(resourceName string, expectedBody *gherkin.DocString) error {
	r, err := h.resource.GetHTTPClient(resourceName)
	if err != nil {
		return err
	}
	_, body, err := r.Response()
	if err != nil {
		return err
	}

	return compare.JSON([]byte(expectedBody.Content), body, false)
}
