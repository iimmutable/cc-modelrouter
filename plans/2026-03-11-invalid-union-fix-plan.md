# Comprehensive Root Cause Analysis: API Request Validation Errors

## Executive Summary

The recurring "Invalid Anthropic Messages API request" errors with `invalid_union` and `invalid_type` codes are caused by **incomplete normalization of thinking block content** in the OpenRouter transformer. The bug manifests when conversation history accumulates multiple thinking blocks from GLM responses that aren't properly normalized before being sent to OpenRouter.

---

## Error Evidence

From log file `inst_20260311_103734.log`:

```
Path: ["messages", 1, "content"]
Error 1: "Invalid input: expected string, received array" at path []
Error 2: "Invalid input: expected string, received undefined" at path [3, "data"]
```

---

## Root Cause

### Primary Issue

**Location**: `internal/transformer/transformers/anthropic.go:97-117`

The `normalizeThinkingBlockMessages()` function **only adds a text block when there's exactly ONE thinking block**:

```go
if len(content) == 1 && content[0].Type == "thinking" {
    // Add text block
}
```

**What fails:**
- Multiple thinking blocks: `[{thinking}, {thinking}, {thinking}]` - `len > 1`, condition fails
- Thinking block at index > 0: `[{text}, {thinking}]` - `content[0].Type != "thinking"`, condition fails

### Why This Is Intermittent

The error occurred at `10:39:02` but succeeded at `10:38:13`. Between these requests:
- Multiple GLM responses accumulated in conversation history
- GLM uses extended thinking which generates multiple thinking blocks per response
- Eventually, a request contained enough thinking-only content to trigger the validation failure

---

## Affected Code Locations

| File | Line | Issue |
|------|------|-------|
| `internal/transformer/transformers/anthropic.go` | 97-117 | Incomplete normalization logic |
| `internal/transformer/transformers/anthropic.go` | 51-87 | `convertUserThinkingToText` - only handles user messages |
| `internal/transformer/transformers/openrouter.go` | 97 | Calls normalization but only works for single thinking block |
| `internal/transformer/transformers/glm_anthropic.go` | 73-74 | Same issue with normalization |

---

## Proposed Fix

### Fix 1: Expand Normalization Logic

**File**: `internal/transformer/transformers/anthropic.go`

Replace the single-condition check with logic that detects if content has ONLY thinking blocks (regardless of count):

```go
// Check if content has ONLY thinking blocks (no text blocks)
hasOnlyThinkingBlocks := len(content) > 0
hasTextBlock := false
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

// If only thinking blocks with no text, add a text block
if hasOnlyThinkingBlocks && !hasTextBlock {
    req.Messages[i].Content = append(content, anthropic.ContentBlock{
        Type: "text",
        Text: " ",
    })
}
```

---

## Test Strategy

1. Add test cases for edge cases in normalization:
   - Multiple thinking blocks in single message
   - Thinking block after text block (index > 0)
   - Mixed content with thinking blocks at various positions
2. Verify existing test cases still pass

---

## Status

- **Analysis**: COMPLETE
- **Fix Implementation**: COMPLETE
- **Testing**: PASSED