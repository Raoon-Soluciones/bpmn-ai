# AGENTS.md

## What this is

Standalone BPMN 2.0 execution engine written in Go. High-performance, production-ready. Module path: `github.com/Raoon-Soluciones/bpmn-ai`.

## Project structure

| Directory | Purpose |
|---|---|
| `cmd/engine/` | CLI entry point — wires deps, starts HTTP server, graceful shutdown |
| `internal/engine/` | Core execution engine — iterative loop, flow router, fail-safe manager, element registry |
| `internal/element/` | BPMN element implementations — events, gateways, activities, flows |
| `internal/process/` | Process & thread management — state machine, instance lifecycle |
| `internal/queue/` | Job queue system — worker pool, retry policy, dead letter queue |
| `internal/observability/` | Logging, metrics, events — slog, Prometheus, event dispatcher |
| `pkg/bpmn/` | BPMN model types & XML parser |
| `pkg/store/` | Persistence interface + in-memory implementation (testing) |
| `api/http/` | REST API with chi router — handlers, routes, health checks |
| `api/middleware/` | HTTP middleware — logging, recovery, request ID, rate limiter, CSRF |
| `config/` | Configuration system |
| `testdata/` | BPMN test files |

## Key architecture

- **Engine core**: `internal/engine/engine.go` — iterative execution loop with goroutine workers (no recursion)
- **Flow router**: `internal/engine/router.go` — determines next elements, handles parallel branches with thread tracking
- **Fail-safe**: `internal/engine/failsafe.go` — timeout + loop detection
- **Element registry**: `internal/engine/registry.go` — factory pattern for BPMN elements
- **HTTP API**: `api/http/server.go` — chi router with middleware chain (logging, recovery, request ID, CORS, CSRF, rate limiter)
- **Job queue**: `internal/queue/worker.go` — worker pool with configurable concurrency, exponential backoff retry, dead letter queue

## Supported BPMN elements

| Category | Elements |
|---|---|
| **Events** | StartEvent, EndEvent, TerminateEvent, TimerEvent, MessageThrow, MessageCatch |
| **Gateways** | ExclusiveGateway, ParallelGateway, InclusiveGateway, EventBasedGateway |
| **Activities** | UserTask, ScriptTask, ServiceTask |
| **Flows** | SequenceFlow |

## State machine

```
CREATED → IN_PROGRESS → COMPLETED
    │         │
    │         ├──→ WAITING → IN_PROGRESS
    │         │      └──→ TERMINATED
    │         ├──→ SUSPENDED → IN_PROGRESS
    │         └──→ ERROR → IN_PROGRESS
    └──→ ERROR
```

## Conventions

- **Go 1.26+** — use modern patterns, no external DI framework (factory functions)
- **No ORM** — explicit queries via `pkg/store` interface (pgx for PostgreSQL, in-memory for tests)
- **chi router** — idiomatic HTTP routing, no reflection overhead
- **slog** — structured logging from stdlib, no external logger dependency
- **Prometheus** — official `client_golang` for metrics
- **Factory pattern** — `ElementRegistry.Register()` for element types, never hardcode
- **Iterative execution** — goroutine workers + channels, no recursion

## How to verify changes

```bash
# Run all tests
make test

# Unit tests only
make test-unit

# Tests with coverage
make test-coverage

# Build binary
make build

# Lint
make lint

# Tidy dependencies
make tidy
```

After any changes, run `go test -race -count=1 ./internal/... ./pkg/... ./api/... ./config/...` to verify.

## Configuration

Engine is configured via `config/` package with YAML + environment variables. Key settings:

- `server.host` / `server.port` — HTTP listener
- `database.url` — PostgreSQL connection string
- `engine.worker_count` — goroutine worker concurrency
- `engine.max_loops` — loop detection limit
- `engine.execution_timeout` — per-execution timeout
- `engine.queue_poll_interval` — job queue polling frequency
- `log.level` / `log.format` — logging configuration
- `server.disable_csrf` — disables CSRF middleware (useful for tests / local dev)

## Gotchas

- `-race` flag requires `CGO_ENABLED=1` and a C compiler (gcc/MinGW on Windows). Use `go test -count=1 ./...` if cgo is unavailable.
- ServiceTask uses `ActionQueue` — routes to async job queue, process continues without waiting for completion.
- UserTask uses `ActionForm` — instance transitions to `WAITING` state until task is completed externally.
- ParallelGateway creates new threads for each branch — thread index formula: `parentThreadIdx * 10 + branchIndex + 1`.
- The in-memory store (`pkg/store/memory/`) is for testing only — production uses PostgreSQL via `pkg/store/sql/`.
