# RabbitMQ

Steps for interacting with RabbitMQ message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Queue Management

| Step | Description |
|------|-------------|
| `"{resource}" declares queue "orders"` | Declares a queue with default settings |
| `"{resource}" declares durable queue "orders"` | Declares a durable queue |
| `"{resource}" queue "orders" exists` | Asserts a queue exists |
| `"{resource}" purges queue "orders"` | Purges all messages from a queue |



## Exchange Management

| Step | Description |
|------|-------------|
| `"{resource}" declares exchange "events" of type "topic"` | Declares an exchange (direct, fanout, topic, headers) |
| `"{resource}" declares durable exchange "events" of type "topic"` | Declares a durable exchange |
| `"{resource}" exchange "events" exists` | Asserts an exchange exists |



## Bindings

| Step | Description |
|------|-------------|
| `"{resource}" binds queue "orders" to exchange "events"` | Binds a queue to an exchange with empty routing key |
| `"{resource}" binds queue "orders" to exchange "events" with routing key "order.*"` | Binds a queue to an exchange with a routing key |



## Publishing

| Step | Description |
|------|-------------|
| `"{resource}" message header "trace-id" is "abc-123"` | Sets a header on the next message published (any publish step) |
| `"{resource}" publishes to queue "orders":` | Publishes a message directly to a queue |
| `"{resource}" publishes json to queue "orders":` | Publishes a JSON message directly to a queue |
| `"{resource}" publishes to exchange "events" with routing key "order.created":` | Publishes a message to an exchange with routing key |
| `"{resource}" publishes json to exchange "events" with routing key "order.created":` | Publishes a JSON message to an exchange with routing key |
| `"{resource}" publishes messages to queue "orders":` | Publishes multiple messages from a table |


### Examples

**Publishes a message directly to a queue:**
```gherkin
"{resource}" publishes to queue "orders":
  """
  Hello World
  """
```

**Publishes a JSON message directly to a queue:**
```gherkin
"{resource}" publishes json to queue "orders":
  """
  {"order_id": 123}
  """
```

**Publishes a message to an exchange with routing key:**
```gherkin
"{resource}" publishes to exchange "events" with routing key "order.created":
  """
  Order created
  """
```

**Publishes a JSON message to an exchange with routing key:**
```gherkin
"{resource}" publishes json to exchange "events" with routing key "order.created":
  """
  {"event": "order.created"}
  """
```

**Publishes multiple messages from a table:**
```gherkin
"{resource}" publishes messages to queue "orders":
  | routing_key | message |
  | order.1     | msg1    |
```


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from queue "orders"` | Starts consuming messages from a queue |
| `"{resource}" receives from queue "orders" within "5s"` | Waits for a message from a queue within timeout |
| `"{resource}" receives from queue "orders" within "5s":` | Asserts a specific message is received within timeout |


### Examples

**Asserts a specific message is received within timeout:**
```gherkin
"{resource}" receives from queue "orders" within "5s":
  """
  Hello World
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" queue "orders" has "3" messages` | Asserts queue has exactly N messages consumed |
| `"{resource}" queue "orders" is empty` | Asserts no messages have been consumed from queue |
| `"{resource}" last message contains:` | Asserts the last consumed message contains content |
| `"{resource}" last message has routing key "order.created"` | Asserts the last consumed message has specific routing key |
| `"{resource}" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |


### Examples

**Asserts the last consumed message contains content:**
```gherkin
"{resource}" last message contains:
  """
  order_id
  """
```

