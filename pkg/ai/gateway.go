package ai

import (
	"context"
	"encoding/json"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
	Function    func(ctx context.Context, args json.RawMessage) (string, error) `json:"-"`
}

func (t ToolDefinition) WithoutFunction() ToolDefinition {
	return ToolDefinition{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}
}

type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Request struct {
	Model         string
	System        string
	Messages      []Message
	MaxTokens     int
	Temperature   float64
	Tools         []ToolDefinition
	MaxToolRounds int // max tool call loops (default 5)
	OutputSchema  json.RawMessage // JSON Schema for structured output (native JSON mode)
	Stream        bool            // enable token streaming
	OnChunk       func(chunk string) // called for each streamed token (when Stream=true)
}

type Response struct {
	Text       string
	Model      string
	TokensIn   int
	TokensOut  int
	DurationMs int
	ToolCalls  []ToolCall
}

type Gateway interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

func (r Request) WithDefaults() Request {
	if r.MaxTokens <= 0 {
		r.MaxTokens = 4096
	}
	if r.Temperature <= 0 {
		r.Temperature = 0.7
	}
	if r.MaxToolRounds <= 0 {
		r.MaxToolRounds = 5
	}
	return r
}

type CallLog struct {
	Model      string
	TokensIn   int
	TokensOut  int
	DurationMs int
	Success    bool
	Timestamp  time.Time
}
