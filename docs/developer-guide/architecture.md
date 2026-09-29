# Architecture

This page provides an overview of tomato's internal architecture and how components interact during test execution.

## High-level overview

tomato is designed around three core concepts:

1. **Containers** - Docker containers managed via Testcontainers
2. **Resources** - Abstractions over external services (databases, APIs, message queues)
3. **Handlers** - Step definitions that interact with resources

## Where the code lives

| Package | Responsibility |
|---------|----------------|
| `command/` | One file per CLI command, plus the step-docs generator |
| `internal/config` | `tomato.yml` parsing, defaults, validation, preset expansion |
| `internal/container` | Container lifecycle via Testcontainers, wait strategies, logs |
| `internal/apprunner` | Starting the application under test, `app.env` templating |
| `internal/handler` | One file per resource type, each with its step definitions |
| `internal/runner` | godog wiring, hooks, tag filtering, reset between scenarios |
| `internal/formatter` | Console, JUnit and Cucumber report output |
| `internal/presets` | Files copied into preset containers (the MSK IAM jar) |
| `internal/runlog` | Per-run log directory under `.tomato/runs/` |

A resource type is registered in exactly one place, `handlerFactories` in
`internal/handler/registry.go`.

```mermaid
flowchart TB
    subgraph Config["tomato.yml"]
        C[Containers]
        R[Resources]
        F[Features]
    end

    subgraph Runtime["tomato Runtime"]
        TC[Testcontainers]
        H[Handlers]
        G[Gherkin Parser]
    end

    subgraph External["External Services"]
        DB[(PostgreSQL)]
        REDIS[(Redis)]
        KAFKA[(Kafka)]
        RMQ[(RabbitMQ)]
        S3[(S3)]
        HTTP[HTTP Server]
        GRPC[gRPC Server]
        WS[WebSocket]
    end

    C --> TC
    R --> H
    F --> G

    TC --> DB
    TC --> REDIS
    TC --> KAFKA
    TC --> RMQ
    TC --> S3

    H --> DB
    H --> REDIS
    H --> KAFKA
    H --> RMQ
    H --> S3
    H --> HTTP
    H --> GRPC
    H --> WS

    G --> H
```

## Test execution flow

When you run `tomato run`, the following sequence occurs:

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant Config
    participant TC as Testcontainers
    participant App as Application
    participant Handler
    participant Godog

    User->>CLI: tomato run
    CLI->>Config: Parse tomato.yml

    rect rgb(40, 40, 40)
        Note over TC: Container Setup
        Config->>TC: Start containers
        TC-->>TC: Wait for readiness
        TC-->>Config: Container endpoints
    end

    rect rgb(40, 40, 40)
        Note over Handler: Resources the app depends on
        Config->>Handler: Init app-env providers (aws STS)
        Handler-->>Config: Environment for the app
    end

    rect rgb(40, 40, 40)
        Note over App: App Setup (optional)
        Config->>App: Start application
        App-->>App: Wait for ready check
    end

    rect rgb(40, 40, 40)
        Note over Handler: Resource Setup
        Config->>Handler: Initialize remaining handlers
        Handler-->>Handler: Connect to services
    end

    rect rgb(40, 40, 40)
        Note over Godog: Test Execution
        CLI->>Godog: Run features
        loop Each Scenario
            Godog->>Handler: Reset resources
            loop Each Step
                Godog->>Handler: Execute step
                Handler-->>Godog: Result
            end
        end
    end

    Godog-->>CLI: Test results
    CLI->>App: Stop application
    CLI->>Handler: Close resources
    CLI->>TC: Stop containers
    CLI-->>User: Exit code
```

## Handler architecture

Handlers are responsible for translating Gherkin steps into actions against resources:

```mermaid
flowchart LR
    subgraph Steps["Gherkin Steps"]
        S1["Given 'db' table 'users' has values:"]
        S2["When 'api' sends 'GET' to '/users/1'"]
        S3["Then 'api' response status is '200'"]
    end

    subgraph Handlers["Handler Registry"]
        PG[PostgreSQL Handler]
        HTTP[HTTP Client Handler]
        REDIS[Redis Handler]
        KAFKA[Kafka Handler]
        WS[WebSocket Handler]
        SHELL[Shell Handler]
    end

    subgraph Resources["Resources"]
        DB[(Database)]
        API[HTTP API]
        CACHE[(Cache)]
        MQ[(Message Queue)]
        WSS[WebSocket Server]
        CMD[Shell]
    end

    S1 --> PG
    S2 --> HTTP
    S3 --> HTTP

    PG --> DB
    HTTP --> API
    REDIS --> CACHE
    KAFKA --> MQ
    WS --> WSS
    SHELL --> CMD
