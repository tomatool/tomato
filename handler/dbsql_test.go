package handler

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/godog/gherkin"
)

var (
	resourceSQL = &resourceSQLMock{make(map[string][]map[string]string)}
)

type resourceSQLMock struct {
	inserted map[string][]map[string]string
}

func (mgr *resourceSQLMock) Ready() error { return nil }
func (mgr *resourceSQLMock) Close() error { return nil }

func (mgr *resourceSQLMock) Clear(tableName string) error { return nil }
func (mgr *resourceSQLMock) Set(tableName string, rows []map[string]string) error {
	mgr.inserted[tableName] = rows
	return nil
}
func (mgr *resourceSQLMock) Count(tableName string, condition map[string]string) (int, error) {
	for _, row := range mgr.inserted[tableName] {
		if reflect.DeepEqual(condition, row) {
			return 1, nil
		}
	}
	return 0, nil
}

func TestSetTableListOfContent(t *testing.T) {
	expectedRows := []map[string]string{
		{"name": "cembri", "age": "24"},
		{"name": "cembre", "age": "26"},
	}

	input := &gherkin.DataTable{
		Rows: []*gherkin.TableRow{},
	}
	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "name"},
			{Value: "age"},
		},
	})
	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "cembri"},
			{Value: "24"},
		},
	})
	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "cembre"},
			{Value: "26"},
		},
	})

	if err := h.setTableListOfContent("sql-resource", "abc", input); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(resourceSQL.inserted["abc"], expectedRows) {
		t.Errorf("inserted value != expected, %+v != %+v", resourceSQL.inserted["abc"], expectedRows)
	}

	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "cembre"},
		},
	})
	if err := h.setTableListOfContent("sql-resource", "abc", input); err == nil {
		t.Error("expecting error invalid column count, but got nil")
	}
}

func TestTableShouldLookLike(t *testing.T) {
	resourceSQL.inserted["abc"] = []map[string]string{
		{"name": "cembri", "age": "24"},
	}

	input := &gherkin.DataTable{
		Rows: []*gherkin.TableRow{},
	}
	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "name"},
			{Value: "age"},
		},
	})
	input.Rows = append(input.Rows, &gherkin.TableRow{
		Cells: []*gherkin.TableCell{
			{Value: "cembri"},
			{Value: "24"},
		},
	})

	if err := h.tableShouldLookLike("sql-resource", "abc", input); err != nil {
		t.Error(err)
	}

	resourceSQL.inserted["abc"] = []map[string]string{}

	if err := h.tableShouldLookLike("sql-resource", "abc", input); err == nil {
		t.Errorf("expecting error from table should look like, because inserted is empty")
	}
}
