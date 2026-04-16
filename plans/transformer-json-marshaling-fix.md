# Transformer JSON Marshaling Error Handling Fix Plan

**Date**: 2026-02-24
**Status**: DRAFT - Not Implemented
**Related Issues**: Similar to the "[object Object]" issue fix in `handler.go`

---

## Executive Summary

After investigating the transformers following the previous "[object Object]" fix in the streaming handler, **identical JSON marshaling error handling issues** were found in three transformers: `OpenRouterTransformer`, `GeminiTransformer`, and `QwenTransformer`.

These transformers ignore errors from `json.Marshal()` calls using `_`, which can result in `nil` bytes being written to the SSE stream, potentially causing "[object Object]" responses or stream corruption.

---

## Background: Previous Fix

The previous fix in `internal/proxy/handler.go` addressed the same issue:

**Before (buggy code):**
```go
messageStartJSON, _ := json.Marshal(messageStartData)  // Error ignored!
w.Write(messageStartJSON)  // Writes nil if marshaling failed!
```

**After (fixed code):**
```go
messageStartJSON, err := json.Marshal(messageStartData)
if err != nil {
    return fmt.Errorf("failed to marshal message_start event: %w", err)
}
if len(messageStartJSON) == 0 {
    return fmt.Errorf("message_start event marshaled to empty JSON")
}
```

The same pattern needs to be applied to the transformers.

---

## Issue Analysis

### Issue 1: OpenRouterTransformer (`internal/transformer/openrouter.go`)

**Location**: `TransformSSEEvent` method (lines 189-265)

**Problematic Code:**
```go
// Line 208-214: stopData - Error ignored!
stopData, _ := json.Marshal(map[string]any{
    "type":  "content_block_stop",
    "index": 0,
})

// Line 218-224: messageStopData - Error ignored!
messageStopData, _ := json.Marshal(map[string]string{
    "type": "message_stop",
})

// Line 250: data - Error ignored!
data, _ := json.Marshal(anthropicDelta)
```

**Impact**: If any JSON marshaling fails, `nil` bytes are written to the stream, causing malformed SSE events.

---

### Issue 2: GeminiTransformer (`internal/transformer/gemini.go`)

**Location**: `TransformSSEEvent` method (lines 279-340)

**Problematic Code:**
```go
// Line 318-321: stopData - Error ignored!
stopData, _ := json.Marshal(map[string]any{
    "type":  "content_block_stop",
    "index": 0,
})

// Line 327-330: messageStopData - Error ignored!
messageStopData, _ := json.Marshal(map[string]string{
    "type": "message_stop",
})

// Line 305: data - Error ignored!
data, _ := json.Marshal(anthropicChunk)
```

**Impact**: Same as OpenRouter - nil bytes on marshaling failure.

---

### Issue 3: QwenTransformer (`internal/transformer/qwen.go`)

**Location**: `TransformSSEEvent` method (lines 239-316)

**Problematic Code:**
```go
// Line 259-262: stopData - Error ignored!
stopData, _ := json.Marshal(map[string]any{
    "type":  "content_block_stop",
    "index": 0,
})

// Line 269-273: messageStopData - Error ignored!
messageStopData, _ := json.Marshal(map[string]string{
    "type": "message_stop",
})

// Line 301: data - Error ignored!
data, _ := json.Marshal(anthropicDelta)
```

**Impact**: Same as OpenRouter and Gemini.

---

### Issue 4: Handler.go - message_stop Event on Error

**Location**: `tryStreamingTarget` method (lines 488-499)

**Problematic Code:**
```go
// Line 490-495: messageStopData - Error ignored!
messageStopData, _ := json.Marshal(map[string]string{
    "type": "message_stop",
})
w.Write([]byte("event: message_stop\n"))
w.Write([]byte("data: "))
w.Write(messageStopData)  // Could be nil!
w.Write([]byte("\n\n"))
```

**Impact**: On scanner error, the cleanup `message_stop` event could be malformed.

---

## Provider API Streaming Formats

### OpenRouter (OpenAI-Compatible)

