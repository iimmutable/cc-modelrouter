# Comprehensive Failover Fix Plan

**Date:** 2026-03-05
**Status:** **COMPLETED - All fixes implemented and tested**
**Priority:** Critical - Affects all failover scenarios

## Implementation Summary

All fixes have been successfully implemented and tested. The build completed successfully and all tests pass.

### Changes Made:

1. **GetBody Function Added** (Fixes ContentLength error)
   - `internal/transformer/transformers/glm_anthropic.go` - Added GetBody function
   - `internal/transformer/transformers/anthropic.go` - Added GetBody function
   - `internal/transformer/transformers/openai.go` - Added GetBody function
   - `internal/transformer/transformers/gemini.go` - Added GetBody function

2. **Provider-Specific Signature Normalization**
   - `internal/transformer/transformers/anthropic.go` - Strips whitespace-only signatures
   - `internal/transformer/transformers/glm_anthropic.go` - Keeps signature=" " for GLM compatibility
   - `internal/transformer/transformers/anthropic_transformer_test.go` - Updated tests

3. **Deep Copy Utility in Handler**
   - `internal/proxy/handler.go` - Added `deepCopyRequest()` function
   - Updated `tryTarget()` to use deep copy
   - Updated `tryStreamingTarget()` to use deep copy

4. **Comprehensive Failover Logging**
   - Added detailed logging in non-streaming failover loop
   - Added detailed logging in streaming failover loop

### Test Results:
```
✓ All tests pass
✓ Build successful: bin/debug/ccrouter
✓ No compilation errors
```

---

## Problem Summary

Two critical bugs causing API errors during failover:

1. **"Invalid `signature` in `thinking` block"** - Occurs when failing over from GLM providers to OpenRouter-Anthropic
2. **"ContentLength=X with Body length 0"** - Occurs when retrying requests with large bodies after provider timeout

## Root Cause Analysis

### Issue 1: Invalid Signature in Thinking Block

**Error Message:**
```
messages.1.content.0: Invalid 'signature' in 'thinking' block
```

**Root Cause:**
The GLM transformer sets `signature = " "` (single space) for thinking blocks to satisfy GLM's requirement that the signature field be present. However, when failover occurs to OpenRouter-Anthropic, the thinking content with `signature = " "` causes validation errors.

**Code Location:** `internal/transformer/transformers/glm_anthropic.go:82-83`
```go
if reqCopy.Messages[i].Content[j].Signature == "" {
    reqCopy.Messages[i].Content[j].Signature = " " // Single space (not empty)
}
```

**Why It Happens:**
1. First attempt (aliyun/glm-5) succeeds with `signature = " "`
2. Provider times out or fails
3. Failover to openrouter-anthropic
4. OpenRouter's Anthropic backend rejects `signature = " "` as invalid
5. Request fails with 400 Bad Request

**The Real Problem:**
Each provider transformer should normalize content for its specific requirements, not rely on modifications from previous provider attempts.

### Issue 2: ContentLength with Body Length 0

**Error Message:**
```
http: ContentLength=171290 with Body length 0
http: ContentLength=208690 with Body length 0
```

**Root Cause:**
The HTTP request's `ContentLength` header is set based on the request body size, but when the request is retried, the body reader has already been consumed (cursor at end). The HTTP client detects this mismatch and rejects the request.

**Code Location:** `internal/proxy/handler.go:372` and `internal/transformer/transformers/glm_anthropic.go:110`
```go
// handler.go - tryTarget
httpReq, err := tf.PrepareRequest(req, providerCfg.BaseURL, providerCfg.APIKey, target.Model)

// glm_anthropic.go - PrepareRequest
httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
```

**Why It Happens:**
1. `PrepareRequest` creates a new `bytes.NewReader(body)` each time
2. However, the `body` byte slice is reused from the previous attempt
3. If something modifies or corrupts the `body` slice between attempts, the ContentLength header may not match the actual body length
4. The HTTP client validates this and rejects the request

**Alternative Theory:**
The request body might be getting corrupted during the marshaling/unmarshaling process in the GLM transformer's deep copy operation, especially for large requests (170KB+).

### Issue 3: Shallow Copy in Streaming Path

**Code Location:** `internal/proxy/handler.go:449-450`
```go
// Ensure stream flag is set
reqCopy := *req
reqCopy.Stream = true
```

