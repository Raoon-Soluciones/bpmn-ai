# BPMN Engine

🌐 **English** | [Español](README.es.md)

> Standalone BPMN 2.0 execution engine written in Go. High-performance, production-ready.

## Quick Start

### Run in 30 seconds

```bash
# Clone & run
git clone https://github.com/Raoon-Soluciones/bpmn-ai.git && cd bpmn-ai
go run ./cmd/engine
```

### Verify it's running

```bash
curl http://localhost:8080/health
# {"status":"ok","timestamp":"...","version":"0.1.0"}
```

### Try the API

```bash
# Register a process
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test",
    "bpmn_xml": "<?xml version=\"1.0\"?><definitions xmlns=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" targetNamespace=\"test\"><process id=\"p1\" name=\"Test\"><startEvent id=\"s1\"/><endEvent id=\"e1\"/><sequenceFlow id=\"f1\" sourceRef=\"s1\" targetRef=\"e1\"/></process></definitions>"
  }'

# List processes
curl http://localhost:8080/api/v1/processes

# List cases
curl http://localhost:8080/api/v1/cases
```

---

## What's Implemented

All 6 phases of the rewrite plan are **complete** and **production-ready**.

### Phase 1: Foundation ✅
- Domain types, BPMN XML parser
- Store interface + in-memory implementation
- State machine with 8 states and valid transitions
- Configuration system + structured logging (slog)

### Phase 2: Execution Loop ✅
- Iterative engine with goroutine workers (no recursion)
- Flow router with thread tracking for parallel branches
- Fail-safe manager (timeout + loop detection)
- Element registry with factory pattern
- StartEvent, EndEvent, ExclusiveGateway, ParallelGateway

### Phase 3: Elements ✅
- **Activities**: UserTask, ScriptTask, ServiceTask
- **Events**: TimerEvent, MessageThrowEvent, MessageCatchEvent, TerminateEvent
- **Gateways**: InclusiveGateway, EventBasedGateway

### Phase 4: Queue & Async ✅
- Job queue with persistence layer
- Worker pool with configurable concurrency
- Retry policy with exponential backoff + jitter
- Dead letter queue for failed jobs
- Scheduled job support

### Phase 5: API & Observability ✅
- REST API with chi router (13 endpoints)
- Middleware chain: logging, recovery, request ID, CORS
- Prometheus metrics (10 metrics)
- Event dispatcher (12 event types, sync/async)
- Health check + readiness endpoints

### Phase 6: Production Ready ✅
- Docker multi-stage build (~15MB final image)
- docker-compose with PostgreSQL 16
- GitHub Actions CI (test, build, docker)
- 98 tests, all passing with `-race` detector
- Comprehensive README

---

## Running Options

### Option 1: Direct with Go (development)

```bash
go run ./cmd/engine
```

### Option 2: Build + run (production)

```bash
go build -o bpmn-ai ./cmd/engine
./bpmn-ai
```

### Option 3: Docker

```bash
docker build -t bpmn-ai:latest .
docker run -p 8080:8080 bpmn-ai:latest
```

### Option 4: Docker Compose (with PostgreSQL)

```bash
docker-compose up -d
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP API (chi)                        │
│  /health  /metrics  /api/v1/processes  /api/v1/cases        │
├─────────────────────────────────────────────────────────────┤
│                        Engine Core                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Engine  │→ │  Router  │→ │ FailSafe │  │  Registry  │  │
│  │  (loop)  │  │          │  │ Manager  │  │            │  │
│  └──────────┘  └──────────┘  └──────────┘  └────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     BPMN Elements                            │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐    │
│  │ Events  │ │ Gateways │ │Activities│ │    Flows     │    │
│  │ Start   │ │ Parallel │ │ UserTask │ │  Sequence    │    │
│  │ End     │ │ Exclusive│ │ScriptTask│ │              │    │
│  │ Timer   │ │Inclusive │ │SrvceTask │ │              │    │
│  │ Message │ │EventBased│ │          │ │              │    │
│  │Term     │ │          │ │          │ │              │    │
│  └─────────┘ └──────────┘ └──────────┘ └──────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                     Observability                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Prometheus  │  │  Event Disp  │  │  Structured Log  │  │
│  │  Metrics     │  │  (sync/async)│  │  (slog)          │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Persistence Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  PostgreSQL  │  │  In-Memory   │  │   Job Queue      │  │
│  │  (pgx/v5)    │  │  (tests)     │  │   (async)        │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Execution Loop

```
Request → Engine.Run()
            │
            ├── workCh (buffered channel, 1024)
            │     └── N goroutine workers process elements
            │
            ├── resultCh (buffered channel, 1024)
            │     └── Main loop handles results
            │
            ├── FailSafe checks (timeout + loop count)
            │
            ├── FlowRouter determines next elements
            │     └── Parallel branches → new threads
            │
            └── pending tracking → closes when done
