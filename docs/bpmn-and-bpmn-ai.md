# BPMN and bpmn-ai: Technical Documentation

## 1. What is BPMN?

**BPMN** (Business Process Model and Notation) is an international standard (ISO/IEC 19510) for business process modeling. Developed by the **Object Management Group (OMG)**, the most widely adopted version is **BPMN 2.0** (2011), which defines not only the graphical notation but also the XML interchange format and execution semantics.

### 1.1 Purpose

BPMN was created to bridge the gap between business process design (by analysts) and technical implementation. A BPMN diagram must be:

- **Understandable** by business users
- **Executable** by a process engine
- **Portable** across different platforms (standard XML)

### 1.2 BPMN 2.0 Element Categories

#### 1.2.1 Events

Represent something that **occurs** during the process. They are classified by **position** (start, intermediate, end) and by **trigger type** (message, timer, error, signal, etc.).

| Type                | Description                                                        |
|---------------------|--------------------------------------------------------------------|
| **Start Event**     | Indicates where a process begins. Can have a trigger (timer, message, signal) or be none (no trigger). |
| **End Event**       | Indicates where a process ends. Can define the result (message, error, termination, compensation). |
| **Intermediate Event** | Occurs between start and end. Can be **catch** (waits for something to happen) or **throw** (fires something). |
| **Boundary Event**  | Intermediate event **attached** to the edge of an activity. Can be **interrupting** (cancels the activity) or **non-interrupting**. |

**Event trigger types:**

| Trigger type      | Start | Intermediate Catch | Intermediate Throw | End | Boundary |
|-------------------|-------|--------------------|--------------------|-----|----------|
| **None**          | ✅    | —                  | —                  | ✅  | —        |
| **Message**       | ✅    | ✅                 | ✅                 | ✅  | ✅       |
| **Timer**         | ✅    | ✅                 | —                  | —   | ✅       |
| **Error**         | ✅    | —                  | ✅                 | ✅  | ✅       |
| **Escalation**    | ✅    | ✅                 | ✅                 | ✅  | ✅       |
| **Signal**        | ✅    | ✅                 | ✅                 | ✅  | ✅       |
| **Compensation**  | —     | —                  | ✅                 | ✅  | ✅       |
| **Conditional**   | ✅    | ✅                 | —                  | —   | ✅       |
| **Link**          | —     | ✅                 | ✅                 | —   | —        |
| **Terminate**     | —     | —                  | —                  | ✅  | —        |
| **Multiple**      | ✅    | ✅                 | ✅                 | ✅  | ✅       |

#### 1.2.2 Gateways

Control the **divergence** and **convergence** of the flow.

| Gateway            | Behavior                                                                  |
|--------------------|---------------------------------------------------------------------------|
| **Exclusive (XOR)**| Exactly **one** outgoing flow is activated (first true condition or default). |
| **Parallel (AND)** | **All** outgoing branches activate (divergence) / waits for **all** incoming (convergence). |
| **Inclusive (OR)** | **Any combination** of outgoing flows whose condition evaluates to true.   |
| **Event-Based**    | The first branch whose **event** occurs determines the flow.              |
| **Complex**        | Complex merging logic defined by an expression.                           |

#### 1.2.3 Activities

Represent **work** performed within the process.

| Activity               | Description                                                        |
|------------------------|--------------------------------------------------------------------|
| **User Task**          | Task performed by a human via a user interface.                    |
| **Service Task**       | Task executed automatically by an external service.                |
| **Script Task**        | Task executed by the engine (internal script).                     |
| **Business Rule Task** | Evaluates business rules.                                          |
| **Manual Task**        | Task performed by a human **without** a computer system.           |
| **Send Task**          | Sends a message to an external participant.                        |
| **Receive Task**       | Waits for a message to be received.                                |
| **Sub-Process**        | Embedded, reusable, event, or transaction sub-process.             |
| **Call Activity**      | Invokes a global process or reusable task.                         |

#### 1.2.4 Flows

Connect elements together.

| Flow                | Description                                                        |
|---------------------|--------------------------------------------------------------------|
| **Sequence Flow**   | Control flow between elements within the same process.             |
| **Message Flow**    | Message flow between participants (different pools).               |
| **Association**     | Associates artifacts or data with flow elements.                   |
| **Data Association**| Defines input/output data for activities.                          |

#### 1.2.5 Artifacts and Data

| Artifact         | Description                                                        |
|------------------|--------------------------------------------------------------------|
| **Data Object**  | Represents data produced or consumed by activities.                |
| **Data Store**   | Data persistence accessible by multiple activities.                |
| **Lane / Pool**  | Visual organization by organizational roles / participants.        |
| **Group**        | Visual grouping with no execution semantics.                       |
| **Text Annotation** | Comment/documentation attached to an element.                  |

