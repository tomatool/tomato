package handler

import (
	"fmt"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/db/sql"
	"github.com/alileza/tomato/util/conv"
)

func (h *Handler) setTableListOfContent(name, table string, content *gherkin.DataTable) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	dbClient := sql.Cast(r)

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	return dbClient.Set(table, rows)
}

func (h *Handler) tableShouldLookLike(name, tableName string, content *gherkin.DataTable) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	dbClient := sql.Cast(r)

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	for _, row := range rows {
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
