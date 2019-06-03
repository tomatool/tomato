package sql

import (
	"fmt"

	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	Set(key string, value string) error
	Get(key string) (string, error)
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) set(resourceName, key, value string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	return r.Set(key, value)
}

func (h *Handler) compare(resourceName, key, expectedValue string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	v, err := r.Get(key)
	if err != nil {
		return err
	}

	if v != expectedValue {
		return fmt.Errorf("Unexpected key=%s value of=%s", key, v)
	}

	return nil
}
