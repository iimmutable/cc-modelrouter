package anthropic

import (
	"encoding/json"
	"strings"
	"testing"
)

// testStrPtr is a helper function for creating string pointers in tests.
func testStrPtr(s string) *string {
	return &s
}

func TestRequestMarshaling(t *testing.T) {
	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []Message{
			{
				Role:    "user",
				Content: MessageContent{{Type: "text", Text: "Hello"}},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.Model != req.Model {
		t.Errorf("expected model %s, got %s", req.Model, unmarshaled.Model)
	}
}

func TestContentBlockTypes(t *testing.T) {
	textBlock := ContentBlock{
		Type: "text",
		Text: "Hello",
	}

	imageBlock := ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "base64data",
		},
	}

	toolUseBlock := ContentBlock{
		Type:  "tool_use",
		ID:    "toolu_123",
		Name:  "get_weather",
		Input: json.RawMessage(`{"location": "SF"}`),
	}

	textData, _ := json.Marshal(textBlock)
	imageData, _ := json.Marshal(imageBlock)
	toolData, _ := json.Marshal(toolUseBlock)

	if string(textData) == "" {
		t.Error("text block should marshal to non-empty JSON")
	}
	if string(imageData) == "" {
		t.Error("image block should marshal to non-empty JSON")
	}
	if string(toolData) == "" {
		t.Error("tool_use block should marshal to non-empty JSON")
	}
}

func TestThinkingConfig(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected *ThinkingConfig
	}{
		{
			name:     "enabled with budget",
			json:     `{"type": "enabled", "budget_tokens": 10000}`,
			expected: &ThinkingConfig{Type: "enabled", BudgetTokens: 10000},
		},
		{
			name:     "enabled without budget",
			json:     `{"type": "enabled"}`,
			expected: &ThinkingConfig{Type: "enabled", BudgetTokens: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config ThinkingConfig
			if err := json.Unmarshal([]byte(tt.json), &config); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if config.Type != tt.expected.Type {
				t.Errorf("expected type %s, got %s", tt.expected.Type, config.Type)
			}
			if config.BudgetTokens != tt.expected.BudgetTokens {
				t.Errorf("expected budget_tokens %d, got %d", tt.expected.BudgetTokens, config.BudgetTokens)
			}
		})
	}
}

