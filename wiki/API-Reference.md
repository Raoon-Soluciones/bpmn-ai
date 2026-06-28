# API Reference

Complete documentation of the REST API.

---

## Base URL

All endpoints: `http://{host}:{port}/api/v1/` (default `http://localhost:8080/api/v1/`).

---

## Health & Readiness

### GET /health

```json
{"status":"ok","timestamp":"2026-06-28T12:00:00Z"}
```

### GET /ready

```json
{"status":"ok","database":"connected","timestamp":"2026-06-28T12:00:00Z"}
```

### GET /metrics

Prometheus metrics endpoint.

---

## Process Management

### POST /api/v1/processes

Deploy a BPMN process definition.

**Request:** `application/json`
```json
{
  "name": "Order Processing",
  "bpmn_xml": "<?xml version=\"1.0\"?>..."
}
```

**Response (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Order Processing",
  "created_at": "2026-06-28T12:00:00Z"
}
```

| Status | Description |
|---|---|
| 400 | `name` or `bpmn_xml` missing, or invalid XML |
| 500 | Internal error |

### GET /api/v1/processes

```json
[
  {"id":"...","name":"Order Processing","created_at":"..."}
]
```

### GET /api/v1/processes/{id}

```json
{"id":"...","name":"Order Processing","created_at":"..."}
```

---

## Case Management

### POST /api/v1/processes/{id}/start

**Request:**
```json
{
  "variables": {
    "order_id": "ORD-12345",
    "amount": 250.00
  }
}
```

**Response (201):**
```json
{
  "id": "660e8400-...",
  "process_id": "550e8400-...",
  "status": "IN_PROGRESS",
  "created_at": "2026-06-28T12:00:00Z"
}
```

### GET /api/v1/cases

Query params: `?status=WAITING&limit=50&offset=0`

### GET /api/v1/cases/{id}

```json
{
  "id": "660e8400-...",
  "process_id": "550e8400-...",
  "name": "Order Processing",
  "status": "WAITING",
  "variables": {
    "classify_parsed": {"category": "invoice", "confidence": 0.95},
    "ai_total_cost": 0.0025
  },
  "threads": [{"index": 0, "status": "active"}],
  "created_at": "2026-06-28T12:00:00Z",
  "updated_at": "2026-06-28T12:00:05Z"
}
```

### GET /api/v1/cases/{id}/tasks

```json
[
  {
    "id": "770e8400-...",
    "case_id": "660e8400-...",
    "element_id": "review-1",
    "element_name": "Review Order",
    "assignee": "john",
    "candidate_users": ["alice", "bob"],
    "candidate_groups": ["managers"],
    "status": "WAITING",
    "created_at": "2026-06-28T12:00:03Z"
  }
]
```

### GET /api/v1/cases/{id}/history

```json
[
  {"element_id":"start-1","element_type":"startEvent","action":"ROUTE","duration_ms":5,"timestamp":"2026-06-28T12:00:00Z"},
  {"element_id":"classify","element_type":"aiTask","action":"ROUTE","duration_ms":1234,"timestamp":"2026-06-28T12:00:01Z"}
]
```

### GET /api/v1/cases/{id}/diagram

Returns the raw BPMN XML.

---

## Task Operations

### POST /api/v1/tasks/{id}/claim

```json
{"user_id": "john"}
```

**Response:**
```json
{"id":"...","status":"CLAIMED","assignee":"john"}
```

### POST /api/v1/tasks/{id}/complete

```json
{"variables": {"approved": true, "notes": "Looks good"}}
```

**Response:**
```json
{"id":"...","status":"COMPLETED","case_status":"IN_PROGRESS"}
```

---

## Message Correlation

### POST /api/v1/messages

```json
{
  "instance_id": "660e8400-...",
  "message_ref": "payment-received",
  "variables": {"amount": 250.00}
}
```

**Response:**
```json
{"message":"message delivered","instance_id":"660e8400-...","matched_flows":1}
```

Correlates by `instance_id` + `message_ref`. Finds active `MessageCatchEvent` flows.

---

## Signal Broadcasting

### POST /api/v1/signals

```json
{
  "signal_ref": "order-shipped",
  "variables": {"tracking": "1Z999..."}
}
```

**Response:**
```json
{"message":"signal broadcast","instances_resumed":["660e8400-..."],"matched_flows":2}
```

Broadcasts to all non-terminal instances. Empty `signal_ref` catches all signals.

---

## Common Workflow

```bash
# 1. Deploy
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","bpmn_xml":"<?xml...?>"}'

# 2. Start
curl -X POST http://localhost:8080/api/v1/processes/{id}/start

# 3. Check
curl http://localhost:8080/api/v1/cases/{id}

# 4. Complete task
curl -X POST http://localhost:8080/api/v1/tasks/{id}/complete \
  -H "Content-Type: application/json" \
  -d '{"variables":{"approved":true}}'

# 5. Send message
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"instance_id":"{id}","message_ref":"payment-received"}'

# 6. Broadcast signal
curl -X POST http://localhost:8080/api/v1/signals \
  -H "Content-Type: application/json" \
  -d '{"signal_ref":"order-shipped"}'

# 7. History
curl http://localhost:8080/api/v1/cases/{id}/history
```
