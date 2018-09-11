package handler

import (
	"github.com/DATA-DOG/godog/gherkin"
)

func (h *Handler) setResponse(resourceName, path string, code int, body *gherkin.DocString) error {
	r, err := h.resource.GetHTTPServer(resourceName)
	if err != nil {
		return err
	}
	return r.SetResponse(path, code, []byte(body.Content))
}
