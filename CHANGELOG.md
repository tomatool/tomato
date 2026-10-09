# Changelog

All notable changes to tomato are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and tomato follows the
versioning and deprecation rules in [docs/stability.md](docs/stability.md).

Entries up to v2.1.1 were backfilled from the GitHub release notes.

## [Unreleased]

### Security
- Every open Dependabot alert patched: `google.golang.org/grpc` v1.83.2 (authorization bypass via a missing leading slash in `:path`, xDS crash and RBAC bypasses, HTTP/2 memory exhaustion), `github.com/rabbitmq/amqp091-go` v1.15.0 (frame injection and protocol desynchronization, plaintext credential exposure, several DoS paths), `golang.org/x/crypto` v0.57.0 (SSH authorization and certificate-constraint bypasses, DoS), `golang.org/x/net` v0.59.0 (HTML parser DoS), `github.com/moby/go-archive` v0.3.3 (tar extraction outside the target directory), and the OpenTelemetry modules at v1.46.0 (baggage-header allocation DoS).
- `github.com/docker/docker` left the dependency graph entirely — its four alerts have no patched release on that module path, which Docker 29 abandoned. testcontainers-go v0.44.0 and tomato use the `github.com/moby/moby/api` and `github.com/moby/moby/client` modules instead.
- The Go toolchain is pinned to 1.26.6, which carries the standard-library fixes govulncheck flags in earlier 1.26 releases. Building tomato now needs Go 1.26.

