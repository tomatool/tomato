# Kafka

Steps for interacting with Apache Kafka message broker


## Topic Management

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" exists` | Asserts a Kafka topic exists |
| `"{resource}" creates topic "events"` | Creates a Kafka topic with 1 partition |
| `"{resource}" creates topic "events" with "3" partitions` | Creates a Kafka topic with specified partitions |


## Publishing

| Step | Description |
|------|-------------|
| `"{resource}" publishes to "events":
  """
  Hello World
  """` | Publishes a message to a topic |
| `"{resource}" publishes to "events" with key "user-123":
  """
  Hello World
  """` | Publishes a message with a key to a topic |
| `"{resource}" publishes json to "events":
  """
  {"type": "user_created"}
  """` | Publishes a JSON message to a topic |
| `"{resource}" publishes json to "events" with key "user-123":
  """
  {"type": "user_created"}
  """` | Publishes a JSON message with a key |
| `"{resource}" publishes messages to "events":
  | key      | value           |
  | user-1   | {"id": 1}     |` | Publishes multiple messages from a table |


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from "events"` | Starts consuming messages from a topic |
| `"{resource}" receives from "events" within "5s"` | Waits for a message from a topic within timeout |
| `"{resource}" receives from "events" within "5s":
  """
  Hello World
  """` | Asserts a specific message is received within timeout |
| `"{resource}" receives from "events" with key "user-123" within "5s"` | Asserts a message with specific key is received |


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" has "3" messages` | Asserts topic has exactly N messages consumed |
| `"{resource}" topic "events" is empty` | Asserts no messages have been consumed from topic |
| `"{resource}" last message contains:
  """
  user_created
  """` | Asserts the last consumed message contains content |
| `"{resource}" last message has key "user-123"` | Asserts the last consumed message has specific key |
| `"{resource}" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |
| `"{resource}" receives messages from "events" in order:
  | key    | value  |
  | key1   | msg1   |` | Asserts messages are received in specified order |

