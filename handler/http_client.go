package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/olekukonko/tablewriter"
	"github.com/tomatool/tomato/compare"
	"github.com/tomatool/tomato/errors"
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
	code, _, body, err := r.Response()
	if err != nil {
		return err
	}
	if code != expectedCode {
		return fmt.Errorf("expecting response code to be %d, got %d\nresponse body : \n%s", expectedCode, code, string(body))
	}

	return nil
}

func (h *Handler) checkResponseHeader(resourceName string, expectedHeaderName, expectedHeaderValue string) error {
	r, err := h.resource.GetHTTPClient(resourceName)
	if err != nil {
		return err
	}
	_, header, body, err := r.Response()
	if err != nil {
		return err
	}
	hvalue := header.Get(expectedHeaderName)
	if hvalue != expectedHeaderValue {

		return errors.NewStep("unexpected response header `"+expectedHeaderName+"`", map[string]string{
			"expecting":     expectedHeaderValue,
			"actual":        hvalue,
			"response body": string(body),
		})
	}

	return nil
}

func (h *Handler) checkResponseBody(resourceName string, expectedBody *gherkin.DocString) error {
	r, err := h.resource.GetHTTPClient(resourceName)
	if err != nil {
		return err
	}
	_, _, body, err := r.Response()
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

	if err := compare.Value(actual, expected); err != nil {
		b := bytes.NewBufferString("")
		t := tablewriter.NewWriter(b)
		compare.Print(t, "", actual, expected)
		t.Render()
		return errors.NewStep("unexpected response body", map[string]string{
			"": b.String(),
		})
	}

	return nil
}
