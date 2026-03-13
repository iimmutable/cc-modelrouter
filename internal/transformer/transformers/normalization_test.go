package transformers

import (
	"encoding/json"
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// TestNormalizeAssistantMessages_SingleThinkingBlock tests that the normalization
// function adds an empty text block to assistant messages with only a thinking block.
func TestNormalizeAssistantMessages_SingleThinkingBlock(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role:    "assistant",
				Content: anthropic.MessageContent{{Type: "thinking", Thinking: "Deep thinking..."}},
			},
		},
	}

	t.Logf("Before normalization:")
	t.Logf("  messages[1].content length: %d", len(req.Messages[1].Content))

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	t.Logf("After normalization:")
	t.Logf("  messages[1].content length: %d", len(req.Messages[1].Content))

	// Verify the assistant message now has 2 content blocks
	if len(req.Messages[1].Content) != 2 {
		t.Errorf("Expected 2 content blocks after normalization, got %d", len(req.Messages[1].Content))
		return
	}

	// Verify the second block is a text block with single space
	if req.Messages[1].Content[1].Type != "text" {
		t.Errorf("Expected second block to be text type, got %s", req.Messages[1].Content[1].Type)
	}
	if req.Messages[1].Content[1].Text != " " {
		t.Errorf("Expected second block to have single space text, got %q", req.Messages[1].Content[1].Text)
	}

	t.Logf("SUCCESS: Normalization added text block with single space")
	t.Logf("  First block: type=%s, thinking=%d chars", req.Messages[1].Content[0].Type, len(req.Messages[1].Content[0].Thinking))
	t.Logf("  Second block: type=%s, text=%q", req.Messages[1].Content[1].Type, req.Messages[1].Content[1].Text)
}

// TestNormalizeAssistantMessages_ThinkingPlusText_NoChange tests that messages
// with thinking plus text blocks are not modified.
func TestNormalizeAssistantMessages_ThinkingPlusText_NoChange(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{Type: "thinking", Thinking: "Thinking..."},
					{Type: "text", Text: "Response"},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	// Content should not have been modified (already has 2 blocks)
	if len(req.Messages[1].Content) != originalContentLen {
		t.Errorf("Expected content length to remain %d, got %d", originalContentLen, len(req.Messages[1].Content))
	} else {
		t.Logf("SUCCESS: Thinking + text message not modified (already multi-element)")
	}
}

// TestConvertUserThinkingToText_SingleThinkingBlock tests that user messages
// with thinking blocks have them converted to text blocks to prevent
// "expected string, received array" errors with providers like OpenRouter.
func TestConvertUserThinkingToText_SingleThinkingBlock(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "thinking", Thinking: "User thinking..."}},
			},
		},
	}

	originalContentLen := len(req.Messages[0].Content)

	t.Logf("Before conversion:")
	t.Logf("  messages[0].content length: %d", originalContentLen)
	t.Logf("  messages[0].content[0].type: %s", req.Messages[0].Content[0].Type)

	// Convert user thinking blocks to text
	convertUserThinkingToText(req)

	t.Logf("After conversion:")
	t.Logf("  messages[0].content length: %d", len(req.Messages[0].Content))

	// User message with thinking block should be converted to text block
	if len(req.Messages[0].Content) != 1 {
		t.Errorf("Expected user message content length to be 1, got %d", len(req.Messages[0].Content))
		return
	}

	// Verify the block is now a text block with the thinking content wrapped in tags
	if req.Messages[0].Content[0].Type != "text" {
		t.Errorf("Expected block to be text type, got %s", req.Messages[0].Content[0].Type)
	}
	expectedText := "<thinking>User thinking...</thinking>"
	if req.Messages[0].Content[0].Text != expectedText {
		t.Errorf("Expected text to be %q, got %q", expectedText, req.Messages[0].Content[0].Text)
	}

	t.Logf("SUCCESS: User thinking block converted to text block with <thinking> tags")
}

