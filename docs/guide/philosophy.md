# How to write a tomato suite

tomato can run any scenario you write. These are the habits that make the
result worth trusting — and the reasoning behind the things tomato refuses to
let you do.

## A scenario covers one behaviour

A scenario earns its place by failing for exactly one reason. When it covers
three behaviours, a failure tells you something broke without telling you
what, and the second assertion never runs because the first one stopped the
scenario.

Not this:

```gherkin
Feature: Orders

  Scenario: Order lifecycle
    When "orders-client" sends "POST" to "/orders" with json:
      """
      {"sku": "TMT-01", "qty": 2}
      """
    Then "orders-client" response status is "201"
    When "orders-client" sends "GET" to "/orders/1"
    Then "orders-client" response status is "200"
    When "orders-client" sends "DELETE" to "/orders/1"
    Then "orders-client" response status is "204"
```

This:

```gherkin
Feature: Orders

  Scenario: Placing an order returns it
    When "orders-client" sends "POST" to "/orders" with json:
      """
      {"sku": "TMT-01", "qty": 2}
      """
    Then "orders-client" response status is "201"

  Scenario: A placed order can be read back
    Given "orders-db" table "orders" has values:
      | id | sku    | qty |
      | 1  | TMT-01 | 2   |
    When "orders-client" sends "GET" to "/orders/1"
    Then "orders-client" response status is "200"

  Scenario: Deleting an order removes it
    Given "orders-db" table "orders" has values:
      | id | sku    | qty |
      | 1  | TMT-01 | 2   |
    When "orders-client" sends "DELETE" to "/orders/1"
    Then "orders-client" response status is "204"
    And "orders-db" table "orders" is empty
```

The second version is longer and tells you more. Each scenario sets up what it
needs, and a failure names the behaviour that broke. They are also independent:
the read scenario does not depend on the write scenario having run, so running
one of them means something.

If several scenarios need the same starting point, put it in a `Background`
rather than chaining them. tomato runs it after the reset, before each
scenario.

## Assert the effects, not just the response

This is the one that separates a suite that catches regressions from one that
does not.

Your application returning `201` says it did not crash. It does not say the
order was stored, the event was published, or the receipt was written. Those
are the things the rest of your system depends on, and they are exactly what
breaks silently.

A response-only scenario:

```gherkin
Feature: Checkout

  Scenario: Placing an order
    When "orders-client" sends "POST" to "/orders" with json:
      """
      {"sku": "TMT-01", "qty": 2, "email": "ada@example.com"}
      """
    Then "orders-client" response status is "201"
```

That passes if the handler returns `201` and does nothing else.

The same behaviour, asserted through its effects:

```gherkin
Feature: Checkout

  Scenario: A placed order reaches every dependency
    Given "orders-events" consumes from "order.created"
    When "orders-client" sends "POST" to "/orders" with json:
      """
      {"sku": "TMT-01", "qty": 2, "email": "ada@example.com"}
      """
    Then "orders-client" response status is "201"
    And "orders-db" table "orders" has "1" rows
    And "orders-events" receives from "order.created" within "10s"
    And "receipts-storage" object "receipts/1.pdf" should exist within "10s"
```

Four assertions, four different systems, one behaviour. If the order lands in
the database but the event never publishes, this fails and tells you which.

Two things to note. The consume step comes **before** the request: a consumer
that subscribes afterwards may miss the message. And the effects use `within`,
because an application usually does this work after it has answered — a bare
assertion races the write.

## Every scenario starts from the same state

tomato resets every resource before every scenario. There is no way to turn
this off, and that is deliberate: a suite where scenario 7 passes only because
scenario 6 ran is a suite that cannot be run in isolation, reordered, or
trusted when it goes green.

If a scenario needs a starting state, it declares it:

```gherkin
Feature: Reading an order

  Background:
    Given "orders-db" table "orders" has values:
      | id | sku    | status  |
      | 1  | TMT-01 | shipped |

  Scenario: An existing order is returned
    When "orders-client" sends "GET" to "/orders/1"
    Then "orders-client" response status is "200"
    And "orders-client" response json "status" is "shipped"
```

