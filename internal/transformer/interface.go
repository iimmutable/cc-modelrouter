// Package transformer defines the transformer interface for request/response transformation.
package transformer

import (
	"net/http"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// SSEEvent represents a complete server-sent event with type and data.
type SSEEvent struct {
	// EventType is the SSE event type (e.g., "message_start", "content_block_delta").
	EventType string
	// Data is the raw JSON data payload.
	Data []byte
}

// Transformer transforms requests and responses between Anthropic format and provider format.
type Transformer interface {
	// Name returns the transformer name.
	Name() string

	// TransformRequest converts an Anthropic request to a provider-specific HTTP request.
	TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

	// TransformResponse converts a provider response to Anthropic format.
	TransformResponse(resp *http.Response) (*anthropic.Response, error)

	// SupportsStreaming returns true if this transformer supports streaming.
	SupportsStreaming() bool

	// TransformSSEEvent transforms a provider SSE event to one or more Anthropic SSE events.
	// Returns a slice of SSE events in Anthropic format.
	TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error)

	// TransformStreamChunk transforms a streaming chunk to Anthropic format.
	// Deprecated: Use TransformSSEEvent for proper SSE event handling.
	TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