// TestNormalizeThinkingBlockMessages_UserMessage_Normalized tests that user messages
// with single non-text blocks ARE now normalized by normalizeThinkingBlockMessages.
// This was changed to fix OpenRouter validation errors.
//
// Previously, user messages were not normalized, causing "expected string, received array"
// errors when user messages contained single thinking blocks from conversation history.
func TestNormalizeThinkingBlockMessages_UserMessage_Normalized(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "thinking", Thinking: "User thinking..."}},
			},
		},
	}

	originalContentLen := len(req.Messages[0].Content)

	t.Logf("Before normalization:")
	t.Logf("  messages[0].content length: %d", originalContentLen)

	// Normalize the request (NOW normalizes user messages too)
	normalizeThinkingBlockMessages(req)

	t.Logf("After normalization:")
	t.Logf("  messages[0].content length: %d", len(req.Messages[0].Content))

	// User message with single thinking block should NOW be normalized to multi-element array
	if len(req.Messages[0].Content) == 1 {
		t.Errorf("Expected user message to be normalized to multi-element array, content length still %d", len(req.Messages[0].Content))
	}

	// Verify text block was added
	hasText := false
	for _, block := range req.Messages[0].Content {
		if block.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText {
		t.Errorf("Expected text block to be added to user message")
	}

	t.Logf("SUCCESS: User message with single thinking block now normalized to multi-element array")
}

// TestNormalizeAssistantMessages_SingleTextBlock_NoChange tests that messages
// with only a text block are not modified.
func TestNormalizeAssistantMessages_SingleTextBlock_NoChange(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role:    "assistant",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hi there!"}},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	// Single text block message should not be modified
	// (Single text blocks are marshaled as strings by MessageContent.MarshalJSON)
	if len(req.Messages[1].Content) != originalContentLen {
		t.Errorf("Expected content length to remain %d, got %d", originalContentLen, len(req.Messages[1].Content))
	} else {
		t.Logf("SUCCESS: Single text block message not modified")
	}

	// Verify it marshals as a string
	data, _ := json.Marshal(req.Messages[1].Content)
	if string(data) == `"[{\"type\":\"text\",\"text\":\"Hi there!\"}]"` {
		t.Error("Expected single text block to marshal as string, not array")
	}
}

// TestNormalizeAssistantMessages_MultipleThinkingBlocks_AddText tests that messages
// with multiple thinking blocks get a text block added to make it a multi-element array.
// This is required for OpenRouter validation which rejects content arrays with only thinking blocks.
func TestNormalizeAssistantMessages_MultipleThinkingBlocks_AddText(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{Type: "thinking", Thinking: "First thought..."},
					{Type: "thinking", Thinking: "Second thought..."},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	// Multiple thinking blocks SHOULD now be normalized (fixed March 2026)
	// The function should add a text block to make it multi-element
	if len(req.Messages[1].Content) != originalContentLen+1 {
		t.Errorf("Expected content length to be %d after adding text block, got %d",
			originalContentLen+1, len(req.Messages[1].Content))
		return
	}

	// Verify the last block is a text block with single space
	lastBlock := req.Messages[1].Content[len(req.Messages[1].Content)-1]
	if lastBlock.Type != "text" {
		t.Errorf("Expected last block to be text, got %s", lastBlock.Type)
	}
	if lastBlock.Text != " " {
		t.Errorf("Expected text block to be single space, got %q", lastBlock.Text)
	}

	t.Logf("SUCCESS: Multiple thinking blocks message normalized with text block")
}

// TestNormalizeAssistantMessages_ThinkingAfterText_NoChange tests that messages
// with text blocks followed by thinking blocks are NOT modified (already has text).
func TestNormalizeAssistantMessages_ThinkingAfterText_NoChange(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{Type: "text", Text: "Hello!"},
					{Type: "thinking", Thinking: "Processing request..."},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	// Already has text block, should NOT add another
	if len(req.Messages[1].Content) != originalContentLen {
		t.Errorf("Expected content length to remain %d (already has text), got %d",
			originalContentLen, len(req.Messages[1].Content))
	} else {
		t.Logf("SUCCESS: Message with text+thinking not modified (already has text)")
	}
}

