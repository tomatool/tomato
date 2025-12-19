# RabbitMQ

Steps for interacting with RabbitMQ message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Queue Management

| Step | Description |
|------|-------------|
| `"{resource}" declares queue "{queue}"` | Declares a queue with default settings |
| `"{resource}" declares durable queue "{queue}"` | Declares a durable queue |
| `"{resource}" queue "{queue}" exists` | Asserts a queue exists |
| `"{resource}" purges queue "{queue}"` | Purges all messages from a queue |


## Exchange Management

| Step | Description |
|------|-------------|
| `"{resource}" declares exchange "{exchange}" of type "{type}"` | Declares an exchange (direct, fanout, topic, headers) |
| `"{resource}" declares durable exchange "{exchange}" of type "{type}"` | Declares a durable exchange |
| `"{resource}" exchange "{exchange}" exists` | Asserts an exchange exists |


## Bindings

| Step | Description |
|------|-------------|
| `"{resource}" binds queue "{queue}" to exchange "{exchange}"` | Binds a queue to an exchange with empty routing key |
| `"{resource}" binds queue "{queue}" to exchange "{exchange}" with routing key "{key}"` | Binds a queue to an exchange with a routing key |


## Publishing

| Step | Description |
|------|-------------|
| `"{resource}" publishes to queue "{queue}":` | Publishes a message directly to a queue |
| `"{resource}" publishes json to queue "{queue}":` | Publishes a JSON message directly to a queue |
| `"{resource}" publishes to exchange "{exchange}" with routing key "{key}":` | Publishes a message to an exchange with routing key |
| `"{resource}" publishes json to exchange "{exchange}" with routing key "{key}":` | Publishes a JSON message to an exchange |
| `"{resource}" publishes messages to queue "{queue}":` | Publishes multiple messages from a table |

### Examples

**Publish to queue:**
```gherkin
When "mq" publishes to queue "orders":
  """
  {"order_id": 123, "status": "pending"}
  """
```

**Publish JSON to queue:**
```gherkin
When "mq" publishes json to queue "orders":
  """
  {"order_id": 123, "status": "pending"}
  """
```

**Publish to exchange:**
```gherkin
When "mq" publishes to exchange "events" with routing key "order.created":
  """
  Order created
  """
```

**Publish multiple messages:**
```gherkin
When "mq" publishes messages to queue "orders":
  | message           |
  | order message 1   |
  | order message 2   |
```


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from queue "{queue}"` | Starts consuming messages from a queue |
| `"{resource}" receives from queue "{queue}" within "{timeout}"` | Waits for a message from a queue within timeout |
| `"{resource}" receives from queue "{queue}" within "{timeout}":` | Asserts a specific message is received |

### Examples

**Wait for specific message:**
```gherkin
Then "mq" receives from queue "orders" within "5s":
  """
  order_id
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" queue "{queue}" has "{n}" messages` | Asserts queue has exactly N messages consumed |
| `"{resource}" queue "{queue}" is empty` | Asserts no messages have been consumed from queue |
| `"{resource}" last message contains:` | Asserts the last consumed message contains content |
| `"{resource}" last message has routing key "{key}"` | Asserts the last consumed message has specific routing key |
| `"{resource}" last message has header "{name}" with value "{value}"` | Asserts the last message has a header with value |

### Examples

**Assert message content:**
```gherkin
Then "mq" last message contains:
  """
  order_id
  """
```
