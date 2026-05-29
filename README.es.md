# Motor BPMN

🌐 [English](README.md) | **Español**

> Motor de ejecución BPMN 2.0 independiente escrito en Go. Alto rendimiento, listo para producción.

📖 **[Leer la documentación completa →](docs/bpmn-y-bpmn-ai.md)** — Conceptos BPMN, arquitectura del motor y alcance de implementación.

## Inicio Rápido

### Ejecutar en 30 segundos

```bash
# Clonar y ejecutar
git clone https://github.com/Raoon-Soluciones/bpmn-ai.git && cd bpmn-ai
go run ./cmd/engine
```

### Verificar que está funcionando

```bash
curl http://localhost:8080/health
# {"status":"ok","timestamp":"...","version":"0.1.0"}
```

### Probar la API

```bash
# Registrar un proceso
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test",
    "bpmn_xml": "<?xml version=\"1.0\"?><definitions xmlns=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" targetNamespace=\"test\"><process id=\"p1\" name=\"Test\"><startEvent id=\"s1\"/><endEvent id=\"e1\"/><sequenceFlow id=\"f1\" sourceRef=\"s1\" targetRef=\"e1\"/></process></definitions>"
  }'

# Listar procesos
curl http://localhost:8080/api/v1/processes

# Listar casos
curl http://localhost:8080/api/v1/cases
```

---

## Qué Está Implementado

Las 6 fases del plan de reescritura están **completas** y **listas para producción**.

### Fase 1: Fundamentos ✅
- Tipos de dominio, analizador XML BPMN
- Interfaz de almacenamiento + implementación en memoria
- Máquina de estados con 8 estados y transiciones válidas
- Sistema de configuración + logging estructurado (slog)

### Fase 2: Bucle de Ejecución ✅
- Motor iterativo con workers goroutine (sin recursión)
- Enrutador de flujos con seguimiento de hilos para ramas paralelas
- Manager de seguridad (timeout + detección de bucles)
- Registro de elementos con patrón factory
- StartEvent, EndEvent, ExclusiveGateway, ParallelGateway

### Fase 3: Elementos ✅
- **Actividades**: UserTask, ScriptTask, ServiceTask
- **Eventos**: TimerEvent, MessageThrowEvent, MessageCatchEvent, TerminateEvent
- **Compuertas**: InclusiveGateway, EventBasedGateway

### Fase 4: Cola y Asíncrono ✅
- Cola de trabajos con capa de persistencia
- Pool de workers con concurrencia configurable
- Política de reintentos con backoff exponencial + jitter
- Cola de mensajes muertos para trabajos fallidos
- Soporte de trabajos programados

### Fase 5: API y Observabilidad ✅
- API REST con router chi (14 endpoints)
- Cadena de middleware: logging, recovery, request ID, CORS, CSRF, rate limiter
- Métricas Prometheus (10 métricas)
- Dispatcher de eventos (12 tipos de eventos, sync/async)
- Endpoints de health check + readiness

### Fase 6: Listo para Producción ✅
- Docker multi-stage build (~15MB imagen final)
- docker-compose con PostgreSQL 16
- GitHub Actions CI (test, build, docker)
- CSRF protection (double-submit cookie pattern)
- Rate limiter por IP (token bucket)
- Parser XML seguro contra XXE
- Evaluación de expresiones de condición BPMN (govaluate)
- ParallelGateway convergente (espera todas las ramas entrantes)
- SequenceFlow como elemento ejecutable — factory pobla desde ExtensionData, router enruta a través de flow elements
- ScriptTask con ejecución real — tipos business_rule, change_field, assign_team, assign_user, add_related
- TimerEvent con parseo ISO 8601 y cron — ContinueAt programa auto-continuación vía cola de trabajos
- MessageCatch con correlación de mensajes — endpoint `POST /api/v1/messages`, búsqueda por instanceID + MessageRef
- EventBasedGateway con tracking armed/resolved — el primer evento gana, las ramas subsiguientes se descartan
- GatewayDirection — parseado desde XML y usado en lógica de divergencia/convergencia de todas las compuertas
- Fix en parser — intermediateCatchEvent/boundaryEvent ahora mapean al tipo correcto según la definición del evento
- 100+ tests, todos pasando con detector `-race`

---

## Opciones de Ejecución

