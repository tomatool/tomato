@kafka
Feature: Kafka Handler
  Test all Kafka handler steps

  # Topic Management
  Scenario: Create and verify topic exists
    Given "events" creates topic "test-topic-1"
    Then "events" topic "test-topic-1" exists

  Scenario: Create topic with partitions
    Given "events" creates topic "partitioned-topic" with "3" partitions
    Then "events" topic "partitioned-topic" exists

  # Publishing - Simple messages
  Scenario: Publish and consume simple message
    Given "events" creates topic "simple-topic"
    And "events" consumes from "simple-topic"
    When "events" publishes to "simple-topic":
      """
      Hello Kafka!
      """
    Then "events" receives from "simple-topic" within "10s":
      """
      Hello Kafka!
      """

  # Publishing - Messages with key
  Scenario: Publish message with key
    Given "events" creates topic "keyed-topic"
    And "events" consumes from "keyed-topic"
    When "events" publishes to "keyed-topic" with key "user-123":
      """
      User message content
      """
    Then "events" receives from "keyed-topic" with key "user-123" within "10s"
    And "events" last message has key "user-123"

  # Publishing - JSON messages
  Scenario: Publish JSON message
    Given "events" creates topic "json-topic"
    And "events" consumes from "json-topic"
    When "events" publishes json to "json-topic":
      """
      {"event": "user.created", "userId": 123}
      """
    Then "events" receives from "json-topic" within "10s"
    And "events" last message contains:
      """
      user.created
      """

  Scenario: Publish JSON message with key
    Given "events" creates topic "json-keyed-topic"
    And "events" consumes from "json-keyed-topic"
    When "events" publishes json to "json-keyed-topic" with key "order-456":
      """
      {"event": "order.placed", "orderId": 456}
      """
    Then "events" receives from "json-keyed-topic" with key "order-456" within "10s"
    And "events" last message has key "order-456"
    And "events" last message contains:
      """
      order.placed
      """

  # Publishing - Batch messages
  Scenario: Publish multiple messages from table
    Given "events" creates topic "batch-topic" with "1" partitions
    And "events" consumes from "batch-topic"
    When "events" publishes messages to "batch-topic":
      | key    | value           |
      | key-1  | first message   |
      | key-2  | second message  |
      | key-3  | third message   |
    # Wait for the first message to arrive (which triggers background consumption)
    Then "events" receives from "batch-topic" within "15s"
    # Check that all 3 messages were consumed
    And "events" topic "batch-topic" has "3" messages

  # Consuming - Wait for message
  Scenario: Wait for message within timeout
    Given "events" creates topic "wait-topic"
    And "events" consumes from "wait-topic"
    When "events" publishes to "wait-topic":
      """
      Async message
      """
    Then "events" receives from "wait-topic" within "15s"

  # Assertions - Empty topic
  Scenario: Check empty topic
    Given "events" creates topic "empty-topic"
    And "events" consumes from "empty-topic"
    Then "events" topic "empty-topic" is empty

  # Assertions - Message count
  Scenario: Verify message count
    Given "events" creates topic "count-topic" with "1" partitions
    And "events" consumes from "count-topic"
    When "events" publishes to "count-topic":
      """
      message 1
      """
    And "events" publishes to "count-topic":
      """
      message 2
      """
    # Wait for first message to trigger background consumption
    Then "events" receives from "count-topic" within "15s"
    # Verify both messages were consumed
    And "events" topic "count-topic" has "2" messages

  # Assertions - Message ordering
  Scenario: Verify messages received in order
    Given "events" creates topic "order-topic" with "1" partitions
    And "events" consumes from "order-topic"
    When "events" publishes messages to "order-topic":
      | key  | value    |
      | k1   | first    |
      | k2   | second   |
      | k3   | third    |
    # Wait for messages to arrive (background consumer buffers them)
    Then "events" receives from "order-topic" within "15s"
    # Verify all 3 messages arrived and are in order
    And "events" topic "order-topic" has "3" messages
    And "events" receives messages from "order-topic" in order:
      | key  | value    |
      | k1   | first    |
      | k2   | second   |
      | k3   | third    |
