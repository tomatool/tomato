# RabbitMQ

Steps for interacting with RabbitMQ message broker


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
| `"{resource}" publishes to queue "orders":
  """
  Hello World
  """` | Publishes a message directly to a queue |
| `"{resource}" publishes json to queue "orders":
  """
  {"order_id": 123}
  """` | Publishes a JSON message directly to a queue |
| `"{resource}" publishes to exchange "events" with routing key "order.created":
  """
  Order created
  """` | Publishes a message to an exchange with routing key |
| `"{resource}" publishes json to exchange "events" with routing key "order.created":
  """
  {"event": "order.created"}
  """` | Publishes a JSON message to an exchange with routing key |
| `"{resource}" publishes messages to queue "orders":
  | message     |
  | message 1   |
  | message 2   |` | Publishes multiple messages from a table |


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from queue "orders"` | Starts consuming messages from a queue |
| `"{resource}" receives from queue "orders" within "5s"` | Waits for a message from a queue within timeout |
| `"{resource}" receives from queue "orders" within "5s":
  """
  Hello World
  """` | Asserts a specific message is received within timeout |


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" queue "orders" has "3" messages` | Asserts queue has exactly N messages consumed |
| `"{resource}" queue "orders" is empty` | Asserts no messages have been consumed from queue |
| `"{resource}" last message contains:
  """
  order_id
  """` | Asserts the last consumed message contains content |
| `"{resource}" last message has routing key "order.created"` | Asserts the last consumed message has specific routing key |
| `"{resource}" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |
