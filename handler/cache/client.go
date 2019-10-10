package cache

import (
	"fmt"

	"github.com/tomatool/tomato/errors"
	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	Set(key string, value string) error
	Get(key string) (string, error)
	Exists(key string) (bool, error)
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) valueCompare(resourceName, key, expected string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	actual, err := r.Get(key)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.NewStep("unable to find correct value in cache", map[string]string{
			"expected value": expected,
			"cached value":   actual,
		})
	}
	return nil
}

func (h *Handler) valueSet(resourceName, key, expected string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	return r.Set(key, expected)
}

func (h *Handler) valueExists(resourceName, key string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	actual, err := r.Exists(key)
	if err != nil {
		return err
	}
	if !actual {
		return errors.NewStep("unable to find value in cache", map[string]string{
			"expected": "exists",
			"actual":   "not exists",
		})
	}
	return nil
}

func (h *Handler) valueNotExists(resourceName, key string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	actual, err := r.Exists(key)
	if err != nil {
		return err
	}
	if actual {
		return errors.NewStep("found value in cache", map[string]string{
			"expected": "not exists",
			"actual":   "exists",
		})
	}
	return nil
}
