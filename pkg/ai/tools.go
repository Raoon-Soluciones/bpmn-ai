package ai

import (
	"fmt"
	"sync"
)

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolDefinition
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolDefinition),
	}
}

func (r *ToolRegistry) Register(def ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if def.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	r.tools[def.Name] = def
	return nil
}

func (r *ToolRegistry) Get(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	return def, ok
}

func (r *ToolRegistry) GetDefinitions(names ...string) ([]ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(names) == 0 {
		defs := make([]ToolDefinition, 0, len(r.tools))
		for _, def := range r.tools {
			defs = append(defs, def.WithoutFunction())
		}
		return defs, nil
	}

	defs := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		def, ok := r.tools[name]
		if !ok {
			return nil, fmt.Errorf("tool %q not found", name)
		}
		defs = append(defs, def.WithoutFunction())
	}
	return defs, nil
}

func (r *ToolRegistry) GetFunctions(names ...string) ([]ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(names) == 0 {
		defs := make([]ToolDefinition, 0, len(r.tools))
		for _, def := range r.tools {
			defs = append(defs, def)
		}
		return defs, nil
	}

	defs := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		def, ok := r.tools[name]
		if !ok {
			return nil, fmt.Errorf("tool %q not found", name)
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
