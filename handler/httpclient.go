package handler

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/resource/http/client"
	"github.com/tomatool/tomato/util/cmp"
)

func (h *Handler) getResourceHTTPClient(name string) client.Client {
	r, err := h.resource.Get(name)
	if err != nil {
		panic(err)
	}

	return client.Cast(r)
}

func (h *Handler) sendRequestTo(name, endpoint string) error {
	return h.sendRequestToWithBody(name, endpoint, &gherkin.DocString{})
}

func (h *Handler) sendRequestToWithBody(name, endpoint string, payload *gherkin.DocString) error {
	httpClient := h.getResourceHTTPClient(name)

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
	httpClient := h.getResourceHTTPClient(name)

	responseCode, responseBody := httpClient.ResponseCode(), httpClient.ResponseBody()
	if responseCode != code {
		return &ErrMismatch{"response code", code, responseCode, string(responseBody)}
	}

	return nil
}

func (h *Handler) responseBodyShouldBe(name string, body *gherkin.DocString) error {
	httpClient := h.getResourceHTTPClient(name)

	_, responseBody := httpClient.ResponseCode(), httpClient.ResponseBody()

	return cmp.JSON([]byte(body.Content), responseBody, false)
}
