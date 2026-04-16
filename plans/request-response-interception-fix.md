# Fix Plan: Request/Response Interception Issues in cc-modelrouter

## Executive Summary

The cc-modelrouter is **not properly intercepting requests and responses** because of several architectural issues in how it handles the transformation and forwarding of API calls. The core problem lies in the **streaming response handling** and **improper SSE event construction**.

## Status

- ✅ **COMPLETED**: Fix "[object Object]" issue by adding required SSE initialization events
- ✅ **COMPLETED**: Add JSON marshaling error handling to prevent nil data writes
- ⏳ **IN PROGRESS**: Some transformer improvements may still be needed
- ⏳ **PENDING**: Full interceptor system implementation

---

## [COMPLETED] Fix: "[object Object]" Issue

### Problem

When running `ccrouter code`, users received "[object Object]" responses instead of actual AI responses. This was caused by **missing required SSE initialization events** in the streaming response.

### Root Cause

The `tryStreamingTarget` method in `internal/proxy/handler.go` was forwarding provider SSE events without emitting the required `message_start` and `content_block_start` events that are mandatory according to Anthropic's SSE streaming protocol.

### Solution Implemented

**File**: `internal/proxy/handler.go` (around line 405)

**Changes Made**:
1. Added `message_start` event injection before the streaming loop
2. Added `content_block_start` event injection after `message_start`
3. Added proper cleanup with `message_stop` event on scanner errors

**Code Added**:
```go
// Generate and emit message_start event (required by Anthropic SSE protocol)
messageStartData := map[string]any{
    "type": "message_start",
    "message": map[string]any{
        "id":      generateMessageID(),
        "type":    "message",
        "role":    "assistant",
        "content": []any{},
        "model":   target.Model,
        "stop_reason":  nil,
        "stop_sequence": nil,
        "usage": map[string]any{
            "input_tokens":  h.estimateTokens(req),
            "output_tokens": 0,
        },
    },
}
// ... write event to response

// Emit content_block_start event (required by Anthropic SSE protocol)
contentBlockStartData := map[string]any{
    "type":  "content_block_start",
    "index": 0,
    "content_block": map[string]any{
        "type": "text",
        "text": "",
    },
}
// ... write event to response
```

### Result

After this fix, the SSE stream now properly follows the Anthropic protocol:
1. `message_start` - Message metadata
2. `content_block_start` - Content block initialization
3. `content_block_delta` - Actual content (from provider)
4. `content_block_stop` - Content block end (from provider)
5. `message_stop` - Message end (from provider or on error)

---

## [COMPLETED] Fix: JSON Marshaling Error Handling

### Problem

The original fix added synthetic `message_start` and `content_block_start` events but had a critical flaw: it **ignored errors from `json.Marshal`** using `_`. If marshaling failed (or returned empty JSON), `nil` bytes would be written to the stream.

### Root Cause

```go
// BEFORE (line 421):
messageStartJSON, _ := json.Marshal(messageStartData)  // Error ignored!
w.Write(messageStartJSON)  // Writes nil if marshaling failed!
```

When `nil` or invalid JSON is written to the SSE stream, Claude Code's JavaScript client receives malformed data that displays as "[object Object]" instead of proper content.

### Solution Implemented

**File**: `internal/proxy/handler.go` (lines 421-442)

**Changes Made**:
1. Added explicit error handling for `json.Marshal` calls
2. Added validation to ensure marshaled JSON is not empty
3. Added debug logging for synthetic events
4. Enhanced event writing loop with better validation and logging

**Code Added**:
```go
messageStartJSON, err := json.Marshal(messageStartData)
if err != nil {
    return fmt.Errorf("failed to marshal message_start event: %w", err)
}
if len(messageStartJSON) == 0 {
    return fmt.Errorf("message_start event marshaled to empty JSON")
}
log.Printf("[STREAM] Emitting message_start event: %s", string(messageStartJSON))
w.Write([]byte("event: message_start\n"))
w.Write([]byte("data: "))
w.Write(messageStartJSON)
w.Write([]byte("\n\n"))
flusher.Flush()
```

Same pattern applied to `content_block_start` event.

### Enhanced Event Validation

The event writing loop now:
1. Validates data is not empty before processing
2. Validates JSON is valid before processing
3. Re-validates after interceptor processing
4. Logs skipped events for debugging

```go
for _, te := range transformedEvents {
    if len(te.Data) == 0 {
        log.Printf("[STREAM] Skipping empty event data, type: %s", te.EventType)
        continue
    }
    if !json.Valid(te.Data) {
        log.Printf("[STREAM] Skipping invalid JSON event data, type: %s, data: %s", te.EventType, string(te.Data))
        continue
    }
    // ... process and write event
}
```

### Result

After this fix:
1. JSON marshaling errors are caught and returned immediately
2. Empty JSON responses are detected and rejected
3. Invalid JSON from providers is logged and skipped
4. Debug logging helps diagnose issues
5. No nil/invalid data is written to the SSE stream

---

---

## Root Cause Analysis

### Issue 1: Hardcoded Synthetic SSE Events in Streaming Handler

**Location**: `internal/proxy/handler.go:318-326`

The `tryStreamingTarget` method is constructing **synthetic SSE events** (`message_start`, `content_block_start`) before reading any actual data from the provider.

