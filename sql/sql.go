package sql

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx/types"
)

type QueryBuilder struct {
	databaseDriver string
	paramsCount    int
	baseQuery      string
	key            []string
	value          []string
	where          []string
	whereOr        []string
	order          string
	offset         string
	limit          string
	args           []interface{}
}

func NewQueryBuilder(driverName string, baseQuery string) *QueryBuilder {
	return &QueryBuilder{databaseDriver: driverName, baseQuery: baseQuery}
}

func (q *QueryBuilder) inc() string {
	q.paramsCount = q.paramsCount + 1
	switch q.databaseDriver {
	case "mysql":
		return "?"
	case "postgres":
		return fmt.Sprintf("$%d", q.paramsCount)
	}
	return "?"
}

func (q QueryBuilder) SetBaseQuery(query string) *QueryBuilder {
	q.baseQuery = query
	return &q
}

func (q *QueryBuilder) Where(key, operator string, val interface{}) {
	if val == nil {
		q.where = append(q.where, fmt.Sprintf("%s IS NULL", key))
		return
	}
	q.where = append(q.where, fmt.Sprintf("%s %s %s", key, operator, q.inc()))
	q.args = append(q.args, val)
}

func (q *QueryBuilder) Value(key string, val interface{}) {
	if val == nil {
		return
	}
	q.key = append(q.key, key)
	q.value = append(q.value, q.inc())

	valstr, ok := (val.(string))
	if ok && strings.HasPrefix(valstr, ColumnTypeBit) && len(valstr[5:]) == 1 {
		val = types.BitBool(valstr[5] == '1')
	}

	q.args = append(q.args, val)
}

const (
	ColumnTypeArrayVarchar = "array::varchar"
	ColumnTypeBit          = "bit::"
)

func (q *QueryBuilder) WhereOr(key, operator string, val interface{}, valueType ...string) {
	if val == nil {
		q.whereOr = append(q.whereOr, fmt.Sprintf("%s IS NULL", key))
		return
	}
	inc := q.inc()

	params := fmt.Sprintf("%s", inc)
	if len(valueType) > 0 {
		switch valueType[0] {
		case ColumnTypeArrayVarchar:
			params = fmt.Sprintf("array[%s::varchar]", inc)
		}
	}
	q.whereOr = append(q.whereOr, fmt.Sprintf("%s %s %s", key, operator, params))
	q.args = append(q.args, val)
}

func (q *QueryBuilder) Limit(val int) {
	q.limit = fmt.Sprintf(" LIMIT %s", q.inc())
	q.args = append(q.args, val)
}

func (q *QueryBuilder) Offset(val int) {
	q.offset = fmt.Sprintf(" OFFSET %s", q.inc())
	q.args = append(q.args, val)
}

func (q *QueryBuilder) OrderBy(field, order string) {
	q.order = fmt.Sprintf(" ORDER BY %s %s", field, order)
}

func (q *QueryBuilder) Query() string {
	result := q.baseQuery
	if len(q.key) > 0 {
		result += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(q.key, ","), strings.Join(q.value, ","))
	}
	if len(q.where) > 0 || len(q.whereOr) > 0 {
		result += " WHERE "
	}
	if len(q.where) > 0 {
		result += fmt.Sprintf("(%s)", strings.Join(q.where, " AND "))
	}
	if len(q.whereOr) > 0 {
		result += fmt.Sprintf(" AND (%s)", strings.Join(q.whereOr, " OR "))
	}
	if q.order != "" {
		result += q.order
	}
	if q.limit != "" {
		result += q.limit
	}
	if q.offset != "" {
		result += q.offset
	}
	return result
}

func (q *QueryBuilder) Arguments() []interface{} {
	return q.args
}
