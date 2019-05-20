package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/olekukonko/tablewriter"
	"github.com/tomatool/tomato/compare"
	"github.com/tomatool/tomato/errors"
	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	Request(method, path string, body []byte) error
	RequestFromFile(method, path, file string) error
	Response() (int, http.Header, []byte, error)
	SetRequestHeader(string, string) error
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) sendRequest(resourceName, target string) error {
	return h.sendRequestWithBody(resourceName, target, nil)
}

func (h *Handler) setRequestHeader(resourceName, key, value string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	if err := r.SetRequestHeader(key, value); err != nil {
		return err
	}

	return nil
}

func (h *Handler) sendRequestWithBody(resourceName, target string, content *gherkin.DocString) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	tt := strings.Split(target, " ")
	if len(tt) != 2 {
		return fmt.Errorf("unrecognized target format: %s,  should follow `[METHOD] [PATH]`", target)
	}

	var requestBody []byte
	if content != nil {
		requestBody = []byte(strings.TrimSpace(content.Content))
	}
	return r.Request(tt[0], tt[1], requestBody)
}

func (h *Handler) sendRequestWithBodyFromFile(resourceName, target string, file string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	tt := strings.Split(target, " ")
	if len(tt) != 2 {
		return fmt.Errorf("unrecognized target format: %s,  should follow `[METHOD] [PATH]`", target)
	}
	return r.RequestFromFile(tt[0], tt[1], file)
}

func (h *Handler) checkResponseCode(resourceName string, expectedCode int) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
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
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
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

func (h *Handler) checkResponseBodyEquals(resourceName string, expectedBody *gherkin.DocString) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
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

	comparison, err := compare.JSON(body, []byte(expectedBody.Content), true)
	if err != nil {
		return err
	}

	if comparison.ShouldFailStep() {
		return comparison
	}
	return nil
}

func (h *Handler) checkResponseBodyContains(resourceName string, expectedBody *gherkin.DocString) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
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
