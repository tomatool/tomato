# GitHub Action

Run Tomato tests in your CI/CD pipeline using the official GitHub Action.

## Basic usage

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
| `verbose` | Show debug logs | `false` |
| `quiet` | Hide application logs | `false` |
| `skip-validate` | Skip configuration validation before running tests | `false` |
| `comment` | Post/update a test summary comment on the PR | `true` |

## Examples

### Filter by tags

```yaml
- uses: tomatool/tomato@v2
  with:
    tags: '@smoke and not @slow'
```

### Filter by scenario name

```yaml
- uses: tomatool/tomato@v2
  with:
    scenario: 'user registration'
```

### Run specific features

```yaml
- uses: tomatool/tomato@v2
  with:
    features: 'features/api features/auth'
```

### Verbose output

```yaml
- uses: tomatool/tomato@v2
  with:
    verbose: 'true'
```

### PR comment with test results

```yaml
- uses: tomatool/tomato@v2
  with:
    comment: 'true'
```

When enabled on a `pull_request` workflow, the action posts (or updates) a comment on the PR with a summary of test results — including pass/fail counts and failed scenario details.

> **Note:** The `comment` feature requires `pull-requests: write` permission. See the [complete workflow example](#complete-workflow-example) below.

### Publish a JUnit report

Write a JUnit report from `tomato.yml`, then upload it or hand it to a test
reporter. File outputs are kept when `comment` is enabled.

```yaml
# tomato.yml
settings:
  output: "pretty,junit:reports/tomato.xml"
```

```yaml
- uses: tomatool/tomato@v2

- name: Upload test report
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: tomato-report
    path: reports/
```

See [Reports](../configuration/index.md#reports) for all formats.

### Pin to specific version

```yaml
- uses: tomatool/tomato@v2.0.0
  with:
    config: 'tomato.yml'
```

### Try an unreleased commit

Used at a commit or a branch instead of a version tag, the action builds tomato
from that same commit (with Go, which it sets up), so a change can run in your
CI before it is released:

```yaml
- uses: tomatool/tomato@3f2c1e0d9b8a7f6e5d4c3b2a1f0e9d8c7b6a5f4e
  with:
    config: 'tomato.yml'
```

Setting `version` still installs that release instead.

## Complete workflow example

```yaml
name: Integration Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  pull-requests: write

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
