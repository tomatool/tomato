# 🍅 tomato - behavioral testing toolkit

<p align="center">
  <a href="https://github.com/tomatool/tomato/actions/workflows/ci.yml"><img src="https://github.com/tomatool/tomato/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/tomatool/tomato"><img src="https://goreportcard.com/badge/github.com/tomatool/tomato" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/tomatool/tomato"><img src="https://pkg.go.dev/badge/github.com/tomatool/tomato.svg" alt="Go Reference"></a>
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

Tomato is a language-agnostic behavioral testing framework that manages your test infrastructure automatically.

## Features

- **Built-in Container Orchestration** - Automatically manage test containers with Testcontainers
- **Clean State Testing** - Reset resources between scenarios for reliable, isolated tests
- **BDD/Gherkin Support** - Write tests in plain English using Cucumber syntax
- **Multiple Resource Types** - HTTP, PostgreSQL, Redis, Kafka, WebSocket, Shell
- **Auto-generated Documentation** - Keep step docs in sync with code
- **Application Runner** - Start your app connected to test containers

## Installation

```bash
# Quick Install
curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh

# Using Go
go install github.com/tomatool/tomato@latest

# Using Homebrew
brew install tomatool/tap/tomato
```

## Quick Start

```bash
tomato init    # Initialize a new project
tomato run     # Run behavioral tests
tomato steps   # List available steps
```

## Documentation

For complete documentation, visit **[tomatool.github.io/tomato](https://tomatool.github.io/tomato)**

- [Getting Started](https://tomatool.github.io/tomato/getting-started)
- [Configuration Reference](https://tomatool.github.io/tomato/configuration)
- [Architecture](https://tomatool.github.io/tomato/architecture)
- [Step Reference](https://tomatool.github.io/tomato/resources)

## License

Tomato is open source under the [MIT License](LICENSE).