### Opción 1: Directo con Go (desarrollo)

```bash
go run ./cmd/engine
```

### Opción 2: Compilar y ejecutar (producción)

```bash
go build -o bpmn-ai ./cmd/engine
./bpmn-ai
```

### Opción 3: Docker

```bash
docker build -t bpmn-ai:latest .
docker run -p 8080:8080 bpmn-ai:latest
```

### Opción 4: Docker Compose (con PostgreSQL)

```bash
docker-compose up -d
```

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP API (chi)                        │
│  /health  /metrics  /api/v1/processes  /api/v1/cases        │
├─────────────────────────────────────────────────────────────┤
│                        Motor Central                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Motor   │→ │Enrutador │→ │Manager   │  │  Registro  │  │
│  │  (loop)  │  │          │  │Seguridad │  │            │  │
│  └──────────┘  └──────────┘  └──────────┘  └────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Elementos BPMN                           │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐    │
│  │ Eventos │ │Compuertas│ │Actividades│ │    Flujos    │    │
│  │ Inicio  │ │ Paralelo │ │ UserTask │ │  Secuencia   │    │
│  │ Fin     │ │Exclusivo │ │ScriptTask│ │              │    │
│  │ Timer   │ │Inclusivo │ │SrvceTask │ │              │    │
│  │ Mensaje │ │EventBased│ │          │ │              │    │
│  │Term     │ │          │ │          │ │              │    │
│  └─────────┘ └──────────┘ └──────────┘ └──────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                     Observabilidad                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Prometheus  │  │  Disp. Event │  │  Log Estructurado│  │
│  │  Métricas    │  │  (sync/async)│  │  (slog)          │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Capa de Persistencia                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  PostgreSQL  │  │  En Memoria  │  │   Cola Trabajos  │  │
│  │  (pgx/v5)    │  │  (tests)     │  │   (async)        │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Segregación de Interfaces

Cada componente define su propia interfaz de almacenamiento siguiendo el Principio de Segregación de Interfaces (ISP), en lugar de compartir el monolítico `store.Store` de 26 métodos:

| Interfaz | Paquete | Métodos | Usado Por |
|----------|---------|---------|-----------|
| `EngineStore` | `internal/engine/store.go` | 10 (process, instance, flow, thread, job, log) | Motor central |
| `ElementStore` | `internal/element/store.go` | 1 (GetFlowsByInstance) | Elementos BPMN vía ExecutionContext |
| `JobStore` | `internal/queue/store.go` | 4 (CRUD jobs) | WorkerPool, DeadLetterQueue |
| `DeadLetterStore` | `internal/queue/store.go` | 4 (CRUD dead letters) | DeadLetterQueue |

Los stores en memoria y PostgreSQL satisfacen todas las interfaces implícitamente mediante duck typing de Go — sin necesidad de código adaptador. La interfaz completa `store.Store` se conserva para la capa API que necesita el acceso más amplio.

```
┌──────────────────┐    ┌──────────────┐    ┌──────────────────┐
│   Capa API       │    │    Motor     │    │ Elementos BPMN   │
│  (store.Store)   │    │(EngineStore) │    │ (ElementStore)   │
├──────────────────┤    ├──────────────┤    ├──────────────────┤
│                  │    │              │    │                  │
│ SaveProcess      │    │ GetProcess   │    │ GetFlowsByInst   │
│ ListProcesses    │    │ GetInstance  │    │                  │
│ CreateInstance   │    │ UpdateInst   │    │                  │
│ GetFlow          │    │ CreateFlow   │    │                  │
│ UpdateFlow       │    │ GetFlow      │    │                  │
│ ... (26 total)   │    │ ...          │    │                  │
└──────────────────┘    └──────────────┘    └──────────────────┘
         │                      │                     │
         └──────────────────────┼─────────────────────┘
                                │
                     ┌──────────▼──────────┐
                     │   Store (memoria/sql)│
                     │  (satisface implíci- │
                     │  tamente 4 + API)    │
                     └─────────────────────┘
```

El paquete queue se divide además en `JobStore` (4 métodos) y `DeadLetterStore` (4 métodos) — `DeadLetterQueue` requiere ambos porque añadir una carta muerta también actualiza el estado del trabajo.

### Bucle de Ejecución

