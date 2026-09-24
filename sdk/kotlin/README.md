# tomato for Kotlin (experimental)

Write tomato tests as JUnit 5 tests in Kotlin instead of Gherkin. You get
everything tomato gives a feature file: containers and the app started from
`tomato.yml`, every resource and step, and **fresh state before every test**.
You also get the language: types, IDE completion, loops, helpers.

```kotlin
@TomatoTest // uses tomato.yml
class OrdersTest(private val t: Tomato) {
    private val api = t.http("api")
    private val db = t.db("db")
    private val kafka = t.kafka("kafka")

    @Test
    fun `creating an order emits OrderCreated`() {
        kafka.consume("orders")
        api.post("/orders", mapOf("id" to "order-1", "amount" to 4200, "currency" to "EUR"))
            .status(201)
        db.table("orders").contains(mapOf("id" to "order-1", "status" to "CREATED"))
        kafka.receivesAvro("orders", within = 15.seconds, expected = mapOf("id" to "order-1"))
    }
}
```

## How it works

```
JUnit ──> TomatoExtension ──starts──> tomato serve ──> containers, app, resources
  test ──> t.http("api").post(...) ──HTTP──> runs the step  "api" sends "POST" to "/orders" with json:
```

- `@TomatoTest` starts `tomato serve` once per run, and stops it when JUnit finishes.
- Before every test the extension calls `POST /v1/reset`. That's the same reset
  a Gherkin scenario gets: tables truncated (migration tables kept), queues and
  topics cleared, stubs forgotten, variables cleared.
- The typed wrappers (`http`, `db`, `kafka`, `shell`) build the exact step text a
  feature file would use. Behaviour and error messages are identical to a Gherkin
  run, and there is one implementation of every resource, in tomato.
- Anything not wrapped yet is one call away:
  `t.step("\"cache\" key \"user:1\" is \"John\"")`, with `docString` and `table`
  arguments for multi-line steps.

A failed step throws `TomatoStepFailed`, carrying the step and tomato's message:

```
TomatoStepFailed: "db" table "orders" has "2" rows
  table orders has 1 rows, expected 2
    at com.example.orders.OrdersTest.the same id works again(OrdersTest.kt:34)
```

## Setup

The tomato binary must be on the `PATH`, or set with the `tomato.bin` system
property or the `TOMATO_BIN` environment variable. See
`examples/spring-boot-kotlin`: it uses this SDK through a Gradle composite build
(`includeBuild("../../sdk/kotlin")`), and its `test` task builds the jar first.

## Status

Experimental: the API (`tomato serve`'s HTTP endpoints and this SDK) may change.
Not yet: typed wrappers for every resource (only http, postgres, kafka and shell
today), parallel test classes (one server serialises steps), `before_all` /
`before_scenario` hooks, and publishing to Maven.
