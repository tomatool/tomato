# Getting started

This guide takes you from nothing to a passing suite in seven steps.

Once it passes, read [Writing a Suite](../guide/philosophy.md) — it covers the
habits that make a suite worth trusting, in particular asserting the *effects*
your application produces rather than only the response it returns.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) — tomato starts your test containers
- [Go 1.26+](https://go.dev/dl/) — only for the `go install` and from-source methods

## 1. Install tomato

### Quick install (recommended)

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

### Verify installation

```bash
tomato --version
```

## 2. Create a project

Create a new project with the init command:

```bash
mkdir my-project && cd my-project
tomato init
```

This creates:
- `tomato.yml` - Main configuration file
- `features/` - Directory for your feature files

### Project layout

```text
my-project/
├── tomato.yml           # Main configuration
├── features/
│   └── example.feature  # Gherkin test files
└── fixtures/            # Optional: SQL scripts, test data
```

## 3. Describe your dependencies

`tomato.yml` says what your service needs and how the scenarios reach it. The
examples on this page test an `orders` service that stores orders in Postgres
and publishes them to Kafka:

```yaml
version: 2

# Infrastructure tomato starts, named after the software
containers:
  postgres:
    image: postgres:16-alpine
    env: { POSTGRES_USER: test, POSTGRES_PASSWORD: test, POSTGRES_DB: orders }
    ports: ["5432/tcp"]
    wait_for: { type: exec, target: "pg_isready -h 127.0.0.1 -U test" }
  kafka:
    preset: kafka

# Your service
app:
  command: ./orders
  port: 8080
  ready: { type: http, path: /health }
  env:
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port.5432}}/orders"
    KAFKA_BROKERS: "{{.kafka.host}}:{{.kafka.port.9092}}"

# What the scenarios talk to, named <whose>-<role>
resources:
  orders-client:                # calls the orders service
    type: http
    base_url: http://localhost:8080
  orders-db:                    # the orders service's database
    type: postgres
    container: postgres
    database: orders
    options: { user: test, password: test }
  orders-events:                # the topics the orders service uses
    type: kafka
    container: kafka
    options: { topics: [orders.placed, payments] }

features:
  paths: [./features]
```

### Name resources by whose they are and what they do

A step starts with a resource name, and a reader sees the step long before the
config. So the name says **whose** it is and **what role** it plays:

| Role | Name | Reads as |
|---|---|---|
| calls your service | `orders-client` | `"orders-client" sends "POST" to "/orders"` |
| your service's data | `orders-db`, `orders-cache` | `"orders-db" table "orders" has "1" rows` |
| a broker | `orders-events` (Kafka), `orders-queue` (RabbitMQ) | `"orders-events" receives from "orders.placed"` |
| stands in for a service you call | `stripe-stub`, `catalog-stub` | `"stripe-stub" received "POST" "/v1/charges"` |
| object storage | `invoices-storage` | `"invoices-storage" object "…" should exist` |

Not `api`, `db`, `mock` or `http`: those say what kind of thing it is, not whose.
[Naming resources](../guide/philosophy.md#naming-resources) has the full
convention.

## 4. Write scenarios

### Features, scenarios and steps

```gherkin
# One feature file per capability of your service
Feature: Placing orders

  # The state every scenario below starts from
  Background:
    Given "orders-db" table "customers" has values:
      | id | email           |
      | 7  | ada@example.com |

  # One behaviour, named as a fact
  Scenario: A placed order is stored and announced
    # When: the one action
    When "orders-client" sends "POST" to "/orders" with json:
      """
      {"customer_id": 7, "sku": "TMT-01", "qty": 2}
      """
    # Then: what it caused
    Then "orders-client" response status is "201"
    And "orders-db" table "orders" contains:
      | customer_id | sku    | qty | status  |
      | 7           | TMT-01 | 2   | pending |
    And "orders-events" receives from "orders.placed" within "5s"
```

- **Feature**: a file in `features/`, about one capability of your service.
- **Scenario**: one behaviour of that capability, named after the outcome.
- **Background**: the starting state every scenario in the file shares. It runs
  after tomato's reset, before each scenario.
- **Given** sets up state, **When** is the one action, **Then** checks what it
  caused.

### Principles

**1. One scenario, one behaviour.** One `When`. A scenario that also reads the
order back and cancels it fails for three reasons, and tells you none of them.

**2. Check the effects, not only the response.** A `201` says the service
didn't crash. The row in the database, the message on the topic and the file in
the bucket are what the rest of your system relies on, so assert them:

```gherkin
Then "orders-client" response status is "201"
And "orders-db" table "orders" has "1" rows
And "orders-events" receives from "orders.placed" within "5s"
# with an s3 resource for the invoices bucket
And "invoices-storage" object "invoices/inv-1.pdf" should exist
```

**3. Split a flow where it crosses a boundary.** When the service stores an
order, publishes an event, and later consumes another event to mark the order
paid, that's two behaviours, so two scenarios: one ends where the service
publishes, the next starts where tomato publishes the event the service
consumes.

<div class="split-flow" data-mode="long" markdown="0">
  <div class="sf-modes" role="tablist">
    <button type="button" data-mode="long">One long scenario</button>
    <button type="button" data-mode="split">A scenario per behaviour</button>
  </div>
  <div class="sf-canvas">
  <svg viewBox="0 0 760 250" role="img" aria-label="A flow from a request to the service, to its database and Kafka, and back through a consumer, covered by one long scenario or by two scenarios that meet at the topic">
    <defs><marker id="sf-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" class="sf-head"/></marker></defs>
    <g class="sf-node"><rect x="10" y="90" width="110" height="54" rx="8"/><text x="65" y="113">orders-client</text><text class="sub" x="65" y="131">POST /orders</text></g>
    <g class="sf-node"><rect x="170" y="90" width="120" height="54" rx="8"/><text x="230" y="113">orders API</text><text class="sub" x="230" y="131">your service</text></g>
    <g class="sf-node"><rect x="340" y="20" width="120" height="54" rx="8"/><text x="400" y="43">orders-db</text><text class="sub" x="400" y="61">order: pending</text></g>
    <g class="sf-node topic"><rect x="340" y="160" width="120" height="54" rx="8"/><text x="400" y="183">Kafka</text><text class="sub" x="400" y="201">placed · payments</text></g>
    <g class="sf-node"><rect x="510" y="160" width="120" height="54" rx="8"/><text x="570" y="183">consumer</text><text class="sub" x="570" y="201">your service</text></g>
    <g class="sf-node"><rect x="630" y="20" width="120" height="54" rx="8"/><text x="690" y="43">orders-db</text><text class="sub" x="690" y="61">order: paid</text></g>
    <path class="sf-wire w1" d="M120 117 H170" marker-end="url(#sf-arrow)"/>
    <path class="sf-wire w1" d="M290 105 L340 55" marker-end="url(#sf-arrow)"/>
    <path class="sf-wire w1" d="M290 130 L340 180" marker-end="url(#sf-arrow)"/>
    <path class="sf-wire w2" d="M460 187 H510" marker-end="url(#sf-arrow)"/>
    <path class="sf-wire w2" d="M630 175 L680 74" marker-end="url(#sf-arrow)"/>
    <rect class="sf-bracket b-long" x="4" y="10" width="752" height="230" rx="12"/>
    <text class="sf-label b-long" x="16" y="234">one scenario: four reasons to fail, and a wait for the consumer</text>
    <rect class="sf-bracket b1" x="4" y="10" width="462" height="214" rx="12"/>
    <text class="sf-label b1" x="16" y="240">scenario 1: placing an order</text>
    <path class="sf-bracket b2" d="M334 154 H500 V10 H756 V224 H334 Z"/>
    <text class="sf-label b2" x="470" y="240">scenario 2: a payment marks it paid</text>
  </svg>
  </div>
  <div class="sf-text">
    <p data-for="long"><strong>Tempting:</strong> one scenario places the order, waits for the event, publishes the payment and checks the order is paid. It fails for any of four reasons, waits on asynchronous work, and the consumer can't be tested on its own.</p>
    <p data-for="split"><strong>Scenario 1</strong> places an order and checks what the service did: the row, and the event on <code>orders.placed</code>. <strong>Scenario 2</strong> seeds a pending order, has tomato publish the payment, and checks the service marks it paid. They meet at the topic, and each fails for one reason.</p>
  </div>
</div>

```gherkin
Scenario: Placing an order stores it and announces it
  When "orders-client" sends "POST" to "/orders" with json:
    """
    {"customer_id": 7, "sku": "TMT-01", "qty": 2}
    """
  Then "orders-client" response status is "201"
  And "orders-db" table "orders" has "1" rows
  And "orders-events" receives from "orders.placed" within "5s"

Scenario: A payment marks the order paid
  Given "orders-db" table "orders" has values:
    | id | customer_id | sku    | qty | status  |
    | 1  | 7           | TMT-01 | 2   | pending |
  When "orders-events" publishes json to "payments":
    """
    {"order_id": 1, "status": "succeeded"}
    """
  Then "orders-db" query "SELECT status FROM orders WHERE id = 1" returns within "10s":
    | status |
    | paid   |
```

**4. Build the state you need.** tomato empties every database and topic
before every scenario. A scenario that needs an order seeds one in its
`Given` or `Background`; it never relies on another scenario having run.

**5. Variations are more scenarios, not a longer one.** The out-of-stock case,
the invalid request and the provider being down are behaviours too. Give each
its own scenario, or a `Scenario Outline` when only the values change:

```gherkin
Scenario Outline: An invalid order is refused and nothing is stored
  When "orders-client" sends "POST" to "/orders" with json:
    """
    <body>
    """
  Then "orders-client" response status is "400"
  And "orders-db" table "orders" is empty

  Examples:
    | body                           |
    | {"sku": "TMT-01", "qty": 2}    |
    | {"customer_id": 7, "qty": 0}   |
```

[Writing a Suite](../guide/philosophy.md) goes deeper into each of these.

## 5. Check it before you run it

Before running tests, validate your configuration and feature files:

```bash
tomato validate
```

This checks:
- Configuration file syntax and structure
- Resource type validity
- Container references
- Feature file parsing
- Step definitions availability

Use `--plain` for CI environments without interactive output:

```bash
tomato validate --plain
```

## 6. Run it

Run all tests:

```bash
tomato run
```

Run with specific tags:

```bash
tomato run --tags "@smoke"
```

Run with verbose output:

```bash
tomato run -v
```

## 7. Watch it run

`tomato ui` opens a local web page that lists the feature files, shows each
scenario's steps with the resource every step runs against, and runs scenarios
on demand. It watches the feature files and reloads as they change.

```bash
tomato ui
```

It is a development aid, not part of the stable surface: see
[Stability](../stability.md#not-covered).

## Pointing tomato at your application

Tomato can also start your application and connect it to test containers:

```yaml
# tomato.yml
app:
  command: go run ./cmd/server
  port: 8080
  ready:
    type: http
    path: /health
  wait: 5s
  env:
    DATABASE_URL: "postgres://test:test@{{.postgres.host}}:{{.postgres.port.5432}}/test"
    REDIS_URL: "redis://{{.redis.host}}:{{.redis.port.6379}}"
```

The `{{.container.host}}` and `{{.container.port.<port>}}` templates are replaced
with the container's address and its mapped host port. The number is the port
*inside* the container, so `{{.postgres.port.5432}}` resolves to whichever host
port Docker mapped 5432 to. A bare `{{.postgres.port}}` is left untouched.

## Where to go next

- [Writing a Suite](../guide/philosophy.md) — one behaviour per scenario, and
  asserting the effects rather than just the response. Read this before you
  write the second scenario.
- [Configuration Reference](../configuration/index.md) — every `tomato.yml` field
- [Step Reference](../resources/index.md) — every step, grouped by resource
- [Test Coverage](coverage.md) — which lines of your service the scenarios ran
- [GitHub Action](github-action.md) — running the suite in CI
