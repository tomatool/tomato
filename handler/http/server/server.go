package server

import (
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	SetResponse(requestPath string, responseCode int, responseBody []byte) error
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) setResponse(resourceName, path string, code int, body *gherkin.DocString) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found to set response", resourceName)
	}
	return r.SetResponse(path, code, []byte(body.Content))
}