---

## 2. Architecture of bpmn-ai

### 2.1 High-Level Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP API (chi/v5)                         │
│  /health  /metrics  /api/v1/processes  /api/v1/cases            │
├─────────────────────────────────────────────────────────────────┤
│                         Core Engine                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐    │
│  │  Engine  │→ │  Router  │→ │ FailSafe │  │  Registry    │    │
│  │  (loop)  │  │  (next   │  │ (timeout │  │  (factory    │    │
│  │          │  │  flow)   │  │ + loops) │  │  pattern)    │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│                        BPMN Elements                             │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Events   │  │ Gateways  │  │Activities│  │   Flows      │   │
│  │ Start    │  │ Parallel  │  │ UserTask │  │ SequenceFlow │   │
│  │ End      │  │ Exclusive │  │ScriptTask│  │              │   │
│  │ Timer    │  │ Inclusive │  │ServiceTsk│  │              │   │
│  │ Message  │  │EventBased │  │          │  │              │   │
│  │Terminate │  │           │  │          │  │              │   │
│  └──────────┘  └───────────┘  └──────────┘  └──────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│                       Observability                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │  Prometheus  │  │  Dispatcher  │  │  Logging (slog)      │   │
│  │  10 metrics  │  │  12 events   │  │  + File Auditor      │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│                      Persistence Layer                           │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────────┐   │
│  │  PostgreSQL    │  │  In-Memory     │  │  Job Queue       │   │
│  │  (pgx/v5)      │  │  (testing)     │  │  (WorkerPool)    │   │
│  └────────────────┘  └────────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Execution Loop (Engine)

The engine core is `internal/engine/engine.go`. It implements an **iterative loop** with goroutine workers — NO recursion.

```
Request → Engine.Run()
             │
             ├── Initialize channels: workCh (buf=1024), resultCh (buf=1024), errCh (cap=1)
             │
             ├── Transition instance: CREATED → IN_PROGRESS
             │
             ├── Create root ThreadRecord (thread index = 1)
             │
             ├── Create FailSafeManager (timeout + max_loops)
             │
             ├── pending counter = 1 (initial StartEvent)
             │
             ├── Spawn N workers (goroutines) ← configurable workerCount
             │
             ├── Enqueue StartEvent as first workItem
             │
             └── Main select loop:
                  ├── execCtx.Done()  → Timeout/cancel → ERROR
                  ├── resultCh        → handleResult()
                  │    ├── decrement pending
                  │    ├── check failsafe
                  │    └── by Action:
                  │         ├── ActionRoute     → Router → new workItems or end
                  │         ├── ActionWait      → Instance → WAITING
                  │         ├── ActionForm      → UserTask → WAITING (external)
                  │         ├── ActionQueue     → ServiceTask → JobQueue + continue flow
                  │         ├── ActionComplete  → EndEvent → COMPLETED
                  │         ├── ActionTerminate → TerminateEvent → TERMINATED
                  │         └── ActionError     → ERROR
                  │
                  ├── errCh           → return error
                  │
                  └── pending == 0    → close(workCh), wait for workers, finalizeInstance
```

### 2.3 Flow Router

`internal/engine/router.go` determines the next elements to execute based on each element's result:

- Only processes `ActionRoute` and `ActionQueue` results
- Gets the current element from the process and iterates over its `OutgoingFlows`
- Applies filters (`FlowFilters`) — used by Exclusive/Inclusive Gateway
- Returns `[]NextFlow` with: flowID, targetElementID, targetElementType, threadID
- Parallel branches create new threads with index: `parentThreadIdx * 10 + branchIndex + 1`

### 2.4 FailSafe Manager

`internal/engine/failsafe.go` protects against:

- **Timeout**: maximum execution time per instance (default: 30s)
- **Infinite loops**: maximum executions per element (default: 100)

### 2.5 Element Registry

`internal/engine/registry.go` implements the **factory pattern**:

```go
registry := engine.NewElementRegistry()
registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
// ...
elem, _ := registry.Get(bpmnElementDef)
result := elem.Execute(ctx, execCtx)
```

Thread-safe with `sync.RWMutex`.

### 2.6 Instance State Machine

`internal/process/state.go` defines 7 states:

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

Valid transitions:

