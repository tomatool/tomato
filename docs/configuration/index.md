# Configuration reference

Complete reference for `tomato.yml` configuration options.

## File structure

```yaml
version: 2              # Required: config version

settings:               # Test execution settings
  timeout: 5m
  parallel: 1
  fail_fast: false
  output: pretty
  reset:
    level: scenario
    on_failure: reset

app:                    # Application under test (optional)
  command: ./my-app
  port: 8080
  ready:
    type: http
    path: /health
  wait: 5s
  env:
    KEY: value

containers:             # Container definitions
  name:
    image: image:tag
    command: []         # Optional startup command (list or string)
    env: {}
    ports: []
    volumes: []
    depends_on: []
    wait_for: {}

resources:              # Resource/handler definitions
  name:
    type: http|http-server|grpc|postgres|scylladb|cassandra|redis|kafka|rabbitmq|s3|websocket|websocket-server|shell|aws
    # aliases: postgresql, http-client, grpc-client, websocket-client, minio
    container: container_name
    options: {}

hooks:                  # Lifecycle hooks
  before_all: []
  after_all: []
  before_scenario: []
  after_scenario: []

features:               # Feature file settings
  paths:
    - ./features
  tags: ""
```

## Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `5m` | Global test timeout |
| `parallel` | int | `1` | Number of parallel scenarios |
| `fail_fast` | bool | `false` | Stop on first failure |
| `output` | string | `pretty` | Output formats, comma-separated; see [Reports](#reports) |
| `reset.level` | string | `scenario` | Reset level: `scenario`, `feature`, `run`, `none` |
| `reset.on_failure` | string | `reset` | On failure: `reset`, `keep` |

### Reports

`output` takes one or more comma-separated formats. A format without a path
prints to the console; `format:path` writes that format to a file. Use this to
feed CI test reports and test-management tools.

```yaml
settings:
  output: "pretty,junit:reports/tomato.xml,cucumber:reports/cucumber.json"
```

| Format | Writes |
|--------|--------|
| `pretty` | Human-readable console output (default) |
| `progress` | One character per step |
| `junit` | JUnit XML, one `testsuite` per feature and one `testcase` per scenario |
| `cucumber` | Cucumber JSON, the input most Cucumber report tools import |
| `tomato` | Structured events used by the GitHub Action's PR comment |

Tomato creates the report directories if they don't exist. When `--format` is
passed on the command line (the GitHub Action passes `--format tomato` for PR
comments), it replaces the console format but file outputs from `output` are
still written.

## App configuration

Configure your application under test to run with test containers.

```yaml
app:
  # Option 1: Run a command
  command: go run ./cmd/server
  workdir: ./

  # Option 2: Build from Dockerfile
  build:
    dockerfile: Dockerfile
    context: .

  # Connection settings
  port: 8080
  ready:
    type: http          # http, tcp, or exec
    path: /health       # For HTTP checks
    status: 200         # Expected status (default: 200)
    timeout: 30s        # Ready check timeout

  # Wait after ready check passes
  wait: 5s

  # Environment variables (supports container templates)
  env:
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port.5432}}/test"
    REDIS_URL: "redis://{{.redis.host}}:{{.redis.port.6379}}"
```

!!! note "Port already in use"
    In command mode, tomato checks `port` before starting the app. If another
    process is already listening there (a local `make run`, a leftover test
    run), tomato fails instead of starting: otherwise the ready check would
    pass against the other process and the tests would hit the wrong app.

In command mode the app inherits tomato's environment, plus `env`. Some
resources the app depends on while it starts are set up before it and add to
that environment too: the [aws](aws.md) resource points the app's AWS SDK at its
STS and removes `AWS_ACCESS_KEY_ID` and friends inherited from your shell. The
app's own `env` still wins over anything a resource sets.

The app runs in its own process group, and stopping it stops the whole group,
so `command: go run ./cmd/server` (where `go` does not pass signals on) does not
leave the server running and holding the port.

### Template variables

In `app.env`, you can use templates to inject container addresses:

| Template | Description |
|----------|-------------|
| `{{.container_name.host}}` | Container hostname (`localhost` in command mode, the container's DNS name in container mode) |
| `{{.container_name.port.NNNN}}` | Host port that `NNNN`, the port inside the container, is mapped to |
| `{{.resource_name.url}}` | Base URL of an `http-server` resource |

`NNNN` is required. A bare `{{.container_name.port}}` is left in the value
verbatim, so the application receives the literal template text.

**Example with PostgreSQL:**
```yaml
containers:
  postgres:
    image: postgres:15
    env:
      POSTGRES_USER: testuser
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432"

app:
  command: ./my-app
  env:
    # These are resolved at runtime when containers start
    DB_HOST: "{{.postgres.host}}"
    DB_PORT: "{{.postgres.port.5432}}"
    DATABASE_URL: "postgres://testuser:testpass@{{.postgres.host}}:{{.postgres.port.5432}}/testdb"
```

**Note:** The `container` field in resource definitions automatically handles host/port resolution - you don't need to specify connection strings manually for resources.

## Containers

Define Docker containers for your test infrastructure.

```yaml
containers:
  postgres:
    image: postgres:15
    env:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: test
    ports:
      - "5432"
    volumes:
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    depends_on:
      - other_container
    wait_for:
      type: port
      target: "5432"
      timeout: 30s
```

`volumes` are `source:target[:mode]`. A source path (`./x`, `../x`, `/x`, `~/x`)
is bound into the container, relative paths resolved against the directory of
`tomato.yml`; a bare name is a Docker volume. `build` (`context`, `dockerfile`)
builds the image instead of pulling one, with `context` relative to `tomato.yml`
too. The image is named `tomato-<container>` and kept, so the next run rebuilds
from cache into the same image. Each container's output is written to `.tomato/runs/<run>/container-<name>.log`
for as long as it runs, including when it fails to start.

### Presets

A preset is a container tomato knows how to run. It fills in the image, env,
ports and wait strategy; anything the entry sets itself (an `env` key, `image`,
`wait_for`) wins.

```yaml
containers:
  kafka:
    preset: kafka
    auth: aws_msk_iam   # optional: plaintext (default) or aws_msk_iam
```

| Preset | What it runs |
|--------|--------------|
| `kafka` | A single-node KRaft broker, reachable from the host and from other containers. With `auth: aws_msk_iam`, a second listener speaks SASL `AWS_MSK_IAM` like MSK's IAM port. See [Kafka](kafka.md#preset). |

### Wait strategies

| Type | Description | Fields |
|------|-------------|--------|
| `port` | Wait for port to be ready | `target` |
| `log` | Wait for log message | `target` (regex) |
| `http` | Wait for HTTP endpoint | `path`, `target` (port), `method` |
| `exec` | Run command in container | `target` (command) |

### Resetting state

Resetting is configured per resource, not per container — see
`options.reset_strategy` under [Resources](#resources) and on each resource's
page. `settings.reset.level` controls how often it runs.

## Resources

Define handlers for test steps.

### HTTP

```yaml
resources:
  api:
    type: http
    base_url: http://localhost:8080
    options:
      timeout: 30s
```

| Option | Default | Description |
|--------|---------|-------------|
| `timeout` | `30s` | Per-request timeout |
| `no_redirect` | `false` | Return the redirect response instead of following it |
| `port` | `8080` | Container port the base URL is built from. Only with `container` |
| `scheme` | `http` | Scheme for the base URL built from `container` |
| `health_path` | *(none)* | Path polled once at startup to check the service answers |

Request headers are set by steps (`"api" header "Authorization" is "..."`), not
here, and are cleared between scenarios.

### PostgreSQL

```yaml
resources:
  db:
    type: postgres
    container: postgres
    database: test
    options:
      user: test
      password: test
      # Only truncate these tables during reset (if not set, truncates ALL public tables)
      tables:
        - users
        - orders
        - products
      # Never truncate these tables (added to the defaults below)
      exclude:
        - countries        # reference data seeded by a migration
```

#### Reset behavior

By default, PostgreSQL resources truncate **all tables** in the public schema before each scenario. You can control this behavior:

| Option | Description |
|--------|-------------|
| `tables` | If set, only these tables are truncated (instead of all) |
| `exclude` | Extra tables to never truncate, on top of the defaults |

Unless `tables` is set, migration history tables are never truncated, so
migration tools still see their migrations as applied:

| Tool | Tables |
|------|--------|
| golang-migrate, Rails, sqlx | `schema_migrations` |
| goose | `goose_db_version` |
| Flyway | `flyway_schema_history` |
| Liquibase | `databasechangelog`, `databasechangeloglock` |

Reference data that migrations insert (countries, roles, permissions) is
truncated like any other table. Add those tables to `exclude` so scenarios
don't have to re-seed them.

`tables` and `exclude` are mutually exclusive: when `tables` is set, exactly
those tables are truncated and both `exclude` and the migration-table list
above are ignored.

The `container` field automatically provides the connection details - tomato resolves the container's host and port at runtime.

### ScyllaDB / Cassandra

```yaml
resources:
  scylla:
    type: scylladb          # or: cassandra
    container: scylla
    options:
      keyspace: app         # created if missing, then used for unqualified tables
      schema:
        - ./fixtures/schema.cql
      exclude:
        - countries         # reference data kept across scenarios
```

Every table in the keyspace is truncated before each scenario, except those in
`exclude`. See [ScyllaDB Configuration](scylladb.md) for all options.

### Redis

```yaml
resources:
  cache:
    type: redis
    container: redis
    options:
      db: 0
      password: ""
      reset_strategy: flush  # flush or pattern
      reset_pattern: "*"     # For pattern strategy
```

### Kafka

```yaml
resources:
  kafka:
    type: kafka
    container: kafka
    options:
      topics:
        - events
        - notifications
      partitions: 1
      replication_factor: 1
      reset_strategy: delete_recreate
      # Optional: Avro via Confluent Schema Registry
      schema_registry:
        container: schema-registry   # or url: http://localhost:8081
```

See [Kafka: Avro and Schema Registry](kafka.md#avro-and-schema-registry).

### WebSocket

```yaml
resources:
  ws:
    type: websocket
    url: ws://localhost:8080/ws
    # Or use container
    container: app
    options:
      port: "8080"
      path: /ws
      protocols:
        - graphql-ws
      headers:
        Authorization: Bearer token
```

### gRPC

```yaml
resources:
  grpc:
    type: grpc
    address: "localhost:9090"   # or: container: myservice
    options:
      timeout: 30s
```

The server must register the gRPC reflection service. See
[gRPC Configuration](grpc.md).

### S3

```yaml
resources:
  files:
    type: s3
    container: minio
    options:
      buckets:
        - uploads
      reset_strategy: purge   # purge, delete, or none
```

See [S3 Configuration](s3.md) for MinIO and LocalStack setup.

### Shell

```yaml
resources:
  shell:
    type: shell
    options:
      timeout: 30s
      workdir: ./scripts
      env:
        PATH: /usr/local/bin
```

## Hooks

Execute actions at different lifecycle points.

```yaml
hooks:
  before_all:
    - sql_file: ./fixtures/schema.sql
      resource: db
    - shell: ./scripts/setup.sh

  after_all:
    - shell: ./scripts/cleanup.sh

  before_scenario:
    - sql: "DELETE FROM events"
      resource: db

  after_scenario:
    - exec: redis-cli FLUSHDB
      container: redis
```

### Hook types

| Type | Description |
|------|-------------|
| `sql` | Execute SQL query |
| `sql_file` | Execute SQL file |
| `shell` | Run shell command |
| `exec` | Run command in container |

## Features

Configure feature file discovery.

```yaml
features:
  paths:
    - ./features
    - ./integration/features
  tags: "@smoke and not @slow"
```

### Tag expressions

| Expression | Description |
|------------|-------------|
| `@smoke` | Scenarios tagged with @smoke |
| `@smoke and @api` | Both tags |
| `@smoke or @api` | Either tag |
| `not @slow` | Exclude slow tests |
| `@smoke and not @wip` | Smoke tests, excluding WIP |
| `(@smoke or @api) and not @slow` | Parentheses group terms |

`not` binds tighter than `and`, which binds tighter than `or`. godog's legacy
syntax (`@smoke && ~@slow`, `,` for or) is still accepted.

## Environment variables

Use environment variables anywhere in the config:

```yaml
containers:
  postgres:
    image: postgres:${POSTGRES_VERSION}
    env:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
```

Only `$VAR` and `${VAR}` are expanded. Shell default syntax such as
`${VAR:-15}` is not supported: the whole `VAR:-15` is read as the variable
name, so the value expands to an empty string. Set the variable, or keep the
default in `tomato.yml`.