```
Request → Engine.Run()
             │
             ├── workCh (canal bufferizado, 1024)
             │     └── N workers goroutine procesan elementos
             │
             ├── resultCh (canal bufferizado, 1024)
             │     └── Bucle principal maneja resultados
             │
             ├── Verificación FailSafe (timeout + conteo de bucles)
             │
             ├── FlowRouter determina siguientes elementos
             │     └── Ramas paralelas → nuevos hilos
             │
             └── seguimiento pending → cierra cuando termina
```

### Cola de Trabajos y Procesamiento Asíncrono

```
ServiceTask → ActionQueue
     │
     ├── Crear JobRecord (instancia, flujo, tipo, payload)
     ├── Encolar → WorkerPool
     │       │
     │       ├── Workers pollean cada 5s (configurable)
     │       ├── Concurrencia: N workers en paralelo
     │       ├── job.Status = RUNNING
     │       ├── Ejecutar handler
     │       │   ├── OK → COMPLETED
     │       │   └── Error → RetryPolicy
     │       │       ├── reintentos < max → PENDING + backoff exponencial
     │       │       └── reintentos >= max → Cola de Mensajes Muertos
     │       └── Trabajos programados: solo se procesan cuando scheduled_at <= now
     │
     └── Enrutar → siguientes flujos (el proceso continúa sin esperar)
```

#### Política de Reintentos

```
delay = BaseDelay × 2^retryCount  (limitado por MaxDelay)
+ jitter (±25%) para prevenir thundering herd
```

| Parámetro | Defecto | Descripción |
|-----------|---------|-------------|
| `MaxRetries` | 3 | Máximo intentos de reintento |
| `BaseDelay` | 1s | Delay inicial de backoff |
| `MaxDelay` | 5m | Límite máximo de backoff |
| `Jitter` | true | Aleatorizar delays |

#### Estados de Trabajos

```
PENDING ──→ RUNNING ──→ COMPLETED
    │           │
    │           └──→ FAILED ──→ PENDING (reintento)
    │                              │
    │                              └──→ FAILED ──→ ... (hasta max reintentos)
    │                                                  │
    │                                                  └──→ DEAD (cola de muertos)
    └──→ DEAD (directo)
```

---

## Elementos BPMN Soportados

### Eventos
| Elemento | Estado | Descripción |
|---------|--------|-------------|
| Start Event | ✅ | Punto de entrada del proceso |
| End Event | ✅ | Completación del proceso |
| Terminate Event | ✅ | Terminación inmediata del proceso |
| Timer Event | ✅ | Disparadores basados en tiempo (duración, fecha, ciclo) |
| Message Throw Event | ✅ | Enviar mensaje |
| Message Catch Event | ✅ | Esperar mensaje |

### Compuertas
| Elemento | Estado | Descripción |
|---------|--------|-------------|
| Exclusive Gateway | ✅ | XOR — enrutar a primera condición coincidente |
| Parallel Gateway | ✅ | AND — enrutar a todas las ramas / esperar todas |
| Inclusive Gateway | ✅ | OR — enrutar a todas las condiciones coincidentes |
| Event-Based Gateway | ✅ | Enrutar basado en qué evento ocurre primero |

### Actividades
| Elemento | Estado | Descripción |
|---------|--------|-------------|
| User Task | ✅ | Tarea humana con asignación |
| Script Task | ✅ | Ejecución automática de script |
| Service Task | ✅ | Llamada a servicio externo (cola async) |

### Flujos
| Elemento | Estado | Descripción |
|---------|--------|-------------|
| Sequence Flow | ✅ | Flujo por defecto entre elementos |
| Conditional Flow | 🔄 | Flujo con expresión de condición |
| Default Flow | ✅ | Flujo de respaldo para compuertas |

---

## Referencia de API

### Health y Métricas

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/health` | Verificación de salud |
| GET | `/ready` | Verificación de readiness |
| GET | `/metrics` | Métricas Prometheus |

### Procesos

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/v1/processes` | Registrar proceso |
| GET | `/api/v1/processes` | Listar procesos |
| GET | `/api/v1/processes/{id}` | Obtener detalles del proceso |
| POST | `/api/v1/processes/{id}/start` | Iniciar caso |

