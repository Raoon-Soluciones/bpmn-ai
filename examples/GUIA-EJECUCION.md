# Guía de Ejecución: Proceso BPMN con Audit Log

## Estructura del proyecto

```
examples/
├── BPMN-diagram-op1.bpmn     # Proceso de préstamo con 11 elementos, 11 flujos
├── GUIA-EJECUCION.md          # Esta guía
├── run/
│   └── main.go                # Ejemplo programático (Go directo) — 2 escenarios
└── api/
    ├── main.go                # Servidor HTTP con engine + audit log
    └── test_api.sh             # Script de prueba con curl
```

## Proceso BPMN: Loan Processing (BPMN-diagram-op1.bpmn)

```
[Start: Application Received]
         │
         ▼
[UserTask: Record Application]
         │
         ▼
[ExclusiveGateway: Documents Complete?]
       ├── documentsComplete==true ──→ [ExclusiveGateway: Credit Score Approved?]
       │                                ├── creditScoreApproved==true ──→ [UserTask: Notify Approval] ──→ [End: Approved]
       │                                └── default (rechazado)        ──→ [End: Rejected]
       └── default (docs incompletos)   ──→ [End: Rejected]
```

El XML está en `examples/BPMN-diagram-op1.bpmn`. Usa condiciones `documentsComplete == true` y `creditScoreApproved == true`.

---

## Forma 1: Ejecución programática (Go directo)

Ejecuta el engine como biblioteca desde Go sin servidor HTTP. Corre **2 escenarios** (aprobado y rechazado).

```bash
# Desde la raíz del proyecto
go run ./examples/run/
```

**Qué hace:**
1. Crea logger en modo `debug` con formato `text`
2. Crea el audit writer en el directorio `audit_logs/`
3. Conecta el auditor al dispatcher (escucha los 12 tipos de evento)
4. Parsea el BPMN XML (`examples/BPMN-diagram-op1.bpmn`)
5. Crea el engine con todos los elementos registrados
6. **Escenario 1**: Crea instancia con `documentsComplete=true, creditScoreApproved=true`
7. **Escenario 2**: Crea instancia con `documentsComplete=true, creditScoreApproved=false`
8. Imprime el history de ejecución por escenario
9. Lee y muestra todos los archivos `audit_*.log` del directorio

**Salida esperada:**

Cada escenario imprime su history y luego se muestran los audit logs (un archivo `.log` por instancia):

```
  Escenario 1: Loan APROBADO
  Estado: WAITING (en 531.6µs)
  ELEMENTO                       TIPO            ACCION
  StartEvent_...                 startEvent      ROUTE
  Task_RecordApplication_...     userTask        FORM

  AUDIT LOGS POR INSTANCIA

📄 audit_<uuid>.log
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 BPMN Execution Audit
 Process:   Loan Processing Process (Process_LoanProcessing)
 Instance:  <uuid>
 Started:   2026-05-21 07:33:20.691
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  1.  Thread 1  │  StartEvent_... "Application received"  │  startEvent
      ROUTE
  2.  Thread 1  │  Task_RecordApplication_... "Record application"  │  userTask
      FORM
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 Result: COMPLETED
 Duration: 9ms
 Elements: 2
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Ambos escenarios terminan en `WAITING` porque `UserTask` requiere intervención humana. La diferencia de aprobado/rechazado se vería si el proceso avanzara más allá del primer gateway.

---

## Forma 2: Ejecución vía REST API

Un servidor HTTP completo con el engine correctamente integrado, incluyendo reanudación automática después de completar UserTasks.

### Iniciar el servidor

```bash
go run ./examples/api/
```

El servidor escucha en `http://localhost:8080`.

### Endpoints disponibles

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/processes` | Crear proceso (subir BPMN XML) |
| `GET` | `/api/v1/processes` | Listar procesos |
| `GET` | `/api/v1/processes/{id}` | Obtener proceso |
| `POST` | `/api/v1/processes/{id}/start` | Iniciar caso (crea instancia + ejecuta engine) |
| `GET` | `/api/v1/cases` | Listar casos |
| `GET` | `/api/v1/cases/{id}` | Obtener caso |
| `GET` | `/api/v1/cases/{id}/tasks` | Listar tareas activas (UserTasks) |
| `GET` | `/api/v1/cases/{id}/history` | History de ejecución |
| `POST` | `/api/v1/tasks/{id}/complete` | Completar tarea + reanudar engine |
| `GET` | `/api/v1/audit` | Obtener audit logs (todos los archivos `.log`) |

### Flujo completo con curl

```bash
# 1. Variables
BPMN_XML=$(python -c "import json; print(json.dumps(open('examples/BPMN-diagram-op1.bpmn').read()))")
BASE=http://localhost:8080/api/v1

# 2. Crear proceso
curl -s -X POST "$BASE/processes" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Loan Process\",\"bpmn_xml\":$BPMN_XML}"

# 3. Iniciar caso con variables
curl -s -X POST "$BASE/processes/Process_LoanProcessing/start" \
  -H "Content-Type: application/json" \
  -d '{"title":"Loan #42","variables":{"documentsComplete":true,"creditScoreApproved":true}}'

# → Respuesta: {"id":"<uuid>","status":"WAITING"}

# 4. Listar tareas pendientes (UserTask)
curl -s "$BASE/cases/<case_id>/tasks"