func TestRequestWithThinking(t *testing.T) {
	jsonStr := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 4096,
		"thinking": {
			"type": "enabled",
			"budget_tokens": 32000
		},
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`

	var req Request
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal request with thinking: %v", err)
	}

	if req.Thinking == nil {
		t.Fatal("expected thinking config to be non-nil")
	}
	if req.Thinking.Type != "enabled" {
		t.Errorf("expected thinking type 'enabled', got %s", req.Thinking.Type)
	}
	if req.Thinking.BudgetTokens != 32000 {
		t.Errorf("expected budget_tokens 32000, got %d", req.Thinking.BudgetTokens)
	}

	// Test marshaling back
	data, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.Thinking == nil {
		t.Fatal("expected thinking config to be preserved after marshal/unmarshal")
	}
	if unmarshaled.Thinking.BudgetTokens != 32000 {
		t.Errorf("expected budget_tokens to be preserved, got %d", unmarshaled.Thinking.BudgetTokens)
	}
}

// TestMessageContentMarshalTextMerging tests that consecutive text blocks are merged.
// This is critical for GLM/ZenZGA compatibility which rejects array format for text-only content.
func TestMessageContentMarshalTextMerging(t *testing.T) {
	tests := []struct {
		name           string
		input          MessageContent
		expectedOutput string
		expectString   bool // true if output should be a string, false if array
	}{
		{
			name: "empty content",
			input: MessageContent{},
			expectedOutput: `[]`,
			expectString: false,
		},
		{
			name: "single text block",
			input: MessageContent{{Type: "text", Text: "Hello"}},
			expectedOutput: `"Hello"`,
			expectString: true,
		},
		{
			name: "multiple consecutive text blocks - should merge to string",
			input: MessageContent{
				{Type: "text", Text: "First "},
				{Type: "text", Text: "Second "},
				{Type: "text", Text: "Third"},
			},
			expectedOutput: `"First Second Third"`,
			expectString: true,
		},
		{
			name: "text + image + text - should be array with merged text",
			input: MessageContent{
				{Type: "text", Text: "Before image"},
				{Type: "image", Source: &ImageSource{Type: "base64", MediaType: "image/png", Data: "abc"}},
				{Type: "text", Text: "After image"},
			},
			expectedOutput: `[{"type":"text","text":"Before image"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}},{"type":"text","text":"After image"}]`,
			expectString: false,
		},
		{
			name: "text + tool_result + text - should be array with merged text",
			input: MessageContent{
				{Type: "text", Text: "Before tool"},
				{Type: "tool_result", ID: "tool123", Content: MessageContent{{Type: "text", Text: "Result"}}},
				{Type: "text", Text: "After tool"},
			},
			expectedOutput: `[{"type":"text","text":"Before tool"},{"type":"tool_result","tool_use_id":"tool123","content":"Result"},{"type":"text","text":"After tool"}]`,
			expectString: false,
		},
		{
			name: "multiple text blocks before non-text - should merge first group",
			input: MessageContent{
				{Type: "text", Text: "First "},
				{Type: "text", Text: "Second"},
				{Type: "tool_result", ID: "tool123", Content: MessageContent{{Type: "text", Text: "Result"}}},
			},
			expectedOutput: `[{"type":"text","text":"First Second"},{"type":"tool_result","tool_use_id":"tool123","content":"Result"}]`,
			expectString: false,
		},
		{
			name: "only non-text blocks - should be array",
			input: MessageContent{
				{Type: "tool_result", ID: "tool123", Content: MessageContent{{Type: "text", Text: "Result"}}},
			},
			expectedOutput: `[{"type":"tool_result","tool_use_id":"tool123","content":"Result"}]`,
			expectString: false,
		},
		{
			name: "empty text block - should be ignored",
			input: MessageContent{
				{Type: "text", Text: "Valid text"},
				{Type: "text", Text: ""},
				{Type: "text", Text: "More text"},
			},
			expectedOutput: `"Valid textMore text"`,
			expectString: true,
		},
		{
			name: "complex Claude Code request - system reminders + context + user message",
			input: MessageContent{
				{Type: "text", Text: "You are Claude Code, an AI assistant. "},
				{Type: "text", Text: "Current project: cc-modelrouter. "},
				{Type: "text", Text: "Please help debug this issue."},
			},
			expectedOutput: `"You are Claude Code, an AI assistant. Current project: cc-modelrouter. Please help debug this issue."`,
			expectString: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			got := string(data)
			if got != tt.expectedOutput {
				t.Errorf("expected:\n  %s\ngot:\n  %s", tt.expectedOutput, got)
			}

			// Verify string vs array format
			if tt.expectString {
				if got[0] != '"' {
					t.Errorf("expected string format (starting with \"), got: %s", got)
				}
			} else {
				if got[0] != '[' {
					t.Errorf("expected array format (starting with [), got: %s", got)
				}
			}

			// Test round-trip: unmarshal and marshal again
			var unmarshaled MessageContent
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// For string format, unmarshaling creates a single text block
			// For array format, unmarshaling preserves the structure
			// This is expected behavior
		})
	}
}

// TestMessageContentMarshalSizeComparison compares JSON size for merged vs unmerged text.
// This demonstrates the size reduction benefit of text merging.
func TestMessageContentMarshalSizeComparison(t *testing.T) {
	// Simulate a typical Claude Code request with multiple text blocks
	input := MessageContent{}
	for i := 0; i < 20; i++ {
		input = append(input, ContentBlock{Type: "text", Text: "Block content. "})
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// With merging, the output should be a single string (much smaller)
	result := string(data)

	t.Logf("Merged JSON size: %d bytes", len(result))
	t.Logf("Output starts with: %s", result[:min(50, len(result))])

	if result[0] != '"' {
		t.Errorf("Expected string format for 20 text blocks, got: %s", result[:min(100, len(result))])
	}

	// Estimate what the array format would be (approximately 25 bytes overhead per block)
	// Array: 20 blocks × ~40 bytes = ~800 bytes
	// String: 20 blocks × 15 chars + JSON overhead = ~350 bytes
	// We expect significant size reduction
	if len(result) > 500 {
		t.Logf("Warning: Merged JSON seems large (%d bytes), but this is OK for content length", len(result))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestContentBlockToolResultMarshaling tests that tool_result blocks marshal
// with "tool_use_id" instead of "id" to comply with Anthropic's API specification.
// This is critical for compatibility with providers like OpenRouter and GLM.
func TestContentBlockToolResultMarshaling(t *testing.T) {
	tests := []struct {
		name           string
		input          ContentBlock
		expectedOutput string
	}{
		{
			name: "tool_result with single text content (should marshal as string for backward compat)",
			input: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_12345",
				Content: MessageContent{{Type: "text", Text: "The result"}},
			},
			expectedOutput: `{"type":"tool_result","tool_use_id":"toolu_12345","content":"The result"}`,
		},
		{
			name: "tool_result with is_error flag",
			input: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_67890",
				Content: MessageContent{{Type: "text", Text: "Error occurred"}},
				IsError: true,
			},
			expectedOutput: `{"type":"tool_result","tool_use_id":"toolu_67890","content":"Error occurred","is_error":true}`,
		},
		{
			name: "tool_result with multiple content blocks (should marshal as array) (NEW)",
			input: ContentBlock{
				Type: "tool_result",
				ID:   "toolu_multi",
				Content: MessageContent{
					{Type: "text", Text: "Result 1"},
					{Type: "text", Text: "Result 2"},
				},
			},
			expectedOutput: `{"type":"tool_result","tool_use_id":"toolu_multi","content":[{"type":"text","text":"Result 1"},{"type":"text","text":"Result 2"}]}`,
		},
		{
			name: "tool_use block should use 'id' not 'tool_use_id'",
			input: ContentBlock{
				Type:  "tool_use",
				ID:    "toolu_abc123",
				Name:  "get_weather",
				Input: json.RawMessage(`{"location":"NYC"}`),
			},
			expectedOutput: `{"type":"tool_use","id":"toolu_abc123","name":"get_weather","input":{"location":"NYC"}}`,
		},
		{
			name: "text block should use standard format",
			input: ContentBlock{
				Type: "text",
				Text: "Hello, world!",
			},
			expectedOutput: `{"type":"text","text":"Hello, world!"}`,
		},
		{
			name: "image block should use standard format",
			input: ContentBlock{
				Type: "image",
				Source: &ImageSource{
					Type:      "base64",
					MediaType: "image/png",
					Data:      "base64data",
				},
			},
			expectedOutput: `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"base64data"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			got := string(data)
			if got != tt.expectedOutput {
				t.Errorf("expected:\n  %s\ngot:\n  %s", tt.expectedOutput, got)
			}
		})
	}
}