### Casos

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/v1/csrf-token` | Obtener token CSRF (setea cookie, retorna token) |
| GET | `/api/v1/cases` | Listar casos |
| GET | `/api/v1/cases/{id}` | Detalles del caso |
| POST | `/api/v1/messages` | Enviar mensaje a instancia MessageCatch esperando |
| GET | `/api/v1/cases/{id}/tasks` | Tareas pendientes |
| POST | `/api/v1/tasks/{id}/claim` | Reclamar tarea |
| POST | `/api/v1/tasks/{id}/complete` | Completar tarea |
| GET | `/api/v1/cases/{id}/history` | Historial de ejecución |
| GET | `/api/v1/cases/{id}/diagram` | Diagrama del proceso |

### Middleware

Todas las requests incluyen:
- **Request ID** — Header `X-Request-ID` auto-generado
- **Logging estructurado** — método, path, status, duración, request ID
- **Recuperación de panic** — 500 en panic con logging de error
- **CORS** — Orígenes permitidos configurables
- **Rate Limiter** — token bucket por IP (tasa/burst configurable)
- **CSRF Protection** — patrón double-submit cookie en métodos state-changing (POST/PUT/DELETE/PATCH); métodos seguros (GET/HEAD/OPTIONS/TRACE) pasan sin check; deshabilitado vía `DisableCSRF: true` en tests

---

## Configuración

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

audit:
  enabled: true
  dir: ./data/audit
```

### Variables de Entorno

| Variable | Valor por Defecto | Descripción |
|----------|-------------------|-------------|
| `DATABASE_URL` | `postgres://...` | Cadena de conexión PostgreSQL |
| `AUDIT_LOG_ENABLED` | `true` | Activar/desactivar el registro de auditoría |
| `AUDIT_LOG_DIR` | `./data/audit` | Directorio para archivos de auditoría por instancia |

---

## Máquina de Estados

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

### Transiciones Válidas

| Desde | Hacia |
|-------|-------|
| CREATED | IN_PROGRESS, ERROR |
| IN_PROGRESS | WAITING, SUSPENDED, COMPLETED, ERROR, TERMINATED |
| WAITING | IN_PROGRESS, ERROR, TERMINATED |
| SUSPENDED | IN_PROGRESS, ERROR |
| ERROR | IN_PROGRESS |
| COMPLETED | (terminal) |
| TERMINATED | (terminal) |

---

## Observabilidad

### Métricas Prometheus

| Métrica | Tipo | Descripción |
|--------|------|-------------|
| `bpmn_processes_active` | Gauge | Instancias de proceso activas |
| `bpmn_cases_total` | Counter | Total de casos iniciados (por proceso) |
| `bpmn_cases_by_status` | Gauge | Casos por estado |
| `bpmn_element_duration_ms` | Histogram | Duración de ejecución de elemento |
| `bpmn_element_errors_total` | Counter | Errores de ejecución de elemento |
| `bpmn_queue_depth` | Gauge | Trabajos pendientes en cola |
| `bpmn_queue_retries_total` | Counter | Reintentos de trabajos (por tipo) |
| `bpmn_queue_dead_letters_total` | Counter | Trabajos en cola de muertos |
| `bpmn_http_request_duration_ms` | Histogram | Duración de request HTTP |
| `bpmn_http_request_errors_total` | Counter | Errores de request HTTP |

### Dispatcher de Eventos

El motor emite eventos de dominio para observabilidad:

| Evento | Descripción |
|-------|-------------|
| `process.started` | Instancia de proceso iniciada |
| `process.completed` | Instancia de proceso completada |
| `process.terminated` | Instancia de proceso terminada |
| `process.error` | Instancia de proceso con error |
| `element.executed` | Elemento ejecutado |
| `element.error` | Falló ejecución de elemento |
| `task.claimed` | Tarea de usuario reclamada |
| `task.completed` | Tarea de usuario completada |
| `job.queued` | Trabajo encolado |
| `job.completed` | Trabajo completado |
| `job.failed` | Trabajo fallido |
| `job.dead` | Trabajo movido a cola de muertos |

Los eventos pueden ser consumidos sincrónica o asincrónicamente vía el `Dispatcher`.

### Registro de Auditoría

El motor escribe un registro de auditoría legible en un archivo por instancia de proceso dentro del directorio configurado. Cada archivo traza cada elemento BPMN ejecutado, mostrando la ruta de ejecución, las ramas de hilos, las duraciones y los resultados.