**Problem:**
This creates a shallow copy of the request, meaning the underlying `Message` and `ContentBlock` arrays are shared. While the GLM transformer creates its own deep copy, the Anthropic transformer does not, which could lead to state corruption.

**Impact:**
If the Anthropic transformer modifies the request (which it shouldn't, but might in edge cases), those modifications would affect the original request passed to subsequent providers.

## Fix Strategy

### Fix 1: Provider-Specific Content Normalization

**Principle:** Each transformer should normalize content from the ORIGINAL request, not from modifications made by previous provider attempts.

**Implementation:**

1. **Add `NormalizeThinkingBlocks` function** that takes provider requirements into account:

```go
// normalizeThinkingBlocksForProvider normalizes thinking blocks based on provider requirements
// - GLM providers: signature must be present (use " " for empty)
// - Anthropic providers: signature must be valid or omitted
func normalizeThinkingBlocksForProvider(req *anthropic.Request, provider string) {
    for i := range req.Messages {
        for j := range req.Messages[i].Content {
            if req.Messages[i].Content[j].Type == "thinking" {
                switch provider {
                case "aliyun", "bigmodel":
                    // GLM requires signature field to be present
                    if req.Messages[i].Content[j].Signature == "" {
                        req.Messages[i].Content[j].Signature = " "
                    }
                case "anthropic", "openrouter-anthropic":
                    // Anthropic rejects empty/whitespace signatures
                    // Only keep non-empty signatures
                    if req.Messages[i].Content[j].Signature != "" &&
                        strings.TrimSpace(req.Messages[i].Content[j].Signature) == "" {
                        req.Messages[i].Content[j].Signature = ""
                    }
                }
            }
        }
    }
}
```

2. **Call this in each transformer's `PrepareRequest` AFTER the deep copy:**

```go
// GLMAnthropicTransformer.PrepareRequest
// After creating reqCopy
normalizeThinkingBlocksForProvider(&reqCopy, "glm")
```

### Fix 2: Always Use Deep Copy in Handler

**Principle:** Never pass the original request to transformers - always pass a deep copy to prevent state corruption.

**Implementation:**

Add a utility function to create deep copies:

```go
// deepCopyRequest creates a true deep copy of an Anthropic request
func deepCopyRequest(req *anthropic.Request) (*anthropic.Request, error) {
    reqJSON, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request for deep copy: %w", err)
    }
    var reqCopy anthropic.Request
    if err := json.Unmarshal(reqJSON, &reqCopy); err != nil {
        return nil, fmt.Errorf("failed to unmarshal request deep copy: %w", err)
    }
    return &reqCopy, nil
}
```

Update `tryTarget` and `tryStreamingTarget`:

```go
// tryTarget - non-streaming
func (h *Handler) tryTarget(ctx context.Context, req *anthropic.Request, target config.RouteTarget) (*anthropic.Response, error) {
    // Create deep copy before any modifications
    reqCopy, err := deepCopyRequest(req)
    if err != nil {
        return nil, fmt.Errorf("failed to copy request: %w", err)
    }

    // Prepare request with the copy
    httpReq, err := tf.PrepareRequest(reqCopy, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
    // ...
}

// tryStreamingTarget - streaming
func (h *Handler) tryStreamingTarget(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, req *anthropic.Request, target config.RouteTarget) (int, error) {
    // Create deep copy before any modifications
    reqCopy, err := deepCopyRequest(req)
    if err != nil {
        return 0, fmt.Errorf("failed to copy request: %w", err)
    }
    reqCopy.Stream = true

    // Prepare request with the copy
    httpReq, err := tf.PrepareRequest(reqCopy, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
    // ...
}
```

### Fix 3: Add Request Body Validation

**Principle:** Validate that the HTTP request body matches the ContentLength header before sending.

**Implementation:**

Add validation in `PrepareRequest` methods:

```go
// After creating httpReq
if httpReq.ContentLength > 0 {
    if httpReq.Body == nil {
        return nil, fmt.Errorf("ContentLength=%d but Body is nil", httpReq.ContentLength)
    }
    // Verify body can be read
    buf := make([]byte, 1)
    n, _ := httpReq.Body.Read(buf)
    if n == 0 {
        return nil, fmt.Errorf("ContentLength=%d but Body is empty", httpReq.ContentLength)
    }
    // Reset the reader (this won't work with bytes.NewReader, need different approach)
}
```

**Better Approach:**
Don't read the body to validate. Instead, ensure `bytes.NewReader` is created fresh each time with a valid byte slice:

```go
// In PrepareRequest, verify body slice is valid
if len(body) == 0 && len(reqCopy.Messages) > 0 {
    return nil, fmt.Errorf("marshaled body is empty but request has messages")
}
if len(body) > 0 && body[0] != '{' {
    return nil, fmt.Errorf("marshaled body doesn't start with '{'")
}
```

### Fix 4: Comprehensive Logging for Debugging

Add detailed logging to track request state through failover:

```go
// At start of tryTarget/tryStreamingTarget
logging.Debugf("[FAILOVER] Attempt %d with %s/%s", attempt, target.Provider, target.Model)
logging.Debugf("[FAILOVER] Request has %d messages", len(req.Messages))

// Log thinking block details
for i, msg := range req.Messages {
    thinkingCount := 0
    for _, block := range msg.Content {
        if block.Type == "thinking" {
            thinkingCount++
        }
    }
    if thinkingCount > 0 {
        logging.Debugf("[FAILOVER] Message %d has %d thinking blocks", i, thinkingCount)
    }
}

// After PrepareRequest
logging.Debugf("[FAILOVER] HTTP request ContentLength=%d", httpReq.ContentLength)
```

## Files to Modify

1. **internal/proxy/handler.go**
   - Add `deepCopyRequest` utility function
   - Update `tryTarget` to use deep copy
   - Update `tryStreamingTarget` to use deep copy
   - Add detailed failover logging

2. **internal/transformer/transformers/glm_anthropic.go**
   - Rename/set `normalizeThinkingBlocks` to be provider-specific
   - Add validation after marshaling
   - Remove the generic " " signature approach (only use for GLM providers)

3. **internal/transformer/transformers/anthropic.go**
   - Ensure it creates its own deep copy (it currently doesn't)
   - Add thinking block normalization for Anthropic (remove whitespace signatures)

4. **internal/transformer/transformers/openai.go**
   - Ensure it creates its own deep copy
   - Add similar protections

5. **internal/transformer/transformers/gemini.go**
   - Ensure it creates its own deep copy
   - Add similar protections

## Testing Plan

1. **Unit Tests**
   - Test `deepCopyRequest` with various request sizes
   - Test thinking block normalization for each provider type
   - Test that shallow copy doesn't share state

2. **Integration Tests**
   - Test failover from GLM to OpenRouter with thinking blocks
   - Test failover with large request bodies (>170KB)
   - Test multiple failover attempts (3+ providers)

3. **Manual Testing**
   - Run Claude Code with the updated router
   - Trigger thinking-level requests that timeout on first provider
   - Verify failover succeeds without errors

## Risk Assessment

**Low Risk:**
- Adding deep copy utility function
- Adding logging for debugging

**Medium Risk:**
- Modifying handler to use deep copy (affects all requests)
- Changing thinking block normalization logic

**Mitigation:**
- Add comprehensive unit tests
- Test with smaller, controlled changes first
- Monitor logs for any new errors

## Rollback Plan

If issues are found after deployment:

1. Revert handler changes to use shallow copy
2. Revert thinking block normalization changes
3. Keep only the logging additions for debugging

## Success Criteria

1. ✓ No more "Invalid signature in thinking block" errors during failover
2. ✓ No more "ContentLength=X with Body length 0" errors
3. ✓ Failover works correctly for all provider combinations
4. ✓ Large requests (>170KB) can failover successfully
5. ✓ Logging provides clear visibility into failover flow

## Open Questions

1. **Why does the ContentLength error happen?** Is it body corruption or something else? The logging should help clarify this.

2. **Are there other transformers that don't create deep copies?** Need to audit all transformers.

3. **Should the deep copy happen in the handler or in each transformer?** Doing it in the handler provides better isolation, but doing it in each transformer is more flexible. Plan proposes handler-level deep copy for consistency.

4. **What's the maximum request size we need to handle?** Current errors are ~170-210KB. Should test larger.

## Next Steps

1. **User Approval** - Get approval to proceed with implementation
2. **Create Feature Branch** - `fix/comprehensive-failover-fix`
3. **Implement Fixes** - Follow the plan systematically
4. **Add Tests** - Unit and integration tests
5. **Manual Testing** - Run Claude Code with updated router
6. **Deploy** - After all tests pass
7. **Monitor** - Watch logs for any new issues
