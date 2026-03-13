// Package anthropic defines the API types for Anthropic's Messages API.
// These types are used for request/response handling in the proxy.
package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
// Merges consecutive text blocks to ensure compatibility with providers
// that reject array format for text-only content (e.g., GLM/ZenZGA).
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	// Empty content should be an empty array, not null
	if len(mc) == 0 {
		return json.Marshal([]ContentBlock{})
	}

	// Build optimized content with merged consecutive text blocks
	var merged []ContentBlock
	var textBuffer strings.Builder

	for _, block := range mc {
		if block.Type == "text" {
			if block.Text != "" {
				// Accumulate consecutive text blocks
				textBuffer.WriteString(block.Text)
			}
			// Skip empty text blocks entirely
		} else {
			// Flush accumulated text before non-text block
			if textBuffer.Len() > 0 {
				merged = append(merged, ContentBlock{
					Type: "text",
					Text: textBuffer.String(),
				})
				textBuffer.Reset()
			}
			// Add non-text block (image, tool_result, thinking, etc.)
			merged = append(merged, block)
		}
	}

	// Flush any remaining accumulated text
	if textBuffer.Len() > 0 {
		merged = append(merged, ContentBlock{
			Type: "text",
			Text: textBuffer.String(),
		})
	}

	// Handle case where all blocks were empty text
	if len(merged) == 0 {
		return json.Marshal([]ContentBlock{})
	}

	// If single text block, marshal as string
	if len(merged) == 1 && merged[0].Type == "text" {
		return json.Marshal(merged[0].Text)
	}

	// Otherwise marshal as array
	return json.Marshal(merged)
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
	Type            string             `json:"type"`
	Text            string             `json:"text,omitempty"`
	Source          *ImageSource       `json:"source,omitempty"`
	DocumentSource  *DocumentSource    `json:"-"`             // For document blocks, serialized as "source"
	ID              string             `json:"id,omitempty"`
	Name            string             `json:"name,omitempty"`
	Input           json.RawMessage    `json:"input,omitempty"`
	Content         MessageContent     `json:"content,omitempty"`   // For tool_result content (string or array of content blocks)
	Thinking        string             `json:"thinking,omitempty"`  // For extended thinking content
	Signature       *string            `json:"signature,omitempty"` // For thinking signature (pointer distinguishes omit vs empty string)
	Data            string             `json:"-"`                   // For redacted_thinking blocks (custom marshal/unmarshal)
	IsError         bool               `json:"is_error,omitempty"`  // For tool_result errors
	Title           string             `json:"title,omitempty"`     // For document blocks
	Context         string             `json:"context,omitempty"`   // For document blocks
	Citations       *DocumentCitations `json:"citations,omitempty"` // For document blocks
}

