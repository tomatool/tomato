package sql

import (
	"testing"

	"github.com/jmoiron/sqlx/types"
)

func TestQueryBuilder(t *testing.T) {
	var (
		baseQuery = "SELECT * FROM a"
	)
	qb := NewQueryBuilder("postgres", baseQuery)

	if q := qb.SetBaseQuery("ulala yeyeye").Query(); q != "ulala yeyeye" {
		t.Errorf("expecting query to be ulala yeyeye, got %s", q)
	}

	if out := len(qb.Arguments()); out != 0 {
		t.Errorf("expecting query builder argument to be 0, got %d", out)
	}
	if out := qb.Query(); out != baseQuery {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}

	qb.Limit(13)
	if out := qb.Query(); out != baseQuery+" LIMIT $1" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 1 {
		t.Errorf("expecting query builder argument to be 1, got %d", out)
	}
	if out := qb.Arguments()[0]; out != 13 {
		t.Errorf("expecting query builder argument to be 1, got %d", out)
	}

	qb.Where("u", "=", "abc")

	if out := qb.Query(); out != baseQuery+" WHERE (u = $2) LIMIT $1" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 2 {
		t.Errorf("expecting query builder argument to be 2, got %d", out)
	}
	if out := qb.Arguments()[1]; out != "abc" {
		t.Errorf("expecting query builder argument to be abc, got %s", out)
	}

	qb.Offset(738)

	if out := qb.Query(); out != baseQuery+" WHERE (u = $2) LIMIT $1 OFFSET $3" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 3 {
		t.Errorf("expecting query builder argument to be 3, got %d", out)
	}
	if out := qb.Arguments()[2]; out != 738 {
		t.Errorf("expecting query builder argument to be 738, got %d", out)
	}

	qb.WhereOr("a", "@>", 837)
	if out := qb.Query(); out != baseQuery+" WHERE (u = $2) AND (a @> $4) LIMIT $1 OFFSET $3" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 4 {
		t.Errorf("expecting query builder argument to be 4, got %d", out)
	}
	if out := qb.Arguments()[3]; out != 837 {
		t.Errorf("expecting query builder argument to be 837, got %d", out)
	}

	qb.WhereOr("z", "<@", "zzz", ColumnTypeArrayVarchar)
	if out := qb.Query(); out != baseQuery+" WHERE (u = $2) AND (a @> $4 OR z <@ array[$5::varchar]) LIMIT $1 OFFSET $3" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 5 {
		t.Errorf("expecting query builder argument to be 5, got %d", out)
	}
	if out := qb.Arguments()[4]; out != "zzz" {
		t.Errorf("expecting query builder argument to be zzz, got %s", out)
	}

	qb.WhereOr("x", "<@", nil)
	if out := qb.Query(); out != baseQuery+" WHERE (u = $2) AND (a @> $4 OR z <@ array[$5::varchar] OR x IS NULL) LIMIT $1 OFFSET $3" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 5 {
		t.Errorf("expecting query builder argument to be 5, got %d", out)
	}

	qb.Where("v", "<@", nil)
	if out := qb.Query(); out != baseQuery+" WHERE (u = $2 AND v IS NULL) AND (a @> $4 OR z <@ array[$5::varchar] OR x IS NULL) LIMIT $1 OFFSET $3" {
		t.Errorf("expecting query to be `%s`, got %s", baseQuery, out)
	}
	if out := len(qb.Arguments()); out != 5 {
		t.Errorf("expecting query builder argument to be 5, got %d", out)
	}

}

func TestMySQLBit(t *testing.T) {
	var (
		baseQuery = "SELECT * FROM a"
	)
	qb := NewQueryBuilder("mysql", baseQuery)

	qb.Value("key", "bit::0")
	qb.Value("key", "bit::1")

	if out := qb.Arguments()[0]; out != types.BitBool(false) {
		t.Errorf("expecting query builder argument to be BitBool(false), got %d", out)
	}

	if out := qb.Arguments()[1]; out != types.BitBool(true) {
		t.Errorf("expecting query builder argument to be BitBool(true), got %d", out)
	}
}
