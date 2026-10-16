# 🍅 tomato - behavioral testing toolkit

<p align="center">
  <a href="https://github.com/tomatool/tomato/actions/workflows/ci.yml"><img src="https://github.com/tomatool/tomato/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/tomatool/tomato"><img src="https://goreportcard.com/badge/github.com/tomatool/tomato" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/tomatool/tomato"><img src="https://pkg.go.dev/badge/github.com/tomatool/tomato.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/tomatool/tomato" alt="License"></a>
</p>

<p align="center">
  <a href="https://www.youtube.com/watch?v=1zsAXQPFUiE" title="Tomato to keep your application healthy - watch on YouTube">
    <img src="https://img.youtube.com/vi/1zsAXQPFUiE/maxresdefault.jpg" alt="Tomato to keep your application healthy - a one-minute introduction on YouTube" width="720">
  </a>
  <br>
  <a href="https://www.youtube.com/watch?v=1zsAXQPFUiE">▶ Watch the one-minute introduction</a>
</p>

Tomato is a language-agnostic behavioral testing framework that manages your test infrastructure automatically.

## Features

- **Built-in Container Orchestration** - Automatically manage test containers with Testcontainers
- **Clean State Testing** - Every resource is reset before every scenario for reliable, isolated tests
- **BDD/Gherkin Support** - Write tests in plain English using Cucumber syntax
- **Multiple Resource Types** - HTTP client (with Server-Sent Events) and server, gRPC, PostgreSQL, ScyllaDB/Cassandra, Redis, Kafka, RabbitMQ, S3, WebSocket client and server, Shell, and a browser that uses the web UI in a real Chromium
- **Presets** - `preset: kafka` instead of a page of listener config, with MSK-style `AWS_MSK_IAM` authentication on the stock apache/kafka image
- **AWS identity** - an `aws` resource gives the app IRSA against an STS tomato serves
- **Avro + Schema Registry** - Publish and assert Kafka Avro messages as plain JSON
- **Web UI** - `tomato ui` browses the suite, draws each scenario's flow and the topology of what it touches, and runs scenarios from the browser
- **CI Reports** - JUnit XML and Cucumber JSON alongside console output
- **Step Coverage** - `tomato coverage` reports which resource steps your features use, with `--min` to gate CI
- **Auto-generated Documentation** - Keep step docs in sync with code
- **Application Runner** - Start your app connected to test containers

## Installation

### Quick install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh
```

### Homebrew

```bash
brew install tomatool/tap/tomato
```

### GitHub Actions

```yaml
- uses: tomatool/tomato@v2
```

See [GitHub Action documentation](https://tomatool.github.io/tomato/getting-started/github-action) for all options.

### Go install

```bash
go install github.com/tomatool/tomato@latest
```

### From source

Requires Go 1.26 or later.

```bash
git clone https://github.com/tomatool/tomato.git
cd tomato
go build -o tomato .
sudo mv tomato /usr/local/bin/
```

### Upgrading

```bash
tomato update
```

## Quick start

```bash
tomato init      # Initialize a new project
tomato validate  # Check tomato.yml and the feature files before running
tomato run       # Run behavioral tests
tomato ui        # Browse, visualize and run the suite in a web UI
tomato steps     # List available steps
tomato coverage  # Which steps your features use
```

## Documentation

For complete documentation, visit **[tomatool.github.io/tomato](https://tomatool.github.io/tomato)**

- [Documentation Map](https://tomatool.github.io/tomato/overview) - every page on one screen
- [Getting Started](https://tomatool.github.io/tomato/getting-started)
- [Writing a Suite](https://tomatool.github.io/tomato/guide/philosophy) - how to write scenarios worth trusting
- [GitHub Action](https://tomatool.github.io/tomato/getting-started/github-action)
- [Configuration Reference](https://tomatool.github.io/tomato/configuration)
- [Step Reference](https://tomatool.github.io/tomato/resources)
- [Architecture](https://tomatool.github.io/tomato/developer-guide/architecture)
- [Stability and versioning](https://tomatool.github.io/tomato/stability) - what is covered by semantic versioning, and the deprecation policy

Coming from tomato v1? v2 is a rewrite, not an upgrade in place: see
[Migrating from v1](https://tomatool.github.io/tomato/stability/#migrating-from-v1). The
[`v1` branch](https://github.com/tomatool/tomato/tree/v1) no longer receives updates.

## Contributing

See [CONTRIBUTING.md](https://github.com/tomatool/tomato/blob/main/CONTRIBUTING.md) for development setup and how to add steps and
resources, and [MAINTAINERS.md](https://github.com/tomatool/tomato/blob/main/MAINTAINERS.md) for who reviews and how to join.

## License

Tomato is open source under the [MIT License](https://github.com/tomatool/tomato/blob/main/LICENSE).