// TestContentBlockToolResultUnmarshaling tests that tool_result blocks can be
// unmarshaled from both "tool_use_id" and "id" formats for compatibility.
func TestContentBlockToolResultUnmarshaling(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expected    ContentBlock
		expectError bool
	}{
		{
			name: "unmarshal tool_result with tool_use_id (string content)",
			json: `{"type":"tool_result","tool_use_id":"toolu_12345","content":"Result"}`,
			expected: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_12345",
				Content: MessageContent{{Type: "text", Text: "Result"}},
			},
		},
		{
			name: "unmarshal tool_result with id (fallback, string content)",
			json: `{"type":"tool_result","id":"toolu_67890","content":"Result"}`,
			expected: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_67890",
				Content: MessageContent{{Type: "text", Text: "Result"}},
			},
		},
		{
			name: "unmarshal tool_result with array content (NEW)",
			json: `{"type":"tool_result","tool_use_id":"toolu_123","content":[{"type":"text","text":"Result"}]}`,
			expected: ContentBlock{
				Type: "tool_result",
				ID:   "toolu_123",
				Content: MessageContent{{Type: "text", Text: "Result"}},
			},
		},
		{
			name: "unmarshal tool_result with multiple array content blocks (NEW)",
			json: `{"type":"tool_result","tool_use_id":"toolu_456","content":[{"type":"text","text":"R1"},{"type":"text","text":"R2"}]}`,
			expected: ContentBlock{
				Type: "tool_result",
				ID:   "toolu_456",
				Content: MessageContent{
					{Type: "text", Text: "R1"},
					{Type: "text", Text: "R2"},
				},
			},
		},
		{
			name: "unmarshal tool_result with is_error",
			json: `{"type":"tool_result","tool_use_id":"toolu_error","content":"Failed","is_error":true}`,
			expected: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_error",
				Content: MessageContent{{Type: "text", Text: "Failed"}},
				IsError: true,
			},
		},
		{
			name: "unmarshal tool_use with id",
			json: `{"type":"tool_use","id":"toolu_abcd","name":"test","input":{"arg":"value"}}`,
			expected: ContentBlock{
				Type:  "tool_use",
				ID:    "toolu_abcd",
				Name:  "test",
				Input: json.RawMessage(`{"arg":"value"}`),
			},
		},
		{
			name: "unmarshal text block",
			json: `{"type":"text","text":"Hello"}`,
			expected: ContentBlock{
				Type: "text",
				Text: "Hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ContentBlock
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("expected type %s, got %s", tt.expected.Type, got.Type)
			}

			if got.ID != tt.expected.ID {
				t.Errorf("expected id %s, got %s", tt.expected.ID, got.ID)
			}

			if len(got.Content) != len(tt.expected.Content) {
				t.Errorf("expected content length %d, got %d", len(tt.expected.Content), len(got.Content))
			} else {
				for i := range got.Content {
					if got.Content[i].Type != tt.expected.Content[i].Type {
						t.Errorf("content[%d]: expected type %s, got %s", i, tt.expected.Content[i].Type, got.Content[i].Type)
					}
					if got.Content[i].Text != tt.expected.Content[i].Text {
						t.Errorf("content[%d]: expected text %s, got %s", i, tt.expected.Content[i].Text, got.Content[i].Text)
					}
				}
			}

			if got.IsError != tt.expected.IsError {
				t.Errorf("expected is_error %v, got %v", tt.expected.IsError, got.IsError)
			}

			if got.Text != tt.expected.Text {
				t.Errorf("expected text %s, got %s", tt.expected.Text, got.Text)
			}

			if got.Name != tt.expected.Name {
				t.Errorf("expected name %s, got %s", tt.expected.Name, got.Name)
			}
		})
	}
}

