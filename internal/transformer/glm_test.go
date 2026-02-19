package transformer

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestGLMTransformerName(t *testing.T) {
	tr := NewGLMTransformer()
	if tr.Name() != "glm" {
		t.Errorf("expected name 'glm', got '%s'", tr.Name())
	}
}

func TestGLMTransformRequest(t *testing.T) {
	tr := NewGLMTransformer()

	req := &anthropic.Request{
		Model:     "glm-4.7",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://open.bigmodel.cn/api/anthropic", "test-key", "glm-4.7")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	// GLM uses JWT token in Authorization header (Bearer)
	if httpReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("expected Authorization header with Bearer token")
	}

	// Should use /v1/messages endpoint (Anthropic-compatible)
	if !strings.HasSuffix(httpReq.URL.Path, "/v1/messages") {
		t.Errorf("expected path to end with /v1/messages, got %s", httpReq.URL.Path)
	}
}

func TestGLMTransformRequestWithSystem(t *testing.T) {
	tr := NewGLMTransformer()

	req := &anthropic.Request{
		Model:     "glm-4.7",
		MaxTokens: 4096,
		System:    []byte(`"You are a helpful assistant."`),
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://open.bigmodel.cn/api/anthropic", "test-key", "glm-4.7")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	// Request should be valid - just verify it doesn't error
	if httpReq == nil {
		t.Error("expected non-nil request")
	}
}

func TestGLMTransformRequestWithTools(t *testing.T) {
	tr := NewGLMTransformer()

	req := &anthropic.Request{
		Model:    "glm-4.7",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "What's the weather?"}}},
		},
		Tools: []anthropic.Tool{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://open.bigmodel.cn/api/anthropic", "test-key", "glm-4.7")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	// Request should be valid with tools
	if httpReq == nil {
		t.Error("expected non-nil request")
	}
}

func TestGLMTransformResponse(t *testing.T) {
	tr := NewGLMTransformer()

	// GLM response is Anthropic-compatible
	glmResponse := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello! How can I help you?"}],
		"model": "glm-4.7",
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20
		}
	}`

	httpResp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(glmResponse)),
	}

	resp, err := tr.TransformResponse(httpResp)
	if err != nil {
		t.Fatalf("failed to transform response: %v", err)
	}

	if resp.Role != anthropic.RoleAssistant {
		t.Errorf("expected role 'assistant', got '%s'", resp.Role)
	}

	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("expected content text, got '%v'", resp.Content)
	}

	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected input tokens 10, got %d", resp.Usage.InputTokens)
	}

	if resp.Usage.OutputTokens != 20 {
		t.Errorf("expected output tokens 20, got %d", resp.Usage.OutputTokens)
	}
}

func TestGLMTransformResponseWithToolUse(t *testing.T) {
	tr := NewGLMTransformer()

	glmResponse := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{
			"type": "tool_use",
			"id": "toolu_123",
			"name": "get_weather",
			"input": {"location": "San Francisco"}
		}],
		"model": "glm-4.7",
		"stop_reason": "tool_use",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5
		}
	}`

	httpResp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(glmResponse)),
	}

	resp, err := tr.TransformResponse(httpResp)
	if err != nil {
		t.Fatalf("failed to transform response: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("expected content in response")
	}

	if resp.Content[0].Type != "tool_use" {
		t.Errorf("expected content type 'tool_use', got '%s'", resp.Content[0].Type)
	}

	if resp.Content[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%s'", resp.Content[0].Name)
	}

	if resp.Content[0].ID != "toolu_123" {
		t.Errorf("expected tool ID 'toolu_123', got '%s'", resp.Content[0].ID)
	}
}

func TestGLMSupportsStreaming(t *testing.T) {
	tr := NewGLMTransformer()
	if !tr.SupportsStreaming() {
		t.Error("expected GLM transformer to support streaming")
	}
}

func TestGLMTransformStreamChunk(t *testing.T) {
	tr := NewGLMTransformer()

	// GLM uses Anthropic-compatible streaming format
	glmChunk := `{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}`

	result, err := tr.TransformStreamChunk([]byte(glmChunk), "content_block_delta")
	if err != nil {
		t.Fatalf("failed to transform stream chunk: %v", err)
	}

	// Should pass through unchanged
	if string(result) != glmChunk {
		t.Errorf("expected chunk to pass through unchanged, got '%s'", string(result))
	}
}

func TestGLMErrorResponse(t *testing.T) {
	tr := NewGLMTransformer()

	httpResp := &http.Response{
		StatusCode:    http.StatusUnauthorized,
		Status:        "401 Unauthorized",
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(`{"error": {"type": "authentication_error", "message": "Invalid API key"}}`)),
	}

	_, err := tr.TransformResponse(httpResp)
	if err == nil {
		t.Error("expected error for unauthorized response")
	}

	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("expected error to contain API key message, got: %v", err)
	}
}
