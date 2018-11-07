package sql

import (
	"encoding/json"
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/conv"
	"github.com/tomatool/tomato/errors"
	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	Select(tableName string, condition map[string]string) ([]map[string]string, error)
	Insert(tableName string, rows []map[string]string) error
	Delete(tableName string, condition map[string]string) (int, error)
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) tableCompare(resourceName, tableName string, content *gherkin.DataTable) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	expectedRows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	for _, expectedRow := range expectedRows {
		for k, v := range expectedRow {
			if v == "*" {
				delete(expectedRow, k)
			}
		}
		out, err := r.Select(tableName, expectedRow)
		if err != nil {
			return err
		}

		if len(out) == 0 {
			rows, _ := r.Select(tableName, nil)

			r, _ := json.MarshalIndent(expectedRow, "", "    ")
			t, _ := json.MarshalIndent(rows, "", "    ")
			return errors.NewStep("unable to find rows in table `"+tableName+"`", map[string]string{
				"expected row": string(r),
				"table values": string(t),
			})
		}
	}

	return nil
}

func (h *Handler) tableInsert(resourceName, tableName string, content *gherkin.DataTable) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	return r.Insert(tableName, rows)
}