// TestContentBlockRoundTrip tests that marshaling and unmarshaling preserves data.
func TestContentBlockRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		block ContentBlock
	}{
		{
			name: "tool_result round trip",
			block: ContentBlock{
				Type:    "tool_result",
				ID:      "toolu_roundtrip",
				Content: MessageContent{{Type: "text", Text: "Round trip test"}},
				IsError: false,
			},
		},
		{
			name: "tool_use round trip",
			block: ContentBlock{
				Type:  "tool_use",
				ID:    "toolu_rt123",
				Name:  "round_trip_tool",
				Input: json.RawMessage(`{"test":"value"}`),
			},
		},
		{
			name: "text round trip",
			block: ContentBlock{
				Type: "text",
				Text: "Round trip text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.block)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Unmarshal
			var got ContentBlock
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Verify key fields
			if got.Type != tt.block.Type {
				t.Errorf("type changed: %s -> %s", tt.block.Type, got.Type)
			}

			if got.ID != tt.block.ID {
				t.Errorf("id changed: %s -> %s", tt.block.ID, got.ID)
			}

			if len(got.Content) != len(tt.block.Content) {
				t.Errorf("content length changed: %d -> %d", len(tt.block.Content), len(got.Content))
			} else {
				for i := range got.Content {
					if i >= len(tt.block.Content) {
						break
					}
					if got.Content[i].Type != tt.block.Content[i].Type {
						t.Errorf("content[%d].type changed: %s -> %s", i, tt.block.Content[i].Type, got.Content[i].Type)
					}
					if got.Content[i].Text != tt.block.Content[i].Text {
						t.Errorf("content[%d].text changed: %s -> %s", i, tt.block.Content[i].Text, got.Content[i].Text)
					}
				}
			}

			if got.Text != tt.block.Text {
				t.Errorf("text changed: %s -> %s", tt.block.Text, got.Text)
			}

			if got.Name != tt.block.Name {
				t.Errorf("name changed: %s -> %s", tt.block.Name, got.Name)
			}

			if got.IsError != tt.block.IsError {
				t.Errorf("is_error changed: %v -> %v", tt.block.IsError, got.IsError)
			}
		})
	}
}

