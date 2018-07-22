package handler

import (
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/gebet/resource/db/sql"
	"github.com/alileza/gebet/util/conv"
)

func (h *Handler) setTableListOfContent(name, table string, content *gherkin.DataTable) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	dbClient := sql.T(r)

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	return dbClient.Set(table, rows)
}

func (h *Handler) tableShouldLookLike(name, table string, content *gherkin.DataTable) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	dbClient := sql.T(r)

	rows, err := conv.GherkinTableToSliceOfMap(content)
	if err != nil {
		return err
	}

	return dbClient.Compare(table, rows)
}
