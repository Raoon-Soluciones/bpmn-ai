package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type ModelCapability int

const (
	CapabilityToolCall   ModelCapability = 1 << 0
	CapabilityStructured ModelCapability = 1 << 1
	CapabilityVision     ModelCapability = 1 << 2
	CapabilityReasoning  ModelCapability = 1 << 3
	CapabilityStreaming  ModelCapability = 1 << 4
)

type ModelInfo struct {
	Provider     string  `json:"provider"`
	Name         string  `json:"name"`
	ContextWin   int     `json:"context_window"`
	MaxOutput    int     `json:"max_output"`
	PricePer1KIn  float64 `json:"price_per_1k_in"`
	PricePer1KOut float64 `json:"price_per_1k_out"`
	SupportsTools bool    `json:"supports_tools"`
	SupportsJSON  bool    `json:"supports_json"`
	IsFast       bool    `json:"is_fast"`
}

type Profile struct {
	Model       string `json:"model"`
	MaxTokens   int    `json:"max_tokens"`
	Priority    string `json:"priority"` // "cost", "quality", "speed"
}

// DefaultCatalog covers 30+ models across 8 providers, ai-sdk.dev compatible.
var DefaultCatalog = map[string]ModelInfo{
	// ── OpenAI ──
	"openai/gpt-5.5-pro":       {Provider: "openai", Name: "gpt-5.5-pro", ContextWin: 1050000, MaxOutput: 128000, PricePer1KIn: 0.2727, PricePer1KOut: 1.6364, SupportsTools: true, SupportsJSON: true},
	"openai/gpt-5.5":           {Provider: "openai", Name: "gpt-5.5", ContextWin: 1050000, MaxOutput: 128000, PricePer1KIn: 0.03, PricePer1KOut: 0.18, SupportsTools: true, SupportsJSON: true},
	"openai/gpt-5.5-instant":   {Provider: "openai", Name: "gpt-5.5-instant", ContextWin: 400000, MaxOutput: 128000, PricePer1KIn: 0.05, PricePer1KOut: 0.30, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openai/gpt-5.4":           {Provider: "openai", Name: "gpt-5.4", ContextWin: 1050000, MaxOutput: 128000, PricePer1KIn: 0.022, PricePer1KOut: 0.14, SupportsTools: true, SupportsJSON: true},
	"openai/gpt-5.4-mini":      {Provider: "openai", Name: "gpt-5.4-mini", ContextWin: 400000, MaxOutput: 128000, PricePer1KIn: 0.0068, PricePer1KOut: 0.04, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openai/gpt-5.4-nano":      {Provider: "openai", Name: "gpt-5.4-nano", ContextWin: 400000, MaxOutput: 128000, PricePer1KIn: 0.0018, PricePer1KOut: 0.011, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openai/gpt-4o":            {Provider: "openai", Name: "gpt-4o", ContextWin: 128000, MaxOutput: 16384, PricePer1KIn: 0.01, PricePer1KOut: 0.03, SupportsTools: true, SupportsJSON: true},
	"openai/gpt-4o-mini":       {Provider: "openai", Name: "gpt-4o-mini", ContextWin: 128000, MaxOutput: 16384, PricePer1KIn: 0.0015, PricePer1KOut: 0.006, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openai/o4-mini":           {Provider: "openai", Name: "o4-mini", ContextWin: 200000, MaxOutput: 100000, PricePer1KIn: 0.011, PricePer1KOut: 0.044, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openai/o3":                {Provider: "openai", Name: "o3", ContextWin: 200000, MaxOutput: 100000, PricePer1KIn: 0.05, PricePer1KOut: 0.20, SupportsTools: true, SupportsJSON: true},

	// ── Anthropic ──
	"anthropic/claude-opus-4-8":   {Provider: "anthropic", Name: "claude-opus-4-8", ContextWin: 1000000, MaxOutput: 128000, PricePer1KIn: 0.0429, PricePer1KOut: 0.2146, SupportsTools: true, SupportsJSON: true},
	"anthropic/claude-opus-4-7":   {Provider: "anthropic", Name: "claude-opus-4-7", ContextWin: 1000000, MaxOutput: 128000, PricePer1KIn: 0.045, PricePer1KOut: 0.225, SupportsTools: true, SupportsJSON: true},
	"anthropic/claude-sonnet-4-6": {Provider: "anthropic", Name: "claude-sonnet-4-6", ContextWin: 200000, MaxOutput: 64000, PricePer1KIn: 0.03, PricePer1KOut: 0.15, SupportsTools: true, SupportsJSON: true},
	"anthropic/claude-haiku-4-5":  {Provider: "anthropic", Name: "claude-haiku-4-5", ContextWin: 200000, MaxOutput: 8192, PricePer1KIn: 0.008, PricePer1KOut: 0.04, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"anthropic/claude-fable-5":    {Provider: "anthropic", Name: "claude-fable-5", ContextWin: 1000000, MaxOutput: 128000, PricePer1KIn: 0.10, PricePer1KOut: 0.50, SupportsTools: true, SupportsJSON: true},

	// ── Google ──
	"google/gemini-3.5-flash":       {Provider: "google", Name: "gemini-3.5-flash", ContextWin: 1048576, MaxOutput: 65536, PricePer1KIn: 0.0015, PricePer1KOut: 0.009, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"google/gemini-3.1-pro":         {Provider: "google", Name: "gemini-3.1-pro", ContextWin: 1048576, MaxOutput: 65536, PricePer1KIn: 0.02, PricePer1KOut: 0.12, SupportsTools: true, SupportsJSON: true},
	"google/gemini-3.1-flash-lite":  {Provider: "google", Name: "gemini-3.1-flash-lite", ContextWin: 1048576, MaxOutput: 65536, PricePer1KIn: 0.0025, PricePer1KOut: 0.015, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── DeepSeek ──
	"deepseek/deepseek-v4-pro":    {Provider: "deepseek", Name: "deepseek-v4-pro", ContextWin: 1000000, MaxOutput: 384000, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"deepseek/deepseek-v4-flash":  {Provider: "deepseek", Name: "deepseek-v4-flash", ContextWin: 1000000, MaxOutput: 384000, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"deepseek/deepseek-chat":      {Provider: "deepseek", Name: "deepseek-chat", ContextWin: 1000000, MaxOutput: 384000, PricePer1KIn: 0.0014, PricePer1KOut: 0.0028, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── Mistral ──
	"mistral/mistral-large-latest": {Provider: "mistral", Name: "mistral-large-latest", ContextWin: 262144, MaxOutput: 262144, PricePer1KIn: 0.02, PricePer1KOut: 0.06, SupportsTools: true, SupportsJSON: true},
	"mistral/mistral-medium-3.5":   {Provider: "mistral", Name: "mistral-medium-3.5", ContextWin: 262144, MaxOutput: 262144, PricePer1KIn: 0.015, PricePer1KOut: 0.069, SupportsTools: true, SupportsJSON: true},
	"mistral/mistral-small-latest": {Provider: "mistral", Name: "mistral-small-latest", ContextWin: 256000, MaxOutput: 256000, PricePer1KIn: 0.0015, PricePer1KOut: 0.006, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── Meta ──
	"meta/llama-4":                   {Provider: "meta", Name: "llama-4", ContextWin: 131072, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"meta/llama-nemotron-3-super":    {Provider: "meta", Name: "llama-nemotron-3-super", ContextWin: 262144, MaxOutput: 262144, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},

	// ── xAI ──
	"xai/grok-4.3":         {Provider: "xai", Name: "grok-4.3", ContextWin: 1000000, MaxOutput: 30000, PricePer1KIn: 0.0125, PricePer1KOut: 0.025, SupportsTools: true, SupportsJSON: true},
	"xai/grok-build-0.1":   {Provider: "xai", Name: "grok-build-0.1", ContextWin: 256000, MaxOutput: 256000, PricePer1KIn: 0.01, PricePer1KOut: 0.02, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── Cohere ──
	"cohere/command-a-plus": {Provider: "cohere", Name: "command-a-plus", ContextWin: 128000, MaxOutput: 64000, PricePer1KIn: 0.025, PricePer1KOut: 0.10, SupportsTools: true, SupportsJSON: true},

	// ── Zhipu ──
	"zhipu/glm-5.2": {Provider: "zhipu", Name: "glm-5.2", ContextWin: 1000000, MaxOutput: 131072, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"zhipu/glm-5.1": {Provider: "zhipu", Name: "glm-5.1", ContextWin: 200000, MaxOutput: 131072, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},

	// ── Moonshot ──
	"moonshot/kimi-k2.6": {Provider: "moonshot", Name: "kimi-k2.6", ContextWin: 262144, MaxOutput: 262144, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},

	// ── Alibaba ──
	"alibaba/qwen3.6-plus":  {Provider: "alibaba", Name: "qwen3.6-plus", ContextWin: 1000000, MaxOutput: 65536, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"alibaba/qwen3.6-max":   {Provider: "alibaba", Name: "qwen3.6-max", ContextWin: 262144, MaxOutput: 65536, PricePer1KIn: 0.0104, PricePer1KOut: 0.0624, SupportsTools: true, SupportsJSON: true},
	"alibaba/qwen3.5-122b":  {Provider: "alibaba", Name: "qwen3.5-122b", ContextWin: 262144, MaxOutput: 65536, PricePer1KIn: 0.0012, PricePer1KOut: 0.0092, SupportsTools: true, SupportsJSON: true},

	// ── Nvidia ──
	"nvidia/nemotron-3-super":    {Provider: "nvidia", Name: "nemotron-3-super", ContextWin: 262144, MaxOutput: 262144, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"nvidia/nemotron-3-nano":     {Provider: "nvidia", Name: "nemotron-3-nano", ContextWin: 256000, MaxOutput: 65536, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── Xiaomi ──
	"xiaomi/mimo-v2.5-pro": {Provider: "xiaomi", Name: "mimo-v2.5-pro", ContextWin: 1048576, MaxOutput: 131072, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},

	// ── Groq (OpenAI-compatible, ultra-fast inference) ──
	"groq/llama-4":                  {Provider: "groq", Name: "llama-4", ContextWin: 131072, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"groq/llama-3.3-70b":            {Provider: "groq", Name: "llama-3.3-70b", ContextWin: 131072, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"groq/deepseek-distill":         {Provider: "groq", Name: "deepseek-r1-distill-llama-70b", ContextWin: 131072, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: false, SupportsJSON: true, IsFast: true},
	"groq/mixtral-8x7b":             {Provider: "groq", Name: "mixtral-8x7b-32768", ContextWin: 32768, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},

	// ── OpenRouter (unified API gateway — routes to any model via single endpoint) ──
	"openrouter/openai/gpt-4o":              {Provider: "openrouter", Name: "openai/gpt-4o", ContextWin: 128000, MaxOutput: 16384, PricePer1KIn: 0.0025, PricePer1KOut: 0.01, SupportsTools: true, SupportsJSON: true},
	"openrouter/openai/gpt-4o-mini":         {Provider: "openrouter", Name: "openai/gpt-4o-mini", ContextWin: 128000, MaxOutput: 16384, PricePer1KIn: 0.00015, PricePer1KOut: 0.0006, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/anthropic/claude-sonnet-4-6": {Provider: "openrouter", Name: "anthropic/claude-sonnet-4-6", ContextWin: 200000, MaxOutput: 64000, PricePer1KIn: 0.009, PricePer1KOut: 0.045, SupportsTools: true, SupportsJSON: true},
	"openrouter/anthropic/claude-haiku-4-5":  {Provider: "openrouter", Name: "anthropic/claude-haiku-4-5", ContextWin: 200000, MaxOutput: 8192, PricePer1KIn: 0.0025, PricePer1KOut: 0.0125, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/google/gemini-3.5-flash":     {Provider: "openrouter", Name: "google/gemini-3.5-flash", ContextWin: 1048576, MaxOutput: 65536, PricePer1KIn: 0.0003, PricePer1KOut: 0.0015, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/google/gemini-3.1-pro":       {Provider: "openrouter", Name: "google/gemini-3.1-pro", ContextWin: 1048576, MaxOutput: 65536, PricePer1KIn: 0.005, PricePer1KOut: 0.03, SupportsTools: true, SupportsJSON: true},
	"openrouter/deepseek/deepseek-v4-flash":  {Provider: "openrouter", Name: "deepseek/deepseek-v4-flash", ContextWin: 1000000, MaxOutput: 384000, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/deepseek/deepseek-chat":      {Provider: "openrouter", Name: "deepseek/deepseek-chat", ContextWin: 1000000, MaxOutput: 384000, PricePer1KIn: 0.0007, PricePer1KOut: 0.0014, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/meta/llama-4":                {Provider: "openrouter", Name: "meta/llama-4", ContextWin: 131072, MaxOutput: 32768, PricePer1KIn: 0.0, PricePer1KOut: 0.0, SupportsTools: true, SupportsJSON: true},
	"openrouter/mistral/mistral-small":       {Provider: "openrouter", Name: "mistral/mistral-small", ContextWin: 256000, MaxOutput: 256000, PricePer1KIn: 0.001, PricePer1KOut: 0.003, SupportsTools: true, SupportsJSON: true, IsFast: true},
	"openrouter/xai/grok-4.3":                {Provider: "openrouter", Name: "xai/grok-4.3", ContextWin: 1000000, MaxOutput: 30000, PricePer1KIn: 0.003, PricePer1KOut: 0.006, SupportsTools: true, SupportsJSON: true},
	"openrouter/cohere/command-a-plus":       {Provider: "openrouter", Name: "cohere/command-a-plus", ContextWin: 128000, MaxOutput: 64000, PricePer1KIn: 0.0075, PricePer1KOut: 0.03, SupportsTools: true, SupportsJSON: true},
	"openrouter/alibaba/qwen3.5-122b":        {Provider: "openrouter", Name: "alibaba/qwen3.5-122b", ContextWin: 262144, MaxOutput: 65536, PricePer1KIn: 0.0006, PricePer1KOut: 0.0046, SupportsTools: true, SupportsJSON: true},
}

// Aliases maps short names to fully-qualified model names (ai-sdk.dev style).
var ModelAliases = map[string]string{
	"gpt-4o":              "openai/gpt-4o",
	"gpt-4o-mini":         "openai/gpt-4o-mini",
	"gpt-5.5":             "openai/gpt-5.5",
	"gpt-5.5-pro":         "openai/gpt-5.5-pro",
	"gpt-5.4":             "openai/gpt-5.4",
	"gpt-5.4-mini":        "openai/gpt-5.4-mini",
	"gpt-5.4-nano":        "openai/gpt-5.4-nano",
	"claude-sonnet-4-6":   "anthropic/claude-sonnet-4-6",
	"claude-haiku-4-5":    "anthropic/claude-haiku-4-5",
	"claude-opus-4-8":     "anthropic/claude-opus-4-8",
	"claude-fable-5":      "anthropic/claude-fable-5",
	"gemini-3.5-flash":    "google/gemini-3.5-flash",
	"deepseek-v4-flash":   "deepseek/deepseek-v4-flash",
	"deepseek-chat":       "deepseek/deepseek-chat",
	"grok-4.3":            "xai/grok-4.3",

	// OpenRouter short aliases
	"openrouter/gpt-4o":             "openrouter/openai/gpt-4o",
	"openrouter/gpt-4o-mini":        "openrouter/openai/gpt-4o-mini",
	"openrouter/claude-sonnet-4-6":  "openrouter/anthropic/claude-sonnet-4-6",
	"openrouter/claude-haiku-4-5":   "openrouter/anthropic/claude-haiku-4-5",
	"openrouter/deepseek-chat":      "openrouter/deepseek/deepseek-chat",
	"openrouter/llama-4":            "openrouter/meta/llama-4",
	"openrouter/grok-4.3":           "openrouter/xai/grok-4.3",
}

// LabAliases maps models.dev lab names to provider pool keys.
// When a model like "zhipuai/glm-5.2" is requested and no "zhipuai"
// provider is registered, the router checks this map to find a fallback.
var LabAliases = map[string]string{
	// models.dev labs → provider pool keys (in priority order)
	"openai":     "openai",
	"anthropic":  "anthropic",
	"google":     "google",
	"deepseek":   "deepseek",
	"mistral":    "mistral",
	"meta":       "meta",
	"xai":        "xai",
	"cohere":     "cohere",
	"zhipu":      "openrouter",
	"zhipuai":    "openrouter",
	"moonshot":   "openrouter",
	"moonshotai": "openrouter",
	"stepfun":    "openrouter",
	"alibaba":    "openrouter",
	"minimax":    "openrouter",
	"nvidia":     "nvidia",
	"xiaomi":     "xiaomi",
	"groq":       "groq",
	"together":   "openrouter",
	"fireworks":  "openrouter",
	"perplexity": "openrouter",
}

var DefaultProfiles = map[string]Profile{
	"complex": {Model: "openai/gpt-4o", MaxTokens: 8192, Priority: "quality"},
	"fast":    {Model: "openai/gpt-4o-mini", MaxTokens: 2048, Priority: "speed"},
	"cheap":   {Model: "openai/gpt-4o-mini", MaxTokens: 1024, Priority: "cost"},
	"auto":    {Model: "openai/gpt-4o", MaxTokens: 4096, Priority: "quality"},
}

type ProviderPool struct {
	mu        sync.RWMutex
	providers map[string]Gateway
}

func NewProviderPool() *ProviderPool {
	return &ProviderPool{providers: make(map[string]Gateway)}
}

func (p *ProviderPool) Add(name string, gateway Gateway) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers[name] = gateway
}

func (p *ProviderPool) Get(name string) (Gateway, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.providers[name]
	return g, ok
}

func (p *ProviderPool) List() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.providers))
	for n := range p.providers {
		names = append(names, n)
	}
	return names
}

type ModelRouter struct {
	pool     *ProviderPool
	catalog  map[string]ModelInfo
	aliases  map[string]string
	profiles map[string]Profile
}

func NewModelRouter(pool *ProviderPool) *ModelRouter {
	catalog := make(map[string]ModelInfo, len(DefaultCatalog))
	for k, v := range DefaultCatalog {
		catalog[k] = v
	}
	aliases := make(map[string]string, len(ModelAliases))
	for k, v := range ModelAliases {
		aliases[k] = v
	}
	profiles := make(map[string]Profile, len(DefaultProfiles))
	for k, v := range DefaultProfiles {
		profiles[k] = v
	}
	return &ModelRouter{
		pool:     pool,
		catalog:  catalog,
		aliases:  aliases,
		profiles: profiles,
	}
}

func (r *ModelRouter) AddModel(info ModelInfo) {
	r.catalog[info.Provider+"/"+info.Name] = info
}

func (r *ModelRouter) AddAlias(short, qualified string) {
	r.aliases[short] = qualified
}

func (r *ModelRouter) AddProfile(name string, p Profile) {
	r.profiles[name] = p
}

func (r *ModelRouter) SetProfiles(profiles map[string]Profile) {
	for k, v := range profiles {
		r.profiles[k] = v
	}
}

// Resolve returns the best Gateway + model name for the given profile/model/tools/schema.
// Supports ai-sdk.dev style "provider/model-name" format.
func (r *ModelRouter) Resolve(_ context.Context, profileName string, modelName string, tools bool, schema bool) (Gateway, string, error) {
	if modelName == "" {
		prof := r.resolveProfile(profileName)
		modelName = prof.Model
	}

	// Resolve aliases (e.g. "gpt-4o" → "openai/gpt-4o")
	if expanded, ok := r.aliases[modelName]; ok {
		modelName = expanded
	}

	// Check catalog for a fully-qualified match
	info, found := r.catalog[modelName]
	if found {
		provider, ok := r.pool.Get(info.Provider)
		if ok {
			// Check capability requirements
			if (tools && !info.SupportsTools) || (schema && !info.SupportsJSON) {
				alt := r.findBestMatch(tools, schema, info.Provider, "")
				if alt != "" {
					if gw, ok := r.pool.Get(r.catalog[alt].Provider); ok {
						return gw, alt, nil
					}
				}
			}
			return provider, modelName, nil
		}
		// Provider not available — try other provider with same model
		alt := r.findBestMatch(tools, schema, "", modelName)
		if alt != "" {
			if gw, ok := r.pool.Get(r.catalog[alt].Provider); ok {
				return gw, alt, nil
			}
		}
	}

	// Try provider-prefix parsing: "openai/gpt-4o" → provider "openai"
	if prov, model := splitModelName(modelName); prov != "" {
		if gw, ok := r.pool.Get(prov); ok {
			return gw, model, nil
		}
		// Check lab aliases (models.dev compatibility)
		if alias, ok := LabAliases[prov]; ok {
			if gw, ok := r.pool.Get(alias); ok {
				return gw, model, nil
			}
		}
	}

	// Fallback: any available provider
	return r.resolveAnyProvider(modelName)
}

func splitModelName(name string) (provider string, model string) {
	if idx := strings.Index(name, "/"); idx > 0 {
		return name[:idx], name[idx+1:]
	}
	return "", name
}

func (r *ModelRouter) resolveProfile(profileName string) Profile {
	if profileName == "" {
		profileName = "auto"
	}
	if p, ok := r.profiles[profileName]; ok {
		return p
	}
	return r.profiles["auto"]
}

func (r *ModelRouter) resolveAnyProvider(modelName string) (Gateway, string, error) {
	providers := r.pool.List()
	if len(providers) == 0 {
		return nil, "", fmt.Errorf("no AI providers configured")
	}

	gw, ok := r.pool.Get(providers[0])
	if !ok {
		return nil, "", fmt.Errorf("provider %q not found", providers[0])
	}

	// Extract just the model part if provider-prefixed
	_, model := splitModelName(modelName)
	if model == "" {
		model = modelName
	}

	return gw, model, nil
}

func (r *ModelRouter) findBestMatch(needTools, needJSON bool, excludeProvider, modelHint string) string {
	var best string
	var bestScore float64

	for key, info := range r.catalog {
		if excludeProvider != "" && info.Provider == excludeProvider {
			continue
		}
		if needTools && !info.SupportsTools {
			continue
		}
		if needJSON && !info.SupportsJSON {
			continue
		}
		if modelHint != "" {
			_, hintName := splitModelName(modelHint)
			if hintName == "" {
				hintName = modelHint
			}
			if info.Name == hintName {
				return key
			}
		}

		score := float64(info.ContextWin) / 1000
		if info.SupportsTools {
			score += 50
		}
		if info.SupportsJSON {
			score += 20
		}

		if score > bestScore {
			bestScore = score
			best = key
		}
	}
	return best
}

func (r *ModelRouter) ListModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(r.catalog))
	for _, info := range r.catalog {
		models = append(models, info)
	}
	return models
}

func (r *ModelRouter) ListProfiles() map[string]Profile {
	profiles := make(map[string]Profile, len(r.profiles))
	for k, v := range r.profiles {
		profiles[k] = v
	}
	return profiles
}
