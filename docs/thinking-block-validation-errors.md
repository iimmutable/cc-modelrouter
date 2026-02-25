# Thinking Block Validation Errors - Knowledge Base

**Issue ID:** THINK-BLOCK-001
**Status:** Root Cause Identified - Fix Plan Ready
**Severity:** Critical - Causes all OpenRouter Anthropic requests with thinking block history to fail
**Affected Components:** OpenRouter Transformer, GLM Transformer
**Discovery Date:** 2026-03-11
**Analysis Date:** 2026-03-11

## Executive Summary

OpenRouter Anthropic requests fail with "Invalid input: expected string, received array" errors when the conversation history contains assistant messages with thinking blocks (from previous GLM responses). This occurs on direct requests to OpenRouter (Target 0), not during failover, because the OpenRouter transformer incorrectly assumes Anthropic models accept single thinking blocks without normalization.

## Problem Statement

### Symptoms
- 400 Bad Request errors from OpenRouter on **direct requests** (Target 0)
- Error message: `"Invalid input: expected string, received array"`
- Error path: `["messages", 1, "content"]` (first assistant message in conversation history)
- Error occurs **after** GLM has added assistant messages with thinking blocks to conversation history
- All subsequent OpenRouter Anthropic requests fail until conversation is cleared

### Reproduction Steps
1. Configure routes with GLM for `default` and OpenRouter Anthropic for `thinkMore`
2. Send a request with extended thinking (triggers `thinkMore` route)
3. First `thinkMore` request succeeds (no thinking blocks in history yet)
4. Send regular requests that go to GLM (`default` route)
5. GLM returns responses with thinking blocks, added to conversation history
6. Send another `thinkMore` request
7. **FAILS** with 400 error on Target 0 (direct request to OpenRouter)

### Error Pattern
```
Timeline from logs:
09:56:06 - thinkMore to OpenRouter → SUCCESS (no thinking blocks in history)
09:56:19-09:57:02 - Multiple default requests to GLM → SUCCESS
09:57:17 - thinkMore to OpenRouter → FAIL (Target 0, not failover!)

Error: "Invalid input: expected string, received array"
Path: ["messages", 1, "content"]
```

The error occurs at `messages[1].content` - the first assistant message in conversation history (from GLM response).

## Root Cause Analysis

### The Critical Assumption Error

**Code in `openrouter.go` (lines 83-90):**
```go
// OpenRouter-specific: Handle thinking blocks differently based on target model
// Anthropic models (via OpenRouter): Do NOT normalize (accepts single thinking blocks)
// Other models (Google, etc.): Normalize thinking blocks (require multi-element arrays)
isAnthropicModel := strings.HasPrefix(model, "anthropic/")

if !isAnthropicModel {
    // For non-Anthropic models, normalize thinking blocks
    normalizeThinkingBlockMessages(&reqCopy)
}
```

**This comment is WRONG:** "Anthropic models (via OpenRouter): Do NOT normalize (accepts single thinking blocks)"

**Reality:** OpenRouter's validation is **stricter than direct Anthropic API**. OpenRouter rejects single-element content arrays for thinking blocks.

### Technical Flow

1. **First `thinkMore` request:**
   - Conversation history: `[user_msg]`
   - No assistant messages with thinking blocks yet
   - OpenRouter transformer processes: No thinking blocks to normalize
   - Request succeeds ✓

2. **GLM `default` requests:**
   - GLM normalizes request: `[{thinking}, {text: " "}]`
   - GLM returns assistant response with thinking blocks
   - Thinking blocks added to conversation history
   - Format: `[{type: "thinking", thinking: "..."}]` (single-element array)

3. **Second `thinkMore` request:**
   - Conversation history now contains: `[user, assistant_with_thinking, user, ...]`
   - Handler creates deep copy of request (preserves thinking block format)
   - OpenRouter transformer processes:
     - `convertUserThinkingToText()` - Only processes USER messages
     - Check: `isAnthropicModel = true`
     - **SKIPS** `normalizeThinkingBlockMessages()` for Anthropic models
   - Assistant thinking blocks remain as single-element arrays
   - OpenRouter validates: **FAILS** ✗

4. **Why validation fails:**
   - Content format: `"content": [{type: "thinking", ...}]`
   - OpenRouter expects: String OR multi-element array
   - Rejects: Single-element array
   - Error: "expected string, received array"

### Why Deep Copy Doesn't Help

The handler creates a deep copy **before** calling the transformer:
```go
// In handler.go tryStreamingTarget()
reqCopy, err := deepCopyRequest(req)
// ...
httpReq, err := tf.PrepareRequest(reqCopy, ...)
```

The transformer then works with the copy. If the original request (from conversation history) contains single thinking blocks, the deep copy preserves them. The transformer then:
- Processes user messages only (convertUserThinkingToText)
- Skips normalization for Anthropic models
- Single thinking blocks remain unnormalized
- OpenRouter rejects them

### The Validation Schema

OpenRouter's error shows two validation failures for each thinking block:
1. `"expected string, received array"` - Content is array, not string
2. `"expected string, received undefined" at [0, "signature"]` - In the union validation

This indicates OpenRouter's schema expects content to be either:
- A string (for simple text content)
- A multi-element array (for complex content)
- NOT a single-element array

## The Fix

### Solution: Always Normalize for OpenRouter

**File:** `internal/transformer/transformers/openrouter.go`

**Change:** Remove conditional check that skips normalization for Anthropic models

```diff
  // OpenRouter-specific: Handle thinking blocks differently based on target model
  // Anthropic models (via OpenRouter): Do NOT normalize (accepts single thinking blocks)
  // Other models (Google, etc.): Normalize thinking blocks (require multi-element arrays)
  isAnthropicModel := strings.HasPrefix(model, "anthropic/")

- if !isAnthropicModel {
-     // For non-Anthropic models, normalize thinking blocks
+ // CRITICAL FIX: Always normalize thinking blocks for OpenRouter
+ // OpenRouter's validation is stricter than direct Anthropic API
+ // Both Anthropic and non-Anthropic models via OpenRouter require multi-element arrays
+ // to prevent "expected string, received array" validation errors
  normalizeThinkingBlockMessages(&reqCopy)
- }
```

### How It Works

1. **`normalizeThinkingBlockMessages` function:**
   - Iterates through all **assistant** messages (not user messages)
   - Finds messages with only a single thinking block
   - Appends a text block with single space: `" "`
   - Result: `[{thinking}, {text: " "}]` - multi-element array

2. **Why Multi-Element Array Works:**
   - OpenRouter accepts multi-element arrays
   - The single space text block is minimal overhead
   - Preserves thinking content while passing validation

3. **Signature Handling:**
   - Still sets signature to `&""` for Anthropic models
   - Ensures field is present in JSON
   - Clears existing signatures from other providers

### Code Flow After Fix

```
Request with GLM thinking blocks in history
         ↓
Handler creates deep copy
         ↓
OpenRouterTransformer.PrepareRequest()
         ↓
convertUserThinkingToText() - User messages only
         ↓
normalizeThinkingBlockMessages() - ALL assistant messages (NEW)
         ↓
Set signature to &"" for Anthropic models
         ↓
Marshal and send to OpenRouter
         ↓
OpenRouter validates: PASS ✓
```

## Validation

### Test Cases

1. **Direct OpenRouter Request (no history):**
   - Request with thinking block → Normalized → Passes

2. **After GLM Responses (conversation history):**
   - GLM returns thinking → History → OpenRouter → Normalized → Passes

3. **Multiple Thinking Blocks in History:**
   - Multiple assistant messages with thinking → All normalized → Passes

4. **User Messages with Thinking:**
   - User messages with thinking → Converted to text → Passes