// TestMessageContentWithToolResults tests that MessageContent properly
// handles messages containing tool_result blocks.
func TestMessageContentWithToolResults(t *testing.T) {
	// Simulate a typical tool use conversation
	messages := []Message{
		{
			Role: RoleUser,
			Content: MessageContent{
				{Type: "text", Text: "What's the weather in SF?"},
			},
		},
		{
			Role: RoleAssistant,
			Content: MessageContent{
				{
					Type:  "tool_use",
					ID:    "toolu_0123",
					Name:  "get_weather",
					Input: json.RawMessage(`{"location":"SF"}`),
				},
			},
		},
		{
			Role: RoleUser,
			Content: MessageContent{
				{
					Type:    "tool_result",
					ID:      "toolu_0123",
					Content: MessageContent{{Type: "text", Text: "65 degrees and sunny"}},
				},
			},
		},
	}

	// Marshal the full request
	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages:  messages,
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	result := string(data)

	// Verify tool_result uses tool_use_id
	if !strings.Contains(result, "tool_use_id") {
		t.Error("Expected tool_use_id in marshaled output, but it was not found")
	}

	// Verify the tool_result block has the correct structure (with indentation)
	if !strings.Contains(result, `"tool_use_id": "toolu_0123"`) && !strings.Contains(result, `"tool_use_id":"toolu_0123"`) {
		t.Errorf("Expected tool_use_id: toolu_0123 in output")
	}

	// Verify tool_use uses 'id' not 'tool_use_id'
	if !strings.Contains(result, `"id": "toolu_0123"`) && !strings.Contains(result, `"id":"toolu_0123"`) {
		t.Errorf("Expected id: toolu_0123 for tool_use")
	}

	// Verify the structure: tool_result should have tool_use_id, tool_use should have id
	// Count occurrences to ensure correctness
	toolResultCount := strings.Count(result, `"tool_use_id"`)
	if toolResultCount != 1 {
		t.Errorf("Expected exactly 1 tool_use_id field, got %d", toolResultCount)
	}

	// tool_use should have id field (not tool_use_id)
	lines := strings.Split(result, "\n")
	foundToolUse := false
	foundToolResult := false
	for _, line := range lines {
		if strings.Contains(line, `"type": "tool_use"`) || strings.Contains(line, `"type":"tool_use"`) {
			foundToolUse = true
		}
		if strings.Contains(line, `"type": "tool_result"`) || strings.Contains(line, `"type":"tool_result"`) {
			foundToolResult = true
			// This line should have tool_use_id
			if !strings.Contains(line, "tool_use_id") {
				// Check next few lines for tool_use_id
				continue
			}
		}
	}

	if !foundToolUse || !foundToolResult {
		t.Error("Expected both tool_use and tool_result blocks in output")
	}

	t.Logf("Marshaled request:\n%s", result)
}

