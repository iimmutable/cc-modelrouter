// Package transformer defines the transformer interface for request/response transformation.
package transformer

import (
	"net/http"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

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

	// TransformStreamChunk transforms a streaming chunk to Anthropic format.
	TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
