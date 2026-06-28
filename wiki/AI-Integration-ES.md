# Integración de IA

El motor BPMN-AI incluye un **sistema de IA/LLM de primera clase** en `pkg/ai/`. El elemento `AITask` (`scriptType="ai"`) permite que cualquier proceso BPMN llame a modelos de lenguaje, use RAG, invoque herramientas, aplique guardrails y orqueste flujos multi-agente.

---

## Arquitectura

```
Elemento AITask
    ↓
pkg/ai/gateway.go — Interfaz Gateway (API LLM unificada)
    ↓
pkg/ai/router.go — Selección de modelos (60+, 15 proveedores, 4 perfiles)
    ↓
pkg/ai/provider.go — Pool de proveedores (OpenAI, Anthropic, OpenRouter, Groq, ...)
    ↓
pkg/ai/fallback.go — Failover primario → secundario
pkg/ai/cross_provider.go — Failover multi-proveedor ordenado
    ↓
Retorno → Inyección de variables → Guardrails → Caché
```

### Archivos Clave

| Archivo | Propósito |
|---|---|
| `gateway.go` | Interfaz `Gateway`, tipos `Request`, `Response`, `Message`, `ToolDefinition` |
| `openai.go` | Proveedor OpenAI (Go OpenAI SDK) |
| `anthropic.go` | Proveedor Anthropic Claude (SDK oficial) |
| `router.go` | Router de modelos con `DefaultCatalog` (60+ modelos), perfiles, alias |
| `provider.go` | Registro de proveedores (patrón factory) |
| `fallback.go` | Failover primario → secundario |
| `cross_provider.go` | Cadena de failover multi-proveedor ordenada |
| `cache.go` | Caché de respuestas (memoria o Redis) con TTL |
| `guardrail.go` | Redacción PII, límites de costo, límites de token, HITL |
| `rag.go` | Sistema RAG: embedding → búsqueda → enriquecimiento de contexto |
| `vectorstore.go` | Interfaz de almacén vectorial + implementación en memoria |
| `embedding.go` | Interfaz Embedder + Embedder OpenAI |
| `agent.go` | Ejecutor multi-agente secuencial |
| `prompt.go` | Gestor de prompts con plantillas versionadas y hash SHA256 |
| `tools.go` | Registro de herramientas para function calling LLM |
| `schema.go` | Validador JSON Schema para salida estructurada |
| `metrics.go` | Métricas Prometheus de IA |

---

## Router de Modelos

**Archivo**: `pkg/ai/router.go`

El router tiene un `DefaultCatalog` con **más de 60 modelos** en **15 proveedores**:

| Proveedor | Modelos Ejemplo |
|---|---|
| OpenAI | `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `o1`, `o3-mini` |
| Anthropic | `claude-sonnet-4-20250514`, `claude-3-5-sonnet-latest` |
| OpenRouter | 200+ modelos via API OpenRouter |
| Groq | `llama-3.3-70b-versatile`, `mixtral-8x7b-32768` |
| Google | `gemini-2.0-flash-001` |
| DeepSeek | `deepseek-chat`, `deepseek-reasoner` |
| Cohere | `command-r-plus`, `command-r` |
| Amazon | `nova-pro-v1`, `nova-lite-v1` |
| Microsoft | `phi-4` |
| Mistral | `mistral-large-latest`, `codestral-latest` |
| xAI | `grok-2-latest` |
| Perplexity | `sonar-pro` |
| Together | `llama-3.1-8b` |
| Fireworks | `llama-v3p1-405b-instruct` |
| Custom | Cualquier endpoint compatible con OpenAI via proveedor `custom` |

### Perfiles

El router soporta 4 perfiles de ruteo:

| Perfil | Estrategia | Caso de Uso |
|---|---|---|
| `auto` | Equilibra costo + capacidad | Propósito general |
| `fast` | Prioriza velocidad (baja latencia) | Tiempo real, tareas simples |
| `cheap` | Minimiza costo | Procesamiento por lotes |
| `complex` | Maximiza capacidad | Análisis, razonamiento |

Los alias de modelos permiten failover entre proveedores: `fast` → `cheap` → `auto` → `complex`.

---

## Elemento AITask

**Archivo**: `internal/element/activities/ai_task.go`

Registrado en `cmd/engine/main.go:68-88` como `bpmn.ElementTypeAITask` con constructor que recibe `Gateway`, `ToolRegistry`, `ModelRouter`, `RAGSystem`, `PromptManager`.

### Atributos XML

| Atributo | Descripción | Ejemplo |
|---|---|---|
| `scriptBody` | El prompt para el LLM (soporta interpolación `{{variable}}`) | `Analiza: {{input_text}}` |
| `model` | Sobrescritura de modelo | `gpt-4o`, `claude-sonnet-4` |
| `profile` | Perfil de ruteo | `auto`, `fast`, `cheap`, `complex` |
| `systemPrompt` | Prompt de sistema | `Eres un analista útil.` |
| `tools` | Nombres de herramientas separados por coma | `buscar,calcular` |
| `rag` | Nombre de colección RAG | `base-conocimiento` |
| `outputSchema` | JSON Schema para salida estructurada | `{"type":"object","properties":{...}}` |
| `label` | Prefijo de variable de proceso | `mi_var` |
| `stream` | Habilitar streaming de tokens | `true` |

### Variables de Salida

| Variable | Descripción |
|---|---|
| `{id}_result` | Respuesta de texto completo |
| `{id}_model` | Modelo utilizado |
| `{id}_tokens_in` | Tokens de entrada |
| `{id}_tokens_out` | Tokens de salida |
| `{id}_cost` | Costo de esta llamada |
| `{id}_parsed` | Salida estructurada parseada (si `outputSchema`) |
| `{id}_validation_error` | Error de validación de esquema |
| `{id}_tool_calls` | Resultados de llamadas a herramientas |
| `ai_total_cost` | Costo acumulado en todas las llamadas IA |

### Ejemplo

```xml
<scriptTask id="clasificar" name="Clasificar Ticket" scriptType="ai"
    scriptBody="Clasifica este ticket de soporte: {{ticket_text}}
