package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type SubAgentTask struct {
	Name         string `json:"name"`
	Prompt       string `json:"prompt"`
	Model        string `json:"model,omitempty"`
	SystemPrompt string `json:"systemPrompt,omitempty"`
	OutputKey    string `json:"outputKey,omitempty"`
}

type MultiAgentExecutor struct {
	gateway Gateway
}

func NewMultiAgentExecutor(gateway Gateway) *MultiAgentExecutor {
	return &MultiAgentExecutor{gateway: gateway}
}

func (m *MultiAgentExecutor) Execute(ctx context.Context, agents []SubAgentTask, globalVars map[string]string) (map[string]string, error) {
	results := make(map[string]string)

	for _, agent := range agents {
		prompt := agent.Prompt
		for k, v := range globalVars {
			prompt = strings.ReplaceAll(prompt, "{{"+k+"}}", v)
		}
		for k, v := range results {
			prompt = strings.ReplaceAll(prompt, "{{"+k+"}}", v)
		}

		model := agent.Model
		if model == "" {
			model = "gpt-4o"
		}

		resp, err := m.gateway.Generate(ctx, Request{
			Model:    model,
			System:   agent.SystemPrompt,
			Messages: []Message{{Role: RoleUser, Content: prompt}},
		})
		if err != nil {
			results[agent.OutputKey] = fmt.Sprintf("error: %v", err)
			continue
		}

		results[agent.OutputKey] = resp.Text
	}

	return results, nil
}

func ParseAgents(data string) ([]SubAgentTask, error) {
	if data == "" {
		return nil, nil
	}
	var agents []SubAgentTask
	if err := json.Unmarshal([]byte(data), &agents); err != nil {
		return nil, fmt.Errorf("parse agents: %w", err)
	}
	return agents, nil
}
