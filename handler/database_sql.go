package handler

import (
	"encoding/json"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/conv"
	"github.com/tomatool/tomato/errors"
)

func (h *Handler) tableCompare(resourceName, tableName string, content *gherkin.DataTable) error {
	r, err := h.resource.GetDatabaseSQL(resourceName)
	if err != nil {
		return err
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
	r, err := h.resource.GetDatabaseSQL(resourceName)
	if err != nil {
		return err
	}

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	return r.Insert(tableName, rows)
}
