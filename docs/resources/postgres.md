# PostgreSQL

Steps for interacting with PostgreSQL databases

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Data Setup

| Step | Description |
|------|-------------|
| `"app-db" table "users" has values:` | Insert rows from table |
| `"app-db" clears table "users"` | Truncate a table (removes all rows) |
| `"app-db" clears tables:` | Truncate multiple tables from list |
| `"app-db" executes:` | Execute raw SQL |
| `"app-db" executes file "fixtures/seed.sql"` | Execute SQL from file |



## Assertions

| Step | Description |
|------|-------------|
| `"app-db" table "users" contains:` | Assert table contains rows |
| `"app-db" table "users" is empty` | Assert table is empty |
| `"app-db" table "users" has "5" rows` | Assert row count |
| `"app-db" query "SELECT id, name FROM users" returns:` | Assert exact match of query result rows |
| `"app-db" query "SELECT status FROM orders" returns within "10s":` | Waits until the query result matches the rows exactly, for state the app writes asynchronously |
| `"app-db" query result of "SELECT id, name FROM users" contains:` | Assert query result contains expected rows (superset) |


