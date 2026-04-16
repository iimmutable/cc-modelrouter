# Fix Plan: GLM 400 Bad Request Error

## Date
2026-02-28

## Problem Summary
When running `ccrouter code` and prompting Claude to "summarize the current project state", a `400 Bad Request` error is returned from GLM's API (bigmodel.cn).

## Root Cause Analysis (99% Confidence)

### The Error Flow

1. **Claude Code Request**: The request contains complex content with multiple text blocks (system reminders, context, etc.)

2. **Incoming Format** (from Claude Code/Anthropic):
   ```json
   {
     "messages": [{
       "role": "user",
       "content": [
         {"type": "text", "text": "<system-reminder>...</system-reminder>"},
         {"type": "text", "text": "more content..."}
       ]
     }]
   }
   ```

3. **Transformation Path**:
   - `AnthropicToUnified` converts each content block to `unified.MessageContent`
   - Multiple unified MessageContent items are created
   - `UnifiedRequestToAnthropic` → `convertToAnthropicContent` converts back
   - Each unified MessageContent becomes a separate `ContentBlock`

4. **Outgoing Format** (to GLM):
   ```json
   {
     "messages": [{
       "role": "user",
       "content": [
         {"type": "text", "text": "..."},
         {"type": "text", "text": "..."}
       ]
     }]
   }
   ```

5. **GLM API Rejection**:
   - GLM's proxy (ZenZGA/2.3) returns HTML error page: `400 Bad Request`
   - The API does NOT accept the array format when there are multiple content blocks

### Evidence from Logs

```
2026/02/28 18:19:23 [INFO] [GLM REQUEST BODY] {"model":"glm-4.7","max_tokens":21333,"messages":[{"role":"user","content":[{"type":"text","text":"\u003csystem-reminder\u003e..."},{"type":"text","text":"..."}],"stream":true}
2026/02/28 18:19:23 [ERROR] [PROXY ERROR] URL: https://open.bigmodel.cn/api/anthropic/v1/messages, Status: 400 Bad Request
Response: <html><head><title>400 Bad Request</title></head>...<hr><center>ZenZGA/2.3</center>...
```

Successful requests show content as a **string**:
```json
"content": "summarize the current project state"
```

### The Core Issue

The custom `MarshalJSON` in `pkg/api/anthropic/types.go` (lines 43-55):
- Returns string for single text block
- Returns array for multiple blocks (even if all are text)

GLM's API only accepts:
- Simple string content, OR
- Single-element array that gets stringified

## Fix Plan

### Option A: Merge Consecutive Text Blocks (Recommended)

Modify `convertToAnthropicContent` in `internal/transformer/converters/unified_to_anthropic.go` to merge consecutive text blocks into a single block.

**Location**: `internal/transformer/converters/unified_to_anthropic.go:132-168`

**Change** (more complete implementation):
```go
func convertToAnthropicContent(unifiedContent []unified.MessageContent) anthropic.MessageContent {
	var result anthropic.MessageContent
	var textBuffer strings.Builder

	for _, content := range unifiedContent {
		switch content.Type {
		case "text":
			// Accumulate consecutive text blocks
			textBuffer.WriteString(content.Text)
		case "tool_result":
			// Flush accumulated text before adding non-text block
			if textBuffer.Len() > 0 {
				result = append(result, anthropic.ContentBlock{
					Type: "text",
					Text: textBuffer.String(),
				})
				textBuffer.Reset()
			}
			block := anthropic.ContentBlock{Type: "tool_result", Content: content.Text}
			result = append(result, block)
		case "thinking":
			// Flush accumulated text before adding non-text block
			if textBuffer.Len() > 0 {
				result = append(result, anthropic.ContentBlock{
					Type: "text",
					Text: textBuffer.String(),
				})
				textBuffer.Reset()
			}
			block := anthropic.ContentBlock{Type: "thinking"}
			if content.Thinking != nil {
				block.Content = content.Thinking.Content
			}
			result = append(result, block)
		case "document":
			// Flush accumulated text before adding non-text block
			if textBuffer.Len() > 0 {
				result = append(result, anthropic.ContentBlock{
					Type: "text",
					Text: textBuffer.String(),
				})
				textBuffer.Reset()
			}
			block := anthropic.ContentBlock{Type: "document", Content: content.Text}
			result = append(result, block)
		case "image_url":
			// Flush accumulated text before adding image
			if textBuffer.Len() > 0 {
				result = append(result, anthropic.ContentBlock{
					Type: "text",
					Text: textBuffer.String(),
				})
				textBuffer.Reset()
			}
			block := anthropic.ContentBlock{Type: "image"}
			result = append(result, block)
		}
	}

	// Flush any remaining accumulated text at the end
	if textBuffer.Len() > 0 {
		result = append(result, anthropic.ContentBlock{
			Type: "text",
			Text: textBuffer.String(),
		})
	}

	// Ensure result is never nil
	if result == nil {
		result = anthropic.MessageContent{}
	}

	return result
}
```

