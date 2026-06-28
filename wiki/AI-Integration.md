# AI Integration

The BPMN-AI engine includes a **first-class AI/LLM system** in `pkg/ai/`. The `AITask` element (`scriptType="ai"`) allows any BPMN process to call language models, use RAG, invoke tools, enforce guardrails, and orchestrate multi-agent workflows.

---

## Architecture

```
AITask Element
    ↓
pkg/ai/gateway.go — Gateway interface (unified LLM API)
    ↓
pkg/ai/router.go — Model selection (60+ models, 15 providers, 4 profiles)
    ↓
pkg/ai/provider.go — Provider pool (OpenAI, Anthropic, OpenRouter, Groq, ...)
    ↓
pkg/ai/fallback.go — Primary → secondary failover
pkg/ai/cross_provider.go — Ordered multi-provider failover
    ↓
Return → Variable injection → Guardrails → Cache
```

### Key Files

| File | Purpose |
|---|---|
| `gateway.go` | `Gateway` interface, `Request`, `Response`, `Message`, `ToolDefinition` types |
| `openai.go` | OpenAI provider (Go OpenAI SDK) |
| `anthropic.go` | Anthropic Claude provider (official SDK) |
| `router.go` | Model router with `DefaultCatalog` (60+ models), profiles, aliases |
| `provider.go` | Provider registry (factory pattern) |
| `fallback.go` | Primary → secondary provider failover |
| `cross_provider.go` | Ordered multi-provider failover chain |
| `cache.go` | Response caching (in-memory or Redis) with TTL |
| `guardrail.go` | PII redaction, cost limits, token limits, HITL confidence checks |
| `rag.go` | RAG system: embedding → search → context enrichment |
| `vectorstore.go` | Vector store interface + in-memory implementation |
| `embedding.go` | Embedder interface + OpenAI embedder |
| `agent.go` | Multi-agent sequential executor |
| `prompt.go` | Prompt manager with versioned templates and SHA256 hashing |
| `tools.go` | Tool registry for LLM function calling |
| `schema.go` | JSON Schema validator for structured output |
| `metrics.go` | Prometheus AI metrics |

---

## Model Router

**File**: `pkg/ai/router.go`

The router has a `DefaultCatalog` with **60+ model entries** across **15 providers**:

| Provider | Example Models |
|---|---|
| OpenAI | `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `o1`, `o3-mini` |
| Anthropic | `claude-sonnet-4-20250514`, `claude-3-5-sonnet-latest` |
| OpenRouter | 200+ models via OpenRouter API |
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
| Custom | Any OpenAI-compatible endpoint via `custom` provider |

### Profiles

The router supports 4 routing profiles:

| Profile | Strategy | Use Case |
|---|---|---|
| `auto` | Balances cost + capability | General purpose |
| `fast` | Prioritizes speed (low latency) | Real-time, simple tasks |
| `cheap` | Minimizes cost | Batch processing, low priority |
| `complex` | Maximizes capability | Analysis, reasoning |

Model aliases enable cross-provider fallback: `fast` → `cheap` → `auto` → `complex`.

---

## AITask Element

**File**: `internal/element/activities/ai_task.go`

Registered in `cmd/engine/main.go:68-88` as `bpmn.ElementTypeAITask` with constructor receiving `Gateway`, `ToolRegistry`, `ModelRouter`, `RAGSystem`, `PromptManager`.

### XML Attributes

| Attribute | Description | Example |
|---|---|---|
| `scriptBody` | The LLM prompt (supports `{{variable}}` interpolation) | `Analyze: {{input_text}}` |
| `model` | Model override | `gpt-4o`, `claude-sonnet-4` |
| `profile` | Routing profile | `auto`, `fast`, `cheap`, `complex` |
| `systemPrompt` | System prompt | `You are a helpful analyst.` |
| `tools` | Comma-separated tool names | `search,calculate` |
| `rag` | RAG collection name | `knowledge-base` |
| `outputSchema` | JSON Schema for structured output | `{"type":"object","properties":{...}}` |
| `label` | Process variable prefix override | `my_var` |
| `stream` | Enable token streaming | `true` |

### Output Variables

| Variable | Description |
|---|---|
| `{id}_result` | Full text response |
| `{id}_model` | Model used |
| `{id}_tokens_in` | Input tokens |
| `{id}_tokens_out` | Output tokens |
| `{id}_cost` | Cost of this call |
| `{id}_parsed` | Parsed structured output (if `outputSchema`) |
| `{id}_validation_error` | Schema validation error (if validation fails) |
| `{id}_tool_calls` | Tool call results |
| `ai_total_cost` | Cumulative cost across all AI calls in the instance |

### Example

```xml
<scriptTask id="classify" name="Classify Ticket" scriptType="ai"
    scriptBody="Classify this support ticket: {{ticket_text}}
Categories: billing, technical, account, general"
    systemPrompt="You are a customer support classifier."
    profile="fast" model="gpt-4o-mini"
    outputSchema='{"type":"object","properties":{"category":{"type":"string"},"confidence":{"type":"number"},"priority":{"type":"string"}},"required":["category","confidence"]}'/>
```

---

## RAG System

**File**: `pkg/ai/rag.go`

Semantic search pipeline:
1. User query → embedding (`pkg/ai/embedding.go`)
2. Similarity search in vector store (`pkg/ai/vectorstore.go`)
3. Retrieved documents → context enrichment
4. Injected into LLM prompt as context

```xml
<scriptTask id="qa" name="Answer Question" scriptType="ai"
    scriptBody="Answer based on context: {{question}}"
    rag="product-docs"
    systemPrompt="Answer using only the provided context."/>
```

---

## Guardrails

**File**: `pkg/ai/guardrail.go`

| Type | Description |
|---|---|
| PII Redaction | Detects and redacts PII before sending to LLM |
| Cost Limiting | Aborts if per-call or cumulative cost exceeds threshold |
| Token Limits | Enforces max input/output tokens |
| HITL | Human-in-the-loop confidence check — requires approval if confidence below threshold |

---

## Multi-Agent

**File**: `pkg/ai/agent.go`

Sequential sub-agent orchestration. Each agent receives the previous agent's output:

```xml
<scriptTask id="agent-pipeline" name="Multi-Agent" scriptType="ai"
    scriptBody="Generate a report: {{input}}"
    agents="extract,analyze,summarize"
    systemPrompt="You are the final summarizer."/>
```

---

## Caching

**File**: `pkg/ai/cache.go`

- In-memory or Redis backend
- TTL-based expiration
- Keyed by concatenated messages + model + parameters
- Skips cache for streaming or tool calls

---

## Cost Tracking

Every AI call records:
- `{id}_cost` — individual call cost
- `ai_total_cost` — cumulative across the process instance

Cost is calculated using model-specific token pricing from the model catalog.

---

## Streaming

When `stream="true"`, tokens are streamed to the response:
- `{id}_partial` variable is updated in real-time
- Streaming is incompatible with tool calling on some providers

---

## Tools

**File**: `pkg/ai/tools.go`

Register Go functions as LLM-callable tools:

```go
registry.Register(ToolDefinition{
    Name:        "calculate",
    Description: "Perform a calculation",
    Parameters:  map[string]interface{}{...},
    Handler:     func(args map[string]interface{}) (interface{}, error) { ... },
})
```

Tools execute in multi-round fashion — the LLM can call multiple tools sequentially.