```

## Resource lifecycle

Each resource follows a consistent lifecycle:

```mermaid
stateDiagram-v2
    [*] --> Configured: Parse config
    Configured --> Initialized: Initialize handler
    Initialized --> Connected: Connect to service
    Connected --> Ready: Ready for tests

    Ready --> Reset: Before scenario
    Reset --> Ready: Clean state

    Ready --> Closed: Tests complete
    Closed --> [*]
```

## Container orchestration

tomato uses Testcontainers to manage Docker containers:

```mermaid
flowchart TB
    subgraph Config["Configuration"]
        YAML["tomato.yml"]
    end

    subgraph Testcontainers["Testcontainers Runtime"]
        CM[Container Manager]
        NW[Network]

        subgraph Containers
            C1[postgres:15]
            C2[redis:7]
            C3[apache/kafka:3.9.1]
        end
    end

    subgraph WaitStrategies["Wait Strategies"]
        PORT[Port Check]
        HTTP_CHECK[HTTP Check]
        LOG[Log Pattern]
    end

    YAML --> CM
    CM --> NW
    CM --> C1
    CM --> C2
    CM --> C3

    C1 --> PORT
    C2 --> PORT
    C3 --> LOG
```

## Data flow example

Here's how a typical database test scenario flows through the system:

```mermaid
sequenceDiagram
    participant Feature as Feature File
    participant Godog
    participant PG as PostgreSQL Handler
    participant DB as PostgreSQL

    Note over Feature: Scenario: Create user

    Feature->>Godog: Given "db" table "users" has values:
    Godog->>PG: SetTableValues(table, data)
    PG->>DB: INSERT INTO users...
    DB-->>PG: OK
    PG-->>Godog: Success

    Feature->>Godog: When "db" executes:
    Godog->>PG: ExecuteQuery(query)
    PG->>DB: SELECT * FROM users
    DB-->>PG: Results
    PG-->>Godog: Store results

    Feature->>Godog: Then "db" query "..." returns:
    Godog->>PG: AssertQueryResult(expected)
    PG-->>Godog: Match/No match
```

## HTTP request/response flow

```mermaid
sequenceDiagram
    participant Feature as Feature File
    participant HTTP as HTTP Handler
    participant API as Target API

    Feature->>HTTP: Given "api" header "X-Token" is "abc"
    HTTP-->>HTTP: Store header

    Feature->>HTTP: When "api" sends "POST" to "/users" with json:
    HTTP->>API: POST /users
    API-->>HTTP: 201 Created + JSON body
    HTTP-->>HTTP: Store response

    Feature->>HTTP: Then "api" response status is "201"
    HTTP-->>HTTP: Assert status

    Feature->>HTTP: And "api" response json "id" exists
    HTTP-->>HTTP: Assert JSON path
```

## Configuration structure

```mermaid
flowchart TB
    subgraph tomato.yml
        direction TB
        V[version: 2]

        subgraph containers
            C1[name: postgres<br/>image: postgres:15<br/>env: ...<br/>wait_for: ...]
            C2[name: redis<br/>image: redis:7]
        end

        subgraph resources
            R1[name: db<br/>type: postgres<br/>container: postgres]
            R2[name: cache<br/>type: redis<br/>container: redis]
            R3[name: api<br/>type: http<br/>base_url: ...]
        end

        subgraph app
            A[command: go run ./cmd/server<br/>port: 8080<br/>ready: http /health<br/>env: ...]
        end

        subgraph features
            F[paths:<br/>  - ./features]
        end
    end
```

## Error handling

When a step fails, tomato provides detailed error information:

```mermaid
flowchart TB
    Step[Step Execution] --> Check{Success?}
    Check -->|Yes| Next[Next Step]
    Check -->|No| Error[Error Details]

    Error --> Type{Error Type}
    Type -->|Connection| Conn[Connection failed to resource]
    Type -->|Assertion| Assert[Expected vs Actual mismatch]
    Type -->|Timeout| Timeout[Operation timed out]
    Type -->|Syntax| Syntax[Invalid step syntax]

    Conn --> Report[Report to User]
    Assert --> Report
    Timeout --> Report
    Syntax --> Report

    Report --> Cleanup[Cleanup Resources]
    Cleanup --> Exit[Exit with error code]
```
