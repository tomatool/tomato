# Kafka Configuration

This guide covers how to configure Apache Kafka for integration testing with tomato.

## Overview

Kafka integration testing with testcontainers requires special configuration due to how Kafka advertises its brokers to clients. This guide explains the recommended setup using Zookeeper-based Kafka with fixed port mapping.

## The Challenge

When running Kafka in a container with dynamic port mapping, clients connecting from the host machine need to know the actual mapped port. However, Kafka's advertised listeners are configured at startup time, before the dynamic port is assigned.

**The solution**: Use fixed port mapping for the external listener and dual-listener configuration.

## Recommended Configuration

### Container Setup

The recommended approach uses Zookeeper with Kafka and dual listeners:

```yaml
containers:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.6.1
    env:
      ZOOKEEPER_CLIENT_PORT: "2181"
      ZOOKEEPER_TICK_TIME: "2000"
    ports:
      - "2181/tcp"
    wait_for:
      type: port
      target: "2181/tcp"
      timeout: 30s

  kafka:
    image: confluentinc/cp-kafka:7.6.1
    depends_on:
      - zookeeper
    env:
      KAFKA_BROKER_ID: "1"
      KAFKA_ZOOKEEPER_CONNECT: "{{.zookeeper.host}}:{{.zookeeper.port.2181}}"
      # Dual listeners: internal (kafka:29092) and external (localhost:9092)
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: "1"
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: "0"
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
    ports:
      # Fixed port for external access (host connects here)
      - "9092:9092"
      # Internal port for container-to-container (dynamic is fine)
      - "29092/tcp"
    wait_for:
      type: port
      target: "9092/tcp"
      timeout: 60s
```

### Key Configuration Explained

#### Dual Listeners

Kafka is configured with two listeners:

| Listener | Address | Purpose |
|----------|---------|---------|
| `PLAINTEXT` | `kafka:29092` | Internal communication between containers |
| `PLAINTEXT_HOST` | `localhost:9092` | External access from host machine (tests) |

#### Fixed Port Mapping

The external port uses fixed mapping (`9092:9092`) so the advertised listener (`localhost:9092`) matches the actual port:

```yaml
ports:
  - "9092:9092"    # Fixed: host:container
  - "29092/tcp"    # Dynamic: just container port
```

#### Environment Variable Templates

Tomato supports templates in container environment variables:

| Template | Resolves To |
|----------|-------------|
| `{{.container_name.host}}` | Container's DNS name (for inter-container communication) |
| `{{.container_name.port.XXXX}}` | Container's internal port (not mapped host port) |

For Zookeeper connection:
```yaml
KAFKA_ZOOKEEPER_CONNECT: "{{.zookeeper.host}}:{{.zookeeper.port.2181}}"
```

This resolves to `zookeeper:2181` (container DNS name + internal port).

## Resource Configuration

Configure the Kafka resource to connect to the container:

```yaml
resources:
  events:
    type: kafka
    container: kafka
    options:
      topics:
        - orders
        - events
        - notifications
      partitions: 1
      replication_factor: 1
      reset_strategy: delete_recreate
```

### Resource Options

| Option | Type | Description |
|--------|------|-------------|
| `topics` | list | Topics to create/manage |
| `partitions` | int | Number of partitions per topic (default: 1) |
| `replication_factor` | int | Replication factor (default: 1) |
| `reset_strategy` | string | How to reset between scenarios |

### Reset Strategies

| Strategy | Description |
|----------|-------------|
| `delete_recreate` | Delete and recreate topics (recommended) |
| `none` | No reset between scenarios |

## Complete Example

Here's a complete `tomato.yml` with Kafka:

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: false
  output: pretty
  reset:
    level: scenario

containers:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.6.1
    env:
      ZOOKEEPER_CLIENT_PORT: "2181"
      ZOOKEEPER_TICK_TIME: "2000"
    ports:
      - "2181/tcp"
    wait_for:
      type: port
      target: "2181/tcp"
      timeout: 30s

  kafka:
    image: confluentinc/cp-kafka:7.6.1
    depends_on:
      - zookeeper
    env:
      KAFKA_BROKER_ID: "1"
      KAFKA_ZOOKEEPER_CONNECT: "{{.zookeeper.host}}:{{.zookeeper.port.2181}}"
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: "1"
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: "0"
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
    ports:
      - "9092:9092"
      - "29092/tcp"
    wait_for:
      type: port
      target: "9092/tcp"
      timeout: 60s

resources:
  events:
    type: kafka
    container: kafka
    options:
      topics:
        - user-events
        - order-events
      partitions: 1
      replication_factor: 1
      reset_strategy: delete_recreate

features:
  paths:
    - ./features
```

## Writing Kafka Tests

Once configured, you can write Gherkin scenarios to test Kafka:

```gherkin
Feature: Event Processing

  Scenario: Order events are published correctly
    Given "events" creates topic "order-events"
    And "events" consumes from "order-events"
    When "events" publishes json to "order-events" with key "order-123":
      """
      {"event": "order.created", "orderId": "123", "amount": 99.99}
      """
    Then "events" receives from "order-events" with key "order-123" within "10s"
    And "events" last message contains:
      """
      order.created
      """
```

See [Kafka Steps](../resources/kafka.md) for the complete list of available steps.

## Troubleshooting

### Connection Refused

If tests fail with "connection refused":

1. Verify Kafka container is healthy: `docker ps`
2. Check port 9092 is mapped: `docker port <container_id>`
3. Ensure no other process is using port 9092: `lsof -i :9092`

### Consumer Not Receiving Messages

1. Ensure `consumes from` is called before publishing
2. The consumer starts at `OffsetNewest` - only messages published after consumer starts are received
3. Verify topic exists with `topic "name" exists`

### Port Already in Use

Since Kafka uses fixed port mapping (`9092:9092`), ensure no other Kafka instance is running:

```bash
# Kill any process using port 9092
lsof -ti:9092 | xargs kill -9
```

## Alternative: KRaft Mode (Experimental)

For newer Kafka versions, KRaft mode eliminates Zookeeper. However, the advertised listener challenge remains. If you want to try KRaft:

```yaml
containers:
  kafka:
    image: confluentinc/cp-kafka:7.6.1
    env:
      KAFKA_NODE_ID: "1"
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://kafka:29092,CONTROLLER://kafka:29093,PLAINTEXT_HOST://0.0.0.0:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:29093
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: "1"
      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk
    ports:
      - "9092:9092"
      - "29092/tcp"
```

!!! warning "KRaft Compatibility"
    KRaft mode may have compatibility issues with some Kafka client libraries. The Zookeeper-based setup is recommended for maximum compatibility.
