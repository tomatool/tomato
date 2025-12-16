---
layout: default
title: GitHub Action
nav_order: 5
---

# GitHub Action

Run Tomato tests in your CI/CD pipeline using the official GitHub Action.

## Basic Usage

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Tomato Tests
        uses: tomatool/tomato@v2
```

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `version` | Tomato version (e.g., `v1.0.0`) | Action's tag version |
| `config` | Config file path | `tomato.yml` |
| `features` | Feature files or directories | |
| `tags` | Filter by tags (e.g., `@smoke and not @slow`) | |
| `scenario` | Filter by scenario name (regex) | |
| `no-reset` | Skip state reset between scenarios | `false` |
| `verbose` | Show debug logs | `false` |
| `quiet` | Hide application logs | `false` |

## Examples

### Filter by Tags

```yaml
- uses: tomatool/tomato@v2
  with:
    tags: '@smoke and not @slow'
```

### Filter by Scenario Name

```yaml
- uses: tomatool/tomato@v2
  with:
    scenario: 'user registration'
```

### Run Specific Features

```yaml
- uses: tomatool/tomato@v2
  with:
    features: 'features/api features/auth'
```

### Verbose Output

```yaml
- uses: tomatool/tomato@v2
  with:
    verbose: 'true'
```

### Pin to Specific Version

```yaml
- uses: tomatool/tomato@v2.0.0
  with:
    config: 'tomato.yml'
```

## Complete Workflow Example

```yaml
name: Integration Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Run Tomato Tests
        uses: tomatool/tomato@v2
        with:
          config: 'tomato.yml'
          verbose: 'true'
```

## Requirements

- Docker must be available in the runner (default for `ubuntu-latest`)
- Repository must contain a valid `tomato.yml` configuration
