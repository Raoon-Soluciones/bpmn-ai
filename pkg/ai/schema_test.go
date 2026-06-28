package ai

import (
	"encoding/json"
	"testing"
)

func TestSchemaValidator_Valid(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {"type": "string"},
			"score": {"type": "number"}
		},
		"required": ["category", "score"]
	}`)

	data := `{"category": "billing", "score": 85}`

	v := NewSchemaValidator()
	result, err := v.Validate(schema, data)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result["category"] != "billing" {
		t.Errorf("expected category 'billing', got %v", result["category"])
	}
	if result["score"] != float64(85) {
		t.Errorf("expected score 85, got %v", result["score"])
	}
}

func TestSchemaValidator_InvalidJSON(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	v := NewSchemaValidator()
	_, err := v.Validate(schema, "not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	var ve *ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestSchemaValidator_SchemaViolation(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {"name": {"type": "string"}}
	}`)
	v := NewSchemaValidator()
	_, err := v.Validate(schema, `{"category": "billing"}`)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
	var ve *ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestSchemaValidator_TypeMismatch(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"count": {"type": "integer"}
		}
	}`)
	v := NewSchemaValidator()
	_, err := v.Validate(schema, `{"count": "not-a-number"}`)
	if err == nil {
		t.Fatal("expected validation error for type mismatch")
	}
}

func TestSchemaValidator_NotObject(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	v := NewSchemaValidator()
	_, err := v.Validate(schema, `["array", "not", "object"]`)
	if err == nil {
		t.Fatal("expected error for non-object JSON")
	}
}

func asValidationError(err error, target **ValidationError) bool {
	ve, ok := err.(*ValidationError)
	if ok {
		*target = ve
	}
	return ok
}

func TestValidationError_Error(t *testing.T) {
	e := &ValidationError{Message: "test error", Raw: `{"a": 1}`}
	if e.Error() != "test error" {
		t.Errorf("expected 'test error', got '%s'", e.Error())
	}
}
