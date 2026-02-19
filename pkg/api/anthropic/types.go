// Package anthropic defines the API types for Anthropic's Messages API.
// These types are used for request/response handling in the proxy.
package anthropic

import "encoding/json"

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Request represents an Anthropic Messages API request.
type Request struct {
	Model      string          `json:"model"`
	MaxTokens  int             `json:"max_tokens"`
	Messages   []Message       `json:"messages"`
	System     json.RawMessage `json:"system,omitempty"`
	Tools      []Tool          `json:"tools,omitempty"`
	ToolChoice any             `json:"tool_choice,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Stream     bool            `json:"stream,omitempty"`
	Thinking   *ThinkingConfig `json:"thinking,omitempty"`
}

// ThinkingConfig represents the thinking configuration for extended thinking.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    Role           `json:"role"`
	Content MessageContent `json:"content"`
}

// MessageContent can be either a string or array of ContentBlocks.
type MessageContent []ContentBlock

// MarshalJSON implements custom marshaling for MessageContent.
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	// If single text block, marshal as string
	if len(mc) == 1 && mc[0].Type == "text" && mc[0].Text != "" {
		return json.Marshal(mc[0].Text)
	}
	// Otherwise marshal as array
	return json.Marshal([]ContentBlock(mc))
}

// UnmarshalJSON implements custom unmarshaling for MessageContent.
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// Try as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*mc = MessageContent{{Type: "text", Text: str}}
		return nil
	}

	// Try as array
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	*mc = blocks
	return nil
}

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text,omitempty"`
	Source  *ImageSource    `json:"source,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Content string          `json:"content,omitempty"`
}

// ImageSource represents the source of an image.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// Response represents an Anthropic Messages API response.
type Response struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage represents token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents a streaming event from the API.
type StreamEvent struct {
	Type         string        `json:"type"`
	Message      *Response     `json:"message,omitempty"`
	Index        int           `json:"index,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
	Delta        *StreamDelta  `json:"delta,omitempty"`
	Usage        *Usage        `json:"usage,omitempty"`
}

// StreamDelta represents a delta in a streaming response.
type StreamDelta struct {
	Type        string          `json:"type,omitempty"`
	Text        string          `json:"text,omitempty"`
	StopReason  string          `json:"stop_reason,omitempty"`
	ToolUseID   string          `json:"tool_use_id,omitempty"`
	PartialJSON json.RawMessage `json:"partial_json,omitempty"`
}
