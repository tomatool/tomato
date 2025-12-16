# PostgreSQL

Steps for interacting with PostgreSQL databases


## Data Setup

| Step | Description |
|------|-------------|
| `"db" table "users" has values:` | Insert rows from table |
| `"db" clears table "users"` | Truncate a table (removes all rows) |
| `"db" clears tables:` | Truncate multiple tables from list |
| `"db" executes:` | Execute raw SQL |
| `"db" executes file "fixtures/seed.sql"` | Execute SQL from file |


## Assertions

| Step | Description |
|------|-------------|
| `"db" table "users" contains:` | Assert table contains rows |
| `"db" table "users" is empty` | Assert table is empty |
| `"db" table "users" has "5" rows` | Assert row count |