```

### Job Queue & Async Processing

```
ServiceTask → ActionQueue
    │
    ├── Create JobRecord (instance, flow, type, payload)
    ├── Enqueue → WorkerPool
    │       │
    │       ├── Workers poll every 5s (configurable)
    │       ├── Concurrency: N workers in parallel
    │       ├── job.Status = RUNNING
    │       ├── Execute handler
    │       │   ├── OK → COMPLETED
    │       │   └── Error → RetryPolicy
    │       │       ├── retries < max → PENDING + exponential backoff
    │       │       └── retries >= max → Dead Letter Queue
    │       └── Scheduled jobs: only processed when scheduled_at <= now
    │
    └── Route → next flows (process continues without waiting)
```

#### Retry Policy

```
delay = BaseDelay × 2^retryCount  (capped at MaxDelay)
+ jitter (±25%) to prevent thundering herd
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `MaxRetries` | 3 | Maximum retry attempts |
| `BaseDelay` | 1s | Initial backoff delay |
| `MaxDelay` | 5m | Maximum backoff cap |
| `Jitter` | true | Randomize delays |

#### Job States

```
PENDING ──→ RUNNING ──→ COMPLETED
    │           │
    │           └──→ FAILED ──→ PENDING (retry)
    │                              │
    │                              └──→ FAILED ──→ ... (until max retries)
    │                                                  │
    │                                                  └──→ DEAD (dead letter)
    └──→ DEAD (direct)
```

---

## Supported BPMN Elements

### Events
| Element | Status | Description |
|---------|--------|-------------|
| Start Event | ✅ | Process entry point |
| End Event | ✅ | Process completion |
| Terminate Event | ✅ | Immediate process termination |
| Timer Event | ✅ | Time-based triggers (duration, date, cycle) |
| Message Throw Event | ✅ | Send message |
| Message Catch Event | ✅ | Wait for message |

### Gateways
| Element | Status | Description |
|---------|--------|-------------|
| Exclusive Gateway | ✅ | XOR — route to first matching condition |
| Parallel Gateway | ✅ | AND — route to all branches / wait for all |
| Inclusive Gateway | ✅ | OR — route to all matching conditions |
| Event-Based Gateway | ✅ | Route based on which event occurs first |

### Activities
| Element | Status | Description |
|---------|--------|-------------|
| User Task | ✅ | Human task with assignment |
| Script Task | ✅ | Automated script execution |
| Service Task | ✅ | External service call (async queue) |

### Flows
| Element | Status | Description |
|---------|--------|-------------|
| Sequence Flow | ✅ | Default flow between elements |
| Conditional Flow | 🔄 | Flow with condition expression |
| Default Flow | ✅ | Fallback flow for gateways |

---

## API Reference

### Health & Metrics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/metrics` | Prometheus metrics |

### Processes

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/processes` | Register process |
| GET | `/api/v1/processes` | List processes |
| GET | `/api/v1/processes/{id}` | Get process details |
| POST | `/api/v1/processes/{id}/start` | Start case |

### Cases

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/cases` | List cases |
| GET | `/api/v1/cases/{id}` | Case details |
| GET | `/api/v1/cases/{id}/tasks` | Pending tasks |
| POST | `/api/v1/tasks/{id}/claim` | Claim task |
| POST | `/api/v1/tasks/{id}/complete` | Complete task |
| GET | `/api/v1/cases/{id}/history` | Execution history |
| GET | `/api/v1/cases/{id}/diagram` | Process diagram |

### Middleware

All requests include:
- **Request ID** — auto-generated `X-Request-ID` header
- **Structured logging** — method, path, status, duration, request ID
- **Panic recovery** — 500 on panic with error logging
- **CORS** — configurable allowed origins

---

## Configuration

```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 15s
  write_timeout: 30s

database:
  url: postgres://postgres:postgres@localhost:5432/bpmn?sslmode=disable
  max_conns: 25
  min_conns: 5
  max_idle_time: 30m

engine:
  worker_count: 4
  max_loops: 100
  execution_timeout: 30s
  queue_poll_interval: 5s
  max_retries: 3

log:
  level: info
  format: json
```

---

## State Machine

