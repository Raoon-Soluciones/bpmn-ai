# Architecture

---

## Overview

The BPMN-AI Engine is structured into three main layers.

### Layer 1: HTTP API (`api/http/`)

Built on chi router, provides RESTful endpoints and hosts a middleware stack.

### Layer 2: Engine Core (`internal/engine/`)

The engine uses a **non-recursive, iterative execution loop**. Workers process elements from a channel, the router determines next elements, and the failsafe manager enforces safety limits. An element registry (factory pattern) resolves element types.

### Layer 3: BPMN Elements (`internal/element/`)

19+ element types implementing the `BPMNElement` interface. The method `Execute(ctx *ExecutionContext, instance *process.ProcessInstance, element *bpmn.BaseElement) (*element.ExecutionResult, error)` returns one of several actions:

| Action | Meaning |
|---|---|
| `ActionRoute` | Proceed to next element(s) normally |
| `ActionWait` | Pause execution, instance → `WAITING` |
| `ActionForm` | UserTask — pause for external completion |
| `ActionQueue` | ServiceTask — async via job queue |
| `ActionThrowError` | ErrorEndEvent — propagate for error catch |
| `ActionTerminate` | TerminateEvent — stop all execution |

### Cross-Cutting: AI System (`pkg/ai/`)

The AI module provides LLM integration for the `AITask` element, with model routing, RAG, guardrails, caching, and tool calling.

---

## Execution Flow

```
HTTP Request → Engine.Iterate() → ExecuteElement()
    → Router.DetermineNext() → HandleResult() → [loop]
```

1. Engine receives a start request
2. Creates a ProcessInstance in `CREATED` state
3. Transition to `IN_PROGRESS`
4. Enters iterative loop:
   a. Execute current element
   b. Handle result (route, wait, queue, error, or terminate)
   c. Determine next element(s) via router
   d. Schedule boundary events if needed
   e. Repeat until no more elements or WAITING/ERROR
5. Instance → `COMPLETED` when all threads finish

---

## Sub-Process & Call Activity

**Sub-Process**: Inner XML parsed via `xml:",innerxml"`, elements are flattened into the parent process with ID prefix `{subID}.{origID}`. A synthetic entry flow `{id}_sp_entry` routes into the sub-process.

**Call Activity**: Parses `calledElement` attribute. Loads the called process from the store, flattens elements with `ca-{id}.` prefix, creates synthetic entry flow, executes internally, routes back on completion.

---

## Thread & Instance State

- **CREATED**: Initial state after instance creation
- **IN_PROGRESS**: Actively being executed by the engine
- **WAITING**: Paused on UserTask, MessageCatch, SignalCatch, or boundary event
- **SUSPENDED**: Explicitly suspended by API
- **ERROR**: Unrecoverable error
- **TERMINATED**: Stopped by TerminateEvent or API
- **COMPLETED**: All threads finished successfully

Thread index formula: `parentThreadIdx * 10 + branchIndex + 1`

---

## Boundary Events

Boundary events are attached to activities. When an activity enters `WAITING` or `FORM` state, `scheduleBoundaryTimers()` creates flow records for:
- **Timer boundary**: Creates a job scheduled by `CalculateSchedule()`
- **Message boundary**: Creates an active flow record found by `SendMessage()`
- **Error boundary**: Found by `findErrorCatch()` on error throw

Interrupting boundaries cancel attached flows. Non-interrupting (`cancelActivity="false"`) fire without cancellation.

---

## Job Queue (`internal/queue/`)

The job queue handles async execution:
- Worker pool with configurable concurrency
- Exponential backoff retry
- Dead letter queue for failed jobs

Used by ServiceTask, Timer catch events, and scheduled boundary timers.