5. **Mixed Provider History:**
   - GLM + OpenRouter responses in history → All normalized → Passes

### Expected Behavior After Fix

**Before:**
```
[INFO] [STREAM] Starting stream to openrouter-anthropic/anthropic/claude-opus-4.5
[ERROR] [PROXY STREAM ERROR] URL: https://openrouter.ai/api/v1/messages, Status: 400 Bad Request
[ERROR] Target 0 (openrouter-anthropic/anthropic/claude-opus-4.5) failed
"Invalid input: expected string, received array"
```

**After:**
```
[INFO] [STREAM] Starting stream to openrouter-anthropic/anthropic/claude-opus-4.5
[INFO] [STREAM] [STREAM SUMMARY] Stream completed
[INFO] [STREAM] Stream completed successfully
```

## Related Issues

### Also Fixed in Same Commit

1. **User Message Thinking Blocks:**
   - Function: `convertUserThinkingToText`
   - Converts thinking blocks in user messages to text with `<thinking>` tags
   - Prevents format issues when Claude Code resends assistant responses

2. **Assistant Message Normalization:**
   - Function: `normalizeThinkingBlockMessages`
   - Only processes assistant messages (not user messages)
   - Adds text block to single thinking block messages

3. **Signature Field Type:**
   - Changed from `string` to `*string`
   - Allows distinguishing "omit field" vs "include empty string"

## Prevention

### Code Review Checklist

When modifying thinking block handling:

- [ ] Always normalize for OpenRouter (both Anthropic and non-Anthropic models)
- [ ] Only normalize assistant messages (not user messages)
- [ ] Convert user thinking blocks to text (not normalize)
- [ ] Set signature to `&""` for OpenRouter Anthropic models
- [ ] Set signature to `nil` for direct Anthropic API
- [ ] Always deep copy requests before modification
- [ ] Test with conversation history containing thinking blocks

### Testing Checklist

- [ ] Test direct request to OpenRouter with thinking blocks
- [ ] Test after GLM responses (conversation history with thinking)
- [ ] Test with multiple thinking blocks in history
- [ ] Test with user messages containing thinking blocks
- [ ] Verify signature field is correctly set/omitted
- [ ] Test both thinkMore and default routes

## Implementation Plan

### Files to Modify
- `internal/transformer/transformers/openrouter.go` (lines 85-90)
- Update comment to reflect correct behavior

### Steps
1. Remove `if !isAnthropicModel` conditional
2. Add comment explaining why normalization is always needed
3. Update documentation
4. Add test case for conversation history with thinking blocks
5. Verify fix with integration test

## References

### Related Documentation
- [Troubleshooting Guide](troubleshooting.md#thinking-block-validation-errors-critical)
- [Transformer Guide](transformers.md#thinking-block-handling-critical)
- [Architecture Documentation](architecture.md)

### Analysis Logs
- Log file: `~/.cc-modelrouter/logs/inst_20260311_095546.log`
- Error occurs at line 134 (Target 0 - direct request, not failover)

## Lessons Learned

1. **OpenRouter ≠ Direct Anthropic API:**
   - OpenRouter has its own validation layer
   - Cannot assume compatibility based on model name
   - Must test with actual conversation history scenarios

2. **Conversation History Matters:**
   - Direct requests may work (empty history)
   - Requests with history fail (contains thinking blocks)
   - Test both scenarios

3. **Comments Can Be Wrong:**
   - Old comment: "Anthropic models: Do NOT normalize"
   - This was incorrect and caused the bug
   - Keep documentation in sync with reality

4. **Target 0 ≠ Failover:**
   - Target 0 means FIRST target (direct request)
   - Not a fallback from previous provider
   - Root cause is conversation history, not failover state

5. **Deep Copy Preserves Problems:**
   - Handler copies request before transformer
   - Transformer processes the copy
   - If copy contains bad format, transformer must fix it
   - Deep copy doesn't solve validation issues
