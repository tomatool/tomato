package handler

import (
	"fmt"
	"reflect"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/util/conv"
)

func (h *Handler) tableCompare(resourceName, tableName string, content *gherkin.DataTable) error {
	r, err := h.resource.GetDatabaseSQL(resourceName)
	if err != nil {
		return err
	}

	tableRows, err := r.Select(tableName, nil)
	if err != nil {
		return err
	}

	expectedRows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(expectedRows, tableRows) {
		return fmt.Errorf("expecting=%+v\nactual=%+v", expectedRows, tableRows)
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
