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

func TestGeminiTransformerName(t *testing.T) {
	tr := NewGeminiTransformer()
	if tr.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got '%s'", tr.Name())
	}
}

func TestGeminiTransformRequest(t *testing.T) {
	tr := NewGeminiTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://generativelanguage.googleapis.com/v1beta", "test-key", "gemini-2.0-flash")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	// Gemini uses query param for API key
	if httpReq.URL.Query().Get("key") != "test-key" {
		t.Error("expected 'key' query parameter to be set")
	}

	// Verify the Gemini request structure
	var geminiReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&geminiReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	contents, ok := geminiReq["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Error("expected 'contents' array in request")
	}

	firstContent, ok := contents[0].(map[string]any)
	if !ok {
		t.Fatal("expected content to be an object")
	}

	if firstContent["role"] != "user" {
		t.Errorf("expected role 'user', got '%v'", firstContent["role"])
	}

	parts, ok := firstContent["parts"].([]any)
	if !ok || len(parts) == 0 {
		t.Error("expected 'parts' array in content")
	}

	firstPart, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatal("expected part to be an object")
	}

	if firstPart["text"] != "Hello" {
		t.Errorf("expected text 'Hello', got '%v'", firstPart["text"])
	}
}

func TestGeminiTransformRequestWithSystem(t *testing.T) {
	tr := NewGeminiTransformer()

	req := &anthropic.Request{
		Model:     "gemini-2.0-flash",
		MaxTokens: 4096,
		System:    json.RawMessage(`"You are a helpful assistant."`),
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://generativelanguage.googleapis.com/v1beta", "test-key", "gemini-2.0-flash")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	var geminiReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&geminiReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	systemInstruction, ok := geminiReq["systemInstruction"].(map[string]any)
	if !ok {
		t.Fatal("expected 'systemInstruction' in request")
	}

	parts, ok := systemInstruction["parts"].([]any)
	if !ok || len(parts) == 0 {
		t.Fatal("expected 'parts' in systemInstruction")
	}

	firstPart, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatal("expected part to be an object")
	}

	if firstPart["text"] != "You are a helpful assistant." {
		t.Errorf("expected system instruction text, got '%v'", firstPart["text"])
	}
}

func TestGeminiTransformRequestWithTools(t *testing.T) {
	tr := NewGeminiTransformer()

	req := &anthropic.Request{
		Model:    "gemini-2.0-flash",
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

	httpReq, err := tr.TransformRequest(req, "https://generativelanguage.googleapis.com/v1beta", "test-key", "gemini-2.0-flash")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	var geminiReq map[string]any
	if err := json.NewDecoder(httpReq.Body).Decode(&geminiReq); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	tools, ok := geminiReq["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("expected 'tools' array in request")
	}

	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatal("expected tool to be an object")
	}

	functionDeclarations, ok := tool["functionDeclarations"].([]any)
	if !ok || len(functionDeclarations) == 0 {
		t.Fatal("expected 'functionDeclarations' in tool")
	}

	funcDecl, ok := functionDeclarations[0].(map[string]any)
	if !ok {
		t.Fatal("expected function declaration to be an object")
	}

	if funcDecl["name"] != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%v'", funcDecl["name"])
	}
}

func TestGeminiTransformResponse(t *testing.T) {
	tr := NewGeminiTransformer()

	// Mock Gemini response
	geminiResponse := `{
		"candidates": [{
			"content": {
				"role": "model",
				"parts": [{"text": "Hello! How can I help you?"}]
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 20,
			"totalTokenCount": 30
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(geminiResponse))
	}))
	defer server.Close()

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(geminiResponse)),
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

func TestGeminiTransformResponseWithToolCall(t *testing.T) {
	tr := NewGeminiTransformer()

	geminiResponse := `{
		"candidates": [{
			"content": {
				"role": "model",
				"parts": [{
					"functionCall": {
						"name": "get_weather",
						"args": {"location": "San Francisco"}
					}
				}]
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"totalTokenCount": 15
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(geminiResponse)),
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
}

func TestGeminiSupportsStreaming(t *testing.T) {
	tr := NewGeminiTransformer()
	if !tr.SupportsStreaming() {
		t.Error("expected Gemini transformer to support streaming")
	}
}

func TestGeminiTransformStreamChunk(t *testing.T) {
	tr := NewGeminiTransformer()

	// Gemini streaming chunk format
	geminiChunk := `{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":"STOP"}]}`

	result, err := tr.TransformStreamChunk([]byte(geminiChunk), "message")
	if err != nil {
		t.Fatalf("failed to transform stream chunk: %v", err)
	}

	// Should produce Anthropic SSE format
	if !strings.Contains(string(result), `"type"`) {
		t.Errorf("expected Anthropic format in chunk, got '%s'", string(result))
	}
}
