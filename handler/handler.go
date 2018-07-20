package handler

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/resource"
	"github.com/alileza/gebet/resource/http/client"
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
		s.Step(`^set "([^"]*)" table "([^"]*)" list of content$`, h.setTableListOfContent)
		s.Step(`^"([^"]*)" table "([^"]*)" should look like$`, h.tableShouldLookLike)
		s.Step(`^set "([^"]*)" response code to (\d+) and response body$`, h.setResponseCodeToAndResponseBody)
	}
}

func (h *Handler) sendRequestToWithBody(name, endpoint string, payload *gherkin.DocString) error {
	resource, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := client.T(resource)

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