// TestNormalizeAssistantMessages_ThreeThinkingBlocks_AddText tests that messages
// with three or more thinking blocks get a text block added.
func TestNormalizeAssistantMessages_ThreeThinkingBlocks_AddText(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{Type: "thinking", Thinking: "Thought 1..."},
					{Type: "thinking", Thinking: "Thought 2..."},
					{Type: "thinking", Thinking: "Thought 3..."},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeThinkingBlockMessages(req)

	// Three thinking blocks SHOULD get a text block added
	if len(req.Messages[1].Content) != originalContentLen+1 {
		t.Errorf("Expected content length to be %d after adding text block, got %d",
			originalContentLen+1, len(req.Messages[1].Content))
		return
	}

	// Verify the last block is a text block
	lastBlock := req.Messages[1].Content[len(req.Messages[1].Content)-1]
	if lastBlock.Type != "text" {
		t.Errorf("Expected last block to be text, got %s", lastBlock.Type)
	}

	t.Logf("SUCCESS: Three thinking blocks normalized with text block")
}

// TestNormalizeAssistantMessages_EmptyContent_NoChange tests that messages
// with empty content are not modified.
func TestNormalizeAssistantMessages_EmptyContent_NoChange(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "assistant",
				Content: anthropic.MessageContent{},
			},
		},
	}

	originalContentLen := len(req.Messages[0].Content)

	// Normalize the request
	normalizeSingleElementContent(req)

	// Empty content should not be modified
	if len(req.Messages[0].Content) != originalContentLen {
		t.Errorf("Expected content length to remain %d, got %d", originalContentLen, len(req.Messages[0].Content))
	} else {
		t.Logf("SUCCESS: Empty content message not modified")
	}
}

// ============================================
// New Tests for Comprehensive Normalization
// ============================================

// TestNormalizeSingleElementContent_SingleImageBlock tests that single image blocks
// get a text block added to prevent provider validation errors.
func TestNormalizeSingleElementContent_SingleImageBlock(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Describe this image"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{
						Type: "image",
						Source: &anthropic.ImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "abc123",
						},
					},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeSingleElementContent(req)

	t.Logf("Before: %d blocks, After: %d blocks", originalContentLen, len(req.Messages[1].Content))

	// Single image block should get text block added
	if len(req.Messages[1].Content) != originalContentLen+1 {
		t.Errorf("Expected content length to be %d after adding text block, got %d",
			originalContentLen+1, len(req.Messages[1].Content))
		return
	}

	// Verify the last block is a text block
	lastBlock := req.Messages[1].Content[len(req.Messages[1].Content)-1]
	if lastBlock.Type != "text" {
		t.Errorf("Expected last block to be text, got %s", lastBlock.Type)
	}

	t.Logf("SUCCESS: Single image block normalized with text block")
}

// TestNormalizeSingleElementContent_SingleToolResultBlock tests that single tool_result
// blocks get a text block added.
func TestNormalizeSingleElementContent_SingleToolResultBlock(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Use the tool"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{
						Type:       "tool_result",
						ID:         "toolu_123",
						Content:    anthropic.MessageContent{{Type: "text", Text: "Tool result data"}},
						IsError:    false,
					},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// Normalize the request
	normalizeSingleElementContent(req)

	// Single tool_result block should get text block added
	if len(req.Messages[1].Content) != originalContentLen+1 {
		t.Errorf("Expected content length to be %d after adding text block, got %d",
			originalContentLen+1, len(req.Messages[1].Content))
		return
	}

	// Verify the last block is a text block
	lastBlock := req.Messages[1].Content[len(req.Messages[1].Content)-1]
	if lastBlock.Type != "text" {
		t.Errorf("Expected last block to be text, got %s", lastBlock.Type)
	}

	t.Logf("SUCCESS: Single tool_result block normalized with text block")
}