A corollary worth knowing: **do not assert on generated identifiers you did not
set**. Sequences, UUIDs and timestamps are not part of the reset contract in
every store, and a scenario asserting `id` is `1` is asserting the order it
happens to run in. Seed the id, or assert on something you control.

## Naming resources

A step is read far more often than the `tomato.yml` behind it, usually by
someone looking at a failure. Its first word is the resource name, so that
name has to say what is being talked to without opening the config.

Not this:

```gherkin
Feature: Checkout

  Scenario: A paid order is stored
    Given "mock" stub "POST" "/v1/charges" returns "201"
    When "api" sends "POST" to "/orders"
    Then "db" table "orders" has "1" rows
    And "mock" received "POST" "/v1/charges"
```

Which API? A mock of what? Whose database? In a suite with two services and
three dependencies, every one of those is a guess.

This:

```gherkin
Feature: Checkout

  Scenario: A paid order is stored
    Given "stripe-stub" stub "POST" "/v1/charges" returns "201"
    When "orders-client" sends "POST" to "/orders"
    Then "orders-db" table "orders" has "1" rows
    And "stripe-stub" received "POST" "/v1/charges"
```

The rule is `<name>-<role>`, lower-case and hyphenated. `<name>` is the
service, dependency, owner or person; `<role>` comes from the resource type:

| Resource types | Role suffix | Name is | Examples |
|---|---|---|---|
| `http-client`, `grpc`, `websocket-client` | `-client` | the service it calls, usually the app under test | `orders-client`, `catalog-client` |
| two clients of one service over different protocols | `-<protocol>-client` | | `orders-http-client`, `orders-grpc-client` |
| `http-server`, `grpc-server`, `websocket-server` | `-stub` | the dependency it stands in for | `stripe-stub`, `catalog-stub`, `prices-stub` |
| `postgres`, `scylladb` | `-db` | the owner of the data | `orders-db` |
| `redis` | `-cache` (`-db` when it is the primary store) | the owner | `sessions-cache` |
| `kafka` | `-events` | the owner or stream | `orders-events` |
| `rabbitmq` | `-queue` | the owner or stream | `payments-queue` |
| `s3` | `-storage` | the owner | `invoices-storage` |
| `browser` | `-browser` | the person using it | `alice-browser`, `shopper-browser` |
| `shell` | `-cli` when it runs one tool, else plain `shell` | the tool | `todo-cli`, `shell` |
| `pdf` | `-pdf` | the document | `invoice-pdf` |
| `aws`, `lifecycle` | none: there is one of each | | `aws`, `lifecycle` |

A few consequences:

- **Never a bare type or a generic word.** Not `api`, `http`, `client`, `db`,
  `mock`, `server` or `grpc`: each of them fits every resource of its kind,
  so it tells the reader nothing.
- **The app under test's client is named after the app**: `orders-client`.
  Examples with no particular app, like the ones in this documentation's step
  reference, use `app-client`.
- **Containers keep naming the software.** Entries under `containers:` —
  `postgres`, `kafka`, `minio`, `chrome` — are infrastructure, not roles; only
  entries under `resources:` follow this rule.
- **Templates follow the name**: `{{.stripe-stub.url}}`,
  `{{.catalog-stub.address}}`. A rename covers `tomato.yml`, every feature
  file, and any prose that mentions the resource.

## A suite is configuration, not code

Everything above is `tomato.yml` and `.feature` files. There is no glue code to
write, no test harness to maintain, no fixtures library that drifts away from
the application.

If you find yourself needing to script around tomato — shelling out to set
something up, writing a helper to assert something — that is a gap in tomato,
not a gap in your repository. [Report it](https://github.com/tomatool/tomato/issues);
the fix belongs in tomato as a step, a resource option or a preset, so that
everyone gets it.

## Test it as the thing it is

tomato starts your application the way production does — as a process or a
container — and talks to it over the network. It does not import your code,
call your functions, or know your framework.

That costs some speed and buys you the thing unit tests cannot give: the
scenario exercises your real routing, your real serialisation, your real
database driver, against real dependencies in containers. When it passes, the
thing that passed is the thing you ship.

## See also

- [Getting started](../getting-started/index.md) — installing tomato and writing a first suite
- [Step reference](../resources/index.md) — every step, by resource
- [Configuration reference](../configuration/index.md) — `tomato.yml` in full
