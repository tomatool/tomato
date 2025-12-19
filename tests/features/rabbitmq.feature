@rabbitmq
Feature: RabbitMQ Handler
  Test all RabbitMQ handler steps

  # Queue Management
  Scenario: Declare and verify queue exists
    Given "mq" declares queue "test-queue-1"
    Then "mq" queue "test-queue-1" exists

  Scenario: Declare durable queue
    Given "mq" declares durable queue "durable-queue"
    Then "mq" queue "durable-queue" exists

  # Exchange Management
  Scenario: Declare and verify exchange exists
    Given "mq" declares exchange "test-exchange" of type "direct"
    Then "mq" exchange "test-exchange" exists

  Scenario: Declare topic exchange
    Given "mq" declares exchange "topic-exchange" of type "topic"
    Then "mq" exchange "topic-exchange" exists

  Scenario: Declare fanout exchange
    Given "mq" declares exchange "fanout-exchange" of type "fanout"
    Then "mq" exchange "fanout-exchange" exists

  # Bindings
  Scenario: Bind queue to exchange
    Given "mq" declares queue "bound-queue"
    And "mq" declares exchange "binding-exchange" of type "direct"
    When "mq" binds queue "bound-queue" to exchange "binding-exchange"
    Then "mq" queue "bound-queue" exists

  Scenario: Bind queue with routing key
    Given "mq" declares queue "routed-queue"
    And "mq" declares exchange "routing-exchange" of type "topic"
    When "mq" binds queue "routed-queue" to exchange "routing-exchange" with routing key "order.*"
    Then "mq" queue "routed-queue" exists

  # Publishing - Direct to Queue
  Scenario: Publish and consume message from queue
    Given "mq" declares queue "simple-queue"
    And "mq" consumes from queue "simple-queue"
    When "mq" publishes to queue "simple-queue":
      """
      Hello RabbitMQ!
      """
    Then "mq" receives from queue "simple-queue" within "15s":
      """
      Hello RabbitMQ!
      """

  Scenario: Publish JSON to queue
    Given "mq" declares queue "json-queue"
    And "mq" consumes from queue "json-queue"
    When "mq" publishes json to queue "json-queue":
      """
      {"event": "user.created", "userId": 123}
      """
    Then "mq" receives from queue "json-queue" within "15s"
    And "mq" last message contains:
      """
      user.created
      """

  # Publishing - Via Exchange
  Scenario: Publish to exchange with routing key
    Given "mq" declares queue "order-queue"
    And "mq" declares exchange "order-exchange" of type "topic"
    And "mq" binds queue "order-queue" to exchange "order-exchange" with routing key "order.created"
    And "mq" consumes from queue "order-queue"
    When "mq" publishes to exchange "order-exchange" with routing key "order.created":
      """
      New order placed
      """
    Then "mq" receives from queue "order-queue" within "15s":
      """
      New order placed
      """
    And "mq" last message has routing key "order.created"

  Scenario: Publish JSON to exchange
    Given "mq" declares queue "event-queue"
    And "mq" declares exchange "event-exchange" of type "direct"
    And "mq" binds queue "event-queue" to exchange "event-exchange" with routing key "events"
    And "mq" consumes from queue "event-queue"
    When "mq" publishes json to exchange "event-exchange" with routing key "events":
      """
      {"type": "notification", "message": "Hello"}
      """
    Then "mq" receives from queue "event-queue" within "15s"
    And "mq" last message contains:
      """
      notification
      """

  # Fanout Exchange
  Scenario: Fanout exchange broadcasts to all queues
    Given "mq" declares exchange "broadcast" of type "fanout"
    And "mq" declares queue "subscriber-1"
    And "mq" declares queue "subscriber-2"
    And "mq" binds queue "subscriber-1" to exchange "broadcast"
    And "mq" binds queue "subscriber-2" to exchange "broadcast"
    And "mq" consumes from queue "subscriber-1"
    And "mq" consumes from queue "subscriber-2"
    When "mq" publishes to exchange "broadcast" with routing key "":
      """
      Broadcast message
      """
    Then "mq" receives from queue "subscriber-1" within "15s":
      """
      Broadcast message
      """
    And "mq" receives from queue "subscriber-2" within "15s":
      """
      Broadcast message
      """

  # Batch Publishing
  Scenario: Publish multiple messages from table
    Given "mq" declares queue "batch-queue"
    And "mq" consumes from queue "batch-queue"
    When "mq" publishes messages to queue "batch-queue":
      | message         |
      | first message   |
      | second message  |
      | third message   |
    Then "mq" receives from queue "batch-queue" within "15s"
    And "mq" queue "batch-queue" has "3" messages

  # Assertions
  Scenario: Check empty queue
    Given "mq" declares queue "empty-queue"
    And "mq" consumes from queue "empty-queue"
    Then "mq" queue "empty-queue" is empty

  Scenario: Verify message count
    Given "mq" declares queue "count-queue"
    And "mq" consumes from queue "count-queue"
    When "mq" publishes to queue "count-queue":
      """
      message 1
      """
    And "mq" publishes to queue "count-queue":
      """
      message 2
      """
    Then "mq" receives from queue "count-queue" within "15s"
    And "mq" queue "count-queue" has "2" messages

  # Purge Queue
  Scenario: Purge queue removes all messages
    Given "mq" declares queue "purge-queue"
    When "mq" publishes to queue "purge-queue":
      """
      message to purge
      """
    And "mq" purges queue "purge-queue"
    And "mq" consumes from queue "purge-queue"
    Then "mq" queue "purge-queue" is empty
