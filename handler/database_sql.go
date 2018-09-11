package handler

import (
	"bytes"
	"errors"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/compare"
	"github.com/alileza/tomato/util/conv"
	"github.com/olekukonko/tablewriter"
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

	if len(tableRows) == 0 && len(expectedRows) == 0 {
		return nil
	}

	var actualColumns []string
	for column := range tableRows[0] {
		actualColumns = append(actualColumns, column)
	}
	for _, row := range expectedRows {
		for _, column := range actualColumns {
			if _, ok := row[column]; !ok {
				row[column] = "*"
			}
		}
	}
	if len(expectedRows) < len(tableRows) {
		for i := 0; i < len(tableRows)-len(expectedRows); i++ {
			row := make(map[string]string)
			for _, column := range actualColumns {
				if _, ok := row[column]; !ok {
					row[column] = "*"
				}
			}
			expectedRows = append(expectedRows, row)
		}
	}

	if !compare.Value(tableRows, expectedRows) {
		b := bytes.NewBufferString("\nTable mismatch\n\n")
		t := tablewriter.NewWriter(b)
		compare.Print(t, "", tableRows, expectedRows)
		t.Render()
		return errors.New(b.String())
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