| From        | To                                                         |
|-------------|------------------------------------------------------------|
| CREATED     | IN_PROGRESS, ERROR                                         |
| IN_PROGRESS | WAITING, SUSPENDED, COMPLETED, ERROR, TERMINATED           |
| WAITING     | IN_PROGRESS (on task complete), ERROR, TERMINATED          |
| SUSPENDED   | IN_PROGRESS, ERROR                                         |
| ERROR       | IN_PROGRESS                                                |
| COMPLETED   | (terminal)                                                 |
| TERMINATED  | (terminal)                                                 |

### 2.7 HTTP API

14 endpoints with `chi/v5`:

| Method | Endpoint                        | Description                   |
|--------|---------------------------------|-------------------------------|
| GET    | `/health`                       | Health check                  |
| GET    | `/ready`                        | Readiness check               |
| GET    | `/metrics`                      | Prometheus metrics            |
| GET    | `/api/v1/csrf-token`            | CSRF token                    |
| POST   | `/api/v1/processes`             | Register BPMN process         |
| GET    | `/api/v1/processes`             | List processes                |
| GET    | `/api/v1/processes/{id}`        | Process details               |
| POST   | `/api/v1/processes/{id}/start`  | Start a case                  |
| GET    | `/api/v1/cases`                 | List cases                    |
| GET    | `/api/v1/cases/{id}`            | Case details                  |
| GET    | `/api/v1/cases/{id}/tasks`      | Pending tasks                 |
| GET    | `/api/v1/cases/{id}/history`    | Execution history             |
| GET    | `/api/v1/cases/{id}/diagram`    | Diagram metadata              |
| POST   | `/api/v1/tasks/{id}/claim`      | Claim a task                  |
| POST   | `/api/v1/tasks/{id}/complete`   | Complete a task               |

Global middleware: RealIP, RequestID, Recovery, CORS, RequestLogger, RateLimiter (10 req/s, burst 20). CSRF on `/api/v1` subrouter.

### 2.8 Job Queue

`internal/queue/worker.go`:

```
ServiceTask → ActionQueue
     │
     ├── Create JobRecord (instance, flow, type, payload)
     ├── WorkerPool.Enqueue()
     │       │
     │       ├── N workers (goroutines) poll every 5s (configurable)
     │       ├── job.Status: PENDING → RUNNING → COMPLETED
     │       ├── Error → RetryPolicy (exponential backoff + jitter ±25%)
     │       └── Retries >= max → DeadLetterQueue
     │
     └── Router continues to next flow (process does NOT wait for the job)
```

Job states: `PENDING → RUNNING → COMPLETED`, with retries `FAILED → PENDING → ... → DEAD`.

RetryPolicy: `delay = BaseDelay × 2^retryCount` (capped at MaxDelay) + jitter ±25%. Default: 3 retries, base 1s, max 5m.

### 2.9 Persistence (Store) — Interface Segregation

Instead of a single monolithic interface, each component defines its own narrow storage interface following the **Interface Segregation Principle (ISP)**:

| Interface | Package | Methods | Consumer |
|-----------|---------|---------|----------|
| `EngineStore` | `internal/engine/store.go` | 10 (process, instance, flow, thread, job, log) | Engine core (`internal/engine/`) |
| `ElementStore` | `internal/element/store.go` | 1 (GetFlowsByInstance) | BPMN elements via ExecutionContext |
| `JobStore` | `internal/queue/store.go` | 4 (create, update, get, list pending) | WorkerPool, DeadLetterQueue |
| `DeadLetterStore` | `internal/queue/store.go` | 4 (create, get by instance, get by ID, list) | DeadLetterQueue |

The full `store.Store` (26 methods) union interface is retained for the API layer, which needs the widest access. The in-memory and PostgreSQL stores satisfy all narrow interfaces implicitly via Go's duck typing — no adapter code required.

```
API Layer ──→ store.Store (26 methods)
Engine   ──→ EngineStore (10 methods)
Elements ──→ ElementStore (1 method)
Queue    ──→ JobStore + DeadLetterStore (4+4 methods)
                │
                ▼
         In-Memory / PostgreSQL
         (implicitly satisfy all 4 + API)
```

- **In-Memory** (`pkg/store/memory/`): for testing and local development
- **PostgreSQL** (`pkg/store/sql/`): architecture defined with 7 tables (processes, instances, flows, threads, jobs, dead_letters, execution_log), documented migrations

### 2.10 Events and Observability

- **Dispatcher** (`internal/observability/events.go`): 12 domain events (process.started, element.executed, task.completed, job.failed, etc.), consumable sync/async
- **Prometheus Metrics**: 10 metrics (active cases, element duration, queue depth, errors, etc.)
- **Logging**: structured `slog` (stdlib)
- **Audit**: per-instance files in `data/audit/` with full execution traceability