// TestNormalizeSingleElementContent_MixedNonTextContent tests that mixed non-text
// content (thinking + image) gets normalized.
func TestNormalizeSingleElementContent_MixedNonTextContent(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "user",
				Content: anthropic.MessageContent{{Type: "text", Text: "Analyze this"}},
			},
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{Type: "thinking", Thinking: "Let me analyze..."},
					{
						Type: "image",
						Source: &anthropic.ImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "xyz789",
						},
					},
				},
			},
		},
	}

	originalContentLen := len(req.Messages[1].Content)

	// This content has NO text block, so it should get normalized
	hasTextBlock := false
	for _, block := range req.Messages[1].Content {
		if block.Type == "text" {
			hasTextBlock = true
			break
		}
	}

	if !hasTextBlock && originalContentLen == 2 {
		// Should add text block
		normalizeSingleElementContent(req)

		if len(req.Messages[1].Content) != originalContentLen+1 {
			t.Errorf("Expected content length to be %d after adding text block, got %d",
				originalContentLen+1, len(req.Messages[1].Content))
			return
		}
		t.Logf("SUCCESS: Mixed non-text content normalized with text block")
	} else {
		t.Logf("Content already has text block or unexpected structure")
	}
}

// TestValidateAndRepairBlocks_ImageWithMissingData tests that image blocks with
// missing data are repaired.
func TestValidateAndRepairBlocks_ImageWithMissingData(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{
						Type: "image",
						Source: &anthropic.ImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "", // Missing data!
						},
					},
				},
			},
		},
	}

	// Validate and repair
	validateAndRepairBlocks(req)

	// The image block should be replaced with text
	if req.Messages[0].Content[0].Type != "text" {
		t.Errorf("Expected block to be converted to text, got %s", req.Messages[0].Content[0].Type)
		return
	}

	expectedText := "[Image: data unavailable]"
	if req.Messages[0].Content[0].Text != expectedText {
		t.Errorf("Expected text to be %q, got %q", expectedText, req.Messages[0].Content[0].Text)
	}

	t.Logf("SUCCESS: Image with missing data repaired")
}

// TestValidateAndRepairBlocks_ThinkingWithMissingContent tests that thinking
// blocks with missing content are repaired.
func TestValidateAndRepairBlocks_ThinkingWithMissingContent(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role: "assistant",
				Content: anthropic.MessageContent{
					{
						Type:     "thinking",
						Thinking: "", // Missing content!
					},
				},
			},
		},
	}

	// Validate and repair
	validateAndRepairBlocks(req)

	// The thinking block should be replaced with text
	if req.Messages[0].Content[0].Type != "text" {
		t.Errorf("Expected block to be converted to text, got %s", req.Messages[0].Content[0].Type)
		return
	}

	expectedText := "[Thinking: content unavailable]"
	if req.Messages[0].Content[0].Text != expectedText {
		t.Errorf("Expected text to be %q, got %q", expectedText, req.Messages[0].Content[0].Text)
	}

	t.Logf("SUCCESS: Thinking with missing content repaired")
}

// TestNormalizeSingleElementContent_TextBlock_NoChange tests that messages
// with text blocks are not modified (already valid).
func TestNormalizeSingleElementContent_TextBlock_NoChange(t *testing.T) {
	req := &anthropic.Request{
		Model:     "claude-sonnet-4.5",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{
				Role:    "assistant",
				Content: anthropic.MessageContent{{Type: "text", Text: "Hello!"}},
			},
		},
	}

	originalContentLen := len(req.Messages[0].Content)

	// Single text block should NOT get text block added (already valid as string)
	normalizeSingleElementContent(req)

	if len(req.Messages[0].Content) != originalContentLen {
		t.Errorf("Expected content length to remain %d, got %d", originalContentLen, len(req.Messages[0].Content))
	} else {
		t.Logf("SUCCESS: Single text block not modified (will marshal as string)")
	}
}
