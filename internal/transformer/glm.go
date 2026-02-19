package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// GLMTransformer is a pass-through transformer for Zhipu BigModel GLM API.
// GLM uses Anthropic-compatible API format.
type GLMTransformer struct{}

// NewGLMTransformer creates a new GLM transformer.
func NewGLMTransformer() *GLMTransformer {
	return &GLMTransformer{}
}

// Name returns the transformer name.
func (t *GLMTransformer) Name() string {
	return "glm"
}

// TransformRequest creates an HTTP request for the GLM API.
func (t *GLMTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	// Copy request and override model
	reqCopy := *req
	reqCopy.Model = model

	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// GLM uses Anthropic-compatible endpoint
	endpoint := strings.TrimSuffix(baseURL, "/") + "/v1/messages"

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// GLM uses JWT token with Bearer auth
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	return httpReq, nil
}

// TransformResponse converts the HTTP response to Anthropic format.
func (t *GLMTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
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
func (t *GLMTransformer) SupportsStreaming() bool {
	return true
}

// TransformStreamChunk passes through the chunk unchanged (Anthropic-compatible).
func (t *GLMTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return chunk, nil
}
