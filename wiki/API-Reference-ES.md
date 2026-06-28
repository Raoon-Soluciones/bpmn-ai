# Referencia de API

Documentación completa de la API REST.

---

## URL Base

Todos los endpoints: `http://{host}:{port}/api/v1/` (por defecto `http://localhost:8080/api/v1/`).

---

## Salud y Disponibilidad

### GET /health

```json
{"status":"ok","timestamp":"2026-06-28T12:00:00Z"}
```

### GET /ready

```json
{"status":"ok","database":"connected","timestamp":"2026-06-28T12:00:00Z"}
```

### GET /metrics

Endpoint de métricas Prometheus.

---

## Gestión de Procesos

### POST /api/v1/processes

Desplegar una definición de proceso BPMN.

**Solicitud:** `application/json`
```json
{
  "name": "Procesamiento de Órdenes",
  "bpmn_xml": "<?xml version=\"1.0\"?>..."
}
```

**Respuesta (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Procesamiento de Órdenes",
  "created_at": "2026-06-28T12:00:00Z"
}
```

| Estado | Descripción |
|---|---|
| 400 | Falta `name` o `bpmn_xml`, o XML inválido |
| 500 | Error interno |

### GET /api/v1/processes

```json
[
  {"id":"...","name":"Procesamiento de Órdenes","created_at":"..."}
]
```

### GET /api/v1/processes/{id}

```json
{"id":"...","name":"Procesamiento de Órdenes","created_at":"..."}
```

---

## Gestión de Casos

### POST /api/v1/processes/{id}/start

**Solicitud:**
```json
{
  "variables": {
    "order_id": "ORD-12345",
    "amount": 250.00
  }
}
```

**Respuesta (201):**
```json
{
  "id": "660e8400-...",
  "process_id": "550e8400-...",
  "status": "IN_PROGRESS",
  "created_at": "2026-06-28T12:00:00Z"
}
```

### GET /api/v1/cases

Parámetros: `?status=WAITING&limit=50&offset=0`

### GET /api/v1/cases/{id}

```json
{
  "id": "660e8400-...",
  "process_id": "550e8400-...",
  "name": "Procesamiento de Órdenes",
  "status": "WAITING",
  "variables": {
    "clasificar_parsed": {"categoria": "factura", "confianza": 0.95},
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
    "element_name": "Revisar Orden",
    "assignee": "juan",
    "candidate_users": ["ana", "pedro"],
    "candidate_groups": ["gerentes"],
    "status": "WAITING",
    "created_at": "2026-06-28T12:00:03Z"
  }
]
```

### GET /api/v1/cases/{id}/history

```json
[
  {"element_id":"start-1","element_type":"startEvent","action":"ROUTE","duration_ms":5,"timestamp":"2026-06-28T12:00:00Z"},
  {"element_id":"clasificar","element_type":"aiTask","action":"ROUTE","duration_ms":1234,"timestamp":"2026-06-28T12:00:01Z"}
]
```

### GET /api/v1/cases/{id}/diagram

Devuelve el XML BPMN original.

---

## Operaciones de Tareas

### POST /api/v1/tasks/{id}/claim

```json
{"user_id": "juan"}
```

**Respuesta:**
```json
{"id":"...","status":"CLAIMED","assignee":"juan"}
```

### POST /api/v1/tasks/{id}/complete

```json
{"variables": {"aprobado": true, "notas": "Se ve bien"}}
```

**Respuesta:**
```json
{"id":"...","status":"COMPLETED","case_status":"IN_PROGRESS"}
```

---

## Correlación de Mensajes

### POST /api/v1/messages

```json
{
  "instance_id": "660e8400-...",
  "message_ref": "pago-recibido",
  "variables": {"amount": 250.00}
}
```

**Respuesta:**
```json
{"message":"mensaje entregado","instance_id":"660e8400-...","matched_flows":1}
```

Correlaciona por `instance_id` + `message_ref`. Busca flujos `MessageCatchEvent` activos.

---

## Transmisión de Señales

### POST /api/v1/signals

```json
{
  "signal_ref": "orden-enviada",
  "variables": {"tracking": "1Z999..."}
}
```

**Respuesta:**
```json
{"message":"señal transmitida","instances_resumed":["660e8400-..."],"matched_flows":2}
```

Transmite a todas las instancias no terminales. `signal_ref` vacío captura todas las señales.

---

## Flujo de Trabajo Común

```bash
# 1. Desplegar
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{"name":"Prueba","bpmn_xml":"<?xml...?>"}'

# 2. Iniciar
curl -X POST http://localhost:8080/api/v1/processes/{id}/start

# 3. Consultar
curl http://localhost:8080/api/v1/cases/{id}

# 4. Completar tarea
curl -X POST http://localhost:8080/api/v1/tasks/{id}/complete \
  -H "Content-Type: application/json" \
  -d '{"variables":{"aprobado":true}}'

# 5. Enviar mensaje
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"instance_id":"{id}","message_ref":"pago-recibido"}'

# 6. Transmitir señal
curl -X POST http://localhost:8080/api/v1/signals \
  -H "Content-Type: application/json" \
  -d '{"signal_ref":"orden-enviada"}'

# 7. Historial
curl http://localhost:8080/api/v1/cases/{id}/history
```
