# Onboarding a repository

The goal is a suite that is only `tomato.yml`, feature files and a CI job, and
that fails the way production fails.

## 1. Inventory the production path

Before writing config, list from the code and the deployment manifests:

- Every dependency the application talks to: databases, brokers, caches, buckets, the services it calls, where it gets cloud credentials.
- How production wires each one: connection settings, auth mechanism, runtime flags, feature switches.
- The interfaces its behavior shows up on: its API, rows it writes, messages it publishes, calls it makes.

Each of them becomes something tomato starts or serves: a container, a preset, an
`http-server` mock, the `aws` resource. Nothing in the suite may reach outside the
run, not even read-only: no staging database, real cloud account or third-party
API.

Anything on that list the suite leaves out, or replaces with something that can't fail the same way, is a gap. Name each gap in a comment where it is taken. A dependency tomato has no resource, preset or step for is a gap in tomato: follow [gaps.md](gaps.md), don't fake it.

## 2. Write tomato.yml

`tomato init` is interactive; write the file directly. Check field names in the
[configuration reference](https://tomatool.github.io/tomato/configuration/).

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: true
  # containers.reuse: true keeps the containers between runs, which makes a
  # rerun start in about a second. Local only: it means a container an earlier
  # run corrupted is no longer thrown away.

containers:
  # One container per dependency, pinned to the version production runs.
  # Another service's database gets its own container, never a schema in ours.
  app-db:
    image: postgres:16.4
    env:
      POSTGRES_DB: app
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
    ports:
      - "5432/tcp"
    wait_for:
      type: port
      target: "5432/tcp"
      timeout: 30s

  kafka:
    preset: kafka   # plus `auth: aws_msk_iam` when production authenticates to MSK with IAM

app:
  # The artifact CI builds, started the way production starts it.
  command: ./bin/app serve
  # Off the application's default port, so a local run can't answer in its place.
  port: 18080
  ready:
    type: http
    path: /healthz
    status: 200
    timeout: 60s
  env:
    PORT: "18080"
    DATABASE_URL: "postgres://app:app@{{.app-db.host}}:{{.app-db.port.5432}}/app?sslmode=disable"
    KAFKA_BROKERS: "{{.kafka.host}}:{{.kafka.port.9092}}"
    PAYMENTS_URL: "{{.payments.url}}"   # the http-server below stands in for the payments service
    TZ: UTC                              # identical timestamps on every machine

resources:
  api:
    type: http
    base_url: http://localhost:18080
  db:
    type: postgres
    container: app-db
    database: app
    options:
      user: app
      password: app
  events:
    type: kafka
    container: kafka
    options:
      topics: [orders.created.v1]
  payments:
    type: http-server

features:
  paths:
    - ./features
```

Wiring rules:

| Need | Use |
|---|---|
| A container's address in `app.env` | `{{.<container>.host}}`, `{{.<container>.port.<port>}}` |
| A mock's address in `app.env` | `{{.<resource>.url}}` of an `http-server` resource |
| Kafka | `preset: kafka`; host clients use `port.9092`, other containers `<name>:29092` |
| Kafka with MSK IAM | `auth: aws_msk_iam` on the preset, an `aws` resource with the service's role, clients on `port.9098` |
| Cloud credentials through IRSA | an `aws` resource; it serves STS and sets the SDK's environment |
| A CLI the application ships | a `shell` resource running that CLI: the real client, not a stand-in |

The `aws` resource removes inherited `AWS_*` credentials from the application's
environment, so a developer's own credentials never stand in for the role.

tomato splits `app.command` on spaces and runs it without a shell: no quotes,
variables, pipes or `&&`. Build first, in CI or by hand, and point `app.command`
at the result.

## 3. First features

Start with one scenario per interface that proves the wiring, then the behaviors
that matter most. Each scenario is one action with a Then for every effect it
has: the response, the row, the message, the object, the call to a dependency.
Lay them out by interface from the first file, like
`features/http/api/v1/health.feature`. See [scenarios.md](scenarios.md).

## 4. Verify

```bash
tomato validate
tomato run
tomato run   # again: timing bugs pass once
```

On failure, read [debugging.md](debugging.md) before changing anything.

## 5. CI

Build the same artifact the release builds, where `app.command` expects it, then
run the action. It posts the results on the pull request.

```yaml
name: Tomato

on:
  pull_request:

jobs:
  tomato-test:
    name: Tomato Test
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write   # the results comment
    steps:
      - uses: actions/checkout@v4
      - run: make build
      - uses: tomatool/tomato@v2
        with:
          config: tomato.yml
```

`version:` pins a release. Before a change is released, `tomatool/tomato@<commit sha>`
builds tomato from that commit.
