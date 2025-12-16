# Architecture

This page provides an overview of tomato's internal architecture and how components interact during test execution.

## High-Level Overview

tomato is designed around three core concepts:

1. **Containers** - Docker containers managed via Testcontainers
2. **Resources** - Abstractions over external services (databases, APIs, message queues)
3. **Handlers** - Step definitions that interact with resources

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
        HTTP[HTTP Server]
        WS[WebSocket]
    end

    C --> TC
    R --> H
    F --> G

    TC --> DB
    TC --> REDIS
    TC --> KAFKA

    H --> DB
    H --> REDIS
    H --> KAFKA
    H --> HTTP
    H --> WS

    G --> H
```

## Test Execution Flow

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
        Note over App: App Setup (optional)
        Config->>App: Start application
        App-->>App: Wait for ready check
    end

    rect rgb(40, 40, 40)
        Note over Handler: Resource Setup
        Config->>Handler: Initialize handlers
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
    CLI->>TC: Stop containers
    CLI->>App: Stop application
    CLI-->>User: Exit code
```

## Handler Architecture

Handlers are responsible for translating Gherkin steps into actions against resources:

```mermaid
flowchart LR
    subgraph Steps["Gherkin Steps"]
        S1["Given I set 'db' table..."]
        S2["When I send 'GET' request..."]
        S3["Then response status is '200'"]
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

## Resource Lifecycle

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

## Container Orchestration

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
            C3[kafka:latest]
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

## Data Flow Example

Here's how a typical database test scenario flows through the system:

```mermaid
sequenceDiagram
    participant Feature as Feature File
    participant Godog
    participant PG as PostgreSQL Handler
    participant DB as PostgreSQL

    Note over Feature: Scenario: Create user

    Feature->>Godog: Given I set "db" table "users"...
    Godog->>PG: SetTableValues(table, data)
    PG->>DB: INSERT INTO users...
    DB-->>PG: OK
    PG-->>Godog: Success

    Feature->>Godog: When I execute on "db" query...
    Godog->>PG: ExecuteQuery(query)
    PG->>DB: SELECT * FROM users
    DB-->>PG: Results
    PG-->>Godog: Store results

    Feature->>Godog: Then "db" query result should be...
    Godog->>PG: AssertQueryResult(expected)
    PG-->>Godog: Match/No match
```

## HTTP Request/Response Flow

```mermaid
sequenceDiagram
    participant Feature as Feature File
    participant HTTP as HTTP Handler
    participant API as Target API

    Feature->>HTTP: Given I set request header...
    HTTP-->>HTTP: Store header

    Feature->>HTTP: When I send "POST" to "/users"
    HTTP->>API: POST /users
    API-->>HTTP: 201 Created + JSON body
    HTTP-->>HTTP: Store response

    Feature->>HTTP: Then response status is "201"
    HTTP-->>HTTP: Assert status

    Feature->>HTTP: And response JSON "id" exists
    HTTP-->>HTTP: Assert JSON path
```

## Configuration Structure

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

## Error Handling

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