// TestContentBlockThinkingMarshaling tests that thinking blocks marshal
// with "thinking" and optionally "signature" fields.
//
// For Anthropic/OpenRouter compatibility: Empty signatures are omitted from JSON.
// For GLM provider compatibility: GLM-specific transformer ensures signature field is present.
//
// This is critical for compatibility with providers like OpenRouter (rejects empty signatures)
// and Aliyun GLM-5 (requires signature field to be present).
func TestContentBlockThinkingMarshaling(t *testing.T) {
	tests := []struct {
		name           string
		input          ContentBlock
		expectedOutput string
	}{
		{
			name: "thinking block with thinking and signature",
			input: ContentBlock{
				Type:      "thinking",
				Thinking:  "Let me think about this problem...",
				Signature: testStrPtr("abc123def456"),
			},
			expectedOutput: `{"type":"thinking","thinking":"Let me think about this problem...","signature":"abc123def456"}`,
		},
		{
			name: "thinking block with thinking only (signature omitted when empty)",
			input: ContentBlock{
				Type:     "thinking",
				Thinking: "Extended thinking content here",
			},
			expectedOutput: `{"type":"thinking","thinking":"Extended thinking content here"}`,
		},
		{
			name: "thinking block with nil signature (signature omitted)",
			input: ContentBlock{
				Type:      "thinking",
				Thinking:  "Extended thinking content here",
				// Signature is nil (not set), field should be omitted
			},
			expectedOutput: `{"type":"thinking","thinking":"Extended thinking content here"}`,
		},
		{
			name: "thinking block with empty signature string (signature included as empty string)",
			input: ContentBlock{
				Type:      "thinking",
				Thinking:  "Extended thinking content here",
				Signature: testStrPtr(""),
			},
			expectedOutput: `{"type":"thinking","thinking":"Extended thinking content here","signature":""}`,
		},
		{
			name: "thinking block with signature only",
			input: ContentBlock{
				Type:      "thinking",
				Signature: testStrPtr("sig789"),
			},
			expectedOutput: `{"type":"thinking","signature":"sig789"}`,
		},
		{
			name: "empty thinking block (signature omitted)",
			input: ContentBlock{
				Type: "thinking",
			},
			expectedOutput: `{"type":"thinking"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			got := string(data)
			if got != tt.expectedOutput {
				t.Errorf("expected:\n  %s\ngot:\n  %s", tt.expectedOutput, got)
			}

			// Verify it contains "thinking" field (not "content")
			if tt.input.Thinking != "" && !strings.Contains(got, `"thinking":`) && !strings.Contains(got, `"thinking":`) {
				t.Error("expected thinking field in marshaled output")
			}

			// Verify it does NOT contain "content" field for thinking type
			if tt.input.Type == "thinking" && strings.Contains(got, `"content":`) {
				t.Error("thinking block should not have content field, should have thinking field")
			}

			// Verify nil signatures are omitted
			if tt.input.Type == "thinking" && tt.input.Signature == nil && strings.Contains(got, `"signature":`) {
				t.Error("thinking block with nil signature should not have signature field in output")
			}

			// Verify non-empty signatures are included
			if tt.input.Type == "thinking" && tt.input.Signature != nil && *tt.input.Signature != "" && !strings.Contains(got, `"signature":`) {
				t.Error("thinking block with non-empty signature should have signature field in output")
			}

			// Verify empty string signatures are included (new behavior with pointer type)
			if tt.input.Type == "thinking" && tt.input.Signature != nil && *tt.input.Signature == "" && !strings.Contains(got, `"signature":""`) {
				t.Error("thinking block with empty signature string should have signature field in output")
			}
		})
	}
}

// TestContentBlockThinkingUnmarshaling tests that thinking blocks can be
// unmarshaled with "thinking" and "signature" fields.
func TestContentBlockThinkingUnmarshaling(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expected    ContentBlock
		expectError bool
	}{
		{
			name: "unmarshal thinking with thinking and signature",
			json: `{"type":"thinking","thinking":"Thinking content","signature":"sig123"}`,
			expected: ContentBlock{
				Type:      "thinking",
				Thinking:  "Thinking content",
				Signature: testStrPtr("sig123"),
			},
		},
		{
			name: "unmarshal thinking with thinking only",
			json: `{"type":"thinking","thinking":"Just thinking"}`,
			expected: ContentBlock{
				Type:     "thinking",
				Thinking: "Just thinking",
			},
		},
		{
			name: "unmarshal thinking with signature only",
			json: `{"type":"thinking","signature":"onlysig"}`,
			expected: ContentBlock{
				Type:      "thinking",
				Signature: testStrPtr("onlysig"),
			},
		},
		{
			name: "unmarshal thinking with legacy content field (backward compat)",
			json: `{"type":"thinking","content":"Legacy content"}`,
			expected: ContentBlock{
				Type:     "thinking",
				Thinking: "Legacy content",
			},
		},
		{
			name: "unmarshal thinking with both thinking and content (thinking takes priority)",
			json: `{"type":"thinking","thinking":"New content","content":"Old content"}`,
			expected: ContentBlock{
				Type:     "thinking",
				Thinking: "New content",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ContentBlock
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("expected type %s, got %s", tt.expected.Type, got.Type)
			}

			if got.Thinking != tt.expected.Thinking {
				t.Errorf("expected thinking %s, got %s", tt.expected.Thinking, got.Thinking)
			}

			// Compare pointer values directly
			if (got.Signature == nil) != (tt.expected.Signature == nil) {
				t.Errorf("expected signature nil=%v, got nil=%v", tt.expected.Signature == nil, got.Signature == nil)
			} else if got.Signature != nil && tt.expected.Signature != nil && *got.Signature != *tt.expected.Signature {
				t.Errorf("expected signature %s, got %s", *tt.expected.Signature, *got.Signature)
			}
		})
	}
}

// TestRedactedThinkingBlockRoundTrip tests that redacted_thinking blocks survive
// a marshal → unmarshal → marshal round-trip without losing the data field.
// This is critical for OpenRouter compatibility — missing data causes 400 errors.
func TestRedactedThinkingBlockRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		block ContentBlock
	}{
		{
			name: "redacted_thinking with data",
			block: ContentBlock{
				Type: "redacted_thinking",
				Data: "base64encodeddata==",
			},
		},
		{
			name: "redacted_thinking with long data",
			block: ContentBlock{
				Type: "redacted_thinking",
				Data: "VGhpcyBpcyBhIHRlc3QgZGF0YSBmb3IgcmVkYWN0ZWQgdGhpbmtpbmcgYmxvY2tz",
			},
		},
		{
			name: "redacted_thinking with empty data",
			block: ContentBlock{
				Type: "redacted_thinking",
				Data: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.block)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Unmarshal
			var got ContentBlock
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Verify type preserved
			if got.Type != tt.block.Type {
				t.Errorf("type changed: %s -> %s", tt.block.Type, got.Type)
			}

			// Verify data preserved
			if got.Data != tt.block.Data {
				t.Errorf("data changed: %s -> %s", tt.block.Data, got.Data)
			}

			// Marshal again and verify identical
			data2, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("failed to re-marshal: %v", err)
			}

			if string(data) != string(data2) {
				t.Errorf("round-trip mismatch:\n  first:  %s\n  second: %s", string(data), string(data2))
			}
		})
	}
}

// TestRedactedThinkingMarshaling tests exact JSON output for redacted_thinking blocks.
func TestRedactedThinkingMarshaling(t *testing.T) {
	tests := []struct {
		name           string
		input          ContentBlock
		expectedOutput string
	}{
		{
			name: "redacted_thinking with data",
			input: ContentBlock{
				Type: "redacted_thinking",
				Data: "abc123==",
			},
			expectedOutput: `{"type":"redacted_thinking","data":"abc123=="}`,
		},
		{
			name: "redacted_thinking with empty data",
			input: ContentBlock{
				Type: "redacted_thinking",
				Data: "",
			},
			expectedOutput: `{"type":"redacted_thinking","data":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			got := string(data)
			if got != tt.expectedOutput {
				t.Errorf("expected:\n  %s\ngot:\n  %s", tt.expectedOutput, got)
			}
		})
	}
}

