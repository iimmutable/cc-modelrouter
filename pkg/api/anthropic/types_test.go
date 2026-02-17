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
