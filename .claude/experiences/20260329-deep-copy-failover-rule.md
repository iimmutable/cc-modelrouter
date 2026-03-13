# Deep Copy Rule for Failover Safety

**Date:** 2026-03-29
**Tags:** #failover #transformer #deep-copy #architecture

## Problem

Transformers modify Anthropic requests (adding text blocks, changing signatures, normalizing content). When failover occurs, the original request is reused for the next provider. If a transformer mutated the original, the second provider receives corrupted state.

## Solution

Every transformer's `PrepareRequest()` MUST deep-copy the request before modification:

```go
var reqCopy anthropic.Request
reqJSON, err := json.Marshal(req)
if err != nil {
    return nil, fmt.Errorf("failed to marshal request for deep copy: %w", err)
}
if err := json.Unmarshal(reqJSON, &reqCopy); err != nil {
    return nil, fmt.Errorf("failed to unmarshal request deep copy: %w", err)
}
```

## Why json.Marshal/Unmarshal

- `reqCopy := *req` is a shallow copy — slices and maps share underlying arrays
- `append()` on a slice in the copy may modify the original's slice backing array
- JSON round-trip creates truly independent copies of all nested structures
- Performance cost is negligible relative to the HTTP round-trip to the provider

## When This Was Discovered

GLM transformer added text blocks to thinking-only content during the first provider attempt. On failover to OpenRouter, the request already had the text blocks added, but in the wrong format. OpenRouter then rejected it with "expected string, received array".

## Rule

If you write a transformer, deep-copy first. No exceptions.
