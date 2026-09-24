# Testing JVM and Kotlin Services

Tomato tests a service from the outside, over the network, so the language
the service is written in doesn't matter. For a Spring Boot, Ktor or Micronaut
service this means black-box tests with no test code in the JVM, no
Testcontainers setup in Gradle, and no mocks. A complete, runnable example
lives in [`examples/spring-boot-kotlin`](https://github.com/tomatool/tomato/tree/main/examples/spring-boot-kotlin).

## Start the jar

Build the jar first, then let tomato start it as a local process:

```yaml
app:
  command: java -jar build/libs/orders.jar
  port: 8080
  ready:
    type: http
    path: /actuator/health
    timeout: 120s      # JVM startup plus Flyway migrations
  env:
    SPRING_DATASOURCE_URL: "jdbc:postgresql://{{.postgres.host}}:{{.postgres.port.5432}}/orders"
    SPRING_KAFKA_BOOTSTRAP_SERVERS: "localhost:9092"
    SPRING_KAFKA_PROPERTIES_SCHEMA_REGISTRY_URL: "http://{{.schema-registry.host}}:{{.schema-registry.port.8081}}"
```

Container addresses are injected as environment variables, which Spring
Boot maps onto its properties. `./gradlew bootRun` also works, but a built jar
starts faster and matches what you deploy.

## Fresh state every scenario

Before each scenario tomato truncates the database tables and clears what it
consumed from Kafka. Scenarios can reuse the same ids, run in any order, and
never see each other's data. For JVM migration tools:

- Flyway's `flyway_schema_history` and Liquibase's `databasechangelog` /
  `databasechangeloglock` are never truncated.
- Reference data a migration inserts (currencies, roles, countries) is kept by
  listing the tables under `exclude`:

```yaml
resources:
  db:
    type: postgres
    container: postgres
    database: orders
    options:
      exclude: [currencies]
```

## Kafka with Avro

Services using `KafkaAvroSerializer` and `KafkaAvroDeserializer` are tested
with plain JSON in the feature files. Tomato encodes and decodes the Confluent
wire format through the Schema Registry:

```gherkin
Given "kafka" registers schema for subject "payments-value" from file "src/main/resources/avro/payment_received.avsc"
When "kafka" publishes avro to "payments":
  """
  {"paymentId": "pay-1", "orderId": "order-7", "amount": 4200}
  """
Then "kafka" receives avro from "orders-paid" within "20s":
  """
  {"id": "order-7"}
  """
```

See [Kafka: Avro and Schema Registry](../configuration/kafka.md#avro-and-schema-registry).
If the app creates its topics (`NewTopic` beans), set `reset_strategy: none`
on the Kafka resource so tomato doesn't delete them between scenarios.

## Code coverage

Attach the JaCoCo agent in `app.command`. The JVM writes the execution data
when tomato stops the app:

```yaml
app:
  command: java -javaagent:build/jacoco/jacocoagent.jar=destfile=build/jacoco/tomato.exec -jar build/libs/orders.jar
```

The example's `build.gradle.kts` copies the agent (`org.jacoco:org.jacoco.agent:<version>:runtime`)
next to the jar and adds a `tomatoCoverage` report task. On the example, 6
scenarios cover every line and branch of the service.

## CI

```yaml
- uses: actions/setup-java@v4
  with: { distribution: temurin, java-version: "21" }
- run: ./gradlew bootJar
- uses: tomatool/tomato@v2
- run: ./gradlew tomatoCoverage
```

Add `junit:build/reports/tomato/junit.xml` to `settings.output` to publish
test results. See [Reports](../configuration/index.md#reports).
