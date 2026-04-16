# Failover Safety Patterns

**Date:** 2026-03-30
**Tags:** #failover #transformer #deep-copy #architecture #composite
**Captured:** composite (merged from deep-copy-failover-rule + comprehensive-failover-fix)

---

## Problem

Two critical bugs causing API errors during failover:

1. **"Invalid `signature` in `thinking` block"** — Occurs when failing over from GLM providers to OpenRouter-Anthropic due to provider-specific content requirements

2. **"ContentLength=X with Body length 0"** — Occurs when retrying requests with large bodies after provider timeout because request bodies can only be read once

## Root Causes

### Issue 1: Content Mutation Across Providers

Transformers modify Anthropic requests (adding text blocks, changing signatures, normalizing content). When failover occurs, the original request is reused for the next provider. If a transformer mutated the original, the second provider receives corrupted state.

**Example:** GLM transformer adds text blocks to thinking-only content. On failover to OpenRouter, the request already has text blocks in the wrong format. OpenRouter rejects it with "expected string, received array".

### Issue 2: HTTP Body Exhaustion

HTTP request bodies can only be read once. After the first provider attempt reads the body, subsequent failover attempts see ContentLength > 0 but Body length 0.

## Solutions

### Solution 1: Deep Copy Before Modification

Every transformer's `PrepareRequest()` MUST deep-copy the request before modification.

```go
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

**Why JSON round-trip:**
- `reqCopy := *req` is a shallow copy — slices and maps share underlying arrays
- `append()` on a slice may modify the original's backing array
- JSON creates truly independent copies of all nested structures
- Performance cost is negligible relative to HTTP round-trip

### Solution 2: GetBody for Retry Support

Set `GetBody` function on HTTP requests to allow re-reading the body:

```go
bodyCopy := make([]byte, len(body))
copy(bodyCopy, body)
httpReq.GetBody = func() (io.ReadCloser, error) {
    return io.NopCloser(bytes.NewReader(bodyCopy)), nil
}
```

## Implementation Checklist

### Transformer Requirements

- [ ] Deep-copy request before any modification
- [ ] Normalize content for provider-specific requirements
- [ ] Handle provider-specific signature fields
- [ ] Merge consecutive text blocks where required
- [ ] Set GetBody for all requests

### Failover Safety Rules

1. **If you write a transformer, deep-copy first. No exceptions.**
2. **Each transformer normalizes for its own requirements** — don't rely on modifications from previous attempts
3. **Provider-specific handling** — GLM, OpenRouter, Anthropic each have unique requirements
4. **Test failover scenarios** — Always test failover from Provider A to Provider B with the same request

## Changes Applied

### GetBody Function Added
- `internal/transformer/transformers/glm_anthropic.go`
- `internal/transformer/transformers/anthropic.go`
- `internal/transformer/transformers/openai.go`
- `internal/transformer/transformers/gemini.go`

### Provider-Specific Signature Normalization
- `internal/transformer/transformers/anthropic.go` — Strips whitespace-only signatures
- `internal/transformer/transformers/glm_anthropic.go` — Keeps signature for GLM compatibility

### Deep Copy Utility
- `internal/proxy/handler.go` — Added `deepCopyRequest()` function
- Updated `tryTarget()` to use deep copy
- Updated `tryStreamingTarget()` to use deep copy

### Comprehensive Failover Logging
- Added detailed logging in non-streaming failover loop
- Added detailed logging in streaming failover loop

## Test Results

✓ All tests pass
✓ Build successful: bin/debug/ccrouter
✓ No compilation errors

## Related

- **Provider Compatibility:** See "Provider Thinking Block Compatibility Matrix"
- **SSE Streaming:** See "SSE Streaming Implementation Checklist"

---

**Merged from:**
- `20260329-deep-copy-failover-rule.md` (experience)
- `2026-03-05-comprehensive-failover-fix.md` (plan) — Status: COMPLETED

**Status:** COMPLETED — All fixes implemented and tested
