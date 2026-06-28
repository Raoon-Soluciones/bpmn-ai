# Elementos BPMN

El motor soporta **19+ tipos de elementos**, cada uno implementando la interfaz `BPMNElement`.

---

## Eventos (`internal/element/events/`)

### Start Event
**Archivo**: `internal/element/events/start.go`
- Punto de entrada de un proceso
- Retorna `ActionRoute` → avanza al siguiente elemento

### End Event
**Archivo**: `internal/element/events/end.go`
- Terminación normal del proceso
- Soporta `ExtensionData` para ruteo de salida de sub-procesos

### Terminate Event
**Archivo**: `internal/element/events/terminate.go`
- Detiene inmediatamente toda ejecución en la instancia
- Retorna `ActionTerminate`

### Timer Event (Catch)
**Archivo**: `internal/element/events/timer.go`
- Soporta duración ISO 8601 (`PT5M`), fecha (`2026-07-01T00:00:00Z`) y cron (`*/5 * * * *`)
- `CalculateSchedule()` calcula el próximo momento de disparo
- Continúa automáticamente vía cola de trabajos

### Message Throw Event
**Archivo**: `internal/element/events/message_throw.go`
- Envía un mensaje vía `POST /api/v1/messages`
- Correlaciona por `instance_id` + `message_ref`

### Message Catch Event
**Archivo**: `internal/element/events/message_catch.go`
- Espera un mensaje correlacionado
- Crea registro de flujo activo cuando la actividad comienza
- Reanudado cuando `SendMessage()` coincide con `instance_id` + `message_ref`

### Error End Event
**Archivo**: `internal/element/events/error_event.go`
- Retorna `ActionThrowError` con código de error
- El motor busca un `ErrorCatchEvent` coincidente

### Error Catch Event (Borde)
- Encontrado via `findErrorCatch()` por `AttachedToRef` + código de error
- Código de error vacío captura todos los errores

### Signal Throw Event
**Archivo**: `internal/element/events/signal_throw.go`
- Transmite una señal por `signal_ref` via `SendSignal()`
- Pasante al siguiente flujo

### Signal Catch Event
**Archivo**: `internal/element/events/signal_catch.go`
- Espera una señal transmitida
- Encontrado por `SendSignal()` en todas las instancias

---

## Compuertas (`internal/element/gateways/`)

### Exclusive Gateway (XOR)
**Archivo**: `internal/element/gateways/exclusive.go`
- Evalúa condiciones usando govaluate
- Usa flujo por defecto si ninguna condición coincide
- Soporta `GatewayDirection` para convergencia

### Parallel Gateway (AND)
**Archivo**: `internal/element/gateways/parallel.go`
- Crea nuevos hilos para cada rama saliente
- Índice de hilo: `parentThreadIdx * 10 + branchIndex + 1`
- Seguimiento de convergencia en entrada

### Inclusive Gateway (OR)
**Archivo**: `internal/element/gateways/inclusive.go`
- Evalúa condiciones, toma todas las ramas coincidentes
- Seguimiento de convergencia para todos los flujos entrantes

### Event-Based Gateway
**Archivo**: `internal/element/gateways/event_based.go`
- Arma todos los elementos de evento salientes
- El primer evento en dispararse gana; los demás se cancelan
- Usa seguimiento de `armedFlows` / `resolvedFlows`

---

## Actividades (`internal/element/activities/`)

### UserTask
**Archivo**: `internal/element/activities/user_task.go`
- Asignado, usuarios candidatos, grupos candidatos
- Instancia → estado `WAITING` hasta completarse externamente
- Soporta eventos de borde interruptores

### ScriptTask (govaluate)
**Archivo**: `internal/element/activities/script_task.go`
- Ejecuta expresiones via govaluate
- Tipos: `business_rule`, `change_field`, `assign_team`, `assign_user`, `add_related`

### ScriptTask (IA / AITask)
**Archivo**: `internal/element/activities/ai_task.go` (registrado como `scriptType="ai"`)
- Elemento potenciado por LLM
- Usa `pkg/ai/` para ruteo de modelos, herramientas, RAG, guardrails
- Ver [Integración de IA](AI-Integration-ES.md)

### ServiceTask
**Archivo**: `internal/element/activities/service_task.go`
- Ejecución asíncrona via `ActionQueue`
- El proceso continúa sin esperar finalización

### Sub-Process (Embedded)
**Archivo**: `internal/element/activities/sub_process.go`
- XML interno parseado via `xml:",innerxml"`
- Elementos aplanados con prefijo `{subID}.{origID}`
- Flujo sintético de entrada `{id}_sp_entry`
- Ruteo de salida via `ExtensionData`

### CallActivity
**Archivo**: `internal/element/activities/call_activity.go`
- Carga proceso llamado desde el store
- Aplana con prefijo `ca-{id}.`
- Flujo sintético de entrada, ejecuta, enruta de vuelta

---

## Flujos (`internal/element/flows/`)

### SequenceFlow
**Archivo**: `internal/element/flows/sequence_flow.go`
- Elemento ejecutable
- Flujos sintéticos `_synth` para continuidad de ruteo
- Flujos condicionales usan expresiones govaluate

---

## Eventos de Borde

Los eventos de borde se programan cuando su actividad adjunta comienza:
- **Timer**: `scheduleBoundaryTimers()` crea trabajos con `CalculateSchedule()`
- **Message**: Registro de flujo activo creado, encontrado por `SendMessage()`
- **Error**: Encontrado por `findErrorCatch()` en `ActionThrowError`
- Interruptores: Cancelan flujos adjuntos via `cancelAttachedFlows()`
- No interruptores: Se disparan sin cancelación
