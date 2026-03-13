# Knowledge Base: Invalid Anthropic Messages API Request Errors

## Executive Summary

This document summarizes the root cause analysis and fixes for recurring "Invalid Anthropic Messages API request" errors (invalid_union, invalid_type, invalid_input) that occur when routing requests through OpenRouter to Anthropic models.

**Last Updated:** 2026-03-11
**Status:** Fix in progress

---

## Error Signature

### Typical Error Response

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid Anthropic Messages API request"
  },
  "metadata": {
    "raw": "[
      {
        "code": "invalid_union",
        "errors": [
          [
            {
              "expected": "string",
              "code": "invalid_type",
              "path": [],
              "message": "Invalid input: expected string, received array"
            }
          ],
          [
            {
              "expected": "string",
              "code": "invalid_type",
              "path": [3, "data"],
              "message": "Invalid input: expected string, received undefined"
            }
          ]
        ],
        "path": ["messages", 1, "content"],
        "message": "Invalid input"
      }
    ]"
  }
}
```

### Key Error Paths

| Error | Path | Meaning |
|-------|------|---------|
| Expected string, received array | `messages[N].content` | Single-element array where string expected |
| Expected string, received undefined | `messages[N].content[M].data` | Missing required field in content block |

---

## Root Cause Analysis

### Primary Issue: Incomplete Content Normalization

**Location:** `internal/transformer/transformers/anthropic.go`

The `normalizeThinkingBlockMessages()` function **only normalizes content with ONLY thinking blocks**:

```go
// Current logic (BUGGY)
for _, block := range content {
    if block.Type == "text" {
        hasTextBlock = true
        break
    }
    if block.Type != "thinking" {
        hasOnlyThinkingBlocks = false
        break
    }
}
```

**This fails for:**

| Content Type | Current Behavior | Result |
|--------------|------------------|--------|
| Single thinking block | Normalized ✅ | Fixed |
| Multiple thinking blocks | Normalized ✅ | Fixed |
| Thinking + text | Not modified (valid) ✅ | Fixed |
| Single image block | NOT normalized ❌ | Still array → fails |
| Single tool_result | NOT normalized ❌ | Still array → fails |
| Mixed non-text (thinking + image) | NOT normalized ❌ | Still array → fails |

### Secondary Issue: Block Field Validation

The error at path `[3]["data"]` indicates there's a content block with a missing required `data` field:
- Likely an image block from conversation history with corrupted/incomplete data
- Or a transformed response that lost required fields during conversion

### Why This Keeps Recurring

1. The original fix addressed thinking-only content
2. Other single-element content types still become arrays
3. GLM/other providers may return content that doesn't meet Anthropic schema
4. No validation/repair of blocks with missing required fields
5. Error manifests intermittently based on conversation history content types

---

## MessageContent Marshaling Behavior

**File:** `pkg/api/anthropic/types.go:94-100`

```go
func (mc MessageContent) MarshalJSON() ([]byte, error) {
    // ... build merged content ...

    // If single text block, marshal as string
    if len(merged) == 1 && merged[0].Type == "text" {
        return json.Marshal(merged[0].Text)
    }

    // Otherwise marshal as array
    return json.Marshal(merged)
}
```

**Key insight:** Only single text blocks become strings. Everything else (single image, single thinking, single tool_result) becomes an array.

---

## The Fix

### Step 1: Replace Normalization Function

**File:** `internal/transformer/transformers/anthropic.go`

Replace `normalizeThinkingBlockMessages` with:

```go
// normalizeSingleElementContent ensures ANY single-element content array
// (not just thinking) is normalized to prevent provider validation errors.
// This handles: single thinking, single image, single tool_result, etc.
func normalizeSingleElementContent(req *anthropic.Request) {
    for i := range req.Messages {
        // Only process assistant messages
        if req.Messages[i].Role != anthropic.RoleAssistant {
            continue
        }

        content := req.Messages[i].Content
        if len(content) == 0 {
            continue
        }

        // Check if content has a text block (content would be valid as array)
        hasTextBlock := false
        for _, block := range content {
            if block.Type == "text" {
                hasTextBlock = true
                break
            }
        }

        // If single-element array WITHOUT text block, add text block
        // This creates a multi-element array that passes validation
        if len(content) == 1 && !hasTextBlock {
            req.Messages[i].Content = append(content, anthropic.ContentBlock{
                Type: "text",
                Text: " ",
            })
        }
    }
}
```

### Step 2: Add Block Validation

**File:** `internal/transformer/transformers/anthropic.go`

```go
// validateAndRepairBlocks checks for blocks with missing/invalid required fields
// and repairs them where possible.
func validateAndRepairBlocks(req *anthropic.Request) {
    for i := range req.Messages {
        for j := range req.Messages[i].Content {
            block := &req.Messages[i].Content[j]

            // Validate image blocks
            if block.Type == "image" && block.Source != nil {
                if block.Source.Data == "" {
                    // Replace invalid image with text placeholder
                    block.Type = "text"
                    block.Text = "[Image: data unavailable]"
                    block.Source = nil
                }
            }

            // Validate thinking blocks
            if block.Type == "thinking" && block.Thinking == "" {
                block.Type = "text"
                block.Text = "[Thinking: content unavailable]"
            }
        }
    }
}
```

### Step 3: Update Call Sites

**File:** `internal/transformer/transformers/openrouter.go:97`

```go
// Replace:
normalizeThinkingBlockMessages(&reqCopy)

// With:
normalizeSingleElementContent(&reqCopy)
validateAndRepairBlocks(&reqCopy)
```

Also update:
- `internal/transformer/transformers/glm_anthropic.go`

---

## Test Coverage

Add tests for:

1. **Single image block** → should add text block
2. **Single tool_result block** → should add text block
3. **Thinking + image (mixed)** → should add text block
4. **Content with missing data fields** → should be repaired

---

## Prevention Strategy

1. **Comprehensive normalization:** Normalize ALL single-element content, not just thinking
2. **Block validation:** Validate required fields before sending to provider
3. **Defensive marshaling:** Consider always ensuring at least one text block in assistant messages
4. **Testing:** Add edge case tests for all content block types

---

## Related Issues and Fixes

| Date | Issue | Fix |
|------|-------|-----|
| 2026-03-03 | Thinking blocks used wrong field name ("content" vs "thinking") | Added Thinking/Signature fields |
| 2026-03-04 | Single thinking blocks rejected by OpenRouter | Initial normalization |
| 2026-03-10 | Multiple thinking blocks not normalized | Expanded normalization |
| 2026-03-11 | Other single-element content not normalized | **Current fix** |

---

## References

- Error logs: `~/.cc-modelrouter/logs/inst_*.log`
- Test file: `internal/transformer/transformers/normalization_test.go`
- Core logic: `pkg/api/anthropic/types.go:50-101`
- Transformer: `internal/transformer/transformers/openrouter.go`