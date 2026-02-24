package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// GeminiTransformer transforms requests to Google Gemini API format.
type GeminiTransformer struct{}

// NewGeminiTransformer creates a new Gemini transformer.
func NewGeminiTransformer() *GeminiTransformer {
	return &GeminiTransformer{}
}

// Name returns the transformer name.
func (t *GeminiTransformer) Name() string {
	return "gemini"
}

// GeminiRequest represents the Gemini API request format.
type GeminiRequest struct {
	Contents          []GeminiContent `json:"contents"`
	SystemInstruction *GeminiContent  `json:"systemInstruction,omitempty"`
	Tools             []GeminiTool    `json:"tools,omitempty"`
	GenerationConfig  any             `json:"generationConfig,omitempty"`
}

// GeminiContent represents a content item in Gemini format.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part in Gemini content.
type GeminiPart struct {
	Text         string            `json:"text,omitempty"`
	InlineData   *GeminiInlineData `json:"inlineData,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
}

// GeminiInlineData represents inline data (images) in Gemini.
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiFunctionCall represents a function call in Gemini.
type GeminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// GeminiTool represents a tool in Gemini format.
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDecl `json:"functionDeclarations,omitempty"`
}

// GeminiFunctionDecl represents a function declaration.
type GeminiFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// TransformRequest creates an HTTP request for the Gemini API.
func (t *GeminiTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	geminiReq := t.convertRequest(req)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Gemini uses models/{model}:generateContent endpoint with API key in query
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey))

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	return httpReq, nil
}

// convertRequest converts Anthropic request to Gemini format.
func (t *GeminiTransformer) convertRequest(req *anthropic.Request) *GeminiRequest {
	geminiReq := &GeminiRequest{}

	// Convert system prompt
	if len(req.System) > 0 {
		var systemText string
		// Try to parse as string first
		if err := json.Unmarshal(req.System, &systemText); err == nil {
			geminiReq.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{{Text: systemText}},
			}
		} else {
			// It's already a string in raw form, use it directly
			geminiReq.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{{Text: string(req.System)}},
			}
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		content := GeminiContent{
			Role:  t.convertRole(string(msg.Role)),
			Parts: t.convertContent(msg.Content),
		}
		geminiReq.Contents = append(geminiReq.Contents, content)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		tool := GeminiTool{}
		for _, t := range req.Tools {
			tool.FunctionDeclarations = append(tool.FunctionDeclarations, GeminiFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		geminiReq.Tools = []GeminiTool{tool}
	}

	// Add generation config for max_tokens
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig = map[string]any{
			"maxOutputTokens": req.MaxTokens,
		}
	}

	return geminiReq
}

// convertRole converts Anthropic role to Gemini role.
func (t *GeminiTransformer) convertRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	default:
		return role
	}
}

// convertContent converts Anthropic content blocks to Gemini parts.
func (t *GeminiTransformer) convertContent(content []anthropic.ContentBlock) []GeminiPart {
	var parts []GeminiPart
	for _, block := range content {
		switch block.Type {
		case "text":
			parts = append(parts, GeminiPart{Text: block.Text})
		case "image":
			if block.Source != nil {
				parts = append(parts, GeminiPart{
					InlineData: &GeminiInlineData{
						MimeType: block.Source.MediaType,
						Data:     block.Source.Data,
					},
				})
			}
		case "tool_use":
			// Convert tool use to function call
			var args map[string]any
			if len(block.Input) > 0 {
				json.Unmarshal(block.Input, &args)
			}
			parts = append(parts, GeminiPart{
				FunctionCall: &GeminiFunctionCall{
					Name: block.Name,
					Args: args,
				},
			})
		}
	}
	return parts
}

// GeminiResponse represents Gemini API response format.
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// GeminiCandidate represents a candidate in Gemini response.
type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason,omitempty"`
	SafetyRatings []any         `json:"safetyRatings,omitempty"`
}

// GeminiUsageMetadata represents usage information in Gemini.
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// TransformResponse converts Gemini response to Anthropic format.
func (t *GeminiTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return t.convertResponse(&geminiResp), nil
}

