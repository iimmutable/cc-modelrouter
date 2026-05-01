package transformers

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestTruncateToolNames(t *testing.T) {
	t.Run("short name unchanged", func(t *testing.T) {
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: "short_tool", InputSchema: map[string]interface{}{}},
			},
		}
		truncateToolNames(req)
		if req.Tools[0].Name != "short_tool" {
			t.Errorf("expected 'short_tool', got %q", req.Tools[0].Name)
		}
	})

	t.Run("exact 64 chars unchanged", func(t *testing.T) {
		name := make([]byte, 64)
		for i := range name {
			name[i] = 'a'
		}
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: string(name), InputSchema: map[string]interface{}{}},
			},
		}
		originalName := string(name)
		truncateToolNames(req)
		if req.Tools[0].Name != originalName {
			t.Errorf("expected name unchanged at 64 chars, got %q", req.Tools[0].Name)
		}
	})

	t.Run("65 chars truncated to 64", func(t *testing.T) {
		name := make([]byte, 65)
		for i := range name {
			name[i] = 'a'
		}
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: string(name), InputSchema: map[string]interface{}{}},
			},
		}
		truncateToolNames(req)
		if len(req.Tools[0].Name) != 64 {
			t.Errorf("expected 64 chars, got %d", len(req.Tools[0].Name))
		}
		// Should end with _XXXXXX (6-char hex)
		if req.Tools[0].Name[57] != '_' {
			t.Errorf("expected '_' at position 57, got %c", req.Tools[0].Name[57])
		}
	})

	t.Run("very long name truncated", func(t *testing.T) {
		name := "mcp__plugin_everything-claude-code_github__create_pull_request_review_long"
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: name, InputSchema: map[string]interface{}{}},
			},
		}
		truncateToolNames(req)
		if len(req.Tools[0].Name) != 64 {
			t.Errorf("expected 64 chars, got %d", len(req.Tools[0].Name))
		}
		// Prefix should be preserved
		expectedPrefix := name[:57]
		if req.Tools[0].Name[:57] != expectedPrefix {
			t.Errorf("prefix mismatch: expected %q, got %q", expectedPrefix, req.Tools[0].Name[:57])
		}
	})

	t.Run("no tools", func(t *testing.T) {
		req := &anthropic.Request{}
		truncateToolNames(req) // should not panic
	})

	t.Run("multiple tools", func(t *testing.T) {
		short := "short_name"
		long := make([]byte, 70)
		for i := range long {
			long[i] = 'b'
			}
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: short, InputSchema: map[string]interface{}{}},
				{Name: string(long), InputSchema: map[string]interface{}{}},
			},
		}
		truncateToolNames(req)
		if req.Tools[0].Name != short {
			t.Errorf("short name should be unchanged, got %q", req.Tools[0].Name)
		}
		if len(req.Tools[1].Name) != 64 {
			t.Errorf("long name should be 64 chars, got %d", len(req.Tools[1].Name))
		}
	})

	t.Run("same prefix produces same truncation", func(t *testing.T) {
		prefix := "very_long_tool_name_prefix_"
		name1 := prefix + "tool_a"
		name2 := prefix + "tool_b"
		req := &anthropic.Request{
			Tools: []anthropic.Tool{
				{Name: name1, InputSchema: map[string]interface{}{}},
				{Name: name2, InputSchema: map[string]interface{}{}},
			},
		}
		truncateToolNames(req)
			// Same prefix should produce different suffixes due to different hashes
		if req.Tools[0].Name == req.Tools[1].Name {
			t.Error("different tool names should produce different truncated names")
		}
	})
}
