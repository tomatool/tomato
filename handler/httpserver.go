package handler

import (
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/http/server"
)

func (h *Handler) setResponseCodeToAndResponseBody(name string, code int, body *gherkin.DocString) error {
	return h.setWithPathResponseCodeToAndResponseBody(name, "", code, body)
}

func (h *Handler) setWithPathResponseCodeToAndResponseBody(name, path string, code int, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpServer := server.Cast(r)

	httpServer.SetResponsePath(path, code, []byte(body.Content))

	return nil
}
