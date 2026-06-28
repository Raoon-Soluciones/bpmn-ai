package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client *anthropic.Client
}

func NewAnthropicProvider(apiKey, baseURL string) (Gateway, error) {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	return &AnthropicProvider{client: &client}, nil
}

func (p *AnthropicProvider) Generate(ctx context.Context, req Request) (Response, error) {
	req = req.WithDefaults()
	start := time.Now()

	params := anthropic.MessageNewParams{
		Model:       anthropic.Model(req.Model),
		MaxTokens:   int64(req.MaxTokens),
		Temperature: anthropic.Float(req.Temperature),
		Messages:    p.buildMessages(ctx, req),
		System:      p.buildSystem(req),
		Tools:       p.buildTools(req.Tools),
	}

	if len(req.OutputSchema) > 0 {
		var schemaMap map[string]any
		if err := json.Unmarshal(req.OutputSchema, &schemaMap); err == nil {
			params.OutputConfig = anthropic.OutputConfigParam{
				Format: anthropic.JSONOutputFormatParam{Schema: schemaMap},
			}
		}
	}

	message, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: %w", err)
	}

	text, toolCalls := p.parseContent(message.Content)

	if len(toolCalls) == 0 {
		return Response{
			Text:       text,
			Model:      string(message.Model),
			TokensIn:   int(message.Usage.InputTokens),
			TokensOut:  int(message.Usage.OutputTokens),
			DurationMs: int(time.Since(start).Milliseconds()),
		}, nil
	}

	return p.toolCallLoop(ctx, req, message, start)
}

func (p *AnthropicProvider) toolCallLoop(
	ctx context.Context,
	req Request,
	firstMessage *anthropic.Message,
	start time.Time,
) (Response, error) {
	messages := p.buildMessages(ctx, req)
	totalTokensIn := int(firstMessage.Usage.InputTokens)
	totalTokensOut := int(firstMessage.Usage.OutputTokens)

	messages = append(messages, anthropic.MessageParam{
		Role: anthropic.MessageParamRoleAssistant,
		Content: p.contentToParam(firstMessage.Content),
	})

	for round := 0; round < req.MaxToolRounds; round++ {
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range firstMessage.Content {
			if block.Type != "tool_use" {
				continue
			}
			toolUse := block.AsToolUse()
			def, found := p.findTool(req.Tools, toolUse.Name)
			if !found {
				return Response{}, fmt.Errorf("anthropic: tool %q not found", toolUse.Name)
			}

			result, err := def.Function(ctx, toolUse.Input)
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}

			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, false))
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))

		params := anthropic.MessageNewParams{
			Model:       anthropic.Model(req.Model),
			MaxTokens:   int64(req.MaxTokens),
			Temperature: anthropic.Float(req.Temperature),
			Messages:    messages,
			System:      p.buildSystem(req),
			Tools:       p.buildTools(req.Tools),
		}
		if len(req.OutputSchema) > 0 {
			var schemaMap map[string]any
			if err := json.Unmarshal(req.OutputSchema, &schemaMap); err == nil {
				params.OutputConfig = anthropic.OutputConfigParam{
					Format: anthropic.JSONOutputFormatParam{Schema: schemaMap},
				}
			}
		}
		message, err := p.client.Messages.New(ctx, params)
		if err != nil {
			return Response{}, fmt.Errorf("anthropic: tool round %d: %w", round+1, err)
		}

		totalTokensIn += int(message.Usage.InputTokens)
		totalTokensOut += int(message.Usage.OutputTokens)

		text, toolCalls := p.parseContent(message.Content)
		if len(toolCalls) == 0 {
			return Response{
				Text:       text,
				Model:      string(message.Model),
				TokensIn:   totalTokensIn,
				TokensOut:  totalTokensOut,
				DurationMs: int(time.Since(start).Milliseconds()),
			}, nil
		}

		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: p.contentToParam(message.Content),
		})
		firstMessage = message
	}

	return Response{}, fmt.Errorf("anthropic: exceeded max tool rounds (%d)", req.MaxToolRounds)
}

func (p *AnthropicProvider) buildSystem(req Request) []anthropic.TextBlockParam {
	if req.System == "" {
		return nil
	}
	return []anthropic.TextBlockParam{
		{Text: req.System, Type: "text"},
	}
}

func (p *AnthropicProvider) buildMessages(_ context.Context, req Request) []anthropic.MessageParam {
	var msgs []anthropic.MessageParam

	for _, m := range req.Messages {
		switch m.Role {
		case RoleAssistant:
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(m.Content)},
			})
		case RoleTool:
			continue
		default:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	return msgs
}

func (p *AnthropicProvider) buildTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	params := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		inputSchema := t.Parameters
		if inputSchema == nil {
			inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var schemaProps map[string]any
		_ = json.Unmarshal(inputSchema, &schemaProps)

		params = append(params, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type:       "object",
					Properties: schemaProps,
				},
			},
		})
	}
	return params
}

func (p *AnthropicProvider) parseContent(blocks []anthropic.ContentBlockUnion) (string, []ToolCall) {
	var text string
	var toolCalls []ToolCall

	for _, block := range blocks {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			text = v.Text
		case anthropic.ToolUseBlock:
			toolCalls = append(toolCalls, ToolCall{
				ID:        v.ID,
				Name:      v.Name,
				Arguments: v.Input,
			})
		}
	}

	return text, toolCalls
}

func (p *AnthropicProvider) contentToParam(blocks []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
	var params []anthropic.ContentBlockParamUnion
	for _, block := range blocks {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			params = append(params, anthropic.NewTextBlock(v.Text))
		case anthropic.ToolUseBlock:
			var input any
			if err := json.Unmarshal(v.Input, &input); err != nil {
				input = string(v.Input)
			}
			params = append(params, anthropic.NewToolUseBlock(v.ID, input, v.Name))
		}
	}
	return params
}

func (p *AnthropicProvider) findTool(tools []ToolDefinition, name string) (ToolDefinition, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return ToolDefinition{}, false
}