// MarshalJSON implements custom marshaling for ContentBlock.
// For thinking blocks, it uses "thinking" and optionally "signature" fields.
// For tool_result blocks, it uses "tool_use_id" instead of "id" to comply with
// Anthropic's API specification. This is required for compatibility with
// providers like OpenRouter, GLM, and others that validate the schema strictly.
//
// Thinking blocks signature handling:
// - Empty signatures are omitted by default during JSON marshaling
// - Transformers (Anthropic, GLM) ensure signature is set to space before marshaling
//   to satisfy provider validation requirements (OpenRouter, GLM, etc.)
//
// Document blocks:
// - DocumentSource is serialized to the "source" field
// - Images use the Source field
func (cb ContentBlock) MarshalJSON() ([]byte, error) {
	// For document blocks, DocumentSource takes precedence and is serialized as "source"
	if cb.Type == "document" {
		type documentBlock struct {
			Type     string             `json:"type"`
			Source   *DocumentSource    `json:"source,omitempty"`
			Title    string             `json:"title,omitempty"`
			Context  string             `json:"context,omitempty"`
			Citations *DocumentCitations `json:"citations,omitempty"`
		}
		db := documentBlock{
			Type:      cb.Type,
			Source:    cb.DocumentSource,
			Title:     cb.Title,
			Context:   cb.Context,
			Citations: cb.Citations,
		}
		return json.Marshal(db)
	}

	// For thinking blocks, we need special handling to use "thinking" and optionally "signature" fields
	if cb.Type == "thinking" {
		// CRITICAL: Signature is now a pointer to distinguish between:
		// - nil (omit field) - for direct Anthropic API
		// - &"" (include empty string) - for OpenRouter Anthropic models
		// - &"value" (include value) - for actual signatures
		if cb.Signature == nil {
			// Omit signature field entirely (for direct Anthropic API)
			type thinkingBlockNoSig struct {
				Type     string `json:"type"`
				Thinking string `json:"thinking,omitempty"`
			}
			tb := thinkingBlockNoSig{
				Type:     cb.Type,
				Thinking: cb.Thinking,
			}
			return json.Marshal(tb)
		}

		// Include signature field (even if empty string) for OpenRouter and GLM
		// This ensures the field is present in JSON, satisfying provider validation
		type thinkingBlockWithSig struct {
			Type      string `json:"type"`
			Thinking  string `json:"thinking,omitempty"`
			Signature *string `json:"signature"` // Pointer allows empty string to be included
		}
		tb := thinkingBlockWithSig{
			Type:      cb.Type,
			Thinking:  cb.Thinking,
			Signature: cb.Signature,
		}
		return json.Marshal(tb)
	}

	// For tool_result blocks, we need special handling to use "tool_use_id" instead of "id"
	if cb.Type == "tool_result" {
		// Define a special struct for tool_result marshaling
		type toolResultBlock struct {
			Type       string      `json:"type"`
			ToolUseID  string      `json:"tool_use_id,omitempty"` // Note: tool_use_id not id
			Content    interface{} `json:"content,omitempty"`      // Can be string or array
			IsError    bool        `json:"is_error,omitempty"`
		}

		trb := toolResultBlock{
			Type:     cb.Type,
			ToolUseID: cb.ID, // Map ID to tool_use_id
			IsError:  cb.IsError,
		}

		// Serialize content appropriately:
		// - Single text block → string (for backward compatibility)
		// - Multiple/complex blocks → array
		if len(cb.Content) == 1 && cb.Content[0].Type == "text" {
			trb.Content = cb.Content[0].Text // String format
		} else if len(cb.Content) > 0 {
			trb.Content = []ContentBlock(cb.Content) // Array format
		}

		return json.Marshal(trb)
	}

	// For redacted_thinking blocks - preserve the data field
	if cb.Type == "redacted_thinking" {
		type redactedThinkingBlock struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		rtb := redactedThinkingBlock{
			Type: cb.Type,
			Data: cb.Data,
		}
		return json.Marshal(rtb)
	}

	// For all other block types, use standard marshaling
	// Define a standard block struct without the custom tool_use_id field
	type standardBlock struct {
		Type   string         `json:"type"`
		Text   string         `json:"text,omitempty"`
		Source *ImageSource   `json:"source,omitempty"`
		ID     string         `json:"id,omitempty"`
		Name   string         `json:"name,omitempty"`
		Input  json.RawMessage `json:"input,omitempty"`
		Content MessageContent `json:"content,omitempty"`
	}

	sb := standardBlock{
		Type:    cb.Type,
		Text:    cb.Text,
		Source:  cb.Source,
		ID:      cb.ID,
		Name:    cb.Name,
		Input:   cb.Input,
		Content: cb.Content,
	}
	return json.Marshal(sb)
}

