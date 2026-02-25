# Token Usage Tracking Root Cause Analysis

**Date:** 2026-03-02
**Issue:** Token usage tracking shows very low counts despite actual token consumption being much higher
**Status:** Root Cause Identified

---

## Executive Summary

The token usage tracking system in ccrouter has a **fundamental design flaw** where it estimates tokens from the **request only** (input), completely ignoring the **actual token usage** data returned by providers (input + output tokens).

### Key Finding
The tracking calls use `h.estimateTokens(req)` which only counts **input tokens** from the request, while the actual response contains accurate `InputTokens` and `OutputTokens` data that is **being logged but NOT tracked**.

---

## Phase 1: Root Cause Investigation

### 1.1 Evidence Gathering

#### Location 1: Non-streaming usage tracking (Line 235)
```go
// internal/proxy/handler.go:233-236
// Track usage
if h.usageTracker != nil {
    h.usageTracker.Record(h.instanceID, routeName, successfulModel,
        h.estimateTokens(req),  // ❌ ONLY INPUT TOKENS
        fallbackCount)
}
```

**Immediately after (Line 239-240):**
```go
logging.Infof("[RESPONSE] Success with %s/%s, StopReason: %s, InputTokens: %d, OutputTokens: %d",
    target.Provider, target.Model, resp.StopReason,
    resp.Usage.InputTokens, resp.Usage.OutputTokens)  // ✓ ACTUAL DATA AVAILABLE
```

**Issue:** The code logs the accurate token data but uses the estimated value for tracking.

#### Location 2: Streaming usage tracking (Line 291)
```go
// internal/proxy/handler.go:289-292
// Track usage on successful stream
if h.usageTracker != nil {
    h.usageTracker.Record(h.instanceID, routeName, target.Model,
        h.estimateTokens(req),  // ❌ ONLY INPUT TOKENS
        i)
}
```

**Issue:** Streaming responses also use the estimate, ignoring the actual usage data in `message_delta` events.

### 1.2 Data Flow Analysis

```
Provider Response (Accurate Data)
    │
    ├─► resp.Usage.InputTokens     ✓ Logged to console (line 240)
    ├─► resp.Usage.OutputTokens    ✓ Logged to console (line 240)
    │
    └─► Usage Tracker              ✗ Uses estimateTokens(req) - INPUT ONLY
```

### 1.3 The `estimateTokens` Method (Lines 600-611)

```go
func (h *Handler) estimateTokens(req *anthropic.Request) int {
    // Rough estimation: ~4 chars per token
    total := 0
    for _, msg := range req.Messages {
        for _, block := range msg.Content {
            if block.Type == "text" {
                total += len(block.Text) / 4  // ❌ INPUT TOKENS ONLY
            }
        }
    }
    return total
}
```

**Problems:**
1. Only counts input tokens (request text)
2. Uses crude approximation (÷4)
3. Completely ignores output tokens
4. Ignores system prompt, tools, images, and other token-consuming elements

---

## Phase 2: Pattern Analysis

### 2.1 What Does Work?

The logging system correctly captures and displays actual token usage:
```go
logging.Infof("[RESPONSE] Success with %s/%s, StopReason: %s, InputTokens: %d, OutputTokens: %d",
    target.Provider, target.Model, resp.StopReason,
    resp.Usage.InputTokens, resp.Usage.OutputTokens)
```

This proves the data is available and accurate.

### 2.2 Provider Response Structure

From `pkg/api/anthropic/types.go:113-116`:
```go
type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

This structure is part of the response and contains the actual token counts.

### 2.3 Streaming Events

For streaming, providers send `message_delta` events with usage data:
```json
{
  "type": "message_delta",
  "delta": {"stop_reason": "end_turn"},
  "usage": {"output_tokens": 123}
}
```

Evidence from transformer code:
- `internal/transformer/providers/openai.go:311-330` - Generates message_delta with output_tokens
- `internal/transformer/providers/openrouter.go:320-339` - Same pattern
- `internal/transformer/providers/glm.go` - Similar handling

**Current behavior:** These events are transformed and forwarded to client but **not captured** for usage tracking.

---

## Phase 3: Root Cause Confirmation

### 3.1 The Bug

**Primary Issue:** Usage tracking uses estimated input tokens instead of actual provider-reported usage data.

**Impact:**
- Usage reports show only ~25% of actual consumption (input only vs input+output)
- No visibility into output token consumption (typically 50-75% of total)
- Cost tracking is severely underestimated
- Decision-making based on usage data is flawed

### 3.2 Why This Happened

1. **Initial Implementation Shortcut:** The `estimateTokens` method was likely added as a quick placeholder
2. **Missing Follow-up:** The actual usage data was never connected to the tracking system
3. **Streaming Complexity:** Streaming responses require extracting usage from events, not just the final response

### 3.3 Hypothesis Testing

**Hypothesis:** The usage tracker receives only input token estimates.

**Test:** Check what `Record()` is called with.

**Evidence:**
- Line 235: `h.estimateTokens(req)` - passes input-only estimate
- Line 291: `h.estimateTokens(req)` - passes input-only estimate
- Actual data (`resp.Usage.InputTokens + OutputTokens`) is never passed to `Record()`

**Conclusion:** Hypothesis CONFIRMED.

---

## Phase 4: Fix Strategy (No Implementation)

### 4.1 Non-Streaming Fix

**Current Code (Line 235):**
```go
h.usageTracker.Record(h.instanceID, routeName, successfulModel,
    h.estimateTokens(req), fallbackCount)
