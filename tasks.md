# Tomato v2 - Development Tasks

## Design Principles

1. **One config to rule them all** - `tomato.yml` defines containers, resources, hooks, and test config. No docker-compose.yml, no scattered files.

2. **Clean state, every time** - Each scenario starts fresh. No test pollution. Deterministic results.

3. **Everything through `tomato`** - One CLI for containers, tests, debugging, logs. No context switching.

4. **Zero infrastructure leakage** - Containers auto-cleanup. No orphans. No manual teardown.

5. **Batteries included, but removable** - Sensible defaults, full override capability.

---

## Phase 1: Foundation

### Project Setup
- [x] Initialize Go module (`github.com/tomatool/tomato`)
- [x] Set up project structure (cmd/, internal/, pkg/)
- [ ] Configure CI/CD (GitHub Actions)
- [ ] Set up linting (golangci-lint)
- [ ] Set up pre-commit hooks
- [x] Create Makefile with common targets (build, test, lint, release)

### Core Dependencies
- [x] Integrate Cobra for CLI framework
- [ ] Integrate Viper for configuration management
- [x] Integrate Testcontainers-go for container orchestration
- [x] Integrate Godog for BDD/Gherkin support
- [x] Integrate Zerolog for structured logging

### Configuration System
- [x] Define v2 config schema (YAML)
- [x] Implement config parser with validation
- [x] Support environment variable substitution
- [ ] Implement v1 config detection and migration hints
- [ ] Add `tomato migrate` command for v1 → v2 conversion

### Container Management
- [x] Implement container lifecycle manager (start, stop, restart)
- [x] Implement wait strategies (port, log, http, exec)
- [x] Implement container dependency ordering
- [ ] Implement parallel container startup
- [x] Integrate Testcontainers Ryuk for auto-cleanup
- [ ] Add container health checking
- [x] Implement `depends_on` resolution

---

## Phase 2: Resource System

### Resource Interface
- [x] Define `Resource` interface (Init, Ready, Reset, Steps, Cleanup)
- [x] Define `ResetStrategy` interface
- [x] Implement resource registry
- [x] Implement resource factory pattern

### Database Resources
- [x] **PostgreSQL**
  - [x] Connection management
  - [x] Truncate reset strategy
  - [ ] Snapshot/restore reset strategy
  - [x] Step definitions (SET table, compare table, execute SQL)
  - [ ] Schema support
- [ ] **MySQL**
  - [ ] Connection management
  - [ ] Truncate reset strategy
  - [ ] Step definitions

### Cache Resources
- [x] **Redis**
  - [x] Connection management
  - [x] Flush reset strategy
  - [x] Pattern-based reset strategy
  - [x] Step definitions (SET, GET, EXISTS, TTL, hash, list, set, incr/decr)

### Message Queue Resources
- [ ] **RabbitMQ**
  - [ ] Connection management
  - [ ] Purge reset strategy
  - [ ] Step definitions (publish, consume, count, compare)
- [x] **Kafka**
  - [x] Connection management (admin, producer, consumer)
  - [x] Delete/recreate reset strategy
  - [ ] Seek-to-end reset strategy
  - [x] Step definitions (publish, consume, ordering, headers)
  - [x] Topic management (create, delete)
  - [ ] Consumer group management
  - [ ] Schema Registry integration (optional)

### HTTP Resources
- [x] **HTTP Client**
  - [x] Request builder (GET, POST, PUT, DELETE, PATCH)
  - [x] Header management
  - [x] Body payload support (JSON, form, raw)
  - [x] Response assertions (status, headers, body, JSON path)
  - [ ] Cookie handling
  - [x] JSON matchers (@string, @number, @boolean, @array, @object, @any, @null, @notnull)
  - [x] Response time assertions
- [ ] **Wiremock**
  - [ ] Stub management
  - [ ] Reset mappings strategy
  - [ ] Step definitions (stub, verify calls)

### WebSocket Resource
- [x] Connection management (connect, disconnect)
- [x] Message sending (text, JSON)
- [x] Message receiving with timeout
- [x] Disconnect/reconnect reset strategy
- [x] Step definitions (connect, send, receive, assert)
- [x] Subprotocol support
- [x] Header/auth support

### Shell Resource
- [ ] Command execution
- [ ] Stdout/stderr capture
- [ ] Exit code verification
- [ ] Step definitions (execute, assert output, assert exit code)

---

## Phase 3: Reset System

