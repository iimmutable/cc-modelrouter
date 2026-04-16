# SSE Streaming Debug and Fix Plan

## Status: COMPLETED ✓

The fix has been successfully implemented and tested.

## Problem Summary
When running `ccrouter code` with Claude Code, responses contained "[object Object]" entries instead of properly formatted text.

## Root Cause Identified

The proxy was emitting **synthetic** `message_start` and `content_block_start` events for ALL providers. However, GLM's API already sends these events in Anthropic-compatible format. This caused **duplicate events**:

1. Proxy's synthetic `message_start` → GLM's `message_start`
2. Proxy's synthetic `content_block_start` → GLM's `content_block_start`
3. GLM's `ping` event (non-Anthropic, not filtered)

The duplicate events broke JavaScript's SSE parsing, causing "[object Object]" to appear.

## Fix Implemented

### 1. Removed Synthetic Event Emission
**File:** `internal/proxy/handler.go:405-457`

**Before:** The proxy emitted synthetic `message_start` and `content_block_start` events before forwarding the provider's stream.

**After:** The proxy now forwards the provider's events as-is, trusting Anthropic-compatible providers to send the correct events.

### 2. Added Non-Anthropic Event Filtering
**File:** `internal/proxy/handler.go:419-422`

```go
// Filter out non-Anthropic events that some providers send (e.g., GLM's "ping")
if eventType == "ping" || eventType == "keepalive" {
    log.Printf("[STREAM] Filtering out non-Anthropic event: %s", eventType)
    continue
}
```

### 3. Updated Integration Test
**File:** `test/integration_sse_test.go:22-85`

The mock provider now sends the complete SSE stream including `message_start` and `content_block_start` events, matching real provider behavior.

## Verification

### Test Results
```
=== RUN   TestIntegrationStreamingSSE
--- PASS: TestIntegrationStreamingSSE (0.00s)
PASS
```

### SSE Output After Fix
```
event: message_start
data: {"type": "message_start", "message": {...}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, ...}

event: content_block_delta
data: {"type": "content_block_delta", "delta": {"text": "Hello"}}

event: content_block_stop
data: {"type": "content_block_stop", ...}

event: message_delta
data: {"type": "message_delta", ...}

event: message_stop
data: {"type": "message_stop"}
```

- **No duplicate** `message_start` or `content_block_start` events
- **No `ping` event** (filtered out)
- Clean, valid SSE stream

## Files Modified

1. `internal/proxy/handler.go` - Removed synthetic event emission, added filtering
2. `internal/proxy/streaming.go` - Cleaned up debug logging
3. `test/integration_sse_test.go` - Updated to match new behavior

## Additional Notes

- GLM's `/api/anthropic` endpoint is Anthropic-compatible and requires no transformation
- The fix assumes all configured providers follow Anthropic's SSE format
- For providers that DON'T follow Anthropic format, a custom transformer would need to inject synthetic events
- The `ping` event filtering prevents non-standard events from breaking clients
