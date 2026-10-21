@rabbitmq
Feature: RabbitMQ Handler
  Test all RabbitMQ handler steps

  # Queue Management
  Scenario: Declare and verify queue exists
    Given "app-queue" declares queue "test-queue-1"
    Then "app-queue" queue "test-queue-1" exists

  Scenario: Declare durable queue
    Given "app-queue" declares durable queue "durable-queue"
    Then "app-queue" queue "durable-queue" exists

  # Exchange Management
  Scenario: Declare and verify exchange exists
    Given "app-queue" declares exchange "test-exchange" of type "direct"
    Then "app-queue" exchange "test-exchange" exists

  Scenario: Declare topic exchange
    Given "app-queue" declares exchange "topic-exchange" of type "topic"
    Then "app-queue" exchange "topic-exchange" exists

  Scenario: Declare fanout exchange
    Given "app-queue" declares exchange "fanout-exchange" of type "fanout"
    Then "app-queue" exchange "fanout-exchange" exists

  # Bindings
  Scenario: Bind queue to exchange
    Given "app-queue" declares queue "bound-queue"
    And "app-queue" declares exchange "binding-exchange" of type "direct"
    When "app-queue" binds queue "bound-queue" to exchange "binding-exchange"
    Then "app-queue" queue "bound-queue" exists

  Scenario: Bind queue with routing key
    Given "app-queue" declares queue "routed-queue"
    And "app-queue" declares exchange "routing-exchange" of type "topic"
    When "app-queue" binds queue "routed-queue" to exchange "routing-exchange" with routing key "order.*"
    Then "app-queue" queue "routed-queue" exists

  # Publishing - Direct to Queue
  Scenario: Publish and consume message from queue
    Given "app-queue" declares queue "simple-queue"
    And "app-queue" consumes from queue "simple-queue"
    When "app-queue" publishes to queue "simple-queue":
      """
      Hello RabbitMQ!
      """
    Then "app-queue" receives from queue "simple-queue" within "5s":
      """
      Hello RabbitMQ!
      """

  Scenario: Publish JSON to queue
    Given "app-queue" declares queue "json-queue"
    And "app-queue" consumes from queue "json-queue"
    When "app-queue" publishes json to queue "json-queue":
      """
      {"event": "user.created", "userId": 123}
      """
    Then "app-queue" receives from queue "json-queue" within "5s"
    And "app-queue" last message contains:
      """
      user.created
      """

  # Publishing - Via Exchange
  Scenario: Publish to exchange with routing key
    Given "app-queue" declares queue "order-queue"
    And "app-queue" declares exchange "order-exchange" of type "topic"
    And "app-queue" binds queue "order-queue" to exchange "order-exchange" with routing key "order.created"
    And "app-queue" consumes from queue "order-queue"
    When "app-queue" publishes to exchange "order-exchange" with routing key "order.created":
      """
      New order placed
      """
    Then "app-queue" receives from queue "order-queue" within "5s":
      """
      New order placed
      """
    And "app-queue" last message has routing key "order.created"

  Scenario: Publish JSON to exchange
    Given "app-queue" declares queue "event-queue"
    And "app-queue" declares exchange "event-exchange" of type "direct"
    And "app-queue" binds queue "event-queue" to exchange "event-exchange" with routing key "events"
    And "app-queue" consumes from queue "event-queue"
    When "app-queue" publishes json to exchange "event-exchange" with routing key "events":
      """
      {"type": "notification", "message": "Hello"}
      """
    Then "app-queue" receives from queue "event-queue" within "5s"
    And "app-queue" last message contains:
      """
      notification
      """

  # Fanout Exchange
  Scenario: Fanout exchange broadcasts to all queues
    Given "app-queue" declares exchange "broadcast" of type "fanout"
    And "app-queue" declares queue "subscriber-1"
    And "app-queue" declares queue "subscriber-2"
    And "app-queue" binds queue "subscriber-1" to exchange "broadcast"
    And "app-queue" binds queue "subscriber-2" to exchange "broadcast"
    And "app-queue" consumes from queue "subscriber-1"
    And "app-queue" consumes from queue "subscriber-2"
    When "app-queue" publishes to exchange "broadcast" with routing key "":
      """
      Broadcast message
      """
    Then "app-queue" receives from queue "subscriber-1" within "5s":
      """
      Broadcast message
      """
    And "app-queue" receives from queue "subscriber-2" within "5s":
      """
      Broadcast message
      """

  # Batch Publishing
  Scenario: Publish multiple messages from table
    Given "app-queue" declares queue "batch-queue"
    And "app-queue" consumes from queue "batch-queue"
    When "app-queue" publishes messages to queue "batch-queue":
      | message         |
      | first message   |
      | second message  |
      | third message   |
    Then "app-queue" receives from queue "batch-queue" within "5s"
    And "app-queue" queue "batch-queue" has "3" messages

  # Assertions
  Scenario: Check empty queue
    Given "app-queue" declares queue "empty-queue"
    And "app-queue" consumes from queue "empty-queue"
    Then "app-queue" queue "empty-queue" is empty

  Scenario: Verify message count
    Given "app-queue" declares queue "count-queue"
    And "app-queue" consumes from queue "count-queue"
    When "app-queue" publishes to queue "count-queue":
      """
      message 1
      """
    And "app-queue" publishes to queue "count-queue":
      """
      message 2
      """
    Then "app-queue" receives from queue "count-queue" within "5s"
    And "app-queue" queue "count-queue" has "2" messages

  # Purge Queue
  Scenario: Purge queue removes all messages
    Given "app-queue" declares queue "purge-queue"
    When "app-queue" publishes to queue "purge-queue":
      """
      message to purge
      """
    And "app-queue" purges queue "purge-queue"
    And "app-queue" consumes from queue "purge-queue"
    Then "app-queue" queue "purge-queue" is empty

  Scenario: Declare a durable exchange
    Given "app-queue" declares durable exchange "durable-events" of type "topic"
    Then "app-queue" exchange "durable-events" exists

  Scenario: Publish a message with headers and assert them
    Given "app-queue" declares queue "header-queue"
    And "app-queue" consumes from queue "header-queue"
    And "app-queue" message header "trace-id" is "abc-123"
    When "app-queue" publishes to queue "header-queue":
      """
      traced message
      """
    Then "app-queue" receives from queue "header-queue" within "5s":
      """
      traced message
      """
    And "app-queue" last message has header "trace-id" with value "abc-123"