// convertResponse converts Gemini response to Anthropic format.
func (t *GeminiTransformer) convertResponse(geminiResp *GeminiResponse) *anthropic.Response {
	result := &anthropic.Response{
		Type:  "message",
		Role:  anthropic.RoleAssistant,
	}

	if geminiResp.UsageMetadata != nil {
		result.Usage = anthropic.Usage{
			InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		}
	}

	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				result.Content = append(result.Content, anthropic.ContentBlock{
					Type: "text",
					Text: part.Text,
				})
			} else if part.FunctionCall != nil {
				inputJSON, _ := json.Marshal(part.FunctionCall.Args)
				result.Content = append(result.Content, anthropic.ContentBlock{
					Type:  "tool_use",
					ID:    fmt.Sprintf("toolu_%s", part.FunctionCall.Name),
					Name:  part.FunctionCall.Name,
					Input: json.RawMessage(inputJSON),
				})
			}
		}

		// Map finish reason
		switch candidate.FinishReason {
		case "STOP":
			result.StopReason = "end_turn"
		case "MAX_TOKENS":
			result.StopReason = "max_tokens"
		default:
			result.StopReason = candidate.FinishReason
		}
	}

	return result
}

// SupportsStreaming returns true.
func (t *GeminiTransformer) SupportsStreaming() bool {
	return true
}

// TransformSSEEvent transforms Gemini SSE events to Anthropic format.
func (t *GeminiTransformer) TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error) {
	// Parse the Gemini chunk
	var geminiChunk GeminiResponse
	if err := json.Unmarshal(event.Data, &geminiChunk); err != nil {
		// If parsing fails, might already be Anthropic format
		return []SSEEvent{*event}, nil
	}

	var result []SSEEvent

	// Extract text from candidates
	if len(geminiChunk.Candidates) > 0 {
		candidate := geminiChunk.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					// Convert to Anthropic content_block_delta format
					anthropicChunk := map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]string{
							"type": "text_delta",
							"text": part.Text,
						},
					}
					data, err := json.Marshal(anthropicChunk)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
					}
					if len(data) == 0 {
						return nil, fmt.Errorf("content_block_delta event marshaled to empty JSON")
					}
					return []SSEEvent{
						{
							EventType: "content_block_delta",
							Data:      data,
						},
					}, nil
				}
			}
		}

		// Check for finish reason (end of stream)
		if candidate.FinishReason != "" && candidate.FinishReason != "IN_PROGRESS" {
			stopData, err := json.Marshal(map[string]any{
				"type":  "content_block_stop",
				"index": 0,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content_block_stop event: %w", err)
			}
			if len(stopData) == 0 {
				return nil, fmt.Errorf("content_block_stop event marshaled to empty JSON")
			}
			result = append(result, SSEEvent{
				EventType: "content_block_stop",
				Data:      stopData,
			})

			messageStopData, err := json.Marshal(map[string]string{
				"type": "message_stop",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal message_stop event: %w", err)
			}
			if len(messageStopData) == 0 {
				return nil, fmt.Errorf("message_stop event marshaled to empty JSON")
			}
			result = append(result, SSEEvent{
				EventType: "message_stop",
				Data:      messageStopData,
			})
			return result, nil
		}
	}

	// No content to return
	return result, nil
}

// TransformStreamChunk transforms Gemini SSE to Anthropic format.
// Deprecated: Use TransformSSEEvent for proper SSE event handling.
func (t *GeminiTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	// Parse the Gemini chunk
	var geminiChunk GeminiResponse
	if err := json.Unmarshal(chunk, &geminiChunk); err != nil {
		// If parsing fails, might already be Anthropic format
		return chunk, nil
	}

	// Extract text from candidates
	if len(geminiChunk.Candidates) > 0 {
		candidate := geminiChunk.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					// Convert to Anthropic content_block_delta format
					anthropicChunk := map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]string{
							"type": "text_delta",
							"text": part.Text,
						},
					}
					return json.Marshal(anthropicChunk)
				}
			}
		}

		// Check for finish reason (end of stream)
		if candidate.FinishReason != "" && candidate.FinishReason != "IN_PROGRESS" {
			return []byte(`{"type":"content_block_stop"}`), nil
		}
	}

	// No content to return
	return nil, nil
}