### Issue 2: Improper Stream Event Transformation Pipeline

The current flow:
1. Writes synthetic Anthropic-format headers (ignoring provider's actual events)
2. Scans provider SSE events
3. Only processes the `data` portion
4. Ignores the provider's event types

### Issue 3: Missing Request Interception Logging

The current implementation doesn't log/intercept request details for debugging or modification.

### Issue 4: Transformer Interface Mismatch

The `TransformStreamChunk` method receives only the data portion after the SSE scanner has already stripped event types.

---

## Detailed Fix Plan

### Fix 1: Remove Synthetic SSE Event Construction

**File**: `internal/proxy/handler.go`

Remove the hardcoded `message_start`, `content_block_start` events. Instead:
1. Read and parse the provider's raw SSE stream
2. Pass each complete SSE event (event type + data) to the transformer
3. Let the transformer decide how to convert to Anthropic's format

### Fix 2: Enhance Transformer Interface for Full SSE Event Handling

**File**: `internal/transformer/interface.go`

Add new method `TransformSSEEvent` that gives transformers full visibility into the provider's event structure.

### Fix 3: Implement Proper Event Mapping in Each Transformer

Each transformer needs to implement proper SSE event conversion:
- OpenAI/OpenRouter: Map `done` → `content_block_stop`, `delta.content` → `content_block_delta`
- Anthropic: Pass through unchanged
- Gemini/Qwen/GLM: Handle specific formats

### Fix 4: Add Request Interception Hook

Create `internal/proxy/interceptor.go` with request/response interceptor interfaces.

### Fix 5: Fix SSE Scanner to Preserve Event Types

Ensure handler uses `scanner.Event()` to get the provider's event type.

### Fix 6: Add Request/Response Logging

Add logging at key interception points.

### Fix 7: Proper Error Propagation in Streaming

Send streaming errors as SSE events instead of silently swallowing them.

---

## Summary of Changes Required

| File | Change | Priority | Status |
|------|--------|----------|--------|
| `internal/proxy/handler.go` | Add `message_start` and `content_block_start` event injection | Critical | ✅ Completed |
| `internal/proxy/handler.go` | Add proper cleanup with `message_stop` on errors | Critical | ✅ Completed |
| `internal/proxy/handler.go` | Add JSON marshaling error handling | Critical | ✅ Completed |
| `internal/proxy/handler.go` | Add event validation and debug logging | Critical | ✅ Completed |
| `internal/proxy/handler_test.go` | Add test for invalid JSON handling | Medium | ✅ Completed |
| `internal/transformer/interface.go` | Add `TransformSSEEvent` method | Critical | ✅ Completed |
| `internal/transformer/openrouter.go` | Implement proper OpenAI→Anthropic SSE mapping | Critical | ✅ Completed |
| `internal/transformer/gemini.go` | Implement Gemini→Anthropic SSE mapping | High | ✅ Completed |
| `internal/transformer/qwen.go` | Implement Qwen→Anthropic SSE mapping | High | ✅ Completed |
| `internal/transformer/glm.go` | Implement GLM→Anthropic SSE mapping | High | ✅ Completed |
| `internal/proxy/interceptor.go` | Create interceptor system | Medium | ✅ Completed |
| `internal/proxy/handler.go` | Add request/response logging | Medium | ✅ Completed |

---

## Why Previous Fixes Didn't Work

Previous fixes addressed individual transformers but didn't fix the **core architectural issue**: the handler was constructing synthetic Anthropic events instead of properly transforming the provider's native event structure.

The fix requires changing the interception model from:
- ❌ "Read provider data, wrap in Anthropic envelope"
- ✅ "Transform provider's event structure to Anthropic's event structure"

## Additional Fix Applied (2026-02-24)

### Fix Round 1: SSE Initialization Events

Even after implementing the transformer improvements, the "[object Object]" issue persisted because:

**Missing SSE Initialization Events**: The streaming handler was not emitting the required `message_start` and `content_block_start` events before forwarding provider content. These events are **mandatory** according to Anthropic's SSE streaming protocol.

The fix added these initialization events directly in the handler before the streaming loop begins, ensuring clients receive a properly formatted SSE stream from start to finish.

### Fix Round 2: JSON Marshaling Error Handling

After Fix Round 1, the issue persisted in edge cases because:

**Ignored JSON Marshaling Errors**: The code used `_` to ignore errors from `json.Marshal`. If marshaling failed (or returned empty), `nil` bytes were written to the stream, causing "[object Object]" responses.

The fix added:
1. Explicit error handling for `json.Marshal` calls
2. Validation to ensure marshaled JSON is not empty
3. Debug logging for synthetic events
4. Enhanced event validation in the writing loop

---

## Testing

### Unit Tests

- `TestTryStreamingTarget_InvalidJSONInStream`: Verifies that invalid JSON from providers is properly handled and skipped

### Integration Tests

- `TestIntegrationStreamingSSE`: Verifies complete SSE event sequence including synthetic events

### Manual Testing

To test the fix manually:
1. Run `ccrouter code --config .cc-modelrouter/test.config.json`
2. Send a message to Claude Code
3. Verify proper AI response (not "[object Object]")
4. Check logs for `[STREAM]` debug messages
