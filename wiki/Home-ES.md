# Motor BPMN-AI

Motor de ejecución BPMN 2.0 listo para producción escrito en Go con integración nativa de IA/LLM. Construido para automatización de procesos de alto rendimiento donde tareas humanas, llamadas a servicios y decisiones de IA coexisten en el mismo flujo de trabajo.

---

## Resumen

Este motor ejecuta diagramas de procesos BPMN 2.0 y los extiende con capacidades de IA de primera clase.

### Características Principales

| Característica | Descripción |
|---|---|
| **Compatible BPMN 2.0** | Eventos, Compuertas, Actividades, Flujos, Eventos de Borde — 19+ tipos |
| **Integración IA Nativa** | Elemento `aiTask` de primera clase con ruteo de modelos, herramientas, RAG, multi-agente |
| **IA Multi-Proveedor** | OpenAI, Anthropic, OpenRouter (200+ modelos), Groq, proveedores personalizados |
| **Motor Iterativo** | Bucle de ejecución basado en goroutines sin recursión |
| **Sub-Proceso y CallActivity** | Diagramas anidados mediante aplanamiento de elementos |
| **Eventos de Borde** | Timer, Message, Error en actividades |
| **Cola de Trabajos** | Ejecución asíncrona con retroexponencial y cola de mensajes muertos |
| **API HTTP** | API RESTful con chi router y pila de middleware |
| **Observabilidad** | Logging estructurado (slog), métricas Prometheus, dispatch de eventos |
| **Almacenamiento** | En memoria (desarrollo) y PostgreSQL (producción) |

---

## Arquitectura de un Vistazo

```
API HTTP (chi) → Motor Central (bucle iterativo) → Elementos BPMN
                       ↕                            ↕
                 Capa Persistencia           Sistema IA (pkg/ai/)
                 (PostgreSQL/memoria)    Router Modelos · RAG · Guardrails
```

---

## Máquina de Estados

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

## Inicio Rápido

### 1. Configuración

Crea un archivo `.env`:

```bash
AI_OPENROUTER_API_KEY=sk-or-...    # o AI_API_KEY=sk-...
```

### 2. Ejecutar el Motor

```bash
go run ./cmd/engine/
```

### 3. Desplegar un Proceso

```bash
curl -X POST http://localhost:8080/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{"name":"Mi Proceso","bpmn_xml":"<?xml...>"}'
```

### 4. Iniciar un Caso

```bash
curl -X POST http://localhost:8080/api/v1/processes/{id}/start
```

---

## Estructura del Repositorio

| Ruta | Propósito |
|---|---|
| `cmd/engine/` | Punto de entrada CLI |
| `internal/engine/` | Motor BPMN central (bucle, router, failsafe, registro) |
| `internal/element/` | Implementaciones de elementos BPMN (19+ tipos) |
| `internal/process/` | Máquina de estados, ciclo de vida de instancias e hilos |
| `internal/queue/` | Cola de trabajos con reintento y DLQ |
| `internal/observability/` | Logging, métricas, eventos, auditoría |
| `pkg/bpmn/` | Tipos de modelo BPMN y parser XML |
| `pkg/store/` | Interfaces de almacenamiento (memoria, PostgreSQL) |
| `pkg/ai/` | Sistema de proveedores de IA (15+ archivos) |
| `api/http/` | API REST con chi router |
| `api/middleware/` | Middleware HTTP |
| `config/` | Sistema de configuración |
| `testdata/` | Archivos BPMN de prueba |
