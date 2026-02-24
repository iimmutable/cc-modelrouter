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

// QwenTransformer transforms requests to Alibaba Qwen API format.
// Qwen uses OpenAI-compatible chat completions format.
type QwenTransformer struct{}

// NewQwenTransformer creates a new Qwen transformer.
func NewQwenTransformer() *QwenTransformer {
	return &QwenTransformer{}
}

// Name returns the transformer name.
func (t *QwenTransformer) Name() string {
	return "qwen"
}

// QwenRequest represents the Qwen/OpenAI chat completion format.
type QwenRequest struct {
	Model       string        `json:"model"`
	Messages    []QwenMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Tools       []QwenTool    `json:"tools,omitempty"`
	ToolChoice  any           `json:"tool_choice,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// QwenMessage represents a message in Qwen/OpenAI format.
type QwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QwenTool represents a tool in Qwen/OpenAI format.
type QwenTool struct {
	Type     string        `json:"type"`
	Function QwenFunction  `json:"function"`
}

// QwenFunction represents a function definition.
type QwenFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

// TransformRequest creates an HTTP request for the Qwen API.
func (t *QwenTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	qwenReq := t.convertRequest(req, model)

	body, err := json.Marshal(qwenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Qwen uses /chat/completions endpoint (OpenAI-compatible)
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

	return httpReq, nil
}

// convertRequest converts Anthropic request to Qwen/OpenAI format.
func (t *QwenTransformer) convertRequest(req *anthropic.Request, model string) *QwenRequest {
	qwenReq := &QwenRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	// Convert system prompt as first message
	if len(req.System) > 0 {
		var systemText string
		if err := json.Unmarshal(req.System, &systemText); err == nil {
			qwenReq.Messages = append(qwenReq.Messages, QwenMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		content := t.extractTextContent(msg.Content)
		qwenReq.Messages = append(qwenReq.Messages, QwenMessage{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	// Convert tools
	for _, tool := range req.Tools {
		qwenTool := QwenTool{
			Type: "function",
			Function: QwenFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
		qwenReq.Tools = append(qwenReq.Tools, qwenTool)
	}

	return qwenReq
}

// extractTextContent extracts text from message content.
func (t *QwenTransformer) extractTextContent(content []anthropic.ContentBlock) string {
	var texts []string
	for _, block := range content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// QwenResponse represents Qwen/OpenAI response format.
type QwenResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// QwenToolCall represents a tool call in the response.
type QwenToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// TransformResponse converts Qwen response to Anthropic format.
func (t *QwenTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var qwenResp QwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&qwenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return t.convertResponse(&qwenResp), nil
}

// convertResponse converts Qwen/OpenAI response to Anthropic format.
func (t *QwenTransformer) convertResponse(qwenResp *QwenResponse) *anthropic.Response {
	result := &anthropic.Response{
		ID:    qwenResp.ID,
		Type:  "message",
		Role:  anthropic.RoleAssistant,
		Model: qwenResp.Model,
		Usage: anthropic.Usage{
			InputTokens:  qwenResp.Usage.PromptTokens,
			OutputTokens: qwenResp.Usage.CompletionTokens,
		},
	}

	for _, choice := range qwenResp.Choices {
		// Handle regular text content
		if choice.Message.Content != "" {
			result.Content = append(result.Content, anthropic.ContentBlock{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		// Handle tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			result.Content = append(result.Content, anthropic.ContentBlock{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: json.RawMessage(toolCall.Function.Arguments),
			})
		}

		// Map finish reason
		switch choice.FinishReason {
		case "stop":
			result.StopReason = "end_turn"
		case "tool_calls":
			result.StopReason = "tool_use"
		case "length":
			result.StopReason = "max_tokens"
		default:
			result.StopReason = choice.FinishReason
		}
	}

	return result
}

// SupportsStreaming returns true.
func (t *QwenTransformer) SupportsStreaming() bool {
	return true
}

// TransformSSEEvent transforms Qwen/OpenAI SSE events to Anthropic format.
func (t *QwenTransformer) TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error) {
	// Parse the OpenAI/Qwen chunk
	var openaiChunk map[string]any
	if err := json.Unmarshal(event.Data, &openaiChunk); err != nil {
		// If it's already Anthropic format, pass through
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

// TransformStreamChunk transforms Qwen/OpenAI SSE to Anthropic format.
// Deprecated: Use TransformSSEEvent for proper SSE event handling.
func (t *QwenTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	// Parse the OpenAI/Qwen chunk
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