**Ejemplo de salida (`audit_<id_instancia>.log`):**

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 BPMN Execution Audit
 Process:   Audit Parallel (proc-audit-par)
 Instance:  550e8400-e29b-41d4-a716-446655440000
 Started:   2026-05-20 10:30:00.123
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  1.  Thread 1  │  start-1 "Start"  │  startEvent
      ROUTE  ·  2ms

  2.  Thread 1  │  gw-div "Split"  │  parallelGateway
      ROUTE  ·  1ms
      Branches: → end-a, → end-b

  3.  Thread 11  │  end-a "End A"  │  endEvent
      COMPLETE  ·  1ms

  4.  Thread 12  │  end-b "End B"  │  endEvent
      COMPLETE  ·  1ms

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 Result: COMPLETED
 Duration: 45ms
 Elements: 4
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Configuración vía archivo `.env`:**

```bash
# .env
AUDIT_LOG_ENABLED=true
AUDIT_LOG_DIR=./data/audit
```

**Inspeccionar los registros de auditoría:**

```bash
# Listar todos los archivos
ls ./data/audit/

# Tail de una instancia específica
tail -f ./data/audit/audit_<id_instancia>.log

# Buscar errores en todos los archivos
grep "ERROR" ./data/audit/*.log
```

El registro de auditoría es aditivo — el `ExecutionLogEntry` existente se conserva. Cuando la auditoría está desactivada (`AUDIT_LOG_ENABLED=false`), los eventos aún se distribuyen pero no se escriben entradas en los archivos.

---

## Estructura del Proyecto

```
bpmn-ai/
├── cmd/engine/                    # Punto de entrada CLI
│   └── main.go                    # Conectar dependencias, graceful shutdown
├── internal/                      # Código privado de aplicación
│   ├── engine/                    # Motor de ejecución central
│   │   ├── engine.go              # Bucle de ejecución iterativo
│   │   ├── context.go             # Contexto de ejecución inmutable
│   │   ├── result.go              # Tipos de resultado de ejecución
│   │   ├── router.go              # Lógica de enrutamiento de flujos
│   │   ├── failsafe.go            # Detección de timeout y bucles
│   │   ├── registry.go            # Factory de elementos
│   │   └── store.go               # Interfaz EngineStore (10 métodos)
│   ├── element/                   # Implementaciones de elementos BPMN
│   │   ├── store.go               # Interfaz ElementStore (1 método)
│   │   ├── element.go             # Interfaces base
│   │   ├── activity.go            # Interfaz de actividad
│   │   ├── gateway.go             # Interfaz de compuerta
│   │   ├── event.go               # Interfaz de evento
│   │   ├── flow.go                # Interfaz de flujo
│   │   ├── events/                # Start, End, Terminate, Timer, Message
│   │   ├── gateways/              # Parallel, Exclusive, Inclusive, EventBased
│   │   ├── activities/            # UserTask, ScriptTask, ServiceTask
│   │   └── flows/                 # SequenceFlow
│   ├── process/                   # Gestión de procesos e hilos
│   │   └── state.go               # Máquina de estados + Instance
│   ├── queue/                     # Sistema de cola de trabajos
│   │   ├── retry.go               # Política de reintentos con backoff exponencial
│   │   ├── deadletter.go          # Cola de mensajes muertos
│   │   ├── worker.go              # Pool de workers con control de concurrencia
│   │   └── store.go               # Interfaces JobStore + DeadLetterStore
│   └── observability/             # Logging, métricas, eventos
│       ├── logger.go              # Logging estructurado (slog)
│       ├── metrics.go             # Métricas Prometheus (10 métricas)
│       └── events.go              # Dispatcher de eventos (sync/async)
├── pkg/                           # Paquetes públicos
│   ├── bpmn/                      # Modelo y parser BPMN
│   │   ├── model.go               # Tipos de dominio
│   │   └── parser.go              # Parser XML BPMN
│   └── store/                     # Interfaces de persistencia
│       ├── store.go               # Interfaz Store
│       ├── sql/                   # Implementación PostgreSQL
│       ├── migrations/            # Migraciones de base de datos
│       └── memory/                # En memoria (testing)
├── api/                           # Capa API
│   ├── http/                      # API REST (chi)
│   │   ├── server.go              # Configuración HTTP server
│   │   ├── routes.go              # Definición de rutas
│   │   ├── handlers.go            # Handlers API
│   │   └── health.go              # Health check
│   └── middleware/                # Middleware HTTP
│       ├── logging.go             # Logging de requests
│       ├── recovery.go            # Recuperación de panic
│       └── requestid.go           # Request ID
├── config/                        # Configuración
├── testdata/                      # Archivos de prueba BPMN
│   ├── simple_sequence.bpmn
│   ├── parallel_gateway.bpmn
│   ├── exclusive_gateway.bpmn
│   ├── inclusive_gateway.bpmn
│   ├── event_based_gateway.bpmn
│   ├── timer_event.bpmn
│   ├── message_catch.bpmn
│   ├── script_task.bpmn
│   ├── service_task.bpmn
│   └── complex_process.bpmn
├── Dockerfile                     # Build multi-stage (~15MB)
├── docker-compose.yml             # Motor + PostgreSQL
├── .github/workflows/ci.yml       # GitHub Actions CI
├── .dockerignore
├── .gitignore
├── go.mod
├── Makefile
├── .golangci.yml
└── README.md
```

