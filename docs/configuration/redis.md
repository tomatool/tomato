# Redis Configuration

This guide covers how to configure Redis for integration testing with tomato.

## Overview

Redis is an in-memory data structure store used as a database, cache, and message broker. Tomato's Redis handler supports strings, hashes, lists, and sets.

## Container Setup

Add a Redis container to your `tomato.yml`:

```yaml
containers:
  redis:
    image: redis:7-alpine
    ports:
      - "6379/tcp"
    wait_for:
      type: port
      target: "6379"
      timeout: 30s
```

### With Password Authentication

```yaml
containers:
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass mysecretpassword
    ports:
      - "6379/tcp"
    wait_for:
      type: port
      target: "6379"
      timeout: 30s
```

### With Persistence (Optional)

For tests that need data persistence across container restarts:

```yaml
containers:
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    ports:
      - "6379/tcp"
    volumes:
      - redis-data:/data
    wait_for:
      type: port
      target: "6379"
      timeout: 30s
```

## Resource Configuration

Configure the Redis resource:

```yaml
resources:
  cache:
    type: redis
    container: redis
    options:
      db: 0
      password: ""
      reset_strategy: flush
```

### Resource Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `db` | int | `0` | Redis database number (0-15) |
| `password` | string | `""` | Redis password (if authentication enabled) |
| `reset_strategy` | string | `flush` | How to reset between scenarios |
| `reset_pattern` | string | `*` | Pattern for selective key deletion (when using `pattern` strategy) |

### Reset Strategies

| Strategy | Description |
|----------|-------------|
| `flush` | Flush all keys in the database (recommended, fastest) |
| `pattern` | Delete keys matching `reset_pattern` (useful for shared databases) |

## Complete Example

Here's a complete `tomato.yml` with Redis:

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: false
  output: pretty
  reset:
    level: scenario

containers:
  redis:
    image: redis:7-alpine
    ports:
      - "6379/tcp"
    wait_for:
      type: port
      target: "6379"
      timeout: 30s

resources:
  cache:
    type: redis
    container: redis
    options:
      db: 0
      reset_strategy: flush

features:
  paths:
    - ./features
```

## Writing Redis Tests

### String Operations

```gherkin
Feature: Caching

  Scenario: Cache user data
    Given "cache" key "user:123" is "John Doe"
    Then "cache" key "user:123" exists
    And "cache" key "user:123" has value "John Doe"

  Scenario: Cache with expiration
    Given "cache" key "session:abc" is "token123" with TTL "1h"
    Then "cache" key "session:abc" has TTL greater than "3500" seconds

  Scenario: Store JSON data
    Given "cache" key "config" is:
      """
      {"debug": true, "timeout": 30}
      """
    Then "cache" key "config" contains "debug"
```

### Hash Operations

```gherkin
Scenario: Store user profile as hash
  Given "cache" hash "user:100" has fields:
    | field | value           |
    | name  | Alice           |
    | email | alice@test.com  |
    | role  | admin           |
  Then "cache" hash "user:100" field "name" is "Alice"
  And "cache" hash "user:100" contains:
    | field | value          |
    | email | alice@test.com |
```

### List Operations

```gherkin
Scenario: Manage task queue
  Given "cache" list "tasks" has values:
    | process-order-1 |
    | process-order-2 |
    | send-email-3    |
  Then "cache" list "tasks" has "3" items
  And "cache" list "tasks" contains "process-order-1"
```

### Set Operations

```gherkin
Scenario: Track unique visitors
  Given "cache" set "visitors" has members:
    | user-1 |
    | user-2 |
    | user-3 |
  Then "cache" set "visitors" has "3" members
  And "cache" set "visitors" contains "user-2"
```

### Counter Operations

```gherkin
Scenario: Track page views
  Given "cache" key "pageviews" is "100"
  When "cache" key "pageviews" is incremented
  Then "cache" key "pageviews" has value "101"
  When "cache" key "pageviews" is incremented by "10"
  Then "cache" key "pageviews" has value "111"
```

See [Redis Steps](../resources/redis.md) for the complete list of available steps.

## Multiple Redis Databases

You can configure multiple Redis resources pointing to different databases:

```yaml
resources:
  cache:
    type: redis
    container: redis
    options:
      db: 0
      reset_strategy: flush

  sessions:
    type: redis
    container: redis
    options:
      db: 1
      reset_strategy: flush
```

## Troubleshooting

### Connection Refused

If tests fail with "connection refused":

1. Verify Redis container is running: `docker ps`
2. Check port mapping: `docker port <container_id>`
3. Ensure no firewall is blocking the connection

### Authentication Failed

If using password authentication:

1. Verify password matches in container command and resource options
2. Check for special characters that may need escaping

### Keys Not Being Reset

If keys persist between scenarios:

1. Verify `reset_strategy` is set (defaults to `flush`)
2. If using `pattern` strategy, verify `reset_pattern` matches your keys
3. Check that `settings.reset.level` is set to `scenario`
