# Testing Deployed Environments

Tomato usually starts every dependency itself. It can also run the same
feature files against services it did not start: a shared staging
environment, a cloud Kafka cluster, a database on another host. Leave out
`containers:` and `app:`, and give each resource connection settings instead
of `container:`.

```yaml
version: 2

resources:
  api:
    type: http
    base_url: https://orders.staging.example.com

  db:
    type: postgres
    url: "postgres://tests:${DB_PASSWORD}@db.staging.internal:5432/orders?sslmode=verify-full&sslrootcert=./ca.pem"

  cache:
    type: redis
    url: "rediss://default:${REDIS_PASSWORD}@redis.staging.internal:6380/0"

  kafka:
    type: kafka
    brokers:
      - kafka-1.staging.internal:9093
      - kafka-2.staging.internal:9093
    options:
      tls: true
      sasl:
        mechanism: SCRAM-SHA-512
        user: ${KAFKA_USER}
        password: ${KAFKA_PASSWORD}
      schema_registry:
        url: https://schema-registry.staging.internal
        user: ${SR_KEY}
        password: ${SR_SECRET}

  mq:
    type: rabbitmq
    url: "amqps://tests:${MQ_PASSWORD}@mq.staging.internal:5671/orders"

features:
  paths: [features]
```

Docker is only needed when `containers:` or a containerised `app:` is
declared. Secrets come from the environment: `${VAR}` is expanded anywhere in
`tomato.yml`.

## Reset is off for remote hosts

Resetting a disposable container between scenarios is what keeps tomato tests
independent. Doing the same to a shared database would wipe other people's
data. So a resource that points at a host other than `localhost` / `127.0.0.1`
does **not** reset, and tomato logs a warning saying so:

```
WRN resource points at a remote host; reset is disabled (set `reset: true` on the resource to allow it) host=db.staging.internal resource=db
```

Opt in per resource when the target really is yours to wipe, such as a
per-branch preview database:

```yaml
resources:
  db:
    type: postgres
    url: "postgres://tests:${DB_PASSWORD}@pr-123.preview.internal/orders"
    reset: true
```

Without reset, write scenarios that create their own uniquely named data
(ids with a random or run-specific suffix) and assert only on that.

## Connection options

| Resource | Connect with | TLS |
|----------|--------------|-----|
| `http` | `base_url` | `https://` |
| `grpc` | `address` | see [gRPC](grpc.md) |
| `websocket` | `url` | `wss://` |
| `postgres` | `url` (URL or keyword DSN), or `options.host` + `options.port` | `options.sslmode`, `sslrootcert`, `sslcert`, `sslkey`. Defaults to `require` for remote hosts, `disable` for local ones |
| `redis` | `url` (`redis://` or `rediss://`), or `options.host` + `options.port` | `rediss://` or `options.tls` |
| `kafka` | `brokers` | `options.tls`, `options.sasl` (`PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`) |
| `rabbitmq` | `url` (`amqp://` or `amqps://`) | `amqps://` or `options.tls` |
| `s3` | `options.endpoint` | `https://` endpoint |
| `scylladb` / `cassandra` | `options.hosts` | `options.tls` |

### `options.tls`

`tls: true` enables TLS with the system CA roots. A map configures it further:

```yaml
options:
  tls:
    ca_file: ./certs/ca.pem          # trust this CA
    cert_file: ./certs/client.pem    # client certificate (mutual TLS)
    key_file: ./certs/client.key
    server_name: kafka.internal      # when it differs from the address
    insecure_skip_verify: false
```

For Kafka, the Schema Registry takes its own `tls`, `user` and `password`
under `options.schema_registry`.