```
CREATED ──→ IN_PROGRESS ──→ COMPLETED
    │           │
    │           ├──→ WAITING ──→ IN_PROGRESS
    │           │      │
    │           │      └──→ TERMINATED
    │           │
    │           ├──→ SUSPENDED ──→ IN_PROGRESS
    │           │
    │           └──→ ERROR ──→ IN_PROGRESS
    │
    └──→ ERROR
```

### Valid Transitions

| From | To |
|------|-----|
| CREATED | IN_PROGRESS, ERROR |
| IN_PROGRESS | WAITING, SUSPENDED, COMPLETED, ERROR, TERMINATED |
| WAITING | IN_PROGRESS, ERROR, TERMINATED |
| SUSPENDED | IN_PROGRESS, ERROR |
| ERROR | IN_PROGRESS |
| COMPLETED | (terminal) |
| TERMINATED | (terminal) |

---

## Observability

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `bpmn_processes_active` | Gauge | Active process instances |
| `bpmn_cases_total` | Counter | Total cases started (by process) |
| `bpmn_cases_by_status` | Gauge | Cases by status |
| `bpmn_element_duration_ms` | Histogram | Element execution duration |
| `bpmn_element_errors_total` | Counter | Element execution errors |
| `bpmn_queue_depth` | Gauge | Pending jobs in queue |
| `bpmn_queue_retries_total` | Counter | Job retries (by type) |
| `bpmn_queue_dead_letters_total` | Counter | Dead lettered jobs |
| `bpmn_http_request_duration_ms` | Histogram | HTTP request duration |
| `bpmn_http_request_errors_total` | Counter | HTTP request errors |

### Event Dispatcher

The engine emits domain events for observability:

| Event | Description |
|-------|-------------|
| `process.started` | Process instance started |
| `process.completed` | Process instance completed |
| `process.terminated` | Process instance terminated |
| `process.error` | Process instance errored |
| `element.executed` | Element executed |
| `element.error` | Element execution failed |
| `task.claimed` | User task claimed |
| `task.completed` | User task completed |
| `job.queued` | Job enqueued |
| `job.completed` | Job completed |
| `job.failed` | Job failed |
| `job.dead` | Job moved to dead letter |

Events can be consumed synchronously or asynchronously via the `Dispatcher`.

---

## Project Structure

```
bpmn-ai/
├── cmd/engine/                    # CLI entry point
│   └── main.go                    # Wire deps, graceful shutdown
├── internal/                      # Private application code
│   ├── engine/                    # Core execution engine
│   │   ├── engine.go              # Iterative execution loop
│   │   ├── context.go             # Immutable execution context
│   │   ├── result.go              # Execution result types
│   │   ├── router.go              # Flow routing logic
│   │   ├── failsafe.go            # Timeout & loop detection
│   │   └── registry.go            # Element factory registry
│   ├── element/                   # BPMN element implementations
│   │   ├── element.go             # Base interfaces
│   │   ├── activity.go            # Activity interface
│   │   ├── gateway.go             # Gateway interface
│   │   ├── event.go               # Event interface
│   │   ├── flow.go                # Flow interface
│   │   ├── events/                # Start, End, Terminate, Timer, Message
│   │   ├── gateways/              # Parallel, Exclusive, Inclusive, EventBased
│   │   ├── activities/            # UserTask, ScriptTask, ServiceTask
│   │   └── flows/                 # SequenceFlow
│   ├── process/                   # Process & thread management
│   │   └── state.go               # State machine + Instance
│   ├── queue/                     # Job queue system
│   │   ├── retry.go               # Retry policy with exponential backoff
│   │   ├── deadletter.go          # Dead letter queue
│   │   └── worker.go              # Worker pool with concurrency control
│   └── observability/             # Logging, metrics, events
│       ├── logger.go              # Structured logging (slog)
│       ├── metrics.go             # Prometheus metrics (10 metrics)
│       └── events.go              # Event dispatcher (sync/async)
├── pkg/                           # Public packages
│   ├── bpmn/                      # BPMN model & parser
│   │   ├── model.go               # Domain types
│   │   └── parser.go              # BPMN XML parser
│   └── store/                     # Persistence interfaces
│       ├── store.go               # Store interface
│       ├── sql/                   # PostgreSQL implementation
│       ├── migrations/            # Database migrations
│       └── memory/                # In-memory (testing)
├── api/                           # API layer
│   ├── http/                      # REST API (chi)
│   │   ├── server.go              # HTTP server setup
│   │   ├── routes.go              # Route definitions
│   │   ├── handlers.go            # API handlers
│   │   └── health.go              # Health check
│   └── middleware/                # HTTP middleware
│       ├── logging.go             # Request logging
│       ├── recovery.go            # Panic recovery
│       └── requestid.go           # Request ID
├── config/                        # Configuration
├── testdata/                      # BPMN test files
│   ├── simple_sequence.bpmn
│   ├── parallel_gateway.bpmn
│   ├── exclusive_gateway.bpmn
│   ├── timer_event.bpmn
│   └── complex_process.bpmn
├── Dockerfile                     # Multi-stage build (~15MB)
├── docker-compose.yml             # Engine + PostgreSQL
├── .github/workflows/ci.yml       # GitHub Actions CI
├── .dockerignore
├── .gitignore
├── go.mod
├── Makefile
├── .golangci.yml
└── README.md
```