```

**Required Change:**
```go
totalTokens := resp.Usage.InputTokens + resp.Usage.OutputTokens
h.usageTracker.Record(h.instanceID, routeName, successfulModel,
    totalTokens, fallbackCount)
```

**Benefits:**
- Uses accurate provider-reported data
- Captures both input and output tokens
- Simple one-line change per location

### 4.2 Streaming Fix

**Challenge:** Usage data comes in `message_delta` events during the stream, not in a single response object.

**Required Approach:**
1. Accumulate usage data during streaming
2. Extract `output_tokens` from `message_delta` events
3. Track usage after stream completes with accumulated data

**Implementation Areas:**
- Modify `tryStreamingTarget` to extract usage from events
- Add accumulator for output tokens during streaming
- Call `Record()` with actual total after stream completes

**Evidence of Usage in Events:**
From test `test/integration/provider_quirks/openrouter_quirks_test.go:443-444`:
```go
w.Write([]byte(`event: message_delta` + "\n"))
w.Write([]byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}` + "\n\n"))
```

### 4.3 Design Considerations

#### Option A: Simple Sum
Track `InputTokens + OutputTokens` as single token count.

**Pros:**
- Simple to implement
- Matches current tracker interface
- Backwards compatible

**Cons:**
- Loses granularity (can't see input vs output breakdown)

#### Option B: Extended Tracker
Modify tracker to accept input and output separately.

**Pros:**
- Preserves granularity
- Better for cost analysis
- More detailed reporting

**Cons:**
- Requires tracker interface change
- Database schema change
- More complex

**Recommendation:** Option A for quick fix, Option B for future enhancement.

---

## Additional Findings

### 5.1 Input Token Estimation Issues

Even for input tokens, the estimation is flawed:
```go
total += len(block.Text) / 4  // Crude approximation
```

**Problems:**
- Ignores system prompts (can be thousands of tokens)
- Ignores tool definitions (function calling can be expensive)
- Ignores images (image tokens vary by resolution)
- Ignores structured formatting overhead
- Character count ÷4 is very rough

### 5.2 Database Schema

From `internal/usage/db.go`:
```go
CREATE TABLE IF NOT EXISTS usage_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    route TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens INTEGER NOT NULL,
    fallbacks INTEGER NOT NULL,
    timestamp DATETIME NOT NULL
)
```

**Current Schema:** Single `tokens` column.

**If implementing Option B (Extended Tracker):**
Would need to add:
- `input_tokens INTEGER`
- `output_tokens INTEGER`

---

## Impact Assessment

### Current State
- Usage tracking shows ~25% of actual token consumption
- Only input tokens (estimated) are tracked
- Output tokens (50-75% of total) are completely ignored
- Cost estimates are severely underestimated

### After Fix
- Accurate token tracking from provider-reported data
- Captures both input and output tokens
- Reliable cost estimation
- Better capacity planning

---

## Testing Strategy

### Unit Tests Required
1. Test non-streaming with actual usage data
2. Test streaming with usage accumulation from events
3. Verify fallback behavior still works correctly

### Integration Tests Required
1. End-to-end test with real provider
2. Verify database stores correct values
3. Verify `ccrouter usage` command displays correct totals

### Manual Verification
1. Make request with known token count
2. Check log output for actual usage
3. Run `ccrouter usage` and verify it matches
4. Compare with provider dashboard

---

## Files Requiring Changes

### Critical Changes
1. `internal/proxy/handler.go` (2 locations)
   - Line 235: Use `resp.Usage` data
   - Line 291: Extract and accumulate usage from streaming events

### Optional Enhancement (if implementing Option B)
2. `internal/usage/tracker.go`
   - Modify `Record()` signature to accept input/output separately
3. `internal/usage/db.go`
   - Add `input_tokens` and `output_tokens` columns
4. `internal/usage/stats.go`
   - Update aggregation functions

---

## Conclusion

**Root Cause:** The usage tracking system uses estimated input tokens instead of actual provider-reported usage data (input + output).

**Fix Complexity:** Low for non-streaming, Medium for streaming.

**Risk Level:** Low (using more accurate data, not changing logic).

**Recommendation:** Implement fix in two phases:
1. Phase 1: Fix non-streaming (quick win, high impact)
2. Phase 2: Fix streaming (requires event accumulation)

**No implementation has been performed per instructions. This report provides complete analysis and fix strategy.**