### Reset Framework
- [ ] Implement reset level configuration (scenario, feature, run)
- [ ] Implement reset orchestrator (calls all resources)
- [ ] Add reset timing/logging
- [ ] Implement `--no-reset` flag for debugging
- [ ] Implement `--reset-strategy` override flag

### Reset Strategies per Resource
- [ ] PostgreSQL: truncate, snapshot, recreate
- [ ] MySQL: truncate, snapshot, recreate
- [ ] Redis: flush, pattern
- [ ] RabbitMQ: purge queues
- [ ] Kafka: delete/recreate topics, seek-to-end
- [ ] WebSocket: disconnect, reconnect
- [ ] Wiremock: reset mappings

### Hooks System
- [ ] Implement `before_all` hooks
- [ ] Implement `after_all` hooks
- [ ] Implement `before_scenario` hooks (run after reset)
- [ ] Implement `after_scenario` hooks
- [ ] Support hook types: sql, sql_file, exec, shell

---

## Phase 4: CLI Commands

### Basic Commands
- [ ] `tomato run` - Execute tests
- [ ] `tomato run <feature>` - Run specific feature file
- [ ] `tomato run --tags "@smoke"` - Filter by tags
- [ ] `tomato up` - Start containers only
- [ ] `tomato down` - Stop and remove containers
- [ ] `tomato validate` - Validate config file
- [ ] `tomato version` - Show version info

### Container Commands
- [ ] `tomato logs <container>` - Stream container logs
- [ ] `tomato logs <container> --follow` - Follow logs
- [ ] `tomato exec <container> <cmd>` - Execute command in container
- [ ] `tomato ps` - List running containers
- [ ] `tomato restart <container>` - Restart specific container

### Utility Commands
- [ ] `tomato init` - Interactive config generator (see detailed breakdown below)
- [ ] `tomato migrate` - Migrate v1 config to v2
- [ ] `tomato reset` - Manually reset all resources
- [ ] `tomato reset <resource>` - Reset specific resource

### `tomato init` - Project Onboarding Wizard

#### Project Detection
- [ ] Detect existing `docker-compose.yml` and offer to import services
- [ ] Detect existing `tomato.yml` (v1) and offer migration
- [ ] Detect project type (Go, Node, Python, Java) from manifest files
- [ ] Detect existing feature files in common locations
- [ ] Detect running Docker containers and offer to use them

#### Interactive Prompts (Bubble Tea)
- [ ] Welcome screen with tomato branding
- [ ] Project name input (default: directory name)
- [ ] Features path selection (default: `./features`)
- [ ] Resource selection checklist:
  ```
  Which resources do you need?
  [x] PostgreSQL
  [ ] MySQL
  [x] Redis
  [ ] RabbitMQ
  [x] Kafka
  [ ] MongoDB
  [ ] Elasticsearch
  [ ] Wiremock (HTTP mocking)
  [ ] LocalStack (AWS)
  ```
- [ ] Per-resource configuration:
  - Database name, user, password
  - Port preferences
  - Image version selection
- [ ] Application service configuration:
  - Image or build context
  - Environment variables
  - Port mappings
  - Health check endpoint
- [ ] Reset strategy preferences (truncate vs recreate)

#### Import from docker-compose.yml
- [ ] Parse existing docker-compose.yml
- [ ] Map services to tomato resources
- [ ] Extract environment variables
- [ ] Preserve volume mounts where relevant
- [ ] Show diff/preview before generating

#### Config Generation
- [ ] Generate `tomato.yml` with selected resources
- [ ] Generate starter feature file (`features/example.feature`)
- [ ] Generate `.gitignore` entries (if needed)
- [ ] Generate CI workflow file (optional, prompt user)

#### Post-Init Actions
- [ ] Validate generated config
- [ ] Offer to run `tomato up` to verify containers start
- [ ] Offer to run example test
- [ ] Print next steps guide

#### Non-Interactive Mode
- [ ] `tomato init --yes` - Accept all defaults
- [ ] `tomato init --from docker-compose.yml` - Import from compose
- [ ] `tomato init --resources postgres,redis,kafka` - Specify resources
- [ ] `tomato init --template microservices` - Use predefined template

#### Templates
- [ ] `basic` - HTTP API + PostgreSQL
- [ ] `microservices` - Multiple services + Kafka + Redis
- [ ] `event-driven` - Kafka + PostgreSQL
- [ ] `realtime` - WebSocket + Redis
- [ ] `full` - All resources (for exploration)

