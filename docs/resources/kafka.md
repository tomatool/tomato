# Kafka

Steps for interacting with Apache Kafka message broker

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Topic Management

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" exists` | Asserts a Kafka topic exists |
| `"{resource}" creates topic "events"` | Creates a Kafka topic with 1 partition |
| `"{resource}" creates topic "events" with "3" partitions` | Creates a Kafka topic with specified partitions |



## Publishing

| Step | Description |
|------|-------------|
| `"{resource}" publishes to "events":` | Publishes a message to a topic |
| `"{resource}" publishes to "events" with key "user-123":` | Publishes a message with a key to a topic |
| `"{resource}" publishes json to "events":` | Publishes a JSON message to a topic |
| `"{resource}" publishes json to "events" with key "user-123":` | Publishes a JSON message with a key |
| `"{resource}" publishes messages to "events":` | Publishes multiple messages from a table |


### Examples

**Publishes a message to a topic:**
```gherkin
"{resource}" publishes to "events":
  """
  Hello World
  """
```

**Publishes a message with a key to a topic:**
```gherkin
"{resource}" publishes to "events" with key "user-123":
  """
  Hello World
  """
```

**Publishes a JSON message to a topic:**
```gherkin
"{resource}" publishes json to "events":
  """
  {"type": "user_created"}
  """
```

**Publishes a JSON message with a key:**
```gherkin
"{resource}" publishes json to "events" with key "user-123":
  """
  {"type": "user_created"}
  """
```

**Publishes multiple messages from a table:**
```gherkin
"{resource}" publishes messages to "events":
  | key      | value           |
  | user-1   | {"id": 1}     |
```


## Avro and Schema Registry

| Step | Description |
|------|-------------|
| `"{resource}" registers schema for subject "orders-value":` | Registers an Avro schema under a subject |
| `"{resource}" registers schema for subject "orders-value" from file "schemas/order.avsc"` | Registers an Avro schema (.avsc) from a file |
| `"{resource}" publishes avro to "orders":` | Publishes JSON as Avro, using the latest schema of the topic's value subject |
| `"{resource}" publishes avro to "orders" with key "order-1":` | Publishes JSON as Avro with a string key |
| `"{resource}" receives avro from "orders" within "10s":` | Waits for an Avro message whose JSON form contains the given fields |
| `"{resource}" last message avro matches:` | Asserts the last message, decoded from Avro, equals the JSON exactly |
| `"{resource}" last message avro contains:` | Asserts the last message, decoded from Avro, contains the JSON fields |


### Examples

**Registers an Avro schema under a subject:**
```gherkin
"{resource}" registers schema for subject "orders-value":
  """
  {"type": "record", "name": "Order", "fields": [{"name": "id", "type": "string"}]}
  """
```

**Publishes JSON as Avro, using the latest schema of the topic's value subject:**
```gherkin
"{resource}" publishes avro to "orders":
  """
  {"id": "order-1"}
  """
```

**Publishes JSON as Avro with a string key:**
```gherkin
"{resource}" publishes avro to "orders" with key "order-1":
  """
  {"id": "order-1"}
  """
```

**Waits for an Avro message whose JSON form contains the given fields:**
```gherkin
"{resource}" receives avro from "orders" within "10s":
  """
  {"id": "order-1"}
  """
```

**Asserts the last message, decoded from Avro, equals the JSON exactly:**
```gherkin
"{resource}" last message avro matches:
  """
  {"id": "order-1", "note": null}
  """
```

**Asserts the last message, decoded from Avro, contains the JSON fields:**
```gherkin
"{resource}" last message avro contains:
  """
  {"id": "order-1"}
  """
```


## Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from "events"` | Starts consuming messages from a topic |
| `"{resource}" receives from "events" within "5s"` | Waits for a message from a topic within timeout |
| `"{resource}" receives from "events" within "5s":` | Asserts a specific message is received within timeout |
| `"{resource}" receives from "events" with key "user-123" within "5s"` | Asserts a message with specific key is received |


### Examples

**Asserts a specific message is received within timeout:**
```gherkin
"{resource}" receives from "events" within "5s":
  """
  Hello World
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" has "3" messages` | Asserts topic has exactly N messages consumed |
| `"{resource}" topic "events" is empty` | Asserts no messages have been consumed from topic |
| `"{resource}" last message contains:` | Asserts the last consumed message contains content |
| `"{resource}" last message has key "user-123"` | Asserts the last consumed message has specific key |
| `"{resource}" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |
| `"{resource}" receives messages from "events" in order:` | Asserts messages are received in specified order |


### Examples

**Asserts the last consumed message contains content:**
```gherkin
"{resource}" last message contains:
  """
  user_created
  """
```

**Asserts messages are received in specified order:**
```gherkin
"{resource}" receives messages from "events" in order:
  | key    | value  |
  | key1   | msg1   |
```

