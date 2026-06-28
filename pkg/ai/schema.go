package ai

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

type SchemaValidator struct{}

func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

func (v *SchemaValidator) Validate(schemaJSON json.RawMessage, data string) (map[string]any, error) {
	var parsedData any
	if err := json.Unmarshal([]byte(data), &parsedData); err != nil {
		return nil, &ValidationError{
			Message: fmt.Sprintf("response is not valid JSON: %v", err),
			Raw:     data,
		}
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	documentLoader := gojsonschema.NewGoLoader(parsedData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		errs := make([]string, 0, len(result.Errors()))
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return nil, &ValidationError{
			Message: fmt.Sprintf("schema validation failed: %v", errs),
			Raw:     data,
		}
	}

	parsedMap, ok := parsedData.(map[string]any)
	if !ok {
		return nil, &ValidationError{
			Message: "parsed JSON is not an object",
			Raw:     data,
		}
	}

	return parsedMap, nil
}

type ValidationError struct {
	Message string
	Raw     string
}

func (e *ValidationError) Error() string {
	return e.Message
}