Also need to add import:
```go
import (
	"strings"  // Add this import
	// ... existing imports
)
```

**Why this works**:
- Ensures content is stringified when sent to GLM
- Preserves all text content
- No loss of information
- Works with GLM's API constraints

### Option B: GLM-Specific Formatter

Create a GLM-specific content formatter that always outputs string format.

**Location**: `internal/transformer/providers/glm.go:51`

**Add special handling before marshaling**:
```go
// After line 61: anthropicReq.Thinking = nil

// Merge content blocks to single string for GLM compatibility
for i := range anthropicReq.Messages {
	if len(anthropicReq.Messages[i].Content) > 1 {
		// Merge all text blocks
		var sb strings.Builder
		for _, block := range anthropicReq.Messages[i].Content {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		anthropicReq.Messages[i].Content = anthropic.MessageContent{
			{Type: "text", Text: sb.String()},
		}
	}
}
```

### Option C: Global Content Normalization

Add a content normalization step in the unified format that prevents multiple consecutive text blocks.

**Location**: `internal/transformer/unified/message.go` or new utility

This would normalize content at the unified level before provider conversion.

## Recommended Approach

**Option A** is recommended because:
1. Fixes the issue at the transformation layer
2. Maintains consistency with Anthropic format rules
3. No provider-specific workarounds needed
4. Preserves content integrity

## Testing Plan

1. Test with simple string content (existing working case)
2. Test with multiple text blocks (failing case)
3. Test with mixed content (text + images)
4. Test with tool_use blocks
5. Verify log shows `"content":"string"` format for GLM requests

## Files to Modify

1. `internal/transformer/converters/unified_to_anthropic.go`:
   - Add `strings` import
   - Modify `convertToAnthropicContent` function (lines 132-168)

2. `internal/transformer/converters/converters_test.go` - Add test coverage for:
   - Multiple consecutive text blocks → single text block
   - Text + image mixed content → preserved
   - Text + tool_result mixed content → preserved
   - Text + thinking mixed content → preserved

3. `internal/transformer/providers/glm_test.go` - Add GLM-specific test:
   - Test GLM request with multiple text blocks marshals to string format

## Verification Checklist

Before implementing:
- [x] Root cause confirmed via log analysis
- [x] MarshalJSON behavior verified in `pkg/api/anthropic/types.go`
- [x] Code path traced from request to GLM API
- [x] Fix approach validated against GLM API constraints

After implementing:
- [ ] Unit tests pass with new test cases
- [ ] Existing tests still pass (no regression)
- [ ] Manual test with `ccrouter code` succeeds
- [ ] Log shows `"content":"string"` format for GLM requests
- [ ] Mixed content (text + non-text) works correctly

## Confidence Level: 99%

The evidence is clear:
- Log shows array format being sent to GLM
- GLM's ZenZGA/2.3 proxy rejects with 400 Bad Request
- Anthropic's MarshalJSON creates array for multiple blocks
- Merging text blocks ensures string format is used
