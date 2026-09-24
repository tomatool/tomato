# Changelog

All notable changes to tomato are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and tomato follows the
versioning and deprecation rules in [docs/stability.md](docs/stability.md).

Entries up to v2.1.1 were backfilled from the GitHub release notes.

## [Unreleased]

### Added
- `grpc-server` resource: stub gRPC dependencies from `.proto` files or a protoset (no codegen), assert on requests and metadata; serves reflection.
- `scylladb` / `cassandra` resource: CQL execution, table seeding and assertions, keyspace bootstrap, per-scenario truncate. ([#150](https://github.com/tomatool/tomato/pull/150))
- Kafka Avro via Confluent Schema Registry: publish and assert Avro messages as plain JSON. ([#152](https://github.com/tomatool/tomato/pull/152))
- Stability and deprecation policy (`docs/stability.md`), CONTRIBUTING, MAINTAINERS, CODEOWNERS, and issue and PR templates.
- `StepDef.Deprecated`: deprecated steps keep working, warn once per run, and are labelled in generated docs.
- `tomato validate` warns when `tomato.yml` has no `version` field.
- CI reports: file outputs such as `junit:reports/tomato.xml` get their directories created, and are kept when `--format` is overridden (for example by the GitHub Action's PR comment). ([#149](https://github.com/tomatool/tomato/pull/149))
- HTTP: the `Host` header is honoured, plus steps for cookies and docstring request bodies.

### Fixed
- `grpc` `response status is` accepts canonical status names (`NOT_FOUND`), not only `NotFound`.

### Changed
- A config with an unsupported `version`, or a v1-style config, is rejected with a pointer to the migration guide instead of a YAML decoding error.
- The app runner refuses to start when `app.port` is already in use, and the startup failure screen prints the reason. ([#149](https://github.com/tomatool/tomato/pull/149))
- Postgres reset never truncates Flyway (`flyway_schema_history`) or Liquibase (`databasechangelog`, `databasechangeloglock`) history tables. ([#149](https://github.com/tomatool/tomato/pull/149))
- `tomato init` generates an `http-server` resource for external HTTP mocks and no longer offers MySQL. ([#149](https://github.com/tomatool/tomato/pull/149))

### Removed
- The `mysql` and `wiremock` resource types, which were no-op stubs. Configs using them now fail with a hint. ([#149](https://github.com/tomatool/tomato/pull/149))

## [2.1.1] - 2026-09-21

### Fixed
- `grpc` is accepted by `tomato validate`; valid resource types are derived from a single table.

## [2.1.0] - 2026-09-21

### Added
- `grpc` resource for unary calls, using server reflection.
- `s3` resource for MinIO, LocalStack and AWS.
- Postgres query result assertion steps.
- Variable interpolation in Postgres steps.

### Fixed
- CI coverage comment no longer fails on pull requests from forks.

### Security
- Bumped `go.opentelemetry.io/otel/sdk` from 1.39.0 to 1.40.0.

## [2.0.9] - 2026-03-03

### Added
- GitHub Action PR comment shows duration, steps and the scenario list.

### Fixed
- GitHub Action reads the correct JSON field for tomato event types.

## [2.0.8] - 2026-03-02

### Added
- GitHub Action posts a PR comment and a job summary with test results.

## [2.0.7] - 2026-03-02

Same changes as 2.0.6, re-released.

## [2.0.6] - 2026-03-02

### Added
- Kafka integration support.
- `rabbitmq` resource.
- HTTP server handler tests and resource URL templating (`{{.resource.url}}`).

### Fixed
- Postgres table assertions compare UUIDs stored as byte arrays correctly.
- Multi-line step examples render correctly in the generated resource docs.

### Security
- Dependency updates for known vulnerabilities.

## [2.0.5] - 2025-12-17

### Added
- App runner can run the application as a local process or in a container.
- Docker availability checks and an improved release workflow.

## [2.0.4] - 2025-12-17

### Added
- GitHub Action validates the config before running tests.

### Changed
- GitHub Action renamed from `tomato` to `tomato-run` on the Marketplace.

## [2.0.3] - 2025-12-16

### Added
- `tomato validate` for config and feature files.
- Web UI (`tomato ui`) and the `tomato` structured output format.

### Fixed
- Feature file discovery and step pattern matching.
- `tomato validate` uses the handler package for valid resource types.

## [2.0.2] - 2025-12-16

### Added
- GitHub Action (`tomatool/tomato@v2`).

## [2.0.1] - 2025-12-16

Same changes as 2.0.2, first release of the GitHub Action.

## [2.0.0] - 2025-12-16

First v2 release: a rewrite with container orchestration via Testcontainers, a
single `tomato.yml`, and resources for HTTP (client and server), PostgreSQL, Redis,
Kafka, WebSocket (client and server) and shell.
v1 lives on the [`v1` branch](https://github.com/tomatool/tomato/tree/v1) and is no
longer updated.

### Fixed
- Go toolchain requirement lowered from 1.25 to 1.24.

[Unreleased]: https://github.com/tomatool/tomato/compare/v2.1.1...HEAD
[2.1.1]: https://github.com/tomatool/tomato/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/tomatool/tomato/compare/v2.0.9...v2.1.0
[2.0.9]: https://github.com/tomatool/tomato/compare/v2.0.8...v2.0.9
[2.0.8]: https://github.com/tomatool/tomato/compare/v2.0.7...v2.0.8
[2.0.7]: https://github.com/tomatool/tomato/compare/v2.0.6...v2.0.7
[2.0.6]: https://github.com/tomatool/tomato/compare/v2.0.5...v2.0.6
[2.0.5]: https://github.com/tomatool/tomato/compare/v2.0.4...v2.0.5
[2.0.4]: https://github.com/tomatool/tomato/compare/v2.0.3...v2.0.4
[2.0.3]: https://github.com/tomatool/tomato/compare/v2.0.2...v2.0.3
[2.0.2]: https://github.com/tomatool/tomato/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/tomatool/tomato/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/tomatool/tomato/releases/tag/v2.0.0