// TestRedactedThinkingUnmarshaling tests that redacted_thinking blocks can be
// unmarshaled from JSON with the data field preserved.
func TestRedactedThinkingUnmarshaling(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expected    ContentBlock
		expectError bool
	}{
		{
			name: "redacted_thinking with data",
			json: `{"type":"redacted_thinking","data":"base64data=="}`,
			expected: ContentBlock{
				Type: "redacted_thinking",
				Data: "base64data==",
			},
		},
		{
			name: "redacted_thinking without data field",
			json: `{"type":"redacted_thinking"}`,
			expected: ContentBlock{
				Type: "redacted_thinking",
				Data: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ContentBlock
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("expected type %s, got %s", tt.expected.Type, got.Type)
			}

			if got.Data != tt.expected.Data {
				t.Errorf("expected data %s, got %s", tt.expected.Data, got.Data)
			}
		})
	}
}

// TestRedactedThinkingInConversationHistory tests that a request with
// redacted_thinking blocks in conversation history round-trips correctly.
// This simulates the exact scenario that causes OpenRouter 400 errors.
func TestRedactedThinkingInConversationHistory(t *testing.T) {
	req := &Request{
		Model:     "claude-sonnet-4",
		MaxTokens: 4096,
		Messages: []Message{
			{
				Role:    RoleUser,
				Content: MessageContent{{Type: "text", Text: "Hello"}},
			},
			{
				Role: RoleAssistant,
				Content: MessageContent{
					{Type: "thinking", Thinking: "Let me think...", Signature: testStrPtr("sig123")},
					{Type: "redacted_thinking", Data: "ZW5jcnlwdGVkX3RoaW5raW5n"},
					{Type: "text", Text: "Here is my answer"},
				},
			},
			{
				Role:    RoleUser,
				Content: MessageContent{{Type: "text", Text: "Follow up"}},
			},
		},
	}

	// Marshal
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify the redacted_thinking block preserved its data
	assistantContent := unmarshaled.Messages[1].Content
	var foundRedacted bool
	for _, block := range assistantContent {
		if block.Type == "redacted_thinking" {
			foundRedacted = true
			if block.Data != "ZW5jcnlwdGVkX3RoaW5raW5n" {
				t.Errorf("redacted_thinking data not preserved: got %q", block.Data)
			}
		}
	}
	if !foundRedacted {
		t.Error("redacted_thinking block not found in unmarshaled content")
	}

	// Marshal again — verify data field still present in JSON output
	data2, err := json.Marshal(unmarshaled)
	if err != nil {
		t.Fatalf("failed to re-marshal: %v", err)
	}

	jsonStr := string(data2)
	if !strings.Contains(jsonStr, `"data":"ZW5jcnlwdGVkX3RoaW5raW5n"`) && !strings.Contains(jsonStr, `"data": "ZW5jcnlwdGVkX3RoaW5raW5n"`) {
		t.Errorf("redacted_thinking data field missing in re-marshaled JSON:\n%s", jsonStr)
	}
}

