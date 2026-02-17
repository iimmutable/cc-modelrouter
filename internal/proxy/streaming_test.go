package proxy

import (
	"testing"
)

func TestSSEWriter(t *testing.T) {
	writer := NewSSEWriter(nil) // nil for testing

	if writer == nil {
		t.Error("expected non-nil SSE writer")
	}
}

func TestParseSSEEvent(t *testing.T) {
	line := "event: content_block_delta\ndata: {\"type\":\"text\",\"text\":\"Hello\"}"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("failed to parse SSE event: %v", err)
	}

	if event != "content_block_delta" {
		t.Errorf("expected event 'content_block_delta', got '%s'", event)
	}

	if string(data) == "" {
		t.Error("expected non-empty data")
	}
}