**SSE Format:**
- Standard OpenAI streaming format
- `delta.content` contains incremental text
- `finish_reason` indicates stream completion

**Reference:** [OpenRouter Documentation](https://openrouter.ai/)

### Qwen (Alibaba Cloud / DashScope)

**SSE Format:**
- OpenAI-compatible via `https://dashscope.aliyuncs.com/compatible-mode/v1`
- Uses `choices[].delta.content` for text chunks
- Uses `choices[].finish_reason` for completion

**References:**
- [Qwen API Reference](https://www.alibabacloud.com/help/en/model-studio/use-qwen-by-calling-api)
- [Stream Mode Documentation](https://www.alibabacloud.com/help/en/model-studio/stream)

### Gemini (Google)

**SSE Format:**
- Unique format using `candidates[0].content.parts[0].text`
- `finishReason` indicates completion
- Can use OpenAI-compatible endpoint on Vertex AI

**References:**
- [OpenAI Compatibility on Vertex AI](https://cloud.google.com/vertex-ai/generative-ai/docs/start/openai?hl=zh-cn)
- [Vertex AI Examples](https://cloud.google.com/vertex-ai/generative-ai/docs/migrate/openai/examples?hl=zh-cn)

---

## Fix Plan

### Fix 1: OpenRouterTransformer - Add JSON Marshaling Error Handling

**File:** `internal/transformer/openrouter.go`

**Changes Required:**

1. In `TransformSSEEvent`, replace all `json.Marshal` calls with proper error handling:

```go
// BEFORE (Line 208-214):
stopData, _ := json.Marshal(map[string]any{
    "type":  "content_block_stop",
    "index": 0,
})

// AFTER:
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
```

2. Same pattern for `messageStopData` (line 218-224):

```go
messageStopData, err := json.Marshal(map[string]string{
    "type": "message_stop",
})
if err != nil {
    return nil, fmt.Errorf("failed to marshal message_stop event: %w", err)
}
if len(messageStopData) == 0 {
    return nil, fmt.Errorf("message_stop event marshaled to empty JSON")
}
```

3. Same pattern for `data` in delta processing (line 250):

```go
data, err := json.Marshal(anthropicDelta)
if err != nil {
    return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
}
if len(data) == 0 {
    return nil, fmt.Errorf("content_block_delta event marshaled to empty JSON")
}
```

---

### Fix 2: GeminiTransformer - Add JSON Marshaling Error Handling

**File:** `internal/transformer/gemini.go`

**Changes Required:**

1. In `TransformSSEEvent`, replace all `json.Marshal` calls with proper error handling:

```go
// BEFORE (Line 305):
data, _ := json.Marshal(anthropicChunk)

// AFTER:
data, err := json.Marshal(anthropicChunk)
if err != nil {
    return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
}
if len(data) == 0 {
    return nil, fmt.Errorf("content_block_delta event marshaled to empty JSON")
}
```

2. Same pattern for `stopData` (line 318-321) and `messageStopData` (line 327-330).

---

### Fix 3: QwenTransformer - Add JSON Marshaling Error Handling

**File:** `internal/transformer/qwen.go`

**Changes Required:**

1. In `TransformSSEEvent`, replace all `json.Marshal` calls with proper error handling:

Same pattern as OpenRouter transformer (lines 259-262, 269-273, 301).

---

### Fix 4: Handler - Add JSON Marshaling Error Handling for Cleanup Event

**File:** `internal/proxy/handler.go`

**Changes Required:**

1. In `tryStreamingTarget`, line 490-495:

```go
// BEFORE:
messageStopData, _ := json.Marshal(map[string]string{
    "type": "message_stop",
})

// AFTER:
messageStopData, err := json.Marshal(map[string]string{
    "type": "message_stop",
})
if err != nil {
    log.Printf("[STREAM] Failed to marshal message_stop event: %v", err)
    // Continue without writing the event - stream is already in error state
} else if len(messageStopData) > 0 {
    w.Write([]byte("event: message_stop\n"))
    w.Write([]byte("data: "))
    w.Write(messageStopData)
    w.Write([]byte("\n\n"))
    flusher.Flush()
}
```

---

## Testing Strategy

### Unit Tests

Add tests for each transformer to verify:

1. **JSON marshaling error handling** - Mock `json.Marshal` to return error, verify error is propagated
2. **Empty JSON detection** - Verify empty JSON is rejected
3. **Valid JSON passes through** - Verify normal operation still works

Example test structure:
```go
func TestOpenRouterTransformer_TransformSSEEvent_MarshalError(t *testing.T) {
    // Test that marshal errors are properly handled
}

func TestOpenRouterTransformer_TransformSSEEvent_EmptyJSON(t *testing.T) {
    // Test that empty JSON is rejected
}
```

### Integration Tests

1. **OpenRouter streaming test** - Verify proper SSE events with real/mock OpenRouter API
2. **Qwen streaming test** - Verify proper SSE events with real/mock Qwen API
3. **Gemini streaming test** - Verify proper SSE events with real/mock Gemini API

### Manual Testing

1. Run `ccrouter code` with each provider
2. Verify no "[object Object]" responses
3. Check logs for `[STREAM]` debug messages
4. Verify complete SSE event sequence

---

## Files to Modify

| File | Changes | Priority |
|------|---------|----------|
| `internal/transformer/openrouter.go` | Add JSON marshaling error handling in `TransformSSEEvent` | **CRITICAL** |
| `internal/transformer/gemini.go` | Add JSON marshaling error handling in `TransformSSEEvent` | **CRITICAL** |
| `internal/transformer/qwen.go` | Add JSON marshaling error handling in `TransformSSEEvent` | **CRITICAL** |
| `internal/proxy/handler.go` | Add JSON marshaling error handling for cleanup event | **HIGH** |
| `internal/transformer/openrouter_test.go` | Add unit tests for error handling | **MEDIUM** |
| `internal/transformer/gemini_test.go` | Add unit tests for error handling | **MEDIUM** |
| `internal/transformer/qwen_test.go` | Add unit tests for error handling | **MEDIUM** |

---

## Risk Assessment

### Low Risk Changes
- The fixes are straightforward error handling additions
- The pattern is already tested in `handler.go`
- No change to the transformation logic itself

### Medium Risk Considerations
- If a provider consistently returns malformed data, streams will fail more visibly (better than silent corruption)
- Error handling may increase log verbosity during failures

### Mitigation
- The handler already validates JSON at lines 451-454 and 469-472
- Returning errors from `TransformSSEEvent` causes the event to be skipped with a log message (line 441-443)

---

## Rollout Plan

1. **Phase 1**: Add error handling to all three transformers
2. **Phase 2**: Add unit tests for error handling
3. **Phase 3**: Run integration tests with each provider
4. **Phase 4**: Manual testing with `ccrouter code`
5. **Phase 5**: Deploy and monitor logs for any unexpected errors

---

## Related Documents

- [SSE Debug and Fix Plan](./sse-debug-and-fix-plan.md) - Previous fix for similar issue in handler
- [Request/Response Interception Fix](./request-response-interception-fix.md) - Context on SSE event handling
- [Fix Summary 2026-02-24](./fix-summary-2026-02-24.md) - Summary of previous "[object Object]" fix

---

## Appendix: Provider Documentation Links

### OpenRouter
- Official Site: https://openrouter.ai/
- OpenAI-Compatible API: `/api/v1/chat/completions`

### Qwen (Alibaba Cloud)
- API Reference: https://www.alibabacloud.com/help/en/model-studio/use-qwen-by-calling-api
- Stream Mode: https://www.alibabacloud.com/help/en/model-studio/stream
- Chinese Docs: https://help.aliyun.com/zh/dashscope/developer-reference/api-details

### Gemini (Google)
- Vertex AI OpenAI Compatibility: https://cloud.google.com/vertex-ai/generative-ai/docs/start/openai?hl=zh-cn
- Migration Examples: https://cloud.google.com/vertex-ai/generative-ai/docs/migrate/openai/examples?hl=zh-cn
