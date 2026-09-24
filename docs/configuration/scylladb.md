# ScyllaDB Configuration

This guide covers how to configure ScyllaDB or Apache Cassandra for integration testing with tomato.

## Overview

The `scylladb` resource talks CQL, so it works with both ScyllaDB and Apache Cassandra (`type: cassandra` is an alias). Tomato creates the keyspace, runs your schema files, truncates tables between scenarios, and provides steps to seed and assert on data. See the [ScyllaDB step reference](../resources/scylladb.md) for all steps.

## Container Setup

### ScyllaDB

```yaml
containers:
  scylla:
    image: scylladb/scylla:6.2
    # Single shard and little memory: enough for tests and CI runners
    command: ["--smp", "1", "--memory", "512M", "--overprovisioned", "1", "--developer-mode", "1"]
    ports:
      - "9042/tcp"
    wait_for:
      type: port
      target: "9042"
      timeout: 120s
```

### Apache Cassandra

```yaml
containers:
  cassandra:
    image: cassandra:5
    env:
      MAX_HEAP_SIZE: 512M
      HEAP_NEWSIZE: 128M
    ports:
      - "9042/tcp"
    wait_for:
      type: port
      target: "9042"
      timeout: 180s
```

The port opens before the node serves CQL. The resource keeps retrying until a query succeeds (`ready_timeout`, 90s by default), so the port wait is enough.

## Resource Configuration

```yaml
resources:
  scylla:
    type: scylladb
    container: scylla
    options:
      keyspace: app
      schema:
        - ./fixtures/schema.cql
        - ./fixtures/reference-data.cql
      exclude:
        - countries
```

### Resource Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `keyspace` | string | - | Keyspace for unqualified table names. Also accepted as the resource's `database` field |
| `create_keyspace` | bool | `true` | Create `keyspace` if it doesn't exist, with `SimpleStrategy` |
| `replication_factor` | int | `1` | Replication factor used when creating the keyspace |
| `schema` | string or list | - | CQL files run once at startup, after the keyspace is created |
| `keyspaces` | list | `[keyspace]` | Keyspaces whose tables are truncated on reset |
| `tables` | list | - | If set, only these tables are truncated on reset |
| `exclude` | list | - | Tables never truncated; `table` or `keyspace.table` |
| `consistency` | string | `ONE` | Consistency level, e.g. `ONE`, `QUORUM`, `LOCAL_QUORUM` |
| `user` / `password` | string | - | Credentials for `PasswordAuthenticator` |
| `hosts` | list | - | Connect to `host:port` instead of a container |
| `ready_timeout` | duration | `90s` | How long to wait for CQL to accept queries |

## Schema Files

Schema files are plain CQL scripts. Statements are split on `;`, and semicolons inside strings, quoted identifiers, `$$` strings and comments are ignored.

```sql
-- fixtures/schema.cql
CREATE TABLE IF NOT EXISTS app.users (
    id uuid PRIMARY KEY,
    email text,
    created_at timestamp,
    tags set<text>
);

CREATE TABLE IF NOT EXISTS app.countries (
    code text PRIMARY KEY,
    name text
);

INSERT INTO app.countries (code, name) VALUES ('DE', 'Germany');
```

If your application creates its own schema on startup (for example with a migration tool), you can leave `schema` out and let the app do it.

## Seeding Data

`table ... has values:` inserts rows with `INSERT ... JSON`, so cells are written as text and CQL converts them to the column type (int, uuid, timestamp, boolean, ...). A cell holding a JSON array or object fills a list, set, map or UDT column, and `null` writes a null.

```gherkin
Given "scylla" table "users" has values:
  | id                                   | email          | created_at           | tags          |
  | 11111111-1111-4111-a111-111111111111 | alice@test.com | 2026-01-01T00:00:00Z | ["admin"]     |
```

## Assertions

CQL has no global row order, so `table ... contains:` and `query result of ... contains:` match rows in any order. `query ... returns:` compares rows in the order the query returns them. Use it with a `WHERE` on the partition key when order matters.

Values are compared as text: timestamps in RFC 3339 UTC (`2026-01-01T00:00:00Z`), UUIDs in canonical form, collections as JSON. An unset text or number column reads back as its zero value (`""`, `0`), as gocql reports it.

## Wiring the App

Pass the container address to your app like any other dependency:

```yaml
app:
  command: java -jar build/libs/app.jar
  env:
    SPRING_CASSANDRA_CONTACT_POINTS: "{{.scylla.host}}:{{.scylla.port.9042}}"
    SPRING_CASSANDRA_KEYSPACE_NAME: app
    SPRING_CASSANDRA_LOCAL_DATACENTER: datacenter1
```

## Hooks

CQL resources work with `sql` and `sql_file` hooks:

```yaml
hooks:
  before_all:
    - sql_file: ./fixtures/extra.cql
      resource: scylla
```
