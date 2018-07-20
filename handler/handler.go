package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/common/cmp"
	"github.com/alileza/gebet/resource"
	rhttp "github.com/alileza/gebet/resource/http"
)

type Handler struct {
	resource *resource.Manager
}

func New(r *resource.Manager) func(s *godog.Suite) {
	h := &Handler{r}
	return func(s *godog.Suite) {
		s.Step(`^"([^"]*)" send request to "([^"]*)" with body$`, h.sendRequestToWithBody)
		s.Step(`^"([^"]*)" response code should be (\d+)$`, h.responseCodeShouldBe)
		s.Step(`^"([^"]*)" response body should be$`, h.responseBodyShouldBe)
	}
}

func (h *Handler) sendRequestToWithBody(name, endpoint string, payload *gherkin.DocString) error {
	resource, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := resource.(*rhttp.Client)

	e := strings.Split(endpoint, " ")
	if len(e) < 2 {
		return errors.New("expecting endpoint to be `[METHOD] [TARGET]`")
	}

	req, err := http.NewRequest(e[0], e[1], bytes.NewBufferString(payload.Content))
	if err != nil {
		return err
	}

	return httpClient.Do(req)
}

type ErrMismatch struct {
	field       string
	expectation interface{}
	result      interface{}
	metadata    string
}

func (e *ErrMismatch) Error() string {
	msg := fmt.Sprintf("\n[MISMATCH] %s\nexpecting\t:\t%+v\ngot\t\t:\t%+v", e.field, e.expectation, e.result)
	if e.metadata != "" {
		msg += fmt.Sprintf("\nmetadata\t:\t%s", e.metadata)
	}
	return msg
}

func (h *Handler) responseCodeShouldBe(name string, code int) error {
	resource, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := resource.(*rhttp.Client)

	res := httpClient.LastResponse()
	if res == nil {
		return errors.New("unexpected nil LastResponse")
	}

	if res.Code != code {
		return &ErrMismatch{"response code", code, res.Code, string(res.Body)}
	}

	return nil
}

func (h *Handler) responseBodyShouldBe(name string, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := resource.HTTP(r)

	res := httpClient.LastResponse()
	if res == nil {
		return errors.New("unexpected nil LastResponse")
	}

	gotResponse := make(map[string]interface{})
	if err := json.Unmarshal(res.Body, &gotResponse); err != nil {
		return err
	}

	expectedResponse := make(map[string]interface{})
	if err := json.Unmarshal([]byte(body.Content), &expectedResponse); err != nil {
		return err
	}

	if err := cmp.Map(expectedResponse, gotResponse); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(res.Body), err.Error())
	}

	return nil
}
