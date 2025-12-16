# Available Resources

Tomato supports the following resource types for behavioral testing.

| Resource | Type | Description |
|----------|------|-------------|
| [http-client](http-client.md) | `http` | Steps for making HTTP requests and validating responses |
| [http-server](http-server.md) | `http-server` | Steps for stubbing HTTP services |
| [postgres](postgres.md) | `postgres` | Steps for interacting with PostgreSQL databases |
| [redis](redis.md) | `redis` | Steps for interacting with Redis key-value store |
| [kafka](kafka.md) | `kafka` | Steps for interacting with Apache Kafka message broker |
| [shell](shell.md) | `shell` | Steps for executing shell commands and scripts |
| [websocket-client](websocket-client.md) | `websocket` | Steps for connecting to WebSocket servers |
| [websocket-server](websocket-server.md) | `websocket-server` | Steps for stubbing WebSocket services |


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