---

## Esquema de Base de Datos

```sql
-- Definiciones de procesos
CREATE TABLE processes (
    id          UUID PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    version     INT NOT NULL DEFAULT 1,
    bpmn_xml    TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Instancias de proceso (casos)
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

-- Registros de ejecución de flujos
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

-- Seguimiento de hilos
CREATE TABLE threads (
    id              SERIAL PRIMARY KEY,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    thread_index    INT NOT NULL,
    parent_index    INT,
    flow_id         UUID NOT NULL REFERENCES flows(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cola de trabajos
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

-- Cola de mensajes muertos
CREATE TABLE dead_letters (
    id              UUID PRIMARY KEY,
    job_id          UUID NOT NULL,
    instance_id     UUID NOT NULL REFERENCES instances(id),
    payload         JSONB NOT NULL,
    error_message   TEXT NOT NULL,
    retry_count     INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Log de ejecución
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

## Desarrollo

### Prerrequisitos

- Go 1.23+
- PostgreSQL 16+ (para producción)
- Docker + Docker Compose (para pruebas)

### Targets del Makefile

```bash
make test              # Ejecutar todos los tests
make test-unit         # Solo tests unitarios
make test-integration  # Tests de integración (requiere Docker)
make test-e2e          # Tests end-to-end
make test-coverage     # Tests con reporte de cobertura
make bench             # Tests de benchmark
make fuzz              # Fuzz testing
make lint              # Ejecutar golangci-lint
make tidy              # go mod tidy + verify
make build             # Compilar binario
```

### Ejecutar Tests

```bash
# Todos los tests
go test ./... -v -count=1

# Con detector de race
go test -race ./...

# Con cobertura
go test -cover ./...

# Paquete específico
go test ./internal/engine/ -v

# Benchmarks
go test -bench=. -benchmem ./internal/engine/

# Fuzz testing
go test -fuzz=Fuzz -fuzztime=30s ./pkg/bpmn/
```

### Cobertura de Tests

```
Paquete                          Cobertura
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

**100+ tests en total**, todos pasando con detector `-race`.

---

