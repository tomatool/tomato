<p align="center">
  <h1 align="center">Tomato</h1>
  <p align="center">
    <strong>Behavioral testing toolkit with built-in container orchestration</strong>
  </p>
  <p align="center">
    <em>One config to rule them all.</em>
  </p>
</p>

<p align="center">
  <a href="https://github.com/tomatool/tomato/actions/workflows/ci.yml"><img src="https://github.com/tomatool/tomato/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/tomatool/tomato/releases"><img src="https://img.shields.io/github/v/release/tomatool/tomato" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/tomatool/tomato"><img src="https://goreportcard.com/badge/github.com/tomatool/tomato" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/tomatool/tomato" alt="License"></a>
</p>

---

> [!WARNING]
> **v2 is currently under active development.** This version contains breaking changes and may not be stable.
>
> Looking for the stable version? [**v1 is available here**](https://github.com/tomatool/tomato/tree/v1), but will not receive future updates as development focuses on v2.
>
> **Use v2 at your own risk** - APIs and features may change without notice.

---

Tomato is a language-agnostic behavioral testing framework that manages your test infrastructure automatically. Define containers, resources, and tests in a single `tomato.yml` file.

## Features

- **Built-in Container Orchestration** - Automatically manage test containers with Testcontainers
- **Clean State Testing** - Reset resources between scenarios for reliable, isolated tests
- **BDD/Gherkin Support** - Write tests in plain English using Cucumber syntax
- **Multiple Resource Types** - HTTP, PostgreSQL, Redis, Kafka, WebSocket, Shell
- **Auto-generated Documentation** - Keep step docs in sync with code
- **Application Runner** - Start your app connected to test containers

## Installation

### Quick Install

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

## Quick Start

### Initialize a new project

```bash
tomato init
```

### Configure your test environment

```yaml
# tomato.yml
version: 2

containers:
  postgres:
    image: postgres:15
    env:
      POSTGRES_PASSWORD: test
    wait_for:
      type: port
      target: "5432"

resources:
  db:
    type: postgres
    container: postgres
    database: test

  api:
    type: http
    base_url: http://localhost:8080

features:
  paths:
    - ./features
```

### Write your tests

```gherkin
# features/users.feature
Feature: User Management

  Scenario: Create and retrieve a user
    Given I set "db" table "users" with values:
      | id | name  | email          |
      | 1  | John  | john@test.com  |
    When I send "GET" request to "api" "/users/1"
    Then "api" response status should be "200"
    And "api" response JSON "name" should be "John"
```

### Run tests

```bash
tomato run
```

## Documentation

- [Getting Started](https://tomatool.github.io/tomato/getting-started)
- [Configuration Reference](https://tomatool.github.io/tomato/configuration)
- [Step Reference](https://tomatool.github.io/tomato/steps)

## Available Steps

Tomato provides steps for:

| Handler | Description |
|---------|-------------|
| **HTTP** | Send requests, validate responses, check JSON paths |
| **PostgreSQL** | Insert data, query tables, execute SQL |
| **Redis** | Set/get keys, work with hashes, lists, sets |
| **Kafka** | Publish/consume messages, validate ordering |
| **WebSocket** | Connect, send/receive messages |
| **Shell** | Execute commands, check output |

List all available steps:

```bash
tomato steps
```

Filter by type:

```bash
tomato steps --type http
tomato steps --filter json
```

## Commands

| Command | Description |
|---------|-------------|
| `tomato init` | Initialize a new tomato project |
| `tomato run` | Run behavioral tests |
| `tomato new <name>` | Create a new feature file |
| `tomato steps` | List available Gherkin steps |
| `tomato docs` | Generate step documentation |

## Testing Your Application

Tomato can start your application and connect it to test containers:

```yaml
app:
  command: go run ./cmd/server
  port: 8080
  ready:
    type: http
    path: /health
  wait: 5s
  env:
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port}}/test"
```

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

Tomato is open source under the [MIT License](LICENSE).
