package transformer

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestOpenRouterTransformerName(t *testing.T) {
	tr := NewOpenRouterTransformer()
	if tr.Name() != "openrouter" {
		t.Errorf("expected name 'openrouter', got '%s'", tr.Name())
	}
}

func TestOpenRouterTransformRequest(t *testing.T) {
	tr := NewOpenRouterTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://openrouter.ai/api/v1", "test-key", "anthropic/claude-sonnet-4.5")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	if httpReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("expected Authorization header to be set")
	}
}
