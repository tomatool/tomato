package handler

import (
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/conv"
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
			return fmt.Errorf("couldn't find %+v in table \n%+v", expectedRow, rows)
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
