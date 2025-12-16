# Available Resources

Tomato supports the following resource types for behavioral testing.

| Resource | Description |
|----------|-------------|
| [HTTP Client](http-client.md) | Steps for making HTTP requests and validating responses |
| [HTTP Server](http-server.md) | Steps for stubbing HTTP services |
| [PostgreSQL](postgres.md) | Steps for interacting with PostgreSQL databases |
| [Redis](redis.md) | Steps for interacting with Redis key-value store |
| [Kafka](kafka.md) | Steps for interacting with Apache Kafka message broker |
| [Shell](shell.md) | Steps for executing shell commands and scripts |
| [WebSocket Client](websocket-client.md) | Steps for connecting to WebSocket servers |
| [WebSocket Server](websocket-server.md) | Steps for stubbing WebSocket services |


## JSON Matchers

When using `response json matches:` or `response json contains:`, you can use these matchers:

### Type Matchers

| Matcher | Description |
|---------|-------------|
| `@string` | Matches any string value |
| `@number` | Matches any numeric value |
| `@boolean` | Matches true or false |
| `@array` | Matches any array |
| `@object` | Matches any object |
| `@null` | Matches null |
| `@notnull` | Matches any non-null value |
| `@any` | Matches any value |
| `@empty` | Matches empty string, array, or object |
| `@notempty` | Matches non-empty string, array, or object |

### String Matchers

| Matcher | Description |
|---------|-------------|
| `@regex:pattern` | Matches string against regex pattern |
| `@contains:text` | Matches if string contains text |
| `@startswith:text` | Matches if string starts with text |
| `@endswith:text` | Matches if string ends with text |

### Numeric Matchers

| Matcher | Description |
|---------|-------------|
| `@gt:n` | Matches if value > n |
| `@gte:n` | Matches if value >= n |
| `@lt:n` | Matches if value < n |
| `@lte:n` | Matches if value <= n |

### Length Matcher

| Matcher | Description |
|---------|-------------|
| `@len:n` | Matches if length equals n (for strings, arrays, objects) |

### Example

```gherkin
Then "api" response json contains:
  """
  {
    "id": "@regex:^[0-9a-f-]{36}$",
    "name": "@notempty",
    "email": "@contains:@",
    "age": "@gt:0",
    "tags": "@array"
  }
  """
```