### Output Formats
- [ ] Pretty output (default, colored, human-readable)
- [ ] JSON output (`--output json`)
- [ ] JUnit XML output (`--output junit`)

---

## Phase 5: Developer Experience

### TUI (Bubble Tea)
- [ ] Main menu (Run, Watch, Up, Down, Debug)
- [ ] Test progress display
- [ ] Container status panel
- [ ] Log viewer
- [ ] Interactive scenario selector

### Watch Mode
- [ ] File watcher for feature files
- [ ] File watcher for config changes
- [ ] Smart re-run (only affected scenarios)
- [ ] Debouncing for rapid changes
- [ ] `tomato run --watch` implementation

### Debug Mode
- [ ] Step-through execution
- [ ] State inspection between steps
- [ ] Resource state viewer (DB tables, Redis keys, etc.)
- [ ] Breakpoint support
- [ ] `tomato debug <feature>:<line>` implementation

### Pretty Output
- [ ] Colored step results (green/red/yellow)
- [ ] Spinner for long-running operations
- [ ] Progress bar for multiple scenarios
- [ ] Reset status indicator
- [ ] Container startup status
- [ ] Summary statistics

---

## Phase 6: Extended Resources

### Additional Databases
- [ ] **MongoDB**
  - [ ] Connection management
  - [ ] Drop/delete reset strategy
  - [ ] Step definitions (insert, find, compare)
- [ ] **Elasticsearch**
  - [ ] Connection management
  - [ ] Delete indices reset strategy
  - [ ] Step definitions (index, search, assert)

### Cloud Emulation
- [ ] **LocalStack**
  - [ ] S3 operations (put, get, list, delete)
  - [ ] SQS operations (send, receive, purge)
  - [ ] DynamoDB operations (put, get, query)
  - [ ] Reset strategy per service

### Chaos Engineering
- [ ] **ToxiProxy**
  - [ ] Proxy management
  - [ ] Toxic injection (latency, bandwidth, timeout)
  - [ ] Step definitions (add toxic, remove toxic)

### gRPC
- [ ] Connection management
- [ ] Unary call support
- [ ] Streaming support (server, client, bidirectional)
- [ ] Step definitions (call, assert response, stream)
- [ ] Reflection support for dynamic calls

### GraphQL
- [ ] Query execution
- [ ] Mutation execution
- [ ] Subscription support (via WebSocket)
- [ ] Step definitions (query, mutate, subscribe)

---

## Phase 7: Advanced Features

### Parallel Execution
- [ ] Parallel scenario execution (`--parallel N`)
- [ ] Worker isolation strategies (database, container, namespace)
- [ ] Per-worker resource cloning
- [ ] Result aggregation

### Plugin System
- [ ] Define plugin interface
- [ ] Plugin discovery mechanism
- [ ] Plugin loading (Go plugins or exec-based)
- [ ] Plugin documentation generator

### CI/CD Integration
- [ ] GitHub Actions example workflow
- [ ] GitLab CI example
- [ ] Exit codes for CI (0 = pass, 1 = fail, 2 = config error)
- [ ] Artifact generation (reports, screenshots)

### Reporting
- [ ] HTML report generation
- [ ] Test history tracking
- [ ] Flaky test detection
- [ ] Performance metrics (step duration)

---

## Phase 8: Documentation & Release

### Documentation
- [ ] README.md with quick start
- [ ] Configuration reference
- [ ] Resource documentation (all step definitions)
- [ ] Migration guide (v1 → v2)
- [ ] Examples directory
- [ ] Architecture documentation

### Examples
- [ ] Basic HTTP API testing
- [ ] Database + HTTP integration
- [ ] Kafka event-driven testing
- [ ] WebSocket real-time testing
- [ ] Full microservices example

### Release
- [ ] Goreleaser configuration
- [ ] Docker image (multi-arch)
- [ ] Homebrew formula
- [ ] Installation script
- [ ] Changelog automation

---

## Backlog / Future Considerations

- [ ] Remote container execution (Testcontainers Cloud)
- [ ] Distributed test execution
- [ ] Visual test recorder (record browser actions → Gherkin)
- [ ] AI-assisted step generation
- [ ] Test data generators (faker integration)
- [ ] Snapshot testing for API responses
- [ ] Contract testing integration (Pact)
- [ ] OpenTelemetry tracing for test runs
- [ ] VS Code extension (syntax highlighting, run tests)