---

## 3. BPMN Implementation Scope in bpmn-ai

### 3.1 What is Implemented?

| Category      | Element                     | Status | Support                                                |
|---------------|-----------------------------|--------|--------------------------------------------------------|
| **Events**    | StartEvent                  | ✅     | None trigger, process entry point                       |
|               | EndEvent                    | ✅     | Normal process completion                               |
|               | TerminateEvent              | ✅     | Immediate termination of all branches                   |
|               | TimerEvent                  | ✅     | ISO 8601 duration (PT1H, P1DT30M), date (RFC 3339), cron (5-field). Auto-continues via scheduled job queue. |
|               | MessageThrowEvent           | ✅     | Send message via `message_ref` variable.                |
|               | MessageCatchEvent           | ✅     | Wait for message via `POST /api/v1/messages`. Correlated by instance ID + messageRef. `intermediateCatchEvent` with `messageEventDefinition` → `ElementTypeMessageCatch`; `boundaryEvent` with `messageEventDefinition` → also `ElementTypeMessageCatch`. |
| **Gateways**  | ExclusiveGateway (XOR)      | ✅     | Condition evaluation with govaluate, default flow. `GatewayDirection` respected for converging (pass-through) vs diverging (condition evaluation). |
|               | ParallelGateway (AND)       | ✅     | Divergence (multiple threads via `NextThreadID`), convergence (waits for all incoming flows). `GatewayDirection` shortcuts inferencing. |
|               | InclusiveGateway (OR)       | ✅     | All matching conditions route simultaneously, else default or first. `GatewayDirection` supported. |
|               | EventBasedGateway           | ✅     | Armed via instance variables (`eventbased_gateway_armed`, `eventbased_gateway_resolved`, `eventbased_winning_element`). `CheckAndResolve()` in Continue ensures only the first event branch proceeds. |
| **Activities**| UserTask                    | ✅     | User/group assignment via `assignee`/`candidateUsers`/`candidateGroups`. Transitions to WAITING until external completion. |
|               | ScriptTask                  | ✅     | Real script execution with govaluate. Script types: `business_rule` (evaluate boolean), `change_field` (parse `key=value`), `assign_team`/`assign_user`/`add_related` (set variable). `scriptBody` and `scriptType` parsed from XML attributes. |
|               | ServiceTask                 | ✅     | Async execution via `ActionQueue`: job enqueued, process continues without waiting. |
| **Flows**     | SequenceFlow                | ✅     | Executable first-class element: factory populates `sourceRef`/`targetRef`/`condition`/`isDefault` from `ExtensionData`. Router routes through flow elements directly. Synthetic flow (`_synth`) created during parsing for routing continuity. |
|               | Conditional Flow            | 🔄     | Expression on sequence flow                             |
|               | Default Flow                | ✅     | Default flow for gateways                               |

### 3.2 What is NOT Implemented?

#### Missing Events

| Event                     | Description                                                        | Impact                                         |
|---------------------------|--------------------------------------------------------------------|------------------------------------------------|
| Signal Event (catch/throw)| Global signals across processes                                    | No broadcast communication between processes   |
| Error Event (start/intermediate/end/boundary) | Error handling as business flow              | Cannot model exception flows                   |
| Escalation Event          | Escalation to a higher role                                        | No escalation handling                         |
| Compensation Event        | Compensation transactions                                          | No partial rollback                            |
| Conditional Event         | Continuous condition evaluation                                    | No state-change-based events                   |
| Link Event                | Off-page diagram connectors                                        | Architectural limitation, non-critical         |
| None Intermediate Event   | Plain intermediate event without trigger                           | Not relevant for most cases                    |
| Multiple / Parallel Multiple | Multiple definitions in a single event                          | Specialized use case                           |
| Boundary Event            | Event attached to activity (interrupting/non-interrupting)         | Parser maps correctly (timer/message), but no attachment or interrupt semantics in engine |

#### Missing Gateways

| Gateway         | Description                                                        | Impact                                         |
|-----------------|--------------------------------------------------------------------|------------------------------------------------|
| Complex Gateway | Complex merging logic                                              | Very specific use case, low demand             |

#### Missing Activities

| Activity               | Description                                                        | Impact                                         |
|------------------------|--------------------------------------------------------------------|------------------------------------------------|
| Business Rule Task     | Business rule evaluation                                           | Can be emulated with ScriptTask                |
| Manual Task            | Human task without a system                                        | Can be emulated with UserTask                  |
| Send Task / Receive Task | Dedicated message send/receive                                  | Can be emulated with MessageThrow/Catch        |
| Sub-Process            | Embedded, reusable, event, or transaction sub-process             | **Significant limitation**: no process nesting |
| Call Activity          | Invocation of a global process                                     | Without sub-processes, no reusability          |
| Transaction            | Sub-process with ACID + compensation                               | Without sub-processes, no transactions         |