// UnmarshalJSON implements custom unmarshaling for ContentBlock.
// It handles both "id" and "tool_use_id" fields for tool_result blocks.
// For document blocks, it unmarshals the "source" field into DocumentSource.
func (cb *ContentBlock) UnmarshalJSON(data []byte) error {
	// First, try to unmarshal into a map to detect the type
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Get the type
	typeData, ok := raw["type"]
	if !ok {
		return fmt.Errorf("missing type field in content block")
	}

	var blockType string
	if err := json.Unmarshal(typeData, &blockType); err != nil {
		return err
	}

	cb.Type = blockType

	// For document blocks, unmarshal source into DocumentSource
	if blockType == "document" {
		if sourceData, ok := raw["source"]; ok {
			cb.DocumentSource = &DocumentSource{}
			if err := json.Unmarshal(sourceData, cb.DocumentSource); err != nil {
				return err
			}
		}
		if titleData, ok := raw["title"]; ok {
			if err := json.Unmarshal(titleData, &cb.Title); err != nil {
				return err
			}
		}
		if contextData, ok := raw["context"]; ok {
			if err := json.Unmarshal(contextData, &cb.Context); err != nil {
				return err
			}
		}
		if citationsData, ok := raw["citations"]; ok {
			cb.Citations = &DocumentCitations{}
			if err := json.Unmarshal(citationsData, cb.Citations); err != nil {
				return err
			}
		}
		return nil
	}

	// For thinking blocks, read thinking and signature fields
	if blockType == "thinking" {
		if thinkingData, ok := raw["thinking"]; ok {
			if err := json.Unmarshal(thinkingData, &cb.Thinking); err != nil {
				return err
			}
		}
		// Also check for legacy "content" field for backward compatibility
		if contentData, ok := raw["content"]; ok && cb.Thinking == "" {
			if err := json.Unmarshal(contentData, &cb.Thinking); err != nil {
				return err
			}
		}
		// Handle signature field - if present, store as pointer
		if signatureData, ok := raw["signature"]; ok {
			var sig string
			if err := json.Unmarshal(signatureData, &sig); err != nil {
				return err
			}
			cb.Signature = &sig // Store as pointer (can be empty string)
		}
		// If signature field is missing, cb.Signature remains nil (field will be omitted)
		return nil
	}

	// For redacted_thinking blocks, read the data field
	if blockType == "redacted_thinking" {
		if dataField, ok := raw["data"]; ok {
			if err := json.Unmarshal(dataField, &cb.Data); err != nil {
				return err
			}
		}
		return nil
	}

	// For tool_result blocks, check for tool_use_id
	if blockType == "tool_result" {
		if toolUseIDData, ok := raw["tool_use_id"]; ok {
			if err := json.Unmarshal(toolUseIDData, &cb.ID); err != nil {
				return err
			}
		} else if idData, ok := raw["id"]; ok {
			// Fall back to "id" field for compatibility
			if err := json.Unmarshal(idData, &cb.ID); err != nil {
				return err
			}
		}

		if contentData, ok := raw["content"]; ok {
			if err := json.Unmarshal(contentData, &cb.Content); err != nil {
				return err
			}
		}

		if isErrorData, ok := raw["is_error"]; ok {
			if err := json.Unmarshal(isErrorData, &cb.IsError); err != nil {
				return err
			}
		}

		return nil
	}

	// For other block types, use standard unmarshaling
	if textData, ok := raw["text"]; ok {
		if err := json.Unmarshal(textData, &cb.Text); err != nil {
			return err
		}
	}

	if sourceData, ok := raw["source"]; ok {
		if err := json.Unmarshal(sourceData, &cb.Source); err != nil {
			return err
		}
	}

	if idData, ok := raw["id"]; ok {
		if err := json.Unmarshal(idData, &cb.ID); err != nil {
			return err
		}
	}

	if nameData, ok := raw["name"]; ok {
		if err := json.Unmarshal(nameData, &cb.Name); err != nil {
			return err
		}
	}

	if inputData, ok := raw["input"]; ok {
		cb.Input = inputData
	}

	if contentData, ok := raw["content"]; ok {
		if err := json.Unmarshal(contentData, &cb.Content); err != nil {
			return err
		}
	}

	return nil
}

// ImageSource represents the source of an image.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// DocumentSource represents the source of a document (PDF, text, etc.)
// for use with the Files API. Follows Anthropic's document block specification.
type DocumentSource struct {
	Type   string `json:"type"`   // "file" for file_id references
	FileID string `json:"file_id"` // ID of uploaded file
}

// DocumentCitations represents citation settings for document blocks.
type DocumentCitations struct {
	Enabled bool `json:"enabled"`
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
