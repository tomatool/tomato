# Documentation map

Everything here, on one page, with what each part is for.

## Start here

| Page | What it covers |
|------|----------------|
| [Getting Started](getting-started/index.md) | Install tomato and get a passing suite, in seven steps |
| [Writing a Suite](guide/philosophy.md) | How to write scenarios worth trusting: one behaviour each, and asserting the effects your application produces rather than only the response it returns |

Read them in that order. The first gets something running; the second is what
makes it worth having.

## Running a suite

| Page | What it covers |
|------|----------------|
| [Test Coverage](getting-started/coverage.md) | Which lines of your service the scenarios ran, per language |
| [GitHub Action](getting-started/github-action.md) | Running the suite in CI |

## Configuring

[Configuration Reference](configuration/index.md) is `tomato.yml` in full —
settings, the application under test, containers, resources, hooks and feature
paths.

Then one page per dependency, for the options it accepts and how to wire a
container to it:

[HTTP Server](configuration/http-server.md) ·
[gRPC](configuration/grpc.md) ·
[gRPC Server](configuration/grpc-server.md) ·
[Kafka](configuration/kafka.md) ·
[RabbitMQ](configuration/rabbitmq.md) ·
[ScyllaDB](configuration/scylladb.md) ·
[Redis](configuration/redis.md) ·
[S3](configuration/s3.md) ·
[AWS](configuration/aws.md) ·
[Lifecycle](configuration/lifecycle.md) ·
[Network](configuration/network.md) ·
[Profile](configuration/profile.md) ·
[Browser](configuration/browser.md) ·
[PDF](configuration/pdf.md)

## Steps, by resource

334 steps across 18 resource types. Every page is generated from the step
definitions themselves, so it cannot disagree with the binary you are running
— and `tomato steps` prints the same thing in your terminal.

| Resource | Steps | For |
|----------|------:|-----|
| [HTTP Client](resources/http-client.md) | 56 | Calling your application and asserting on the response |
| [Kafka](resources/kafka.md) | 29 | Producing, consuming and asserting on topics |
| [S3](resources/s3.md) | 27 | Object storage: MinIO, LocalStack or a real bucket |
| [Redis](resources/redis.md) | 25 | Keys, TTLs, counters and hashes |
| [RabbitMQ](resources/rabbitmq.md) | 23 | Queues, exchanges and routing |
| [Shell](resources/shell.md) | 22 | Running commands and asserting on their output |
| [gRPC](resources/grpc.md) | 19 | Calling unary methods via server reflection |
| [HTTP Server](resources/http-server.md) | 18 | Stubbing the HTTP services your application calls |
| [Browser](resources/browser.md) | 18 | Using the web UI in a real Chromium and asserting on what it shows |
| [WebSocket Client](resources/websocket-client.md) | 17 | Connecting and exchanging messages |
| [gRPC Server](resources/grpc-server.md) | 16 | Stubbing the gRPC services your application calls |
| [PDF](resources/pdf.md) | 13 | What is inside the PDFs your application generates: pages, text, metadata, outline and accessibility |
| [Network](resources/network.md) | 12 | Making the network to a dependency slow, unreliable or down |
| [PostgreSQL](resources/postgres.md) | 11 | Seeding tables, running SQL, asserting on rows |
| [ScyllaDB](resources/scylladb.md) | 9 | Cassandra and ScyllaDB over CQL |
| [Lifecycle](resources/lifecycle.md) | 9 | Stopping and starting the app and its containers mid-scenario |
| [WebSocket Server](resources/websocket-server.md) | 8 | Stubbing a WebSocket your application connects to |
| [AWS](resources/aws.md) | 2 | The IRSA identity tomato hands the application |

[Overview of all resources](resources/index.md) lists them together.

## Changing tomato itself

| Page | What it covers |
|------|----------------|
| [Architecture](developer-guide/architecture.md) | What is in the repository, which parts ship, and which of them CI checks |
| [Adding a Step or Resource](developer-guide/adding-a-resource.md) | The walkthrough for both, and how to deprecate rather than rename |
| [Design: Network Faults](developer-guide/network-faults.md) | Why the network resource embeds Toxiproxy, what a spike found per dependency, and how faults reach the reports |
| [Stability](stability.md) | What counts as a breaking change, and how long a deprecation lasts |

## The short version

tomato runs your application the way production does and talks to it over the
network. A suite is `tomato.yml` plus `.feature` files — no glue code. Every
scenario starts from the same state, because tomato resets every resource
before each one and gives you no way to turn that off.

The part people miss: assert the **effects**. A `201` says your handler did
not crash. Whether the row landed, the event published and the receipt was
written is what the rest of your system depends on, and
[Writing a Suite](guide/philosophy.md) is about asserting those.
