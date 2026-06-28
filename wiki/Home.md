# BPMN-AI Engine

A production-ready BPMN 2.0 execution engine written in Go with native AI/LLM integration. Built for high-performance process automation where human tasks, service calls, and AI decisions coexist in the same workflow.

---

## Overview

This engine executes BPMN 2.0 process diagrams and extends them with first-class AI capabilities.

### Key Features

| Feature | Description |
|---|---|
| **BPMN 2.0 Compliant** | Events, Gateways, Activities, Flows, Boundary Events — 19+ element types |
| **Native AI Integration** | First-class `aiTask` element with model routing, tool calling, RAG, multi-agent |
| **Multi-Provider AI** | OpenAI, Anthropic, OpenRouter (200+ models), Groq, custom providers |
| **Iterative Engine** | Non-recursive goroutine-based execution loop |
| **Sub-Process & Call Activity** | Nested diagram support via element flattening |
| **Boundary Events** | Timer, Message, Error boundary events on activities |
| **Job Queue** | Async execution with exponential backoff retry and dead letter queue |
| **HTTP API** | RESTful API with chi router, middleware stack |
| **Observability** | Structured logging (slog), Prometheus metrics, event dispatcher, audit trail |
| **Storage** | In-memory (dev) and PostgreSQL (production) |

---

## Architecture at a Glance

```
HTTP API (chi) → Engine Core (iterative loop) → BPMN Elements
                       ↕                            ↕
                 Persistence Layer            AI System (pkg/ai/)
                 (PostgreSQL/memory)     Model Router · RAG · Guardrails
```

---

## State Machine

```
CREATED ──→ IN_PROGRESS ──→ COMPLETED
    │            │
    │            ├──→ WAITING ──→ IN_PROGRESS
    │            │       └──→ TERMINATED
    │            ├──→ SUSPENDED ──→ IN_PROGRESS
    │            └──→ ERROR ──→ IN_PROGRESS
    └──→ ERROR
```

---

## Quick Start

### 1. Configuration

Create a `.env` file:

```bash
AI_OPENROUTER_API_KEY=sk-or-...    # or AI_API_KEY=sk-...
```

### 2. Run the Engine

```bash
go run ./cmd/engine/
```

### 3. Deploy a Process

```bash
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{"name":"My Process","bpmn_xml":"<?xml version=\"1.0\"?>..."}'
```

### 4. Start a Case

```bash
curl -X POST http://localhost:8080/api/v1/processes/{id}/start
```

---

## Repository Structure

| Path | Purpose |
|---|---|
| `cmd/engine/` | CLI entry point |
| `internal/engine/` | Core BPMN engine (loop, router, registry, failsafe) |
| `internal/element/` | BPMN element implementations (19+ types) |
| `internal/process/` | State machine, instance & thread lifecycle |
| `internal/queue/` | Job queue with retry & dead letter |
| `internal/observability/` | Logging, metrics, events, audit |
| `pkg/bpmn/` | BPMN model types & XML parser |
| `pkg/store/` | Storage interfaces (in-memory, PostgreSQL) |
| `pkg/ai/` | AI provider system (15+ files) |
| `api/http/` | REST API with chi router |
| `api/middleware/` | HTTP middleware |
| `config/` | Configuration system |
| `testdata/` | BPMN test files |
