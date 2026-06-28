package ai

import (
	"context"
	"encoding/json"
	"testing"
)

func TestToolRegistry_Register(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(ToolDefinition{
		Name:        "get_weather",
		Description: "Get weather for a location",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"loc":{"type":"string"}}}`),
		Function: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "sunny", nil
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestToolRegistry_Register_Duplicate(t *testing.T) {
	r := NewToolRegistry()
	r.Register(ToolDefinition{Name: "foo", Description: "first"})
	err := r.Register(ToolDefinition{Name: "foo", Description: "second"})
	if err == nil {
		t.Fatal("expected error for duplicate tool")
	}
}

func TestToolRegistry_Register_EmptyName(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(ToolDefinition{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestToolRegistry_Get(t *testing.T) {
	r := NewToolRegistry()
	r.Register(ToolDefinition{Name: "foo", Description: "bar"})
	def, ok := r.Get("foo")
	if !ok {
		t.Fatal("expected to find foo")
	}
	if def.Description != "bar" {
		t.Errorf("expected description 'bar', got '%s'", def.Description)
	}
}

func TestToolRegistry_Get_Missing(t *testing.T) {
	r := NewToolRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestToolRegistry_GetDefinitions(t *testing.T) {
	r := NewToolRegistry()
	r.Register(ToolDefinition{
		Name:        "lookup",
		Description: "Lookup data",
		Function: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "data", nil
		},
	})

	defs, err := r.GetDefinitions("lookup")
	if err != nil {
		t.Fatalf("GetDefinitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Function != nil {
		t.Error("expected stripped function in GetDefinitions")
	}
}

func TestToolRegistry_GetDefinitions_All(t *testing.T) {
	r := NewToolRegistry()
	r.Register(ToolDefinition{Name: "a"})
	r.Register(ToolDefinition{Name: "b"})
	defs, err := r.GetDefinitions()
	if err != nil {
		t.Fatalf("GetDefinitions: %v", err)
	}
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestToolRegistry_GetDefinitions_Missing(t *testing.T) {
	r := NewToolRegistry()
	_, err := r.GetDefinitions("missing")
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestToolRegistry_GetFunctions(t *testing.T) {
	r := NewToolRegistry()
	called := false
	r.Register(ToolDefinition{
		Name: "exec",
		Function: func(_ context.Context, _ json.RawMessage) (string, error) {
			called = true
			return "done", nil
		},
	})

	defs, err := r.GetFunctions("exec")
	if err != nil {
		t.Fatalf("GetFunctions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}

	result, err := defs[0].Function(context.Background(), nil)
	if err != nil {
		t.Fatalf("Function: %v", err)
	}
	if result != "done" {
		t.Errorf("expected 'done', got '%s'", result)
	}
	if !called {
		t.Error("expected function to be called")
	}
}

func TestToolRegistry_List(t *testing.T) {
	r := NewToolRegistry()
	r.Register(ToolDefinition{Name: "a"})
	r.Register(ToolDefinition{Name: "b"})
	names := r.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestToolDefinition_WithoutFunction(t *testing.T) {
	fn := func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	}
	def := ToolDefinition{
		Name:        "test",
		Description: "desc",
		Parameters:  json.RawMessage(`{}`),
		Function:    fn,
	}
	stripped := def.WithoutFunction()
	if stripped.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", stripped.Name)
	}
	if stripped.Function != nil {
		t.Error("expected nil function in stripped definition")
	}
}

func TestRequest_WithDefaults_ToolRounds(t *testing.T) {
	r := Request{}
	r = r.WithDefaults()
	if r.MaxToolRounds != 5 {
		t.Errorf("expected MaxToolRounds=5, got %d", r.MaxToolRounds)
	}
}

func TestRequest_WithDefaults_PreservesToolRounds(t *testing.T) {
	r := Request{MaxToolRounds: 2}
	r = r.WithDefaults()
	if r.MaxToolRounds != 2 {
		t.Errorf("expected MaxToolRounds=2, got %d", r.MaxToolRounds)
	}
}
