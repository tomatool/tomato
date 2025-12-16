# Configuration Reference

Complete reference for `tomato.yml` configuration options.

## File Structure

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
    env: {}
    ports: []
    volumes: []
    depends_on: []
    wait_for: {}
    reset: {}

resources:              # Resource/handler definitions
  name:
    type: http|http-server|postgres|redis|kafka|websocket|websocket-server|shell
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
| `output` | string | `pretty` | Output format: `pretty`, `progress`, `junit` |
| `reset.level` | string | `scenario` | Reset level: `scenario`, `feature`, `run`, `none` |
| `reset.on_failure` | string | `reset` | On failure: `reset`, `keep` |

## App Configuration

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
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port}}/test"
    REDIS_URL: "redis://{{.redis.host}}:{{.redis.port}}"
```

### Template Variables

In `app.env`, you can use templates to inject container addresses:

| Template | Description |
|----------|-------------|
| `{{.container_name.host}}` | Container hostname (e.g., `localhost` or Docker network IP) |
| `{{.container_name.port}}` | Container's mapped port (dynamically assigned) |

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
    DB_PORT: "{{.postgres.port}}"
    DATABASE_URL: "postgres://testuser:testpass@{{.postgres.host}}:{{.postgres.port}}/testdb"
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
    reset:
      strategy: truncate
      exclude:
        - schema_migrations
```

### Wait Strategies

| Type | Description | Fields |
|------|-------------|--------|
| `port` | Wait for port to be ready | `target` |
| `log` | Wait for log message | `target` (regex) |
| `http` | Wait for HTTP endpoint | `method`, `path`, `port` |
| `exec` | Run command in container | `target` (command) |

### Reset Strategies

| Strategy | Description |
|----------|-------------|
| `truncate` | Truncate tables (Postgres) |
| `flush` | Flush database (Redis) |
| `delete_recreate` | Delete and recreate topics (Kafka) |
| `none` | No reset |

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
      headers:
        Authorization: Bearer token
```

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
      # Always exclude these tables from truncation
      exclude:
        - schema_migrations
        - goose_db_version
```

#### Reset Behavior

By default, PostgreSQL resources truncate **all tables** in the public schema before each scenario. You can control this behavior:

| Option | Description |
|--------|-------------|
| `tables` | If set, only these tables are truncated (instead of all) |
| `exclude` | Tables to never truncate (default: `schema_migrations`, `goose_db_version`) |

The `container` field automatically provides the connection details - tomato resolves the container's host and port at runtime.

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
```

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

### Hook Types

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

### Tag Expressions

| Expression | Description |
|------------|-------------|
| `@smoke` | Scenarios tagged with @smoke |
| `@smoke and @api` | Both tags |
| `@smoke or @api` | Either tag |
| `not @slow` | Exclude slow tests |
| `@smoke and not @wip` | Smoke tests, excluding WIP |

## Environment Variables

Use environment variables anywhere in the config:

```yaml
containers:
  postgres:
    image: postgres:${POSTGRES_VERSION:-15}
    env:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
```
