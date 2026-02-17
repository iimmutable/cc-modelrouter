package transformer

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestAnthropicTransformerName(t *testing.T) {
	tr := NewAnthropicTransformer()
	if tr.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got '%s'", tr.Name())
	}
}

func TestAnthropicTransformRequest(t *testing.T) {
	tr := NewAnthropicTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://api.anthropic.com", "test-key", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	if httpReq.Header.Get("x-api-key") != "test-key" {
		t.Error("expected x-api-key header to be set")
	}
}
