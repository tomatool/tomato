# Redis configuration

This guide covers how to configure Redis for integration testing with tomato.

## Overview

Redis is an in-memory data structure store used as a database, cache, and message broker. Tomato's Redis handler supports strings, hashes, lists, and sets.

## Preset

The shortest setup is the `redis` preset:

```yaml
containers:
  redis:
    preset: redis

resources:
  sessions-db:
    type: redis
    container: redis
```

### Cluster mode and TLS, as ElastiCache runs it

A service that talks to ElastiCache with cluster mode and in-transit encryption
usually has one Redis client, and it is a cluster client over TLS:

```go
redis.NewClusterClient(&redis.ClusterOptions{
    Addrs:     []string{cfg.URL},
    TLSConfig: &tls.Config{}, // verifies against the system roots
})
```

A plain Redis container can't stand in for that server. `cluster` and `tls` make
the preset one that can, without a change to the service:

```yaml
containers:
  redis:
    preset: redis
    cluster: true   # one node owning all 16384 slots
    tls: true       # TLS only, with a certificate from a CA made for the run

app:
  command: ./auth-proxy
  env:
    REDIS_URL: "{{.redis.host}}:{{.redis.port.6379}}"
    SSL_CERT_FILE: "{{.redis.ca_file}}"   # trust the run's CA

resources:
  sessions-db:
    type: redis
    container: redis   # connects the same way: cluster, TLS, the run's CA
```

The redis steps work as they do against a single server:

```gherkin
Then "sessions-db" key "cli_code_claim:{{code}}" exists
```

What each option does:

| Option | What tomato does |
|--------|------------------|
| `cluster: true` | Starts the server with cluster mode on and gives the node every slot. The node announces `127.0.0.1` and the host port tomato picked before it started, so a cluster client on the host follows `CLUSTER SLOTS` to an address it can dial. |
| `tls: true` | Makes a CA and a server certificate for `localhost` and `127.0.0.1` for the run, and serves TLS only: a plain client is refused, as ElastiCache refuses it. `{{.redis.ca_file}}` is the CA's path on your machine; tomato removes it when the run ends. |

Both set the redis resource's `cluster` and `tls_ca_file` options, so it reaches
the server the way the app does. Resets flush every key, as they do on a single
server.

Things to know:

- **The app runs as a command** (`app.command`). The node announces an address
  on your machine, and the CA is a file there; neither reaches an app in a
  container.
- **`SSL_CERT_FILE` is read on Linux, not on macOS.** On Linux, Go and the
  clients built on OpenSSL (Python, Ruby) trust the file it names; Node needs
  `NODE_EXTRA_CA_CERTS` instead, and Java a trust store. Go on macOS verifies
  through the system and ignores it. On a Mac, run the suite on Linux (in CI or
  a container), or give the service a CA-file setting.
- **A network resource can't stand in front of a cluster.** The client follows
  the address the node announces, which takes it around the proxy.
- **`settings.containers.reuse` can't be used with them.** A reused container
  keeps the CA and the port of the run that started it.

## Container setup

Without the preset, add a Redis container to your `tomato.yml`:

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
| `cluster` | bool | `false` | Connect as a cluster client (the preset's `cluster: true` sets it) |
| `tls_ca_file` | string | | Connect over TLS, trusting this CA (the preset's `tls: true` sets it) |

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
