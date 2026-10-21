# Kafka

Steps for interacting with Apache Kafka message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Topic Management

| Step | Description |
|------|-------------|
| `"orders-events" topic "events" exists` | Asserts a Kafka topic exists |
| `"orders-events" creates topic "events"` | Creates a Kafka topic with 1 partition |
| `"orders-events" creates topic "events" with "3" partitions` | Creates a Kafka topic with specified partitions |



## Publishing

| Step | Description |
|------|-------------|
| `"orders-events" message header "trace-id" is "abc-123"` | Sets a header on the next message published (any publish step) |
| `"orders-events" publishes to "events":` | Publishes a message to a topic |
| `"orders-events" publishes to "events" with key "user-123":` | Publishes a message with a key to a topic |
| `"orders-events" publishes json to "events":` | Publishes a JSON message to a topic |
| `"orders-events" publishes json to "events" with key "user-123":` | Publishes a JSON message with a key |
| `"orders-events" publishes messages to "events":` | Publishes multiple messages from a table |


### Examples

**Publishes a message to a topic:**
```gherkin
"orders-events" publishes to "events":
  """
  Hello World
  """
```

**Publishes a message with a key to a topic:**
```gherkin
"orders-events" publishes to "events" with key "user-123":
  """
  Hello World
  """
```

**Publishes a JSON message to a topic:**
```gherkin
"orders-events" publishes json to "events":
  """
  {"type": "user_created"}
  """
```

**Publishes a JSON message with a key:**
```gherkin
"orders-events" publishes json to "events" with key "user-123":
  """
  {"type": "user_created"}
  """
```

**Publishes multiple messages from a table:**
```gherkin
"orders-events" publishes messages to "events":
  | key      | value           |
  | user-1   | {"id": 1}     |
```


## Avro and Schema Registry

| Step | Description |
|------|-------------|
| `"orders-events" registers schema for subject "orders-value":` | Registers an Avro schema under a subject |
| `"orders-events" registers schema for subject "orders-value" from file "schemas/order.avsc"` | Registers an Avro schema (.avsc) from a file |
| `"orders-events" publishes avro to "orders":` | Publishes JSON as Avro, using the latest schema of the topic's value subject |
| `"orders-events" publishes avro to "orders" with key "order-1":` | Publishes JSON as Avro with a string key |
| `"orders-events" receives avro from "orders" within "10s":` | Waits for an Avro message whose JSON form contains the given fields |
| `"orders-events" last message avro matches:` | Asserts the last message, decoded from Avro, equals the JSON exactly |
| `"orders-events" last message avro contains:` | Asserts the last message, decoded from Avro, contains the JSON fields |


### Examples

**Registers an Avro schema under a subject:**
```gherkin
"orders-events" registers schema for subject "orders-value":
  """
  {"type": "record", "name": "Order", "fields": [{"name": "id", "type": "string"}]}
  """
```

**Publishes JSON as Avro, using the latest schema of the topic's value subject:**
```gherkin
"orders-events" publishes avro to "orders":
  """
  {"id": "order-1"}
  """
```

**Publishes JSON as Avro with a string key:**
```gherkin
"orders-events" publishes avro to "orders" with key "order-1":
  """
  {"id": "order-1"}
  """
```

**Waits for an Avro message whose JSON form contains the given fields:**
```gherkin
"orders-events" receives avro from "orders" within "10s":
  """
  {"id": "order-1"}
  """
```

**Asserts the last message, decoded from Avro, equals the JSON exactly:**
```gherkin
"orders-events" last message avro matches:
  """
  {"id": "order-1", "note": null}
  """
```

**Asserts the last message, decoded from Avro, contains the JSON fields:**
```gherkin
"orders-events" last message avro contains:
  """
  {"id": "order-1"}
  """
```


## Consuming

| Step | Description |
|------|-------------|
| `"orders-events" consumes from "events"` | Starts consuming messages from a topic |
| `"orders-events" receives from "events" within "5s"` | Waits for a message from a topic within timeout |
| `"orders-events" receives from "events" within "5s":` | Asserts a specific message is received within timeout |
| `"orders-events" receives from "events" with key "user-123" within "5s"` | Asserts a message with specific key is received |


### Examples

**Asserts a specific message is received within timeout:**
```gherkin
"orders-events" receives from "events" within "5s":
  """
  Hello World
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"orders-events" topic "events" has "3" messages` | Asserts topic has exactly N messages consumed |
| `"orders-events" topic "events" is empty` | Asserts no messages have been consumed from topic |
| `"orders-events" last message contains:` | Asserts the last consumed message contains content |
| `"orders-events" last message has key "user-123"` | Asserts the last consumed message has specific key |
| `"orders-events" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |
| `"orders-events" receives messages from "events" in order:` | Asserts messages are received in specified order |


### Examples

**Asserts the last consumed message contains content:**
```gherkin
"orders-events" last message contains:
  """
  user_created
  """
```

**Asserts messages are received in specified order:**
```gherkin
"orders-events" receives messages from "events" in order:
  | key    | value  |
  | key1   | msg1   |
```


## Consumer Groups

| Step | Description |
|------|-------------|
| `"orders-events" consumes from "events" as consumer group "billing"` | Joins a consumer group on a topic; its messages are received like any other |
| `"orders-events" consumer group "billing" is consuming "events" within "30s"` | Waits until a member of the consumer group is assigned the topic, as the broker sees it |
| `"orders-events" consumer group "billing" has no members` | Asserts the consumer group has no members, as the broker sees it |


