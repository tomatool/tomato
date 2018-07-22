package handler

import (
	"encoding/json"
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/resource/http/client"
	"github.com/alileza/gebet/util/cmp"
)

func (h *Handler) responseCodeShouldBe(name string, code int) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpClient := client.T(r)

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
	httpClient := client.T(r)

	_, responseBody := httpClient.ResponseCode(), httpClient.ResponseBody()

	gotResponse := make(map[string]interface{})
	if err := json.Unmarshal(responseBody, &gotResponse); err != nil {
		return err
	}

	expectedResponse := make(map[string]interface{})
	if err := json.Unmarshal([]byte(body.Content), &expectedResponse); err != nil {
		return err
	}

	if err := cmp.Map(expectedResponse, gotResponse); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(responseBody), err.Error())
	}

	if err := cmp.Map(gotResponse, expectedResponse); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(responseBody), err.Error())
	}

	return nil
}