# → Respuesta: [{"element_id":"Task_RecordApplication_...","flow_id":"<uuid>","status":"ACTIVE",...}]

# 5. Completar la tarea
curl -s -X POST "$BASE/tasks/<flow_id>/complete" \
  -H "Content-Type: application/json" \
  -d '{"variables":{"documentsComplete":true}}'

# → Respuesta: {"task_id":"<uuid>","status":"COMPLETED","case_status":"WAITING"}

# 6. Completar segunda tarea (si avanzó a Notify Approval)
curl -s -X POST "$BASE/tasks/<flow_id>/complete" \
  -H "Content-Type: application/json" \
  -d '{}'

# 7. Ver el history de ejecución
curl -s "$BASE/cases/<case_id>/history" | python -m json.tool

# 8. Ver el audit log completo
curl -s "$BASE/audit" | python -m json.tool
```

### Usar el script automatizado

```bash
bash examples/api/test_api.sh
```

### Ejemplo de sesión completa (history)

```json
[
  {"ElementID": "StartEvent_ApplicationReceived_...", "ElementType": "startEvent",      "Action": "ROUTE"},
  {"ElementID": "Task_RecordApplication_...",          "ElementType": "userTask",        "Action": "FORM"},
  {"ElementID": "Gateway_DocumentsComplete_...",        "ElementType": "exclusiveGateway","Action": "ROUTE"},
  {"ElementID": "Task_NotifyApproval_...",              "ElementType": "userTask",        "Action": "FORM"},
  {"ElementID": "EndEvent_Approved_...",                "ElementType": "endEvent",        "Action": "COMPLETE"}
]
```

---

## Cómo habilitar el Audit Log

Por defecto, ambos ejemplos tienen el audit log **habilitado**. El audit log escribe **un archivo `.log` por instancia** dentro de un directorio.

### Configuración

En el ejemplo programático:
```go
auditWriter, _ := observability.NewFileAuditWriter("data/audit_logs", true, logger)
```

En el servidor API:
```go
auditWriter, _ := observability.NewFileAuditWriter("./data/audit_logs", true, logger)
```

| Variable | Default | Descripción |
|---|---|---|
| `AUDIT_LOG_ENABLED` | `true` | Activar/desactivar audit log |
| (directorio) | `data/audit_logs/` | Directorio donde se crean los archivos `audit_<uuid>.log` |

### Eventos del audit log

| Evento | Significado |
|---|---|
| `process.started` | Instancia creada y en ejecución |
| `element.executed` | Elemento BPMN ejecutado (start, task, gateway, end) |
| `element.error` | Error en ejecución de elemento |
| `process.completed` | Proceso terminado (COMPLETED o WAITING) |
| `process.terminated` | Terminate Event encontrado |
| `process.error` | Proceso en estado ERROR |
| `task.claimed` | Tarea reclamada por usuario |
| `task.completed` | Tarea completada manualmente |
| `job.queued` | ServiceTask encolado para ejecución asíncrona |
| `job.completed` | Job asíncrono completado |
| `job.failed` | Job asíncrono fallido |
| `job.dead` | Job agotó reintentos (dead letter) |

---

## Personalizar variables

Para probar la rama "Rejected" (mal credit score):

```bash
curl -s -X POST "$BASE/processes/Process_LoanProcessing/start" \
  -H "Content-Type: application/json" \
  -d '{"variables":{"documentsComplete":true,"creditScoreApproved":false}}'
```

El flujo será: `start → record-application → gw-docs → gw-credit → end-rejected`.

---

## Notas técnicas

- **UserTask**: El engine ejecuta hasta encontrar un UserTask, luego pausa (`WAITING`). La reanudación requiere completar la tarea vía API.
- **ServiceTask**: Se encola en el `WorkerPool` para ejecución asíncrona. El proceso continúa inmediatamente sin esperar.
- **ExclusiveGateway**: Evalúa condiciones usando la librería `govaluate`. Usar sintaxis como `documentsComplete == true` (no `${documentsComplete}`).
- **ParallelGateway**: Crea threads paralelos con índice `parentIdx * 10 + branchIndex + 1`.
- **El engine NO está conectado en `cmd/engine/main.go`** — los ejemplos aquí muestran cómo conectarlo correctamente.
- **Nested conditions**: El parser soporta `<conditionExpression>` como hijo directo de `<sequenceFlow>` o dentro de `<extensionElements>`, compatible con Camunda Modeler.

## Solución de problemas

| Problema | Causa | Solución |
|---|---|---|
| `package internal/...` not found | Ejecutando fuera del módulo | Ejecutar desde la raíz del repositorio |
| Audit log vacío | `AUDIT_LOG_ENABLED=false` o race async | Poner `true` o sync (`Dispatch` en vez de `DispatchAsync`) |
| El proceso no avanza tras UserTask | Falta mecanismo de reanudación | Usar `examples/api/` (tiene reanudación) |
| Condición de gateway no funciona | Sintaxis `${var}` no válida | Usar `var == true` en vez de `${var}` |
| Error "Incorrect function" al leer audit en Windows | Race al cerrar writer | Leer audit antes de `Close()` o capturar eventos en memoria |
| Puerto 8080 ocupado | Otro proceso usando el puerto | `export PORT=9090` antes de iniciar |
