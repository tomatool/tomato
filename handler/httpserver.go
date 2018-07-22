package handler

import (
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/resource/http/server"
)

func (h *Handler) setResponseCodeToAndResponseBody(name string, code int, body *gherkin.DocString) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	httpServer := server.Cast(r)

	httpServer.SetResponse(code, []byte(body.Content))

	return nil
}
