package handler

import (
	"encoding/json"
	"errors"
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
	httpClient := client.T(r)

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

	if err := cmp.Map(gotResponse, expectedResponse); err != nil {
		return fmt.Errorf("expectedResponse=%s\n\nactualResponse=%s\n\n%s", body.Content, string(res.Body), err.Error())
	}

	return nil
}
