# BPMN Elements

The engine supports **19+ element types**, each implementing the `BPMNElement` interface.

---

## Events (`internal/element/events/`)

### Start Event
**File**: `internal/element/events/start.go`
- Entry point of a process
- Returns `ActionRoute` → moves to next element
- Output variables: none

### End Event
**File**: `internal/element/events/end.go`
- Normal process termination
- Supports `ExtensionData` for sub-process exit routing

### Terminate Event
**File**: `internal/element/events/terminate.go`
- Immediately stops all execution in the instance
- Returns `ActionTerminate`

### Timer Event (Catch)
**File**: `internal/element/events/timer.go`
- Supports ISO 8601 duration (`PT5M`), date (`2026-07-01T00:00:00Z`), and cron (`*/5 * * * *`)
- `CalculateSchedule()` computes next fire time
- Auto-continues via job queue after timer fires

### Message Throw Event
**File**: `internal/element/events/message_throw.go`
- Sends a message via the HTTP endpoint `POST /api/v1/messages`
- Correlates by `instance_id` + `message_ref`

### Message Catch Event
**File**: `internal/element/events/message_catch.go`
- Waits for a correlated message
- Creates active flow record when activity starts
- Resumed when `SendMessage()` matches `instance_id` + `message_ref`

### Error End Event
**File**: `internal/element/events/error_event.go`
- Returns `ActionThrowError` with error code
- Engine searches for matching `ErrorCatchEvent`

### Error Catch Event (Boundary)
- Found via `findErrorCatch()` by `AttachedToRef` + error code
- Empty error code catches all errors
- Can be boundary event or start event

### Signal Throw Event
**File**: `internal/element/events/signal_throw.go`
- Broadcasts a signal by `signal_ref` via `SendSignal()`
- Pass-through to next flow

### Signal Catch Event
**File**: `internal/element/events/signal_catch.go`
- Waits for broadcast signal
- Found by `SendSignal()` across all instances

---

## Gateways (`internal/element/gateways/`)

### Exclusive Gateway (XOR)
**File**: `internal/element/gateways/exclusive.go`
- Evaluates conditions using govaluate
- Falls back to default flow if no condition matches
- Supports `GatewayDirection` for converging

### Parallel Gateway (AND)
**File**: `internal/element/gateways/parallel.go`
- Creates new threads for each outgoing branch
- Thread index: `parentThreadIdx * 10 + branchIndex + 1`
- Convergence tracking on incoming

### Inclusive Gateway (OR)
**File**: `internal/element/gateways/inclusive.go`
- Evaluates conditions, takes all matching branches
- Convergence tracking for all incoming flows

### Event-Based Gateway
**File**: `internal/element/gateways/event_based.go`
- Arms all outgoing event elements
- First event to fire wins; others are cancelled
- Uses `armedFlows` / `resolvedFlows` tracking

---

## Activities (`internal/element/activities/`)

### UserTask
**File**: `internal/element/activities/user_task.go`
- Assignee, candidate users, candidate groups
- Instance → `WAITING` state until completed externally via `POST /api/v1/tasks/{id}/complete`
- Supports interrupting boundary events

### ScriptTask (govaluate)
**File**: `internal/element/activities/script_task.go`
- Executes expressions via govaluate
- Script types: `business_rule`, `change_field`, `assign_team`, `assign_user`, `add_related`

### ScriptTask (AI / AITask)
**File**: `internal/element/activities/ai_task.go` (registered as `scriptType="ai"`)
- LLM-powered element
- Uses `pkg/ai/` for model routing, tool calling, RAG, guardrails
- See [AI Integration](AI-Integration.md) for full details

### ServiceTask
**File**: `internal/element/activities/service_task.go`
- Async execution via `ActionQueue`
- Process continues without waiting for completion
- Job queue handles retry logic

### Sub-Process (Embedded)
**File**: `internal/element/activities/sub_process.go`
- Inner XML parsed via `xml:",innerxml"`
- Elements flattened with `{subID}.{origID}` prefix
- Synthetic entry flow `{id}_sp_entry`
- Exit routing via `subprocess_exit_flows` `ExtensionData` on internal end event

### CallActivity
**File**: `internal/element/activities/call_activity.go`
- Loads called process from store
- Flattens with `ca-{id}.` prefix
- Synthetic entry flow, executes, routes back on completion

---

## Flows (`internal/element/flows/`)

### SequenceFlow
**File**: `internal/element/flows/sequence_flow.go`
- Executable element
- Synthetic `_synth` flows for routing continuity
- Conditional flows use govaluate expressions
- Default flow as fallback

---

## Boundary Events

Boundary events schedule when their attached activity starts:
- **Timer**: `scheduleBoundaryTimers()` creates jobs with `CalculateSchedule()`
- **Message**: Active flow record created, found by `SendMessage()`
- **Error**: Found by `findErrorCatch()` on `ActionThrowError`
- Interrupting: Cancel attached flows via `cancelAttachedFlows()`
- Non-interrupting: Fire without cancellation