### Added
- `tomato ui`: each scenario has a Flow view and a Sequence view besides its steps. Flow shows tomato, the app and every resource in `tomato.yml`; each Next draws the step's line from tomato to what it acts on, and after a When on the app, dashed lines to the effects the scenario expects of it. Hovering a line shows its steps with their payloads: doc strings and tables. Sequence draws each step as an arrow to its resource. Background steps are included, and a step no step definition matches is flagged before the run.
- Agent skill (`skills/tomato`, agentskills.io format) for teams testing their service with tomato: a coding agent sets up a suite, writes and reviews scenarios, debugs runs, and follows a protocol when tomato lacks a step or resource, with the project's principles as hard rules. Contributors to tomato have a separate skill in `.claude/skills/tomato-dev`. See [Agent Skill](docs/getting-started/agent-skill.md).
- `kafka` container preset (`preset: kafka`): a single-node KRaft broker with its host port picked and advertised, reachable from the host and from other containers; `auth: aws_msk_iam` adds a listener speaking SASL `AWS_MSK_IAM` like MSK's IAM port. Release builds pull `ghcr.io/tomatool/tomato-kafka:<version>`, development builds build it from the binary.
- `aws` resource: IRSA for the app under test. tomato serves STS (`AssumeRoleWithWebIdentity`, `AssumeRole`, `GetCallerIdentity`), writes the web identity token and credential files, points the app's SDK at them and drops inherited `AWS_*` credentials; `ambient_identity` adds the fallback identity a credential chain ends up with when the role cannot be had. An `AWS_MSK_IAM` preset listener lets in only the role sessions it issues. Steps: `role "..." was assumed` / `was not assumed`.
- Resources that the app needs while it starts (`AppEnvProvider`) are initialized before it and add to its environment.
- Kafka consumer groups: `consumes from "t" as consumer group "g"`, `consumer group "g" is consuming "t" within "30s"`, `consumer group "g" has no members`.
- PostgreSQL `query "..." returns within "10s":`, for rows the app writes asynchronously.
- `tomato coverage`: which resource steps the feature files use, per resource type (text, markdown, JSON; `--min` to gate).
- CI runs `make coverage`: unit + integration coverage merged, every resource step must be used by a scenario, code coverage floors in `.coverage-min`, report posted on the PR.
- Kafka and RabbitMQ `message header "k" is "v"` for the next publish; S3 upload `with metadata:`.
- `scylladb` / `cassandra` resource: CQL execution, table seeding and assertions, keyspace bootstrap, per-scenario truncate. ([#150](https://github.com/tomatool/tomato/pull/150))
- Kafka Avro via Confluent Schema Registry: publish and assert Avro messages as plain JSON. ([#152](https://github.com/tomatool/tomato/pull/152))
- Stability and deprecation policy (`docs/stability.md`), CONTRIBUTING, MAINTAINERS, CODEOWNERS, and issue and PR templates.
- `StepDef.Deprecated`: deprecated steps keep working, warn once per run, and are labelled in generated docs.
- `tomato validate` warns when `tomato.yml` has no `version` field.
- CI reports: file outputs such as `junit:reports/tomato.xml` get their directories created, and are kept when `--format` is overridden (for example by the GitHub Action's PR comment). ([#149](https://github.com/tomatool/tomato/pull/149))
- HTTP: the `Host` header is honoured, plus steps for cookies and docstring request bodies.

### Fixed
- `tomato ui`: collapsing either side pane with `[` or `]` did nothing. `transition: grid-template-columns` kept Chrome serving the old track list whenever the width came from a custom property, so the pane never resized.
- `tomato update` replaced a binary Homebrew had installed, so brew still recorded the old version and put it back on its next upgrade or reinstall. It now stops and prints the `brew` command to run instead, and the new-version notice names that command too.
- The last lines of a run's output, the summary among them, could be lost: tomato exited before the pipe that copies its output to the console and the run log was drained.
- Kafka: tomato waits until a topic it creates (the `creates topic` steps, `delete_recreate` resets) answers an offset lookup on every partition, so a `consumes from` right after it no longer fails with "not the leader for some partition". KRaft brokers, such as the `kafka` preset's, accept a topic before they serve it. A `consumes from` that fails can also be retried by a later step instead of being skipped.
- Container `volumes` and `build` were parsed and ignored; they are applied now, with relative paths resolved against `tomato.yml`.
- Container logs were read once, when the wait strategy passed; they are followed for the container's whole life, so a container that fails to start leaves its output behind.
- A container env template tomato cannot resolve (`{{.x}}`) made startup hang.
- Stopping the app signalled only its PID, so `command: go run ./app` left the compiled app running and holding the port; the app runs in its own process group, which is stopped as a whole.
- GitHub Action: the binary is downloaded and extracted in a temp directory, not the workspace (a repo with a `tomato` path at its root failed with `tar: tomato: Cannot open: File exists`), and results reach the comment scripts through env instead of being spliced into them, so a failing run's comment no longer breaks on backticks in its own markdown, and test output cannot inject into the script.
- Receive steps (Kafka, RabbitMQ, WebSocket) take the next unmatched message, so a message that arrived before the step ran is no longer missed (flaky timeouts in fanout and fast-echo scenarios).
- HTTP steps accept an absolute URL instead of prefixing `base_url` to it.
- The integration suite and S3 docs use `cgr.dev/chainguard/minio`; neither Docker Hub's `minio/minio` nor `quay.io/minio/minio` can be pulled anonymously any more.
- `grpc` reflection falls back to v1alpha reliably (a Send EOF hid the Unimplemented status).
- `http-server` `url is stored in` stored nothing; WebSocket server writes are serialised per connection.
- Resources were never cleaned up at the end of a run, so clients stayed connected while their containers stopped (gocql logged every reconnect to a stopped ScyllaDB), and the aws resource left its token and credential files in the temp directory. They are closed now, after the app stops and before the containers do.
- A container built from `build:` got a random image name on every run and left one more image behind each time. The image is named `tomato-<container>:<hash>` and rebuilt from cache into the same image.

### Changed
- The topology view draws network traffic rather than a resource list. A step driving an http/grpc/websocket client now shows both legs of the request — tomato reaches the client, the client reaches whatever its `base_url` addresses — so the app no longer sits unconnected while an arrow points at the client that was only the means of reaching it. Resources tomato serves itself (`http-server`, `websocket-server`) are drawn dashed from the app, because that traffic is the app's and tomato only observes it arriving. Edges are one per route with the number of steps using it; the per-step view is the Flow tab.
- The web UI is a React project under `ui/`, built by Vite with pnpm. `pnpm build` writes `command/ui_assets/dist`, which the binary embeds; the bundle is committed so `go build` still works without Node, and CI (`make ui-check`) fails when it is stale. `make build` and `make install` rebuild it when anything under `ui/` is newer (warning and falling back to the committed bundle when pnpm is absent), `make ui` forces a rebuild, `make ui-dev` runs the dev server, `make ui-check` fails on a stale bundle, and goreleaser builds it before a release.
- The inspector pane's width is draggable from its left edge and remembered per browser; double-clicking the handle resets it. The console's height is remembered too.
- Scenario cards carry a caret and respond to Enter and Space, so it is visible that they open and close. Background cards toggle correctly — their first click used to do nothing.
- `tomato ui` rebuilt on a dark-only design system. The embedded assets are now `index.html`, `styles.css`, `main.js` and vendored Geist/Geist Mono woff2 under `command/ui_assets/` (previously a single HTML file); the light theme is gone. The screen is a four-pane shell — feature tree, feature document, an inspector with Flow / Topology / Runs tabs and a step transport, and the output console — with run state carried in `data-*`/`aria-*` attributes and each status given its own pip shape so it reads without colour. Keys: `/` filter, `j`/`k` scenario, `r` run focused, `R` run all, `Esc` stop, `[`/`]` panes, `←`/`→`/space transport.
- The UI's feature parser reads `Rule:` blocks, whose background and scenarios it used to drop, and records a docstring's fenced language.
- The `tomato` event format carries `stepIndex` and `durationMs`, and adds `step_start`. The UI keys per-step status on the index, so a running step is marked while it runs and every step shows its own duration.
- `/api/config` returns the parsed config (settings, app env, containers, resources, hooks) alongside the raw file, so the config screen does not parse YAML in the browser.
- Console output is coloured with CSS classes (`.c-red`, `.c-green`, …) instead of inline hex, so the palette lives in the stylesheet.
- The `kafka` preset runs the stock `apache/kafka:3.9.1`. Its `AWS_MSK_IAM` server now ships inside the tomato binary and is copied into the container before the broker starts, so there is no preset image to pull, and a development build no longer builds one on its first run. `ghcr.io/tomatool/tomato-kafka` is no longer published; the image 2.1.3 uses stays available.
- GitHub Action: used at a commit or a branch (`tomatool/tomato@<sha>`), it builds tomato from that commit instead of installing the latest release, so the action and the binary match and a change can run in CI before it is released. Version tags and the `version` input install a release as before.
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
