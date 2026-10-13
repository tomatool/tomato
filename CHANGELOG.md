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
- The `http` resource reads Server-Sent Events streams: `"api" opens stream "/events"` with the scenario's headers, cookies and query params, then asserts what arrives — `receives event "order.created" within "5s"`, optionally `with json containing:`, a table of `receives events within` in order with their ids and data, `does not receive an event`, keep-alive comments, gapless ids with `stream event ids are consecutive from "1"`, and the server ending the stream. A client resumes the way a real one does, with `opens stream ... after event id "{{last_id}}"` sending `Last-Event-ID`; tomato never reconnects on its own, so a broken resume can't hide behind a quiet retry. Streams are separate from requests, so `"api" sends` works while one is open, and the `timeout` option no longer cuts them off.
- `http-server` reads back what the app sent: `"pm" last "POST" "/orders" request json "clientProcessId" saved as "{{process_id}}"` saves a field of the last matching request's body (a number exactly, an object or array as JSON), `... request header "Idempotency-Key" saved as` saves a header, and `... request json contains:` asserts the body with the same matchers as `response json contains`. The path accepts variables. For an app that generates an id, sends it to a dependency, and later gets it back in a message the scenario has to publish.
- `http-server` stubs match more than method and path: `"kms" stub "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt" returns "200" with json:`, `with query "page" "2"` and `with body containing "resolveThread"`, any number of them on every stub step. That covers AWS JSON-protocol APIs, paginated GETs and GraphQL, which all put the operation somewhere other than the path. `received "POST" "/" with header "X-Amz-Target" "..."` asserts which operation was called. ([#170](https://github.com/tomatool/tomato/issues/170))
- `http-server` stubs answer in sequence: `returns in sequence:` takes a table of `status`, `json` or `body`, and `delay`, answers each request with the next row and repeats the last one. Polls (`pending`, then `complete`), retries (`503`, then `201`) and a dependency that times out (`delay: 5s`) become one step; a delayed answer ends early when the app gives up.
- `http-server` can go down: `"pm" refuses connections` closes the port (and idle keep-alive connections, so the app can't reuse one), `accepts connections` reopens it on the same address with the stubs kept, and every scenario starts accepting.
- Kafka Avro in the AWS Glue wire format: `schema_registry: {format: glue}` makes tomato write and read `[0x03][compression][schema version UUID][Avro]` (uncompressed or zlib) and serve the Glue Schema Registry API itself (`GetSchemaVersion`, `GetSchemaByDefinition`, `CreateSchema`, `RegisterSchemaVersion`, `GetSchema`) on a local endpoint, with `AWS_ENDPOINT_URL_GLUE` set in the app's environment. An app using the AWS Glue SerDe fetches the schemas the scenario registered, as it does in production, without an AWS account. The Avro steps are unchanged. See [Kafka: AWS Glue Schema Registry](docs/configuration/kafka.md#aws-glue-schema-registry).
- `resource.ConditionalAppEnvProvider`: a resource that provides app environment only in some configurations is initialized before the app only then.
- `lifecycle` resource: take the app or a dependency down mid-scenario, the way production fails. `restarts the app` (optionally `with env:`), `stops the app` / `starts the app`, and `stops`, `starts`, `restarts`, `pauses` and `unpauses container "postgres"`. Every scenario starts with everything back up: the resource resets before all others (`resource.ResetsFirst`), so a postgres reset never meets a stopped database. A container with a `log` wait strategy is ready again only when the line appears once more: its log still holds the line from the first start, which would otherwise pass the wait at once. See [Lifecycle](docs/configuration/lifecycle.md).
- `lifecycle` resource: take the app or a dependency down mid-scenario, the way production fails. `restarts the app` (optionally `with env:`), `stops the app` / `starts the app`, and `stops`, `starts`, `restarts`, `pauses` and `unpauses container "postgres"`. Every scenario starts with everything back up: the resource resets before all others (`resource.ResetsFirst`), so a postgres reset never meets a stopped database. See [Lifecycle](docs/configuration/lifecycle.md).
- `http` sends the same request N times at the same moment: `"api" sends "5" concurrent "POST" to "/users" with json:` builds every request with the scenario's headers, cookies and query params, then releases them together on separate connections. Which one wins a race isn't fixed, so the assertions count: `"1" of the concurrent responses have status "201"`, `... have json "duplicate" is "true"`, `... contain json:`. A wrong count lists every response. For idempotency keys, unique constraints and anything else that only breaks under concurrency.
- The web UI tells a scenario's preparation from its probe and its checks. Every step is classified from what its definition does and the keyword the author chose — `Given "api" sends` is preparation, `When "api" sends` is the probe, a check is a check under any keyword, and a Background is preparation throughout — and shown as a `setup` / `probe` / `check` / `capture` chip beside the resource, with a rule between the preparation block and the first probe. The Flow tab draws preparation quieter and the probe heavier. The rule lives in `ui/src/lib/kind.js`, keyed on the step groups the resources declare.
- The feature document follows the selection: a scenario picked in the tree, or reached with `j`/`k`, scrolls to the top of the document and opens. A toggle beside the inspector's tabs (or `\`) shows the topology in the centre pane at its full width, with the flow in the inspector beside it; picking the inspector's Topology tab, or the toggle again, puts it back. The output pane minimises to its tab bar with the chevron on its right (or `-`), and comes back when a run starts.
- A [Documentation Map](docs/overview.md): every page on one screen with a line on what it is for, so the whole documentation can be navigated from one place. A test fails if a page in the nav is missing from it.
- The sidebar groups collapse. They were rendered as flat always-open headers, which made the step reference's fourteen entries hard to see past.
- The documentation site is in the same palette as `tomato ui` — the Modernist tokens from `ui/src/styles.css`, Geist and Geist Mono, the tomato mark as the favicon, and a dark/light toggle that defaults to dark as the app does. Gherkin is highlighted the way the app colours it: keyword coloured, step text plain, quoted arguments picked out.
- `markdown_extensions` are configured, so admonitions render as admonitions and code blocks are highlighted with a copy button. Sixteen `!!! tip` blocks had been rendering as literal text on the published site, one on every generated resource page.
- [Writing a Suite](docs/guide/philosophy.md): how to write scenarios worth trusting — one behaviour per scenario, and asserting the effects your application produces (the row, the message, the file) rather than only the response it returns. Every example is a real step, checked against the live definitions.
- [Adding a Step or Resource](docs/developer-guide/adding-a-resource.md) in the developer guide, so the walkthrough sits beside the architecture it refers to. CONTRIBUTING links it rather than repeating it.
- Three tests that keep the documentation honest: every Gherkin snippet in `docs/` must match a registered step, every internal link must resolve, and the mkdocs nav and the pages on disk must agree.
- `tomato ui` shows where each file it is displaying actually lives: the absolute path of `tomato.yml`, of the feature file in the document pane, and of an opened run log, each with a copy button. A page served over http cannot link to `file://`, so the path is shown and copied rather than linked.
- `tomato ui` has a light theme. The button beside the wordmark flips between dark and light and remembers the choice per browser; it is applied before the first paint, so light does not flash dark on load. Only colour changes — every size, radius and duration is shared, and the dark palette is untouched.
- `allow_destructive_reset` on a resource. A stateful resource with no `container:` points at a system tomato did not start, and tomato wipes every resource before every scenario — so loading such a config now fails until the suite says the data is meant to be destroyed. `tomato validate` flags it and every run prints which unmanaged resources are about to be wiped. Only `postgres`, `redis`, `kafka`, `rabbitmq`, `s3`, `scylladb` and their aliases are affected.
- `tomato ui`: each scenario has a Flow view and a Sequence view besides its steps. Flow shows tomato, the app and every resource in `tomato.yml`; each Next draws the step's line from tomato to what it acts on, and after a When on the app, dashed lines to the effects the scenario expects of it. Hovering a line shows its steps with their payloads: doc strings and tables. Sequence draws each step as an arrow to its resource. Background steps are included, and a step no step definition matches is flagged before the run.
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
- HTTP response assertions compared against the literal text of a saved variable: `"api" response json "id" is "{{order_id}}"` checked for the string `{{order_id}}`, so asserting a response against a value saved earlier was impossible. `response json ... is`, `response json matches:` and `contains:`, `response header ... is` and `contains`, `response body is:`, and the inline `response body contains` and `does not contain` now substitute variables, as request steps and cookie assertions already did. An unknown `{{name}}` stays literal, so suites without variables are unaffected.
- `http-server` recorded a request body with a single `Read`, which returns whatever has arrived so far: a body sent in pieces was stored cut short, so `received request with body containing` could miss text that was sent. It now reads the body to the end (up to 1MB).
- `tomato init` generated a config that would not parse when Kafka was selected: `KAFKA_BROKERS: {{.kafka.host}}:{{.kafka.port.9092}}` is a YAML syntax error, because a value opening with `{` starts a flow mapping. The other templated env vars survived only because they begin with a scheme. Generated values are quoted now, including the command and dockerfile the user types.
- A YAML syntax error in `tomato.yml` said only `yaml: line 8: did not find expected key`. It now shows the offending line — found by searching forward, since YAML reports where the mapping began rather than where it broke — and, for the common case of an unquoted `{{template}}`, explains why and prints the corrected line.
- Five documentation pages, Getting Started among them, showed `settings.reset.level` in a `tomato.yml` example — a setting removed when reset became unconditional. A test now parses every complete config example in the docs with `KnownFields`, so an example tomato would not accept fails CI.
- Three steps in the S3 configuration page matched no step definition — `"db" table "x" with values:`, `sends "POST" request to`, and `response code should be` — wording from before a rename, published for as long as the page existed. Getting Started also linked the deleted `examples/` directory.
- `tomato init` generated a suite that did not work. Every step in the example feature used wording no resource registers any more — five steps, five undefined — and the config carried `settings.reset.level`, removed when reset became unconditional and silently ignored by the loader ever since. Choosing both Kafka and RabbitMQ produced a config with a duplicate `queue:` key that would not parse at all, and the external-HTTP example called a resource that was never declared. A generated suite now validates clean and runs green, and three tests tie the generator to the live step definitions and the config struct so it cannot drift again.
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
- When two `http-server` stubs match a request, the most specific wins (more conditions), then the **most recently added**. It used to be the first added, so a scenario could not override a stub its Background set.
- With a local Docker, tomato binds each dynamic container port (`ports: ["5432/tcp"]`) to a free host port it picks itself instead of leaving it to Docker. Docker hands out a new host port every time a container starts, so a container stopped and started mid-scenario came back where the app no longer looked. A remote Docker host keeps Docker's allocation.
- The README no longer warns to use v2 at your own risk, and opens with the one-minute introduction video. [Stability](docs/stability.md) records the warning's removal, and the `v1` branch is linked from a migration note instead.
- The topology view draws the suite's wiring, not only a resource list. Containers tomato starts are boxes with the resources they back inside, so two redis resources on one container read as one service with two handles; the app's `env` templates (`{{.postgres.host}}`, `{{.mock.url}}`) and each client's `base_url` are dotted lines; what tomato serves itself, what runs outside tomato and the host are their own groups. Only what the scenario touches is drawn, the rest is listed as not involved with a toggle to show it dimmed. The picture is laid out at natural size — as columns when the inspector is wide enough, as a stack with the traffic beside it when it is not — instead of scaling a fixed drawing down to the pane, which made a sixteen-resource suite unreadable and let long names overflow their boxes.
- Resource code is one package per resource under `internal/resource`, instead of thirteen resources sharing one 13,900-line `internal/handler`. Each depends on the contract in `internal/resource` and on no other resource, which a test enforces; the JSON matcher that lived inside the HTTP client's file is `internal/jsonmatch`, and what builds resources from config is `internal/registry`. The commands no longer hold their own list of resource types — `tomato docs`, `tomato steps` and `tomato validate` ask the registry, so a new resource cannot end up working but undocumented. `internal/container` keeps its single package, split into files by concern. No behaviour changes: `tomato docs` and `tomato steps` produce byte-identical output. See [Architecture](docs/developer-guide/architecture.md) and CONTRIBUTING.
- Request detail in the inspector docks at the foot of the pane instead of floating over the diagram as a card, and both Flow and Topology now share that one dock. Clicking a line or a route pins it there so it survives the pointer moving away; clicking it again, the Unpin button or `Esc` lets go. A pin wins over hovering, and is dropped when the scenario changes.
- The topology view draws network traffic rather than a resource list. A step driving an http/grpc/websocket client now shows both legs of the request — tomato reaches the client, the client reaches whatever its `base_url` addresses — so the app no longer sits unconnected while an arrow points at the client that was only the means of reaching it. Resources tomato serves itself (`http-server`, `websocket-server`) are drawn dashed from the app, because that traffic is the app's and tomato only observes it arriving. Edges are one per route with the number of steps using it; the per-step view is the Flow tab.
- The web UI is a React project under `ui/`, built by Vite with pnpm. `pnpm build` writes `command/ui_assets/dist`, which the binary embeds; the bundle is committed so `go build` still works without Node, and CI (`make ui-check`) fails when it is stale. `make build` and `make install` rebuild it when anything under `ui/` is newer (warning and falling back to the committed bundle when pnpm is absent), `make ui` forces a rebuild, `make ui-dev` runs the dev server, `make ui-check` fails on a stale bundle, and goreleaser builds it before a release.
- The inspector pane's width is draggable from its left edge and remembered per browser; double-clicking the handle resets it. The console's height is remembered too.
- Scenario cards carry a caret and respond to Enter and Space, so it is visible that they open and close. Background cards toggle correctly — their first click used to do nothing.
- `tomato ui` rebuilt on a new design system. The embedded assets are now `index.html`, `styles.css`, `main.js` and vendored Geist/Geist Mono woff2 under `command/ui_assets/` (previously a single HTML file). The screen is a four-pane shell — feature tree, feature document, an inspector with Flow / Topology / Runs tabs and a step transport, and the output console — with run state carried in `data-*`/`aria-*` attributes and each status given its own pip shape so it reads without colour. Keys: `/` filter, `j`/`k` scenario, `r` run focused, `R` run all, `Esc` stop, `[`/`]` panes, `←`/`→`/space transport.
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
- The agent skills, both of them: `skills/tomato` for teams writing a suite and `.claude/skills/tomato-dev` for people changing tomato. The rules the contributor one held are in [CONTRIBUTING.md](CONTRIBUTING.md); the documentation covers what the user-facing one did.
- `settings.parallel`. Scenarios run one at a time, which is what makes the per-scenario reset safe: the resources are shared, so a concurrent scenario's reset would wipe state another scenario is still using. A config setting it above 1 now fails to load with that reason, and `tomato validate` reports it.
- `examples/`. It was referenced by nothing — not the README, the docs nav, or CI — so nothing ran it and nothing stopped it going stale, and it only worked if you `cd examples` first. Every resource it showed is covered by `tests/`, which CI runs and holds at 100% step coverage, and per-resource configuration is in [Configuration](docs/configuration/index.md).
- The `ghcr.io/tomatool/tomato-kafka` package. It had one tag, `2.1.3`, and only tomato v2.1.3 ever pulled it: since v2.1.4 the kafka preset runs the stock `apache/kafka` image with the `AWS_MSK_IAM` server copied in from the binary, so nothing has published or pulled it for three releases. **If you are on v2.1.3 and use `preset: kafka`, upgrade to v2.1.4 or later** — v2.1.3 has no fallback and fails at container startup with `Head "https://ghcr.io/v2/tomatool/tomato-kafka/manifests/2.1.3": denied: denied`, which GHCR returns for a deleted image rather than a 404.
- **Breaking.** `settings.reset.level` and `settings.reset.on_failure`, the `--no-reset` flag, the GitHub Action's `no-reset` input, the per-resource `reset: false`, and `reset_strategy: none`. Every resource is now reset before every scenario, with no way to turn it off: a scenario that needs a starting state builds it in a `Background`. The two `settings.reset` fields were accepted and validated but never read, so every level already behaved as `scenario`; the rest were the remaining ways to opt out of isolation.
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
