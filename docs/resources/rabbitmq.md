# RabbitMQ

Steps for interacting with RabbitMQ message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Queue Management

| Step | Description |
|------|-------------|
| `"orders-queue" declares queue "orders"` | Declares a queue with default settings |
| `"orders-queue" declares durable queue "orders"` | Declares a durable queue |
| `"orders-queue" queue "orders" exists` | Asserts a queue exists |
| `"orders-queue" purges queue "orders"` | Purges all messages from a queue |



## Exchange Management

| Step | Description |
|------|-------------|
| `"orders-queue" declares exchange "events" of type "topic"` | Declares an exchange (direct, fanout, topic, headers) |
| `"orders-queue" declares durable exchange "events" of type "topic"` | Declares a durable exchange |
| `"orders-queue" exchange "events" exists` | Asserts an exchange exists |



## Bindings

| Step | Description |
|------|-------------|
| `"orders-queue" binds queue "orders" to exchange "events"` | Binds a queue to an exchange with empty routing key |
| `"orders-queue" binds queue "orders" to exchange "events" with routing key "order.*"` | Binds a queue to an exchange with a routing key |



## Publishing

| Step | Description |
|------|-------------|
| `"orders-queue" message header "trace-id" is "abc-123"` | Sets a header on the next message published (any publish step) |
| `"orders-queue" publishes to queue "orders":` | Publishes a message directly to a queue |
| `"orders-queue" publishes json to queue "orders":` | Publishes a JSON message directly to a queue |
| `"orders-queue" publishes to exchange "events" with routing key "order.created":` | Publishes a message to an exchange with routing key |
| `"orders-queue" publishes json to exchange "events" with routing key "order.created":` | Publishes a JSON message to an exchange with routing key |
| `"orders-queue" publishes messages to queue "orders":` | Publishes multiple messages from a table |


### Examples

**Publishes a message directly to a queue:**
```gherkin
"orders-queue" publishes to queue "orders":
  """
  Hello World
  """
```

**Publishes a JSON message directly to a queue:**
```gherkin
"orders-queue" publishes json to queue "orders":
  """
  {"order_id": 123}
  """
```

**Publishes a message to an exchange with routing key:**
```gherkin
"orders-queue" publishes to exchange "events" with routing key "order.created":
  """
  Order created
  """
```

**Publishes a JSON message to an exchange with routing key:**
```gherkin
"orders-queue" publishes json to exchange "events" with routing key "order.created":
  """
  {"event": "order.created"}
  """
```

**Publishes multiple messages from a table:**
```gherkin
"orders-queue" publishes messages to queue "orders":
  | routing_key | message |
  | order.1     | msg1    |
```


## Consuming

| Step | Description |
|------|-------------|
| `"orders-queue" consumes from queue "orders"` | Starts consuming messages from a queue |
| `"orders-queue" receives from queue "orders" within "5s"` | Waits for a message from a queue within timeout |
| `"orders-queue" receives from queue "orders" within "5s":` | Asserts a specific message is received within timeout |


### Examples

**Asserts a specific message is received within timeout:**
```gherkin
"orders-queue" receives from queue "orders" within "5s":
  """
  Hello World
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"orders-queue" queue "orders" has "3" messages` | Asserts queue has exactly N messages consumed |
| `"orders-queue" queue "orders" is empty` | Asserts no messages have been consumed from queue |
| `"orders-queue" last message contains:` | Asserts the last consumed message contains content |
| `"orders-queue" last message has routing key "order.created"` | Asserts the last consumed message has specific routing key |
| `"orders-queue" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |


### Examples

**Asserts the last consumed message contains content:**
```gherkin
"orders-queue" last message contains:
  """
  order_id
  """
```

