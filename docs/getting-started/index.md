# Getting Started

This guide sets up tomato and runs your first behavioral test.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) — tomato starts your test containers
- [Go 1.26+](https://go.dev/dl/) — only for the `go install` and from-source methods

## Installation

### Quick install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh
```

### Using Go

```bash
go install github.com/tomatool/tomato@latest
```

### Using Homebrew

```bash
brew install tomatool/tap/tomato
```

### Verify installation

```bash
tomato --version
```

## Initialize a project

Create a new project with the init command:

```bash
mkdir my-project && cd my-project
tomato init
```

This creates:
- `tomato.yml` - Main configuration file
- `features/` - Directory for your feature files

## Project structure

```text
my-project/
├── tomato.yml           # Main configuration
├── features/
│   └── example.feature  # Gherkin test files
└── fixtures/            # Optional: SQL scripts, test data
```

## Configuration

Edit `tomato.yml` to define your test environment:

```yaml
version: 2

# Test settings
settings:
  timeout: 5m
  fail_fast: true
  reset:
    level: scenario  # Reset resources between scenarios

# Define containers to run
containers:
  postgres:
    image: postgres:15
    env:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: test
    wait_for:
      type: port
      target: "5432"

  redis:
    image: redis:7
    wait_for:
      type: port
      target: "6379"

# Define resources (handlers) for your tests
resources:
  db:
    type: postgres
    container: postgres
    database: test

  cache:
    type: redis
    container: redis

  api:
    type: http
    base_url: http://localhost:8080

# Feature file locations
features:
  paths:
    - ./features
  tags: "@smoke"  # Optional: filter by tags
```

## Writing tests

Create a feature file in `features/`:

```gherkin
# features/user_api.feature
Feature: User API

  Background:
    Given "db" table "users" has values:
      | id | name  | email          |
      | 1  | Alice | alice@test.com |

  Scenario: Get user by ID
    When "api" sends "GET" to "/users/1"
    Then "api" response status is "200"
    And "api" response json "name" is "Alice"

  Scenario: Create new user
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "Bob", "email": "bob@test.com"}
      """
    Then "api" response status is "201"
    And "db" table "users" has "2" rows
```

## Validate configuration

Before running tests, validate your configuration and feature files:

```bash
tomato validate
```

This checks:
- Configuration file syntax and structure
- Resource type validity
- Container references
- Feature file parsing
- Step definitions availability

Use `--plain` for CI environments without interactive output:

```bash
tomato validate --plain
```

## Running tests

Run all tests:

```bash
tomato run
```

Run with specific tags:

```bash
tomato run --tags "@smoke"
```

Run with verbose output:

```bash
tomato run -v
```

## Testing your application

Tomato can also start your application and connect it to test containers:

```yaml
# tomato.yml
app:
  command: go run ./cmd/server
  port: 8080
  ready:
    type: http
    path: /health
  wait: 5s
  env:
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port.5432}}/test"
    REDIS_URL: "redis://{{.redis.host}}:{{.redis.port.6379}}"
```

The `{{.container.host}}` and `{{.container.port.<port>}}` templates are replaced
with the container's address and its mapped host port. The number is the port
*inside* the container, so `{{.postgres.port.5432}}` resolves to whichever host
port Docker mapped 5432 to. A bare `{{.postgres.port}}` is left untouched.

## Next steps

- [Configuration Reference](../configuration/index.md) - Full configuration options
- [Resources](../resources/index.md) - All available step definitions
- [Examples](https://github.com/tomatool/tomato/tree/main/examples) - Sample projects