## Usar el Motor Programáticamente

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
    // 1. Parsear un archivo BPMN
    parser := bpmn.NewParser()
    proc, err := parser.ParseFile("process.bpmn")
    if err != nil {
        log.Fatal(err)
    }

    // 2. Registrar implementaciones de elementos
    registry := engine.NewElementRegistry()
    registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
    registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
    registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)

    // 3. Crear store, queue, logger y auditoría
    store := memory.NewStore()
    logger, _ := observability.NewFromConfig("info", "json")
    retry := queue.DefaultRetryPolicy()
    dlq := queue.NewDeadLetterQueue(store, store)
    q := queue.NewWorkerPool(store, nil, retry, dlq, queue.WorkerPoolConfig{
        Concurrency:  4,
        PollInterval: 5 * time.Second,
    })

    dispatcher := observability.NewDispatcher()
    writer, _ := observability.NewFileAuditWriter("./data/audit", true, logger)
    defer writer.Close()
    observability.NewAuditor(dispatcher, writer)
    q.WithDispatcher(dispatcher)

    // 4. Crear motor con auditoría
    eng := engine.New(engine.Config{
        WorkerCount:      4,
        MaxLoops:         100,
        ExecutionTimeout: 30 * time.Second,
    }, registry, store, logger, q).WithDispatcher(dispatcher)

    // 5. Crear y ejecutar instancia
    instance := process.NewInstance(proc, map[string]any{
        "amount": 5000,
    })
    store.CreateInstance(context.Background(), instance.ToRecord())

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := eng.Run(ctx, instance); err != nil {
        log.Fatal(err)
    }

    log.Printf("Proceso completado con estado: %s", instance.State)
}
```

---

## Frameworks y Dependencias

| Categoría | Biblioteca | Import path | Propósito |
|----------|---------|-------------|---------|
| **HTTP Router** | chi/v5 | `github.com/go-chi/chi/v5` | Routing HTTP ligero e idiomático |
| **CORS** | cors | `github.com/go-chi/cors` | Middleware CORS para chi |
| **Prometheus** | client_golang | `github.com/prometheus/client_golang` | Métricas Prometheus oficiales |
| **PostgreSQL** | pgx/v5 | `github.com/jackc/pgx/v5` | Driver DB de alto rendimiento |
| **SQL Utils** | sqlx | `github.com/jmoiron/sqlx` | Named queries, struct scan |
| **Migraciones** | go-migrate | `github.com/golang-migrate/migrate/v4` | Versionado de base de datos |
| **Config** | viper | `github.com/spf13/viper` | Configuración YAML + env |
| **CLI** | cobra | `github.com/spf13/cobra` | Interfaz de línea de comandos |
| **Validación** | validator | `github.com/go-playground/validator/v10` | Validación basada en tags |
| **JWT** | jwt-go | `github.com/golang-jwt/jwt/v5` | Autenticación |
| **UUID** | uuid | `github.com/google/uuid` | UUIDs RFC 4122 |
| **Scheduler** | cron | `github.com/robfig/cron/v3` | Programación de eventos timer |
| **Logger** | log/slog | `log/slog` (stdlib) | Logging estructurado (Go 1.21+) |
| **Context** | context | `context` (stdlib) | Cancelación, timeouts |
| **Testing** | testify | `github.com/stretchr/testify` | Solo assertions |
| **Docker Tests** | dockertest | `github.com/ory/dockertest/v3` | Contenedores para tests de integración |
| **PNG Gen** | gg | `github.com/fogleman/gg` | Dibujo 2D para diagramas de proceso |
| **Rate Limit** | tollbooth | `github.com/didip/tollbooth/v7` | Rate limiting de API |

### No Usados

| Biblioteca | Razón |
|---------|--------|
| Gin/Echo/Fiber | Demasiado "mágicos", chi es más idiomático |
| GORM/ent | ORM pesado, preferir queries explícitas con pgx |
| wire (Google DI) | Excesivo, funciones factory son suficientes |
| zap/zerolog | slog (stdlib) cubre todas las necesidades |
| go-kit | Demasiado complejo para un servicio standalone |

---

## Decisiones de Diseño

| Decisión | Elección | Racional |
|----------|--------|-----------|
| Lenguaje | Go 1.23+ | Concurrencia, rendimiento, binario único |
| Sin ORM | pgx + sqlx | Queries explícitas, sin sorpresas N+1 |
| Sin DI framework | Funciones factory | Go prefiere composición explícita |
| Router | chi | Idiomático, sin overhead de reflexión |
| Logger | slog (stdlib) | Sin dependencia externa necesaria |
| Sin sub-procesos | Fuera de alcance | Reduce complejidad para v1 |
| PostgreSQL | DB principal | JSONB, UUIDs, ecosistema maduro |
| Alpine base | Docker | ~15MB imagen final, usuario non-root |

---

## CI/CD

Pipeline de GitHub Actions en cada push y PR:

```
test → build (6 plataformas) → docker
```

- **Tests**: flag `-race`, contenedor de servicio PostgreSQL 16
- **Cobertura**: Upload a Codecov
- **Build**: linux/darwin/windows × amd64/arm64
- **Docker**: Build + smoke test

---

## Licencia

Licenciado bajo la Licencia Apache, Versión 2.0. Ver [LICENSE](LICENSE) para detalles.
