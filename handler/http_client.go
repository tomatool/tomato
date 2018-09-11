package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/compare"
	"github.com/olekukonko/tablewriter"
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
	expected := make(map[string]interface{})
	if err := json.Unmarshal([]byte(expectedBody.Content), &expected); err != nil {
		return err
	}

	actual := make(map[string]interface{})
	if err := json.Unmarshal(body, &actual); err != nil {
		return err
	}

	if !compare.Value(expected, actual) {
		b := bytes.NewBufferString("\nJSON mismatch\n\n")
		t := tablewriter.NewWriter(b)
		compare.Print(t, "", actual, expected)
		t.Render()
		return errors.New(b.String())
	}

	return nil
}
