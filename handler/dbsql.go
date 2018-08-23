package handler

import (
	"fmt"
	"strings"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/db/sql"
)

func (h *Handler) getResourceDB(name string) sql.SQL {
	r, err := h.resource.Get(name)
	if err != nil {
		panic(err)
	}

	return sql.Cast(r)
}

func (h *Handler) setTableToEmpty(name, tables string) error {
	dbClient := h.getResourceDB(name)

	for _, table := range strings.Split(tables, ",") {
		if table == "*" {
			return dbClient.TruncateAll()
		}

		if err := dbClient.Set(table, &gherkin.DataTable{}); err != nil {
			return err
		}
	}

	return nil
}
func (h *Handler) setTableListOfContent(name, table string, content *gherkin.DataTable) error {
	return h.getResourceDB(name).Set(table, content)
}

func (h *Handler) tableShouldLookLike(name, tableName string, content *gherkin.DataTable) error {
	dbClient := h.getResourceDB(name)

	rowsCount, err := dbClient.Count(tableName, nil)
	if err != nil {
		return err
	}

	if rowsCount != len(content.Rows) {
		return fmt.Errorf("expecting row count to be %d, got %d", len(content.Rows), rowsCount)
	}

	for _, row := range content.Rows {
		count, err := dbClient.Count(tableName, row)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("row not found \n %+v", row)
		}
	}

	return nil
}