---

## Database Schema

```sql
-- Process definitions
CREATE TABLE processes (
    id          UUID PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    version     INT NOT NULL DEFAULT 1,
    bpmn_xml    TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Process instances (cases)
CREATE TABLE instances (
    id              UUID PRIMARY KEY,
    process_id      UUID NOT NULL REFERENCES processes(id),
    title           VARCHAR(255),
    status          VARCHAR(20) NOT NULL DEFAULT 'IN_PROGRESS',
    current_user    UUID,
    variables       JSONB NOT NULL DEFAULT '{}',
    pin             VARCHAR(10) DEFAULT '0000',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

-- Flow execution records
CREATE TABLE flows (
    id              UUID PRIMARY KEY,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    element_id      VARCHAR(100) NOT NULL,
    element_type    VARCHAR(50) NOT NULL,
    thread_id       INT NOT NULL DEFAULT 1,
    previous_id     UUID REFERENCES flows(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    duration_ms     INT
);

CREATE INDEX idx_flows_instance ON flows(instance_id);
CREATE INDEX idx_flows_instance_thread_status ON flows(instance_id, thread_id, status);

-- Thread tracking
CREATE TABLE threads (
    id              SERIAL PRIMARY KEY,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    thread_index    INT NOT NULL,
    parent_index    INT,
    flow_id         UUID NOT NULL REFERENCES flows(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Job queue
CREATE TABLE jobs (
    id              UUID PRIMARY KEY,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    flow_id         UUID REFERENCES flows(id),
    type            VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    scheduled_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at     TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_status_scheduled ON jobs(status, scheduled_at);

-- Dead letter queue
CREATE TABLE dead_letters (
    id              UUID PRIMARY KEY,
    job_id          UUID NOT NULL,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    payload         JSONB NOT NULL,
    error_message   TEXT NOT NULL,
    retry_count     INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Execution log
CREATE TABLE execution_log (
    id              UUID PRIMARY KEY,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    element_id      VARCHAR(100) NOT NULL,
    element_type    VARCHAR(50) NOT NULL,
    action          VARCHAR(20) NOT NULL,
    duration_ms     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exec_log_instance ON execution_log(instance_id);
```

---

## Development

### Prerequisites

- Go 1.23+
- PostgreSQL 16+ (for production)
- Docker + Docker Compose (for testing)

### Makefile Targets

```bash
make test              # Run all tests
make test-unit         # Unit tests only
make test-integration  # Integration tests (requires Docker)
make test-e2e          # End-to-end tests
make test-coverage     # Tests with coverage report
make bench             # Benchmark tests
make fuzz              # Fuzz testing
make lint              # Run golangci-lint
make tidy              # go mod tidy + verify
make build             # Build binary
```

### Running Tests

```bash
# All tests
go test ./... -v -count=1

# With race detector
go test -race ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/engine/ -v

# Benchmarks
go test -bench=. -benchmem ./internal/engine/

# Fuzz testing
go test -fuzz=Fuzz -fuzztime=30s ./pkg/bpmn/
```

### Test Coverage

```
Package                          Coverage
──────────────────────────────────────────
config                           100.0%
internal/element/flows           100.0%
api/middleware                    92.9%
internal/observability            92.9%
internal/process                  89.3%
internal/queue                    84.8%
internal/engine                   81.4%
internal/element/activities       80.5%
internal/element/events           79.6%
pkg/bpmn                          74.3%
api/http                          70.6%
pkg/store/memory                  57.2%
```

**98 tests total**, all passing with `-race` detector.

---

## Using the Engine Programmatically

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/Raoon-Soluciones/bpmn-ai/internal/engine"
    "github.com/Raoon-Soluciones/bpmn-ai/internal/element/events"
    "github.com/Raoon-Soluciones/bpmn-ai/internal/element/gateways"
    "github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
    "github.com/Raoon-Soluciones/bpmn-ai/internal/process"
    "github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
    "github.com/Raoon-Soluciones/bpmn-ai/pkg/store/memory"
    "github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
)

