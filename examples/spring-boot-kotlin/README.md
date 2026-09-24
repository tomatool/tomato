# Spring Boot + Kotlin example

A small order service tested black-box with tomato. There is no test code in
the JVM: tomato starts Postgres, Kafka and a Schema Registry, starts the jar,
and drives it through HTTP, the database and Avro messages.

| What it shows | Where |
|---------------|-------|
| REST calls and JSON assertions | `features/orders.feature` |
| Flyway migrations; reference data kept across resets | `tomato.yml` (`exclude: [currencies]`) |
| Avro events emitted by `KafkaAvroSerializer` | `receives avro from "orders"` |
| Avro events consumed by a `@KafkaListener` | `features/payments.feature` |
| Code coverage from black-box tests | JaCoCo agent in `app.command` |
| JUnit XML for CI | `settings.output` |

## Run it

Needs Docker, JDK 21 and tomato.

```bash
./gradlew bootJar          # the app jar + JaCoCo agent
tomato run                 # 6 scenarios
./gradlew tomatoCoverage   # build/reports/tomato-coverage/index.html
```

## The same tests in Kotlin (experimental)

`src/test/kotlin` has the same scenarios as `features/`, written as JUnit 5
tests with the experimental [tomato Kotlin SDK](../../sdk/kotlin). tomato still
starts everything and resets state before every test; the tests are just Kotlin:

```bash
./gradlew test             # starts `tomato serve`, runs 7 tests
```

Each scenario starts with an empty `orders` table, so scenarios reuse the
same ids and can run in any order. Flyway's history and the `currencies`
table seeded by the migration are kept.
