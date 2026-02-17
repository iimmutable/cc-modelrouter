package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// AnthropicTransformer is a pass-through transformer for Anthropic-compatible APIs.
type AnthropicTransformer struct{}

// NewAnthropicTransformer creates a new Anthropic transformer.
func NewAnthropicTransformer() *AnthropicTransformer {
	return &AnthropicTransformer{}
}

// Name returns the transformer name.
func (t *AnthropicTransformer) Name() string {
	return "anthropic"
}

// TransformRequest creates an HTTP request for the Anthropic API.
func (t *AnthropicTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	// Copy request and override model
	reqCopy := *req
	reqCopy.Model = model

	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	return httpReq, nil
}

// TransformResponse converts the HTTP response to Anthropic format.
func (t *AnthropicTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result anthropic.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SupportsStreaming returns true.
func (t *AnthropicTransformer) SupportsStreaming() bool {
	return true
}

// TransformStreamChunk passes through the chunk unchanged.
func (t *AnthropicTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return chunk, nil
}