func main() {
    // 1. Parse a BPMN file
    parser := bpmn.NewParser()
    proc, err := parser.ParseFile("process.bpmn")
    if err != nil {
        log.Fatal(err)
    }

    // 2. Register element implementations
    registry := engine.NewElementRegistry()
    registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
    registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
    registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)

    // 3. Create store, queue, and logger
    store := memory.NewStore()
    logger, _ := observability.NewFromConfig("info", "json")
    retry := queue.DefaultRetryPolicy()
    dlq := queue.NewDeadLetterQueue(store)
    q := queue.NewWorkerPool(store, nil, retry, dlq, queue.WorkerPoolConfig{
        Concurrency:  4,
        PollInterval: 5 * time.Second,
    })

    // 4. Create engine
    eng := engine.New(engine.Config{
        WorkerCount:      4,
        MaxLoops:         100,
        ExecutionTimeout: 30 * time.Second,
    }, registry, store, logger, q)

    // 5. Create and run instance
    instance := process.NewInstance(proc, map[string]any{
        "amount": 5000,
    })
    store.CreateInstance(context.Background(), instance.ToRecord())

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := eng.Run(ctx, instance); err != nil {
        log.Fatal(err)
    }

    log.Printf("Process completed with state: %s", instance.State)
}
```

---

## Frameworks & Dependencies

| Category | Library | Import path | Purpose |
|----------|---------|-------------|---------|
| **HTTP Router** | chi/v5 | `github.com/go-chi/chi/v5` | Lightweight, idiomatic HTTP routing |
| **CORS** | cors | `github.com/go-chi/cors` | CORS middleware for chi |
| **Prometheus** | client_golang | `github.com/prometheus/client_golang` | Official Prometheus metrics |
| **PostgreSQL** | pgx/v5 | `github.com/jackc/pgx/v5` | High-performance DB driver |
| **SQL Utils** | sqlx | `github.com/jmoiron/sqlx` | Named queries, struct scan |
| **Migrations** | go-migrate | `github.com/golang-migrate/migrate/v4` | Database versioning |
| **Config** | viper | `github.com/spf13/viper` | YAML + env configuration |
| **CLI** | cobra | `github.com/spf13/cobra` | Command-line interface |
| **Validation** | validator | `github.com/go-playground/validator/v10` | Tag-based validation |
| **JWT** | jwt-go | `github.com/golang-jwt/jwt/v5` | Authentication |
| **UUID** | uuid | `github.com/google/uuid` | RFC 4122 UUIDs |
| **Scheduler** | cron | `github.com/robfig/cron/v3` | Timer event scheduling |
| **Logger** | log/slog | `log/slog` (stdlib) | Structured logging (Go 1.21+) |
| **Context** | context | `context` (stdlib) | Cancellation, timeouts |
| **Testing** | testify | `github.com/stretchr/testify` | Assertions only |
| **Docker Tests** | dockertest | `github.com/ory/dockertest/v3` | Integration test containers |
| **PNG Gen** | gg | `github.com/fogleman/gg` | 2D drawing for process diagrams |
| **Rate Limit** | tollbooth | `github.com/didip/tollbooth/v7` | API rate limiting |

### Not Used

| Library | Reason |
|---------|--------|
| Gin/Echo/Fiber | Too "magical", chi is more idiomatic |
| GORM/ent | ORM heavy, prefer explicit queries with pgx |
| wire (Google DI) | Overkill, factory functions are sufficient |
| zap/zerolog | slog (stdlib) covers all needs |
| go-kit | Too complex for a standalone service |

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go 1.23+ | Concurrency, performance, single binary |
| No ORM | pgx + sqlx | Explicit queries, no N+1 surprises |
| No DI framework | Factory functions | Go prefers explicit composition |
| Router | chi | Idiomatic, no reflection overhead |
| Logger | slog (stdlib) | No external dependency needed |
| No sub-processes | Out of scope | Reduces complexity for v1 |
| PostgreSQL | Primary DB | JSONB, UUIDs, mature ecosystem |
| Alpine base | Docker | ~15MB final image, non-root user |

---

## CI/CD

GitHub Actions pipeline on every push and PR:

```
test → build (6 platforms) → docker
```

- **Tests**: `-race` flag, PostgreSQL 16 service container
- **Coverage**: Upload to Codecov
- **Build**: linux/darwin/windows × amd64/arm64
- **Docker**: Build + smoke test

---

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
