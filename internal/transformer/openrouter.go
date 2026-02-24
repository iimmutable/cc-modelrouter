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

// OpenRouterTransformer transforms requests to OpenRouter API format.
type OpenRouterTransformer struct{}

// NewOpenRouterTransformer creates a new OpenRouter transformer.
func NewOpenRouterTransformer() *OpenRouterTransformer {
	return &OpenRouterTransformer{}
}

// Name returns the transformer name.
func (t *OpenRouterTransformer) Name() string {
	return "openrouter"
}

// OpenRouterRequest represents the OpenRouter chat completion format.
type OpenRouterRequest struct {
	Model       string           `json:"model"`
	Messages    []OpenRouterMsg  `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	Tools       []OpenRouterTool `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
}

// OpenRouterMsg represents a message in OpenRouter format.
type OpenRouterMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenRouterTool represents a tool in OpenRouter format.
type OpenRouterTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// TransformRequest creates an HTTP request for the OpenRouter API.
func (t *OpenRouterTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	orReq := t.convertRequest(req, model)

	body, err := json.Marshal(orReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// OpenRouter uses /chat/completions endpoint
	endpoint := baseURL
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		endpoint = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/iimmutable/cc-modelrouter")

	return httpReq, nil
}

// convertRequest converts Anthropic request to OpenRouter format.
func (t *OpenRouterTransformer) convertRequest(req *anthropic.Request, model string) *OpenRouterRequest {
	orReq := &OpenRouterRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	// Convert messages
	for _, msg := range req.Messages {
		content := t.extractTextContent(msg.Content)
		orReq.Messages = append(orReq.Messages, OpenRouterMsg{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	// Convert tools
	for _, tool := range req.Tools {
		orTool := OpenRouterTool{
			Type: "function",
		}
		orTool.Function.Name = tool.Name
		orTool.Function.Description = tool.Description
		orTool.Function.Parameters = tool.InputSchema
		orReq.Tools = append(orReq.Tools, orTool)
	}

	return orReq
}

// extractTextContent extracts text from message content.
func (t *OpenRouterTransformer) extractTextContent(content anthropic.MessageContent) string {
	var texts []string
	for _, block := range content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// OpenRouterResponse represents OpenRouter response format.
type OpenRouterResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// TransformResponse converts OpenRouter response to Anthropic format.
func (t *OpenRouterTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var orResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return t.convertResponse(&orResp), nil
}

// convertResponse converts OpenRouter response to Anthropic format.
func (t *OpenRouterTransformer) convertResponse(orResp *OpenRouterResponse) *anthropic.Response {
	result := &anthropic.Response{
		ID:    orResp.ID,
		Type:  "message",
		Role:  anthropic.RoleAssistant,
		Model: orResp.Model,
		Usage: anthropic.Usage{
			InputTokens:  orResp.Usage.PromptTokens,
			OutputTokens: orResp.Usage.CompletionTokens,
		},
	}

	for _, choice := range orResp.Choices {
		result.Content = append(result.Content, anthropic.ContentBlock{
			Type: "text",
			Text: choice.Message.Content,
		})
		result.StopReason = choice.FinishReason
	}

	return result
}

// SupportsStreaming returns true.
func (t *OpenRouterTransformer) SupportsStreaming() bool {
	return true
}

// TransformSSEEvent transforms OpenAI/OpenRouter SSE events to Anthropic format.
// OpenAI format uses delta.content, while Anthropic uses content_block_delta.
func (t *OpenRouterTransformer) TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error) {
	// Parse the OpenAI/OpenRouter chunk
	var openaiChunk map[string]any
	if err := json.Unmarshal(event.Data, &openaiChunk); err != nil {
		// If parsing fails, pass through unchanged
		return []SSEEvent{*event}, nil
	}

	var result []SSEEvent

	// Check for finish_reason to detect stream completion
	if openaiChunk["choices"] != nil {
		choices, ok := openaiChunk["choices"].([]any)
		if ok && len(choices) > 0 {
			choice, ok := choices[0].(map[string]any)
			if ok {
				finishReason := choice["finish_reason"]
				if finishReason != nil && finishReason != "" {
					// Stream finished - emit content_block_stop event
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

					// Also emit message_stop event
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
		}
	}

	// Extract content from delta
	if openaiChunk["choices"] != nil {
		choices, ok := openaiChunk["choices"].([]any)
		if ok && len(choices) > 0 {
			choice, ok := choices[0].(map[string]any)
			if ok {
				delta, ok := choice["delta"].(map[string]any)
				if ok {
					content, hasContent := delta["content"].(string)
					if hasContent && content != "" {
						// Convert to Anthropic content_block_delta format
						anthropicDelta := map[string]any{
							"type":  "content_block_delta",
							"index": 0,
							"delta": map[string]string{
								"type": "text_delta",
								"text": content,
							},
						}
						data, err := json.Marshal(anthropicDelta)
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
		}
	}

	// Pass through unknown chunks unchanged
	return []SSEEvent{*event}, nil
}

// TransformStreamChunk transforms OpenRouter SSE to Anthropic format.
// Deprecated: Use TransformSSEEvent for proper SSE event handling.
func (t *OpenRouterTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	// Parse the OpenAI/OpenRouter chunk
	var openaiChunk map[string]any
	if err := json.Unmarshal(chunk, &openaiChunk); err != nil {
		// If it's already Anthropic format, pass through
		return chunk, nil
	}

	// Check for done signal
	if openaiChunk["choices"] != nil {
		choices, ok := openaiChunk["choices"].([]any)
		if ok && len(choices) > 0 {
			choice, ok := choices[0].(map[string]any)
			if ok {
				finishReason := choice["finish_reason"]
				if finishReason != nil {
					// Stream finished - return content_block_stop
					return []byte(`{"type":"content_block_stop"}`), nil
				}
			}
		}
	}

	// Extract content from delta
	if openaiChunk["choices"] != nil {
		choices, ok := openaiChunk["choices"].([]any)
		if ok && len(choices) > 0 {
			choice, ok := choices[0].(map[string]any)
			if ok {
				delta, ok := choice["delta"].(map[string]any)
				if ok {
					content, hasContent := delta["content"].(string)
					if hasContent && content != "" {
						// Convert to Anthropic content_block_delta format
						anthropicChunk := map[string]any{
							"type":  "content_block_delta",
							"index": 0,
							"delta": map[string]string{
								"type": "text_delta",
								"text": content,
							},
						}
						return json.Marshal(anthropicChunk)
					}
				}
			}
		}
	}

	// Pass through unknown chunks
	return chunk, nil
}
