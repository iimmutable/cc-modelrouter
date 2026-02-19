package transformer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestQwenTransformerName(t *testing.T) {
	tr := NewQwenTransformer()
	if tr.Name() != "qwen" {
		t.Errorf("expected name 'qwen', got '%s'", tr.Name())
	}
}

func TestQwenTransformRequest(t *testing.T) {
	tr := NewQwenTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://dashscope.aliyuncs.com/compatible-mode/v1", "test-key", "qwen-turbo")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	// Qwen uses Bearer token auth
	if httpReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("expected Authorization header with Bearer token")
	}

	// Verify OpenAI-compatible request structure
	var qwenReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&qwenReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if qwenReq["model"] != "qwen-turbo" {
		t.Errorf("expected model 'qwen-turbo', got '%v'", qwenReq["model"])
	}

	messages, ok := qwenReq["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Error("expected 'messages' array in request")
	}

	firstMsg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatal("expected message to be an object")
	}

	if firstMsg["role"] != "user" {
		t.Errorf("expected role 'user', got '%v'", firstMsg["role"])
	}

	if firstMsg["content"] != "Hello" {
		t.Errorf("expected content 'Hello', got '%v'", firstMsg["content"])
	}
}

func TestQwenTransformRequestWithSystem(t *testing.T) {
	tr := NewQwenTransformer()

	req := &anthropic.Request{
		Model:     "qwen-turbo",
		MaxTokens: 4096,
		System:    json.RawMessage(`"You are a helpful assistant."`),
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://dashscope.aliyuncs.com/compatible-mode/v1", "test-key", "qwen-turbo")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	var qwenReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&qwenReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	messages, ok := qwenReq["messages"].([]any)
	if !ok || len(messages) < 2 {
		t.Fatal("expected at least 2 messages (system + user)")
	}

	// First message should be system
	sysMsg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatal("expected system message to be an object")
	}

	if sysMsg["role"] != "system" {
		t.Errorf("expected first message role 'system', got '%v'", sysMsg["role"])
	}

	if sysMsg["content"] != "You are a helpful assistant." {
		t.Errorf("expected system content, got '%v'", sysMsg["content"])
	}
}

func TestQwenTransformRequestWithTools(t *testing.T) {
	tr := NewQwenTransformer()

	req := &anthropic.Request{
		Model:    "qwen-turbo",
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

	httpReq, err := tr.TransformRequest(req, "https://dashscope.aliyuncs.com/compatible-mode/v1", "test-key", "qwen-turbo")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	var qwenReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&qwenReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	tools, ok := qwenReq["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("expected 'tools' array in request")
	}

	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatal("expected tool to be an object")
	}

	if tool["type"] != "function" {
		t.Errorf("expected tool type 'function', got '%v'", tool["type"])
	}

	function, ok := tool["function"].(map[string]any)
	if !ok {
		t.Fatal("expected function in tool")
	}

	if function["name"] != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%v'", function["name"])
	}
}

func TestQwenTransformResponse(t *testing.T) {
	tr := NewQwenTransformer()

	// OpenAI-compatible response format
	qwenResponse := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "qwen-turbo",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Hello! How can I help you today?"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(qwenResponse)),
	}

	resp, err := tr.TransformResponse(httpResp)
	if err != nil {
		t.Fatalf("failed to transform response: %v", err)
	}

	if resp.Role != anthropic.RoleAssistant {
		t.Errorf("expected role 'assistant', got '%s'", resp.Role)
	}

	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello! How can I help you today?" {
		t.Errorf("expected content text, got '%v'", resp.Content)
	}

	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected input tokens 10, got %d", resp.Usage.InputTokens)
	}

	if resp.Usage.OutputTokens != 20 {
		t.Errorf("expected output tokens 20, got %d", resp.Usage.OutputTokens)
	}
}

func TestQwenTransformResponseWithToolCall(t *testing.T) {
	tr := NewQwenTransformer()

	qwenResponse := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "qwen-turbo",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_123",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"location\": \"San Francisco\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(qwenResponse)),
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

	if resp.Content[0].ID != "call_123" {
		t.Errorf("expected tool ID 'call_123', got '%s'", resp.Content[0].ID)
	}
}

func TestQwenSupportsStreaming(t *testing.T) {
	tr := NewQwenTransformer()
	if !tr.SupportsStreaming() {
		t.Error("expected Qwen transformer to support streaming")
	}
}

func TestQwenTransformStreamChunk(t *testing.T) {
	tr := NewQwenTransformer()

	// OpenAI-compatible streaming chunk format
	qwenChunk := `{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`

	result, err := tr.TransformStreamChunk([]byte(qwenChunk), "message")
	if err != nil {
		t.Fatalf("failed to transform stream chunk: %v", err)
	}

	// Should pass through OpenAI format for now
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestQwenErrorResponse(t *testing.T) {
	tr := NewQwenTransformer()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	// Use httptest to create a proper response with status text
	httpResp := httptest.NewRecorder().Result()
	httpResp.StatusCode = http.StatusUnauthorized
	httpResp.Status = "401 Unauthorized"
	httpResp.Body = io.NopCloser(strings.NewReader(`{"error": {"message": "Invalid API key"}}`))

	_, err := tr.TransformResponse(httpResp)
	if err == nil {
		t.Error("expected error for unauthorized response")
	}

	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("expected error to contain API key message, got: %v", err)
	}
}
