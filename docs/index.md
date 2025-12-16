---
layout: default
title: Home
nav_order: 1
permalink: /
---

# Tomato

**Behavioral testing toolkit with built-in container orchestration.**

One config to rule them all.
{: .fs-6 .fw-300 }

[Get Started]({% link getting-started.md %}){: .btn .btn-primary .fs-5 .mb-4 .mb-md-0 .mr-2 }
[View on GitHub](https://github.com/tomatool/tomato){: .btn .fs-5 .mb-4 .mb-md-0 }

---

## What is Tomato?

Tomato is a language-agnostic behavioral testing framework that manages your test infrastructure automatically. Define containers, resources, and tests in a single `tomato.yml` file.

### Key Features

- **Built-in Container Orchestration** - Automatically manage test containers with Testcontainers
- **Clean State Testing** - Reset resources between scenarios for reliable tests
- **BDD/Gherkin Support** - Write tests in plain English using Cucumber syntax
- **Multiple Resource Types** - HTTP, PostgreSQL, Redis, Kafka, WebSocket, Shell
- **Auto-generated Documentation** - Keep docs in sync with code

## Quick Example

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

features:
  paths:
    - ./features
```

```gherkin
# features/users.feature
Feature: User Management

  Scenario: Create a new user
    Given I set "db" table "users" with values:
      | id | name  | email          |
      | 1  | John  | john@test.com  |
    Then "db" table "users" should have "1" rows
```

```bash
# Run tests
tomato run
```

## Installation

### Quick Install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh
```

### Homebrew

```bash
brew install tomatool/tap/tomato
```

### GitHub Actions

```yaml
- uses: tomatool/tomato@v1
```

See [GitHub Action]({% link github-action.md %}) for full documentation.

### Go Install

```bash
go install github.com/tomatool/tomato@latest
```

### From Source

```bash
git clone https://github.com/tomatool/tomato.git
cd tomato
go build -o tomato .
sudo mv tomato /usr/local/bin/
```

## Philosophy

Tomato follows these principles:

1. **Single Source of Truth** - One `tomato.yml` file defines everything
2. **Reset by Default** - Each scenario starts with a clean slate
3. **Language Agnostic** - Test any application regardless of implementation language
4. **Developer Experience** - Clear output, helpful errors, fast feedback
