package conv

import (
	"fmt"
	"strconv"

	"github.com/DATA-DOG/godog/gherkin"
)

type columnHash map[string]string

func (row columnHash) getBool(column string) (bool, error) {
	return strconv.ParseBool(row[column])
}

func GherkinTableToSliceOfMap(tbl *gherkin.DataTable) ([]map[string]string, error) {
	hash := make([]map[string]string, len(tbl.Rows)-1)

	columns := tbl.Rows[0].Cells
	columnCount := len(columns)
	for i := 1; i < len(tbl.Rows); i++ {
		row := tbl.Rows[i]
		if len(row.Cells) != columnCount {
			return nil, fmt.Errorf("Invalid cells in row %v", i)
		}

		rowHash := make(columnHash, columnCount)
		for i, cell := range row.Cells {
			rowHash[columns[i].Value] = cell.Value
		}

		hash[i-1] = rowHash
	}

	return hash, nil
}
