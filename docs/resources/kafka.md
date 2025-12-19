# Kafka

Steps for interacting with Apache Kafka message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Topic Management

| Step | Description |
|------|-------------|
| `"{resource}" topic "{topic}" exists` | Asserts a Kafka topic exists |
| `"{resource}" creates topic "{topic}"` | Creates a Kafka topic with 1 partition |
| `"{resource}" creates topic "{topic}" with "{n}" partitions` | Creates a Kafka topic with specified partitions |


## Publishing

| Step | Description |
|------|-------------|
| `"{resource}" publishes to "{topic}":` | Publishes a message to a topic (see example below) |
| `"{resource}" publishes to "{topic}" with key "{key}":` | Publishes a message with a key to a topic |
| `"{resource}" publishes json to "{topic}":` | Publishes a JSON message to a topic |
| `"{resource}" publishes json to "{topic}" with key "{key}":` | Publishes a JSON message with a key |
| `"{resource}" publishes messages to "{topic}":` | Publishes multiple messages from a table |

### Examples

**Publish a message:**
```gherkin
When "events" publishes to "user-events":
  """
  Hello World
  """
```

**Publish with key:**
```gherkin
When "events" publishes to "user-events" with key "user-123":
  """
  {"action": "login"}
  """
```

**Publish JSON:**
```gherkin
When "events" publishes json to "user-events":
  """
  {"type": "user_created", "userId": 123}
  """
```

**Publish multiple messages:**
```gherkin
When "events" publishes messages to "user-events":
  | key      | value              |
  | user-1   | {"id": 1}          |
  | user-2   | {"id": 2}          |
```


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from "{topic}"` | Starts consuming messages from a topic |
| `"{resource}" receives from "{topic}" within "{timeout}"` | Waits for a message from a topic within timeout |
| `"{resource}" receives from "{topic}" within "{timeout}":` | Asserts a specific message is received (see example) |
| `"{resource}" receives from "{topic}" with key "{key}" within "{timeout}"` | Asserts a message with specific key is received |

### Examples

**Wait for specific message:**
```gherkin
Then "events" receives from "user-events" within "5s":
  """
  Hello World
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" topic "{topic}" has "{n}" messages` | Asserts topic has exactly N messages consumed |
| `"{resource}" topic "{topic}" is empty` | Asserts no messages have been consumed from topic |
| `"{resource}" last message contains:` | Asserts the last consumed message contains content |
| `"{resource}" last message has key "{key}"` | Asserts the last consumed message has specific key |
| `"{resource}" last message has header "{name}" with value "{value}"` | Asserts the last message has a header with value |
| `"{resource}" receives messages from "{topic}" in order:` | Asserts messages are received in specified order |

### Examples

**Assert message content:**
```gherkin
Then "events" last message contains:
  """
  user_created
  """
```

**Assert message order:**
```gherkin
Then "events" receives messages from "user-events" in order:
  | key    | value         |
  | key1   | message one   |
  | key2   | message two   |
```
