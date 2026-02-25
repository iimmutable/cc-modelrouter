package transformers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iimmutable/cc-modelrouter/internal/logging"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// GLMAnthropicTransformer is a specialized transformer for GLM providers (aliyun, bigmodel)
// that ensures the signature field is present in thinking blocks.
//
// GLM providers (Aliyun DashScope, BigModel) require the signature field to be
// present in thinking blocks, even when empty. This is different from Anthropic's
// API which rejects empty signature values.
//
// This transformer wraps the standard AnthropicTransformer and adds provider-specific
// request sanitization before marshaling.
type GLMAnthropicTransformer struct {
	*AnthropicTransformer
}

// NewGLMAnthropicTransformer creates a new GLM-specific Anthropic transformer.
func NewGLMAnthropicTransformer() *GLMAnthropicTransformer {
	return &GLMAnthropicTransformer{
		AnthropicTransformer: NewAnthropicTransformer(),
	}
}

// Name returns the transformer name.
func (t *GLMAnthropicTransformer) Name() string {
	return "glm-anthropic"
}

// Endpoint returns the API endpoint path (same as Anthropic).
func (t *GLMAnthropicTransformer) Endpoint() string {
	return t.AnthropicTransformer.Endpoint()
}

// PrepareRequest converts Anthropic request to provider HTTP request with GLM-specific handling.
//
// For GLM providers, we ensure that:
// 1. The signature field is present in thinking blocks (even when empty)
// 2. Assistant messages with only thinking blocks have at least one text block
//
// CRITICAL: This method creates a true deep copy of the request to prevent state corruption
// across provider failover attempts. A shallow copy would cause modifications to thinking
// blocks to affect subsequent provider requests, leading to 400 Bad Request errors.
func (t *GLMAnthropicTransformer) PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	// Create a true deep copy using JSON marshal/unmarshal
	// This is necessary because shallow copying the Request struct shares the underlying
	// Message and ContentBlock arrays, causing state corruption when multiple providers
	// are attempted in sequence during failover.
	var reqCopy anthropic.Request
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request for deep copy: %w", err)
	}
	if err := json.Unmarshal(reqJSON, &reqCopy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request deep copy: %w", err)
	}
	reqCopy.Model = model

	// CRITICAL: Process in order to ensure correct content format
	// 1. First, convert user thinking blocks to text (prevents format issues)
	// 2. Then, normalize ALL single-element content (ensures multi-element arrays)
	// 3. Validate and repair blocks with missing required fields
	convertUserThinkingToText(&reqCopy)
	normalizeSingleElementContent(&reqCopy)
	validateAndRepairBlocks(&reqCopy)

	// GLM-specific: Ensure signature field is present for thinking blocks
	// We do this AFTER normalization to ensure signature is set
	// Note: With the pointer type, we can now set empty string to include the field
	// without using a single space workaround.
	for i := range reqCopy.Messages {
		for j := range reqCopy.Messages[i].Content {
			if reqCopy.Messages[i].Content[j].Type == "thinking" {
				// Set signature to empty string if nil (include field in JSON)
				if reqCopy.Messages[i].Content[j].Signature == nil {
					reqCopy.Messages[i].Content[j].Signature = strPtr("") // Include empty string
				}
			}
		}
	}

	// Marshal the request with GLM-specific handling
	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// DIAGNOSTIC: Log request body for debugging
	bodyStr := string(body)
	if len(bodyStr) > 2000 {
		logging.StreamDebugf("[PROXY REQUEST BODY] Model: %s, Body (first 2000 chars): %s...", model, bodyStr[:2000])
	} else {
		logging.StreamDebugf("[PROXY REQUEST BODY] Model: %s, Body: %s", model, bodyStr)
	}

	// Build endpoint
	endpoint := baseURL
	if !strings.HasSuffix(baseURL, "/v1/messages") {
		endpoint = baseURL + "/v1/messages"
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Set GetBody to enable retries
	// The HTTP client may retry the request if there's a network error.
	// Without GetBody, the body reader would be exhausted on retry causing
	// "http: ContentLength=X with Body length 0" errors.
	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)
	httpReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyCopy)), nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("User-Agent", "cc-modelrouter/1.0")
	httpReq.Header.Set("Accept", "application/json")

	return httpReq, nil
}

// ParseResponse converts provider HTTP response to Anthropic response (pass-through).
func (t *GLMAnthropicTransformer) ParseResponse(resp *http.Response) (*anthropic.Response, error) {
	return t.AnthropicTransformer.ParseResponse(resp)
}

// SupportsStreaming returns true (GLM supports streaming).
func (t *GLMAnthropicTransformer) SupportsStreaming() bool {
	return true
}

// TransformStreamEvent passes through Anthropic events unchanged.
func (t *GLMAnthropicTransformer) TransformStreamEvent(event *transformer.SSEEvent) ([]transformer.SSEEvent, error) {
	return t.AnthropicTransformer.TransformStreamEvent(event)
}
