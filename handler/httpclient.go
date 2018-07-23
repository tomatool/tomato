package handler

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/http/client"
	"github.com/alileza/tomato/util/cmp"
)

func (h *Handler) sendRequestTo(name, endpoint string) error {
	return h.sendRequestToWithBody(name, endpoint, &gherkin.DocString{})
}

func (h *Handler) sendRequestToWithBody(name, endpoint string, payload *gherkin.DocString) error {
	resource, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := client.Cast(resource)

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

func (h *Handler) responseCodeShouldBe(name string, code int) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := client.Cast(r)

	responseCode, responseBody := httpClient.ResponseCode(), httpClient.ResponseBody()
	if responseCode != code {
		return &ErrMismatch{"response code", code, responseCode, string(responseBody)}
	}

	return nil
}

func (h *Handler) responseBodyShouldBe(name string, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := client.Cast(r)

	_, responseBody := httpClient.ResponseCode(), httpClient.ResponseBody()

	return cmp.JSON([]byte(body.Content), responseBody, false)
}
