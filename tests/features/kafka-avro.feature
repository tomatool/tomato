@kafka @avro
Feature: Kafka Avro with Schema Registry
  Publish JSON as Confluent wire-format Avro and assert on consumed Avro as JSON

  Scenario: Round-trip an Avro record through a topic
    Given "app-events" registers schema for subject "avro-orders-value" from file "tests/testdata/order.avsc"
    And "app-events" creates topic "avro-orders"
    And "app-events" consumes from "avro-orders"
    When "app-events" publishes avro to "avro-orders" with key "order-1":
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR", "note": "rush"}
      """
    Then "app-events" receives avro from "avro-orders" within "10s":
      """
      {"id": "order-1", "currency": "EUR"}
      """
    And "app-events" last message has key "order-1"
    And "app-events" last message avro matches:
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR", "note": "rush"}
      """

  Scenario: Nullable fields use plain JSON, not Avro-JSON unions
    Given "app-events" registers schema for subject "avro-nulls-value" from file "tests/testdata/order.avsc"
    And "app-events" creates topic "avro-nulls"
    And "app-events" consumes from "avro-nulls"
    When "app-events" publishes avro to "avro-nulls":
      """
      {"id": "order-2", "amount": 1, "currency": "USD", "note": null}
      """
    Then "app-events" receives avro from "avro-nulls" within "10s":
      """
      {"id": "order-2", "note": null}
      """
    And "app-events" last message avro contains:
      """
      {"amount": 1, "currency": "USD"}
      """

  Scenario: Wait for the matching message among several
    Given "app-events" registers schema for subject "avro-many-value" from file "tests/testdata/order.avsc"
    And "app-events" creates topic "avro-many"
    And "app-events" consumes from "avro-many"
    When "app-events" publishes avro to "avro-many":
      """
      {"id": "first", "amount": 1, "currency": "EUR"}
      """
    And "app-events" publishes avro to "avro-many":
      """
      {"id": "second", "amount": 2, "currency": "EUR"}
      """
    Then "app-events" receives avro from "avro-many" within "10s":
      """
      {"id": "second"}
      """
    And "app-events" last message avro contains:
      """
      {"amount": 2}
      """

  Scenario: Inline schema and a subject mapped in config
    Given "app-events" registers schema for subject "com.example.Order":
      """
      {
        "type": "record",
        "name": "Order",
        "namespace": "com.example",
        "fields": [{"name": "id", "type": "string"}]
      }
      """
    And "app-events" creates topic "mapped-orders"
    And "app-events" consumes from "mapped-orders"
    When "app-events" publishes avro to "mapped-orders":
      """
      {"id": "mapped-1"}
      """
    Then "app-events" receives avro from "mapped-orders" within "10s":
      """
      {"id": "mapped-1"}
      """
