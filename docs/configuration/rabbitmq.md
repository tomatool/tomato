# RabbitMQ Configuration

This guide covers how to configure RabbitMQ for integration testing with tomato.

## Overview

RabbitMQ is a popular message broker that implements AMQP (Advanced Message Queuing Protocol). Unlike Kafka's topic-based model, RabbitMQ uses an exchange/queue/binding model for message routing.

## RabbitMQ Concepts

| Concept | Description |
|---------|-------------|
| **Queue** | A buffer that stores messages |
| **Exchange** | Routes messages to queues based on routing rules |
| **Binding** | A link between an exchange and a queue with an optional routing key |
| **Routing Key** | A message attribute used by exchanges to route messages |

### Exchange Types

| Type | Description |
|------|-------------|
| `direct` | Routes messages to queues with matching routing key |
| `fanout` | Broadcasts messages to all bound queues (ignores routing key) |
| `topic` | Routes messages based on routing key patterns (wildcards: `*` single word, `#` zero or more words) |
| `headers` | Routes based on message headers instead of routing key |

## Container Setup

Add a RabbitMQ container to your `tomato.yml`:

```yaml
containers:
  rabbitmq:
    image: rabbitmq:3-alpine
    ports:
      - "5672/tcp"
    env:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    wait_for:
      type: port
      target: "5672"
      timeout: 30s
```

### Management UI (Optional)

For debugging, you can use the management image which includes a web UI:

```yaml
containers:
  rabbitmq:
    image: rabbitmq:3-management-alpine
    ports:
      - "5672/tcp"
      - "15672/tcp"  # Management UI
    env:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    wait_for:
      type: port
      target: "5672"
      timeout: 30s
```

Access the management UI at `http://localhost:<mapped-port>` with credentials `guest/guest`.

## Resource Configuration

Configure the RabbitMQ resource:

```yaml
resources:
  mq:
    type: rabbitmq
    container: rabbitmq
    options:
      user: guest
      password: guest
      vhost: /
      reset_strategy: purge
```

### Resource Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `user` | string | `guest` | RabbitMQ username |
| `password` | string | `guest` | RabbitMQ password |
| `vhost` | string | `/` | Virtual host to connect to |
| `reset_strategy` | string | `purge` | How to reset between scenarios |
| `queues` | list | - | Pre-declare queues on init |
| `exchanges` | list | - | Pre-declare exchanges on init |
| `bindings` | list | - | Pre-declare bindings on init |

### Reset Strategies

| Strategy | Description |
|----------|-------------|
| `purge` | Purge messages from all declared queues (recommended, fastest) |
| `delete_recreate` | Delete and recreate all declared queues and exchanges |
| `none` | No reset between scenarios |

### Pre-declaring Resources

You can pre-declare queues, exchanges, and bindings in the config:

```yaml
resources:
  mq:
    type: rabbitmq
    container: rabbitmq
    options:
      user: guest
      password: guest
      reset_strategy: purge
      queues:
        - name: orders
          durable: true
        - name: notifications
          auto_delete: true
      exchanges:
        - name: events
          type: topic
          durable: true
        - name: broadcast
          type: fanout
      bindings:
        - queue: orders
          exchange: events
          routing_key: "order.*"
        - queue: notifications
          exchange: broadcast
```

## Complete Example

Here's a complete `tomato.yml` with RabbitMQ:

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: false
  output: pretty
  reset:
    level: scenario

containers:
  rabbitmq:
    image: rabbitmq:3-alpine
    ports:
      - "5672/tcp"
    env:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    wait_for:
      type: port
      target: "5672"
      timeout: 30s

resources:
  mq:
    type: rabbitmq
    container: rabbitmq
    options:
      user: guest
      password: guest
      reset_strategy: purge

features:
  paths:
    - ./features
```

## Writing RabbitMQ Tests

### Basic Queue Operations

```gherkin
Feature: Order Processing

  Scenario: Orders are queued for processing
    Given "mq" declares queue "orders"
    And "mq" consumes from queue "orders"
    When "mq" publishes json to queue "orders":
      """
      {"order_id": "123", "amount": 99.99}
      """
    Then "mq" receives from queue "orders" within "5s"
    And "mq" last message contains:
      """
      order_id
      """
```

### Topic Exchange Routing

```gherkin
Scenario: Route orders by type
  Given "mq" declares exchange "orders" of type "topic"
  And "mq" declares queue "domestic-orders"
  And "mq" declares queue "international-orders"
  And "mq" binds queue "domestic-orders" to exchange "orders" with routing key "order.domestic.*"
  And "mq" binds queue "international-orders" to exchange "orders" with routing key "order.international.*"
  And "mq" consumes from queue "domestic-orders"
  When "mq" publishes to exchange "orders" with routing key "order.domestic.new":
    """
    New domestic order
    """
  Then "mq" receives from queue "domestic-orders" within "5s":
    """
    New domestic order
    """
```

### Fanout Exchange Broadcasting

```gherkin
Scenario: Broadcast notifications to all subscribers
  Given "mq" declares exchange "notifications" of type "fanout"
  And "mq" declares queue "email-service"
  And "mq" declares queue "push-service"
  And "mq" declares queue "sms-service"
  And "mq" binds queue "email-service" to exchange "notifications"
  And "mq" binds queue "push-service" to exchange "notifications"
  And "mq" binds queue "sms-service" to exchange "notifications"
  And "mq" consumes from queue "email-service"
  And "mq" consumes from queue "push-service"
  When "mq" publishes to exchange "notifications" with routing key "":
    """
    System maintenance in 1 hour
    """
  Then "mq" receives from queue "email-service" within "5s":
    """
    System maintenance in 1 hour
    """
  And "mq" receives from queue "push-service" within "5s":
    """
    System maintenance in 1 hour
    """
```

See [RabbitMQ Steps](../resources/rabbitmq.md) for the complete list of available steps.

## Troubleshooting

### Connection Refused

If tests fail with "connection refused":

1. Verify RabbitMQ container is healthy: `docker ps`
2. Check the mapped port: `docker port <container_id>`
3. Ensure credentials match configuration

### Consumer Not Receiving Messages

1. Ensure `consumes from queue` is called before publishing
2. Verify the queue exists and is bound to the correct exchange
3. Check routing keys match the binding patterns

### Messages Not Routing

For topic exchanges, verify your routing key patterns:
- `*` matches exactly one word (e.g., `order.*` matches `order.created` but not `order.created.urgent`)
- `#` matches zero or more words (e.g., `order.#` matches `order`, `order.created`, and `order.created.urgent`)

### Queue Already Exists with Different Properties

If you get "PRECONDITION_FAILED" errors:
- Queues and exchanges are idempotent but properties must match
- Either delete the queue manually or use `reset_strategy: delete_recreate`
