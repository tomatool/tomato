# Getting Started

This guide will help you set up Tomato and run your first behavioral test.

## Prerequisites

- [Go 1.24+](https://go.dev/dl/) (for installation)
- [Docker](https://docs.docker.com/get-docker/) (for running containers)

## Installation

### Quick Install (Recommended)

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

### Verify Installation

```bash
tomato --version
```

## Initialize a Project

Create a new project with the init command:

```bash
mkdir my-project && cd my-project
tomato init
```

This creates:
- `tomato.yml` - Main configuration file
- `features/` - Directory for your feature files

## Project Structure

```
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

## Writing Tests

Create a feature file in `features/`:

```gherkin
# features/user_api.feature
Feature: User API

  Background:
    Given I set "db" table "users" with values:
      | id | name  | email          |
      | 1  | Alice | alice@test.com |

  Scenario: Get user by ID
    When I send "GET" request to "api" "/users/1"
    Then "api" response status should be "200"
    And "api" response JSON "name" should be "Alice"

  Scenario: Create new user
    When I set "api" JSON body:
      """
      {"name": "Bob", "email": "bob@test.com"}
      """
    And I send "POST" request to "api" "/users"
    Then "api" response status should be "201"
    And "db" table "users" should have "2" rows
```

## Running Tests

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

## Testing Your Application

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
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port}}/test"
    REDIS_URL: "redis://{{.redis.host}}:{{.redis.port}}"
```

The `{{.container.host}}` and `{{.container.port}}` templates are replaced with actual container addresses.

## Next Steps

- [Configuration Reference](configuration.md) - Full configuration options
- [Resources](resources/index.md) - All available step definitions
- [Examples](https://github.com/tomatool/tomato/tree/main/examples) - Sample projects
