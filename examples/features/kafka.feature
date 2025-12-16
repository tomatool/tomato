Feature: Kafka Message Processing
  As a developer
  I want to test Kafka message publishing and consuming
  So that I can verify event-driven behavior in my application

  Scenario: Publish and consume a simple message
    Given I create kafka topic "orders" on "kafka"
    And I start consuming from "kafka" topic "orders"
    When I publish message to "kafka" topic "orders":
      """
      {"order_id": "123", "status": "created", "total": 99.99}
      """
    Then I should receive message from "kafka" topic "orders" within "10s":
      """
      {"order_id": "123", "status": "created", "total": 99.99}
      """

  Scenario: Publish message with key
    Given I create kafka topic "events" on "kafka"
    And I start consuming from "kafka" topic "events"
    When I publish message to "kafka" topic "events" with key "user-123":
      """
      {"event": "user.login", "user_id": "123", "timestamp": "2024-01-01T00:00:00Z"}
      """
    Then I should receive message from "kafka" topic "events" with key "user-123" within "10s"
    And the last message from "kafka" should have key "user-123"
    And the last message from "kafka" should contain:
      """
      user.login
      """

  Scenario: Publish JSON and verify content
    Given I create kafka topic "notifications" on "kafka"
    And I start consuming from "kafka" topic "notifications"
    When I publish JSON to "kafka" topic "notifications":
      """
      {
        "type": "email",
        "recipient": "alice@test.com",
        "subject": "Welcome!",
        "body": "Thanks for signing up"
      }
      """
    Then I consume message from "kafka" topic "notifications" within "10s"
    And the last message from "kafka" should contain:
      """
      alice@test.com
      """

  Scenario: Publish multiple messages
    Given I create kafka topic "orders" on "kafka"
    And I start consuming from "kafka" topic "orders"
    When I publish messages to "kafka" topic "orders":
      | key       | value                                        |
      | order-1   | {"order_id": "1", "status": "created"}       |
      | order-2   | {"order_id": "2", "status": "created"}       |
      | order-3   | {"order_id": "3", "status": "created"}       |
    Then "kafka" topic "orders" should have "3" messages

  Scenario: Message ordering verification
    Given I create kafka topic "order-events" on "kafka"
    And I start consuming from "kafka" topic "order-events"
    When I publish messages to "kafka" topic "order-events":
      | key       | value                                        |
      | order-123 | {"event": "created", "order_id": "123"}      |
      | order-123 | {"event": "paid", "order_id": "123"}         |
      | order-123 | {"event": "shipped", "order_id": "123"}      |
    Then I should receive messages from "kafka" topic "order-events" in order:
      | key       | value    |
      | order-123 | created  |
      | order-123 | paid     |
      | order-123 | shipped  |

  Scenario: Empty topic after reset
    # This scenario verifies that reset works correctly
    # The topic should be empty after reset
    Given I create kafka topic "test-reset" on "kafka"
    And I start consuming from "kafka" topic "test-reset"
    Then "kafka" topic "test-reset" should be empty