Categorías: facturación, técnico, cuenta, general"
    systemPrompt="Eres un clasificador de soporte al cliente."
    profile="fast" model="gpt-4o-mini"
    outputSchema='{"type":"object","properties":{"categoria":{"type":"string"},"confianza":{"type":"number"},"prioridad":{"type":"string"}},"required":["categoria","confianza"]}'/>
```

---

## Sistema RAG

**Archivo**: `pkg/ai/rag.go`

Pipeline de búsqueda semántica:
1. Consulta de usuario → embedding (`pkg/ai/embedding.go`)
2. Búsqueda por similitud en almacén vectorial (`pkg/ai/vectorstore.go`)
3. Documentos recuperados → enriquecimiento de contexto
4. Inyectados en el prompt del LLM como contexto

```xml
<scriptTask id="qa" name="Responder Pregunta" scriptType="ai"
    scriptBody="Responde basado en el contexto: {{pregunta}}"
    rag="documentos-producto"
    systemPrompt="Responde usando solo el contexto proporcionado."/>
```

---

## Guardrails

**Archivo**: `pkg/ai/guardrail.go`

| Tipo | Descripción |
|---|---|
| Redacción PII | Detecta y redacta PII antes de enviar al LLM |
| Límite de Costo | Aborta si el costo por llamada o acumulado supera el umbral |
| Límites de Token | Aplica máximos de tokens de entrada/salida |
| HITL | Verificación humano-en-el-bucle — requiere aprobación si la confianza es baja |

---

## Multi-Agente

**Archivo**: `pkg/ai/agent.go`

Orquestación secuencial de sub-agentes. Cada agente recibe la salida del agente anterior:

```xml
<scriptTask id="pipeline-agentes" name="Multi-Agente" scriptType="ai"
    scriptBody="Genera un reporte: {{input}}"
    agents="extraer,analizar,resumir"
    systemPrompt="Eres el resumidor final."/>
```

---

## Caché

**Archivo**: `pkg/ai/cache.go`

- Backend en memoria o Redis
- Expiración basada en TTL
- Key por concatenación de mensajes + modelo + parámetros
- Omite caché para streaming o llamadas a herramientas

---

## Seguimiento de Costos

Cada llamada IA registra:
- `{id}_cost` — costo de la llamada individual
- `ai_total_cost` — costo acumulado en toda la instancia

El costo se calcula usando precios de tokens específicos por modelo desde el catálogo de modelos.

---

## Streaming

Cuando `stream="true"`, los tokens se transmiten a la respuesta:
- La variable `{id}_partial` se actualiza en tiempo real
- Streaming es incompatible con llamadas a herramientas en algunos proveedores

---

## Herramientas

**Archivo**: `pkg/ai/tools.go`

Registra funciones Go como herramientas invocables por LLM:

```go
registry.Register(ToolDefinition{
    Name:        "calcular",
    Description: "Realiza un cálculo",
    Parameters:  map[string]interface{}{...},
    Handler:     func(args map[string]interface{}) (interface{}, error) { ... },
})
```

Las herramientas se ejecutan en múltiples rondas — el LLM puede llamar varias herramientas secuencialmente.
