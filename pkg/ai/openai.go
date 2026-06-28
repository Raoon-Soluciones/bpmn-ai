package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goopenai "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *goopenai.Client
}

func NewOpenAIProvider(apiKey, baseURL string) (Gateway, error) {
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIProvider{
		client: goopenai.NewClientWithConfig(cfg),
	}, nil
}

func (p *OpenAIProvider) Generate(ctx context.Context, req Request) (Response, error) {
	req = req.WithDefaults()
	start := time.Now()

	if req.Stream {
		return p.generateStream(ctx, req, start)
	}

	openaiTools := p.buildTools(req.Tools)
	messages := p.buildMessages(req)

	chatReq := goopenai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Tools:       openaiTools,
	}

	if len(req.OutputSchema) > 0 {
		chatReq.ResponseFormat = &goopenai.ChatCompletionResponseFormat{
			Type: goopenai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &goopenai.ChatCompletionResponseFormatJSONSchema{
				Name:   "response",
				Schema: req.OutputSchema,
				Strict: true,
			},
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: no choices returned")
	}

	msg := resp.Choices[0].Message
	usage := resp.Usage

	// Single round, no tool calls
	if len(msg.ToolCalls) == 0 {
		return Response{
			Text:       msg.Content,
			Model:      resp.Model,
			TokensIn:   usage.PromptTokens,
			TokensOut:  usage.CompletionTokens,
			DurationMs: int(time.Since(start).Milliseconds()),
		}, nil
	}

	// Tool calling loop
	return p.toolCallLoop(ctx, req, messages, msg, usage, openaiTools, start)
}

func (p *OpenAIProvider) toolCallLoop(
	ctx context.Context,
	req Request,
	messages []goopenai.ChatCompletionMessage,
	firstMsg goopenai.ChatCompletionMessage,
	usage goopenai.Usage,
	openaiTools []goopenai.Tool,
	start time.Time,
) (Response, error) {

	messages = append(messages, firstMsg)
	totalTokensIn := usage.PromptTokens
	totalTokensOut := usage.CompletionTokens

	for round := 0; round < req.MaxToolRounds; round++ {
		for _, tc := range firstMsg.ToolCalls {
			def, found := p.findTool(req.Tools, tc.Function.Name)
			if !found {
				return Response{}, fmt.Errorf("openai: tool %q not found in request tools", tc.Function.Name)
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return Response{}, fmt.Errorf("openai: parse tool args: %w", err)
			}

			result, err := def.Function(ctx, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}

			messages = append(messages, goopenai.ChatCompletionMessage{
				Role:       goopenai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// Send follow-up request
		chatReq := goopenai.ChatCompletionRequest{
			Model:       req.Model,
			Messages:    messages,
			MaxTokens:   req.MaxTokens,
			Temperature: float32(req.Temperature),
			Tools:       openaiTools,
		}
		if len(req.OutputSchema) > 0 {
			chatReq.ResponseFormat = &goopenai.ChatCompletionResponseFormat{
				Type: goopenai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &goopenai.ChatCompletionResponseFormatJSONSchema{
					Name:   "response",
					Schema: req.OutputSchema,
					Strict: true,
				},
			}
		}
		resp, err := p.client.CreateChatCompletion(ctx, chatReq)
		if err != nil {
			return Response{}, fmt.Errorf("openai: tool round %d: %w", round+1, err)
		}
		if len(resp.Choices) == 0 {
			return Response{}, fmt.Errorf("openai: tool round %d: no choices", round+1)
		}

		msg := resp.Choices[0].Message
		totalTokensIn += resp.Usage.PromptTokens
		totalTokensOut += resp.Usage.CompletionTokens

		// No more tool calls — final response
		if len(msg.ToolCalls) == 0 {
			return Response{
				Text:       msg.Content,
				Model:      resp.Model,
				TokensIn:   totalTokensIn,
				TokensOut:  totalTokensOut,
				DurationMs: int(time.Since(start).Milliseconds()),
			}, nil
		}

		// Continue loop with more tool calls
		messages = append(messages, msg)
		firstMsg = msg
	}

	return Response{}, fmt.Errorf("openai: exceeded max tool rounds (%d)", req.MaxToolRounds)
}

func (p *OpenAIProvider) findTool(tools []ToolDefinition, name string) (ToolDefinition, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return ToolDefinition{}, false
}

func (p *OpenAIProvider) buildMessages(req Request) []goopenai.ChatCompletionMessage {
	var msgs []goopenai.ChatCompletionMessage

	if req.System != "" {
		msgs = append(msgs, goopenai.ChatCompletionMessage{
			Role:    goopenai.ChatMessageRoleSystem,
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		role := string(m.Role)
		switch role {
		case "system":
			role = goopenai.ChatMessageRoleSystem
		case "assistant":
			role = goopenai.ChatMessageRoleAssistant
		case "tool":
			role = goopenai.ChatMessageRoleTool
		default:
			role = goopenai.ChatMessageRoleUser
		}
		msg := goopenai.ChatCompletionMessage{
			Role:    role,
			Content: m.Content,
		}
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}
		msgs = append(msgs, msg)
	}

	return msgs
}

func (p *OpenAIProvider) buildTools(tools []ToolDefinition) []goopenai.Tool {
	if len(tools) == 0 {
		return nil
	}
	openaiTools := make([]goopenai.Tool, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		openaiTools = append(openaiTools, goopenai.Tool{
			Type: goopenai.ToolTypeFunction,
			Function: &goopenai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return openaiTools
}

func (p *OpenAIProvider) generateStream(ctx context.Context, req Request, start time.Time) (Response, error) {
	openaiTools := p.buildTools(req.Tools)
	messages := p.buildMessages(req)

	stream, err := p.client.CreateChatCompletionStream(ctx, goopenai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Tools:       openaiTools,
	})
	if err != nil {
		return Response{}, fmt.Errorf("openai stream: %w", err)
	}
	defer stream.Close()

	var fullText strings.Builder
	var model string
	var tokensIn, tokensOut int

	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			fullText.WriteString(delta.Content)
			if req.OnChunk != nil && delta.Content != "" {
				req.OnChunk(delta.Content)
			}
			if model == "" {
				model = chunk.Model
			}
		}
		tokensIn = chunk.Usage.PromptTokens
		tokensOut = chunk.Usage.CompletionTokens
	}

	return Response{
		Text:       fullText.String(),
		Model:      model,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		DurationMs: int(time.Since(start).Milliseconds()),
	}, nil
}
