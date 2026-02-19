package anthropic

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshaling(t *testing.T) {
	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []Message{
			{
				Role:    "user",
				Content: MessageContent{{Type: "text", Text: "Hello"}},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.Model != req.Model {
		t.Errorf("expected model %s, got %s", req.Model, unmarshaled.Model)
	}
}

func TestContentBlockTypes(t *testing.T) {
	textBlock := ContentBlock{
		Type: "text",
		Text: "Hello",
	}

	imageBlock := ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "base64data",
		},
	}

	toolUseBlock := ContentBlock{
		Type:  "tool_use",
		ID:    "toolu_123",
		Name:  "get_weather",
		Input: json.RawMessage(`{"location": "SF"}`),
	}

	textData, _ := json.Marshal(textBlock)
	imageData, _ := json.Marshal(imageBlock)
	toolData, _ := json.Marshal(toolUseBlock)

	if string(textData) == "" {
		t.Error("text block should marshal to non-empty JSON")
	}
	if string(imageData) == "" {
		t.Error("image block should marshal to non-empty JSON")
	}
	if string(toolData) == "" {
		t.Error("tool_use block should marshal to non-empty JSON")
	}
}

func TestThinkingConfig(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected *ThinkingConfig
	}{
		{
			name:     "enabled with budget",
			json:     `{"type": "enabled", "budget_tokens": 10000}`,
			expected: &ThinkingConfig{Type: "enabled", BudgetTokens: 10000},
		},
		{
			name:     "enabled without budget",
			json:     `{"type": "enabled"}`,
			expected: &ThinkingConfig{Type: "enabled", BudgetTokens: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config ThinkingConfig
			if err := json.Unmarshal([]byte(tt.json), &config); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if config.Type != tt.expected.Type {
				t.Errorf("expected type %s, got %s", tt.expected.Type, config.Type)
			}
			if config.BudgetTokens != tt.expected.BudgetTokens {
				t.Errorf("expected budget_tokens %d, got %d", tt.expected.BudgetTokens, config.BudgetTokens)
			}
		})
	}
}

func TestRequestWithThinking(t *testing.T) {
	jsonStr := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 4096,
		"thinking": {
			"type": "enabled",
			"budget_tokens": 32000
		},
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`

	var req Request
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal request with thinking: %v", err)
	}

	if req.Thinking == nil {
		t.Fatal("expected thinking config to be non-nil")
	}
	if req.Thinking.Type != "enabled" {
		t.Errorf("expected thinking type 'enabled', got %s", req.Thinking.Type)
	}
	if req.Thinking.BudgetTokens != 32000 {
		t.Errorf("expected budget_tokens 32000, got %d", req.Thinking.BudgetTokens)
	}

	// Test marshaling back
	data, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.Thinking == nil {
		t.Fatal("expected thinking config to be preserved after marshal/unmarshal")
	}
	if unmarshaled.Thinking.BudgetTokens != 32000 {
		t.Errorf("expected budget_tokens to be preserved, got %d", unmarshaled.Thinking.BudgetTokens)
	}
}
