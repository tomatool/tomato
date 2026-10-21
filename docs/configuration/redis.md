# Redis configuration

This guide covers how to configure Redis for integration testing with tomato.

## Overview

Redis is an in-memory data structure store used as a database, cache, and message broker. Tomato's Redis handler supports strings, hashes, lists, and sets.

## Container setup

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

### With password authentication

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

### With persistence (optional)

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

## Resource configuration

Configure the Redis resource:

```yaml
resources:
  app-cache:
    type: redis
    container: redis
    options:
      db: 0
      password: ""
      reset_strategy: flush
```

### Resource options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `db` | int | `0` | Redis database number (0-15) |
| `password` | string | `""` | Redis password (if authentication enabled) |
| `reset_strategy` | string | `flush` | How to reset between scenarios |
| `reset_pattern` | string | `*` | Pattern for selective key deletion (when using `pattern` strategy) |

### Reset strategies

| Strategy | Description |
|----------|-------------|
| `flush` | Flush all keys in the database (recommended, fastest) |
| `pattern` | Delete keys matching `reset_pattern` (useful for shared databases) |

## Complete example

Here's a complete `tomato.yml` with Redis:

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: false
  output: pretty

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
  app-cache:
    type: redis
    container: redis
    options:
      db: 0
      reset_strategy: flush

features:
  paths:
    - ./features
```

## Writing Redis tests

### String operations

```gherkin
Feature: Caching

  Scenario: Cache user data
    Given "app-cache" key "user:123" is "John Doe"
    Then "app-cache" key "user:123" exists
    And "app-cache" key "user:123" has value "John Doe"

  Scenario: Cache with expiration
    Given "app-cache" key "session:abc" is "token123" with TTL "1h"
    Then "app-cache" key "session:abc" has TTL greater than "3500" seconds

  Scenario: Store JSON data
    Given "app-cache" key "config" is:
      """
      {"debug": true, "timeout": 30}
      """
    Then "app-cache" key "config" contains "debug"
```

### Hash operations

```gherkin
Scenario: Store user profile as hash
  Given "app-cache" hash "user:100" has fields:
    | field | value           |
    | name  | Alice           |
    | email | alice@test.com  |
    | role  | admin           |
  Then "app-cache" hash "user:100" field "name" is "Alice"
  And "app-cache" hash "user:100" contains:
    | field | value          |
    | email | alice@test.com |
```

### List operations

```gherkin
Scenario: Manage task queue
  Given "app-cache" list "tasks" has values:
    | process-order-1 |
    | process-order-2 |
    | send-email-3    |
  Then "app-cache" list "tasks" has "3" items
  And "app-cache" list "tasks" contains "process-order-1"
```

### Set operations

```gherkin
Scenario: Track unique visitors
  Given "app-cache" set "visitors" has members:
    | user-1 |
    | user-2 |
    | user-3 |
  Then "app-cache" set "visitors" has "3" members
  And "app-cache" set "visitors" contains "user-2"
```

### Counter operations

```gherkin
Scenario: Track page views
  Given "app-cache" key "pageviews" is "100"
  When "app-cache" key "pageviews" is incremented
  Then "app-cache" key "pageviews" has value "101"
  When "app-cache" key "pageviews" is incremented by "10"
  Then "app-cache" key "pageviews" has value "111"
```

See [Redis Steps](../resources/redis.md) for the complete list of available steps.

## Multiple Redis databases

You can configure multiple Redis resources pointing to different databases:

```yaml
resources:
  app-cache:
    type: redis
    container: redis
    options:
      db: 0
      reset_strategy: flush

  sessions-cache:
    type: redis
    container: redis
    options:
      db: 1
      reset_strategy: flush
```

## Troubleshooting

### Connection refused

If tests fail with "connection refused":

1. Verify Redis container is running: `docker ps`
2. Check port mapping: `docker port <container_id>`
3. Ensure no firewall is blocking the connection

### Authentication failed

If using password authentication:

1. Verify password matches in container command and resource options
2. Check for special characters that may need escaping

### Keys not being reset

If keys persist between scenarios:

1. Verify `reset_strategy` is set (defaults to `flush`)
2. If using `pattern` strategy, verify `reset_pattern` matches your keys
