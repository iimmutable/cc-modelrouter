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

// TransformStreamChunk transforms OpenRouter SSE to Anthropic format.
func (t *OpenRouterTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	// OpenRouter uses OpenAI-style streaming
	// For now, pass through - full implementation would convert format
	return chunk, nil
}