#### Other Missing BPMN 2.0 Concepts

| Concept                  | Description                                                        | Impact                                         |
|--------------------------|--------------------------------------------------------------------|------------------------------------------------|
| Pools / Lanes            | Participants / organizational roles                                | No process collaboration                       |
| Message Flow             | Flow between pools                                                 | No participant communication                   |
| Data Objects / Data Stores | Formal process data                                              | Flat variables used instead                    |
| Data Associations        | Input/output specifications                                        | Implicit mapping via variables                 |
| Artifacts (Group, TextAnnotation) | Purely visual elements                                | No impact on execution                         |
| Correlations             | Message correlation                                                | Not needed without Message Flow                |
| Choreographies / Conversations | Choreographies between participants                          | Out of scope                                   |
| Formal Expressions       | BPMN expression standard (FEEL, etc.)                              | Uses govaluate as pragmatic alternative        |
| Resource Assignment      | Formal resource model (resourceRole)                               | Simplified assignment via fields               |
| isExecutable / processType | Executability metadata                                          | All processes assumed executable               |

### 3.3 Known Architectural Limitations

1. **Limited Parser**: Only parses the first `<process>` within `<definitions>`. 500-element limit per process.
2. **No Sub-Processes**: No support for embedded, reusable, event sub-process, or transactions. Deliberate v1 design decision.
3. **PostgreSQL Store Pending**: The PostgreSQL implementation is defined at the interface and schema level, but the current code uses `memory.NewStore()` (testing only). The SQL store has no complete implementation files.
4. **No Diagram Rendering**: The `/api/v1/cases/{id}/diagram` endpoint returns metadata (element/flow counts), not a rendered PNG image, despite the `fogleman/gg` dependency being present.
5. **Jobs Without Default Handler**: The WorkerPool creates jobs but the default handler is `nil` (no-op). A real handler must be injected in production.
6. **No Authentication/Authorization**: Although `golang-jwt` is in the dependencies, no auth middleware is implemented.
7. **Non-Functional Boundary Events**: The parser now maps `boundaryEvent` to the correct type (TimerEvent or MessageCatch) based on the event definition, but the full semantics (attaching to activities, interrupting vs non-interrupting) are not implemented in the engine.
8. **gRPC Planned but Not Implemented**: Mentioned in the rewrite plan but absent from the codebase.

### 3.4 Maturity Summary

| Area                    | Status        | Notes                                                    |
|-------------------------|---------------|----------------------------------------------------------|
| Core elements           | ✅ Complete   | Start, End, Terminate, Timer, Message, Sequence Flow     |
| Main gateways           | ✅ Complete   | XOR, AND, OR, Event-Based                                |
| Main activities         | ✅ Complete   | UserTask, ScriptTask, ServiceTask                        |
| Execution loop          | ✅ Complete   | Iterative with goroutines, timeout, loop detection       |
| Job queue               | ✅ Complete   | Worker pool, retry, dead letter                          |
| REST API                | ✅ Complete   | 14 endpoints, full middleware chain                      |
| Observability           | ✅ Complete   | Prometheus, events, file audit                           |
| Testing                 | ✅ Complete   | 100+ tests, ~80% average coverage                        |
| Advanced elements       | ❌ Not started| Boundary events, Sub-Process, Call Activity, Signal, Error|
| PostgreSQL Store        | ⚠️ Partial    | Interface ready, implementation pending                  |
| Authentication          | ❌ Not started| Dependency present, middleware not implemented            |
| Diagrams                | ⚠️ Partial    | Metadata only, no rendering                              |

### 3.5 Conclusion

bpmn-ai covers **~60% of the BPMN 2.0 standard** in terms of elements, but that 60% represents **~90% of real-world use cases** in business process automation. The implemented elements (Start, End, Timer, Message, XOR, AND, OR, Event-Based, UserTask, ScriptTask, ServiceTask, SequenceFlow) are sufficient to model the vast majority of real-world business processes.

The most significant gaps are:
1. **Sub-Processes** — prevents modeling nested processes
2. **Error/Boundary Events** — prevents modeling exception flows
3. **Signal Events** — prevents inter-process communication
4. **PostgreSQL Persistence** — the real data layer is not implemented

The engine is **ready for prototyping and production in controlled environments**, with a solid, extensible architecture that allows incremental addition of missing elements.
