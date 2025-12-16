---
layout: default
title: Resources
nav_order: 4
has_children: true
---

# Available Resources

Tomato supports the following resource types for behavioral testing.

| Resource | Type | Description |
|----------|------|-------------|
| [HTTP Client](http-client.md) | `http` | Steps for making HTTP requests and validating responses |
| [HTTP Server](http-server.md) | `http-server` | Steps for stubbing HTTP services |
| [PostgreSQL](postgres.md) | `postgres` | Steps for interacting with PostgreSQL databases |
| [Redis](redis.md) | `redis` | Steps for interacting with Redis key-value store |
| [Kafka](kafka.md) | `kafka` | Steps for interacting with Apache Kafka message broker |
| [Shell](shell.md) | `shell` | Steps for executing shell commands and scripts |
| [WebSocket Client](websocket-client.md) | `websocket` | Steps for connecting to WebSocket servers |
| [WebSocket Server](websocket-server.md) | `websocket-server` | Steps for stubbing WebSocket services |


## Variables and Dynamic Values

Tomato supports variables that can be used in URLs, headers, and request bodies. Variables use the `{{name}}` syntax.

### Dynamic Value Generation

Built-in functions generate unique values on each use:

| Function | Example Output | Description |
|----------|----------------|-------------|
| `{{uuid}}` | `f47ac10b-58cc-4372-a567-0e02b2c3d479` | Random UUID v4 |
| `{{timestamp}}` | `2024-01-15T10:30:00Z` | Current ISO 8601 timestamp |
| `{{timestamp:unix}}` | `1705315800` | Unix timestamp in seconds |
| `{{random:N}}` | `A8kL2mN9pQ` | Random alphanumeric string of length N |
| `{{random:N:numeric}}` | `8472910384` | Random numeric string of length N |
| `{{sequence:name}}` | `1`, `2`, `3`... | Auto-incrementing sequence by name |

**Example:**
```gherkin
When "api" sends "POST" to "/api/users" with json:
  """
  {
    "id": "{{uuid}}",
    "username": "user_{{random:8}}",
    "created_at": "{{timestamp}}"
  }
  """
```

### Capturing Response Values

Save values from responses to use in subsequent requests:

| Step | Description |
|------|-------------|
| `"api" response json "path" saved as "{{var}}"` | Save JSON path value to variable |
| `"api" response header "Name" saved as "{{var}}"` | Save response header to variable |

**Example - CRUD workflow:**
```gherkin
Scenario: Create and retrieve a user
  # Create user and capture the ID
  When "api" sends "POST" to "/api/users" with json:
    """
    { "name": "Alice" }
    """
  Then "api" response status is "201"
  And "api" response json "id" saved as "{{user_id}}"

  # Use captured ID in subsequent request
  When "api" sends "GET" to "/api/users/{{user_id}}"
  Then "api" response status is "200"
  And "api" response json "name" is "Alice"

  # Delete the user
  When "api" sends "DELETE" to "/api/users/{{user_id}}"
  Then "api" response status is "204"
```

Variables and sequences are automatically reset between scenarios.

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
