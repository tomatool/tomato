@kafka
Feature: Kafka Handler
  Test all Kafka handler steps

  # Topic Management
  Scenario: Create and verify topic exists
    Given "app-events" creates topic "test-topic-1"
    Then "app-events" topic "test-topic-1" exists

  Scenario: Create topic with partitions
    Given "app-events" creates topic "partitioned-topic" with "3" partitions
    Then "app-events" topic "partitioned-topic" exists

  # A topic tomato creates can be consumed as soon as the step returns. The
  # broker answers CreateTopics before it serves the new partitions, and a
  # consume in between failed with "not the leader for some partition".
  Scenario Outline: A topic tomato just created can be consumed at once
    Given "app-events" creates topic "<topic>" with "<partitions>" partitions
    And "app-events" consumes from "<topic>"
    When "app-events" publishes to "<topic>":
      """
      fresh
      """
    Then "app-events" receives from "<topic>" within "10s":
      """
      fresh
      """

    Examples:
      | topic   | partitions |
      | fresh-1 | 1          |
      | fresh-2 | 3          |
      | fresh-3 | 1          |
      | fresh-4 | 3          |
      | fresh-5 | 1          |
      | fresh-6 | 3          |

  # Publishing - Simple messages
  Scenario: Publish and consume simple message
    Given "app-events" creates topic "simple-topic"
    And "app-events" consumes from "simple-topic"
    When "app-events" publishes to "simple-topic":
      """
      Hello Kafka!
      """
    Then "app-events" receives from "simple-topic" within "10s":
      """
      Hello Kafka!
      """

  # Publishing - Messages with key
  Scenario: Publish message with key
    Given "app-events" creates topic "keyed-topic"
    And "app-events" consumes from "keyed-topic"
    When "app-events" publishes to "keyed-topic" with key "user-123":
      """
      User message content
      """
    Then "app-events" receives from "keyed-topic" with key "user-123" within "10s"
    And "app-events" last message has key "user-123"

  # Publishing - JSON messages
  Scenario: Publish JSON message
    Given "app-events" creates topic "json-topic"
    And "app-events" consumes from "json-topic"
    When "app-events" publishes json to "json-topic":
      """
      {"event": "user.created", "userId": 123}
      """
    Then "app-events" receives from "json-topic" within "10s"
    And "app-events" last message contains:
      """
      user.created
      """

  Scenario: Publish JSON message with key
    Given "app-events" creates topic "json-keyed-topic"
    And "app-events" consumes from "json-keyed-topic"
    When "app-events" publishes json to "json-keyed-topic" with key "order-456":
      """
      {"event": "order.placed", "orderId": 456}
      """
    Then "app-events" receives from "json-keyed-topic" with key "order-456" within "10s"
    And "app-events" last message has key "order-456"
    And "app-events" last message contains:
      """
      order.placed
      """

  # Publishing - Batch messages
  Scenario: Publish multiple messages from table
    Given "app-events" creates topic "batch-topic" with "1" partitions
    And "app-events" consumes from "batch-topic"
    When "app-events" publishes messages to "batch-topic":
      | key    | value           |
      | key-1  | first message   |
      | key-2  | second message  |
      | key-3  | third message   |
    # Wait for the first message to arrive (which triggers background consumption)
    Then "app-events" receives from "batch-topic" within "5s"
    # Check that all 3 messages were consumed
    And "app-events" topic "batch-topic" has "3" messages

  # Consuming - Wait for message
  Scenario: Wait for message within timeout
    Given "app-events" creates topic "wait-topic"
    And "app-events" consumes from "wait-topic"
    When "app-events" publishes to "wait-topic":
      """
      Async message
      """
    Then "app-events" receives from "wait-topic" within "5s"

  # Assertions - Empty topic
  Scenario: Check empty topic
    Given "app-events" creates topic "empty-topic"
    And "app-events" consumes from "empty-topic"
    Then "app-events" topic "empty-topic" is empty

  # Assertions - Message count
  Scenario: Verify message count
    Given "app-events" creates topic "count-topic" with "1" partitions
    And "app-events" consumes from "count-topic"
    When "app-events" publishes to "count-topic":
      """
      message 1
      """
    And "app-events" publishes to "count-topic":
      """
      message 2
      """
    # Wait for first message to trigger background consumption
    Then "app-events" receives from "count-topic" within "5s"
    # Verify both messages were consumed
    And "app-events" topic "count-topic" has "2" messages

  # Assertions - Message ordering
  Scenario: Verify messages received in order
    Given "app-events" creates topic "order-topic" with "1" partitions
    And "app-events" consumes from "order-topic"
    When "app-events" publishes messages to "order-topic":
      | key  | value    |
      | k1   | first    |
      | k2   | second   |
      | k3   | third    |
    # Wait for messages to arrive (background consumer buffers them)
    Then "app-events" receives from "order-topic" within "5s"
    # Verify all 3 messages arrived and are in order
    And "app-events" topic "order-topic" has "3" messages
    And "app-events" receives messages from "order-topic" in order:
      | key  | value    |
      | k1   | first    |
      | k2   | second   |
      | k3   | third    |

  # Headers
  Scenario: Publish a message with headers and assert them
    Given "app-events" creates topic "headers-topic"
    And "app-events" consumes from "headers-topic"
    And "app-events" message header "trace-id" is "abc-123"
    And "app-events" message header "content-type" is "text/plain"
    When "app-events" publishes to "headers-topic":
      """
      traced message
      """
    Then "app-events" receives from "headers-topic" within "10s":
      """
      traced message
      """
    And "app-events" last message has header "trace-id" with value "abc-123"
    And "app-events" last message has header "content-type" with value "text/plain"

  Scenario: The broker sees a consumer group's members and assignments
    Given "app-events" consumes from "test-events" as consumer group "tomato-tests"
    Then "app-events" consumer group "tomato-tests" is consuming "test-events" within "30s"
    When "app-events" publishes to "test-events":
      """
      hello group
      """
    Then "app-events" receives from "test-events" within "10s":
      """
      hello group
      """

  Scenario: A consumer group tomato left has no members
    Then "app-events" consumer group "tomato-tests" has no members
