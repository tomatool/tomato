# ScyllaDB

Steps for interacting with ScyllaDB and Apache Cassandra over CQL

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Data Setup

| Step | Description |
|------|-------------|
| `"scylla" table "users" has values:` | Insert rows from table (values are converted to the column types) |
| `"scylla" clears table "users"` | Truncate a table (removes all rows) |
| `"scylla" executes:` | Execute CQL (several statements separated by `;`) |
| `"scylla" executes file "fixtures/schema.cql"` | Execute CQL from file |



## Assertions

| Step | Description |
|------|-------------|
| `"scylla" table "users" contains:` | Assert table contains rows (in any order) |
| `"scylla" table "users" is empty` | Assert table is empty |
| `"scylla" table "users" has "5" rows` | Assert row count |
| `"scylla" query "SELECT id, name FROM users WHERE id = 1" returns:` | Assert exact match of query result rows |
| `"scylla" query result of "SELECT id, name FROM users" contains:` | Assert query result contains expected rows (superset) |