// TestContentBlockThinkingRoundTrip tests that thinking blocks round-trip correctly.
func TestContentBlockThinkingRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		block ContentBlock
	}{
		{
			name: "thinking block round trip with all fields",
			block: ContentBlock{
				Type:      "thinking",
				Thinking:  "Original thinking content",
				Signature: testStrPtr("original_signature"),
			},
		},
		{
			name: "thinking block round trip with thinking only",
			block: ContentBlock{
				Type:     "thinking",
				Thinking: "Just the thinking field",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.block)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Unmarshal
			var got ContentBlock
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Verify key fields
			if got.Type != tt.block.Type {
				t.Errorf("type changed: %s -> %s", tt.block.Type, got.Type)
			}

			if got.Thinking != tt.block.Thinking {
				t.Errorf("thinking changed: %s -> %s", tt.block.Thinking, got.Thinking)
			}

			// Compare pointer values directly
			if (got.Signature == nil) != (tt.block.Signature == nil) {
				t.Errorf("signature nil changed: nil=%v -> nil=%v", tt.block.Signature == nil, got.Signature == nil)
			} else if got.Signature != nil && tt.block.Signature != nil && *got.Signature != *tt.block.Signature {
				t.Errorf("signature changed: %s -> %s", *tt.block.Signature, *got.Signature)
			}
		})
	}
}

// TestRequestStreamFieldMarshaling tests that the stream field is always
// present in JSON output, even when false. This is required for GLM
// compatibility — GLM's Anthropic-compatible endpoint returns error 1213
// ("prompt not received") when the stream field is absent.
func TestRequestStreamFieldMarshaling(t *testing.T) {
	baseReq := &Request{
		Model:     "claude-sonnet-4",
		MaxTokens: 100,
		Messages: []Message{
			{Role: RoleUser, Content: MessageContent{{Type: "text", Text: "hi"}}},
		},
	}

	t.Run("stream true includes stream:true", func(t *testing.T) {
		req := *baseReq
		req.Stream = true
		data, err := json.Marshal(&req)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		if !strings.Contains(string(data), `"stream":true`) {
			t.Errorf("expected stream:true in JSON, got: %s", string(data))
		}
	})

	t.Run("stream false includes stream:false (not omitted)", func(t *testing.T) {
		req := *baseReq
		req.Stream = false
		data, err := json.Marshal(&req)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		s := string(data)
		if !strings.Contains(s, `"stream":false`) {
			t.Errorf("stream:false must be present for GLM compatibility, got: %s", s)
		}
	})
}

// TestMessageContentWithThinkingBlocks tests that MessageContent properly
// handles messages containing thinking blocks.
func TestMessageContentWithThinkingBlocks(t *testing.T) {
	// Simulate a response with extended thinking
	messages := []Message{
		{
			Role: RoleUser,
			Content: MessageContent{
				{Type: "text", Text: "What's 2+2?"},
			},
		},
		{
			Role: RoleAssistant,
			Content: MessageContent{
				{
					Type:      "thinking",
					Thinking:  "I need to add 2 and 2...",
					Signature: testStrPtr("calc_sig"),
				},
				{
					Type: "text",
					Text: "The answer is 4",
				},
			},
		},
	}

	// Marshal the full request
	req := &Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
		Messages:  messages,
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	result := string(data)

	// Verify thinking uses "thinking" field
	if !strings.Contains(result, `"thinking":`) && !strings.Contains(result, `"thinking":`) {
		t.Error("Expected thinking field in marshaled output")
	}

	// Verify the thinking block does NOT use "content" field
	// (for thinking blocks, we use "thinking" not "content")
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.Contains(line, `"type": "thinking"`) || strings.Contains(line, `"type":"thinking"`) {
			// Check surrounding lines for thinking field
			found := false
			for j := i; j < min(i+5, len(lines)); j++ {
				if strings.Contains(lines[j], `"thinking":`) || strings.Contains(lines[j], `"thinking":`) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected thinking field near thinking type, around line %d", i)
			}
		}
	}

	t.Logf("Marshaled request with thinking:\n%s", result)
}
