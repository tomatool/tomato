@kafka
Feature: Kafka Handler
  Test all Kafka handler steps

  Scenario: Create topic and publish message
    Given I create kafka topic "test-topic" on "events"
    Then kafka topic "test-topic" exists on "events"

  Scenario: Publish and consume simple message
    Given I create kafka topic "orders" on "events"
    And I start consuming from "events" topic "orders"
    When I publish message to "events" topic "orders":
      """
      Hello Kafka!
      """
    Then I should receive message from "events" topic "orders" within "10s":
      """
      Hello Kafka!
      """

  Scenario: Publish message with key
    Given I create kafka topic "keyed-topic" on "events"
    And I start consuming from "events" topic "keyed-topic"
    When I publish message to "events" topic "keyed-topic" with key "user-123":
      """
      User message
      """
    Then I should receive message from "events" topic "keyed-topic" with key "user-123" within "10s"
    And the last message from "events" should have key "user-123"

  Scenario: Publish JSON message
    Given I create kafka topic "json-topic" on "events"
    And I start consuming from "events" topic "json-topic"
    When I publish JSON to "events" topic "json-topic":
      """
      {"event": "user.created", "userId": 123}
      """
    Then I should receive message from "events" topic "json-topic" within "10s":
      """
      {"event": "user.created", "userId": 123}
      """
    And the last message from "events" should contain:
      """
      user.created
      """

  Scenario: Publish multiple messages
    Given I create kafka topic "batch-topic" on "events" with "1" partitions
    And I start consuming from "events" topic "batch-topic"
    When I publish messages to "events" topic "batch-topic":
      | message  |
      | first    |
      | second   |
      | third    |
    Then "events" topic "batch-topic" should have "3" messages

  Scenario: Check empty topic
    Given I create kafka topic "empty-topic" on "events"
    And I start consuming from "events" topic "empty-topic"
    Then "events" topic "empty-topic" should be empty
