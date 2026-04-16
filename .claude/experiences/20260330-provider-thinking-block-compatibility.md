# Provider Thinking Block Compatibility Matrix

**Date:** 2026-03-30
**Tags:** #transformer #provider #validation #thinking-blocks #signature #failover #composite
**Captured:** composite (merged from provider-validation-matrix + provider-specific-signature-fix)

---

## Problem

Each LLM provider validates thinking blocks and content arrays differently. Code that works with one provider breaks with another, especially during failover. Two providers have mutually exclusive requirements for the `signature` field in thinking blocks.

## Provider Requirements Matrix

### Thinking Block Handling

| Provider | Signature Field | Thinking Block Format | Single-Element Array |
|----------|----------------|----------------------|---------------------|
| Direct Anthropic | Omit when empty (`nil`) | Accepts single `[{thinking}]` | Accepted |
| OpenRouter (Anthropic) | Must be present (`""`) | Rejects single, needs `[{thinking},{text}]` | Rejected |
| GLM (BigModel/Aliyun) | Must be present (`""` or `" "`) | Rejects single, needs normalization | Rejected |
| OpenAI | N/A | Converts to text | Accepted (string) |
| Gemini | N/A | Converts to `thoughtPart` | Accepted |

### Signature Field Requirements

| Provider | Requires Field Present | Accepts Empty Value | Accepts Missing Field | Notes |
|----------|----------------------|---------------------|----------------------|-------|
| GLM-5 (aliyun) | ✅ Yes | ✅ Empty string `""` | ❌ No | Requires field or 400 error |
| OpenRouter/Anthropic | ✅ Yes | ❌ Whitespace rejected | ❌ No | Rejects `signature=" "` |
| Direct Anthropic | ❌ No | N/A | ✅ Yes | Prefers omitted when empty |

## Key Lessons

1. **OpenRouter != Direct Anthropic** — OpenRouter adds its own validation layer that is stricter than Anthropic's native API. Never assume Anthropic-compatible means identical behavior.

2. **`*string` pointer type for signature** — Use `*string` to distinguish between omitting the field (`nil`) and including an empty string (`&""`).

3. **Always normalize for OpenRouter** — The OpenRouter transformer must normalize ALL content, not just thinking blocks. Single-element arrays of any type (image, tool_result, thinking) fail validation.

4. **Failover amplifies differences** — Content from GLM conversations gets sent to OpenRouter on failover. Each provider's quirks compound.

5. **GLM sets signature=" " (space)** — GLM transformer sets a single space to satisfy GLM's requirement, but this fails on OpenRouter-Anthropic failover.

## Implementation Pattern

```go
// Provider-specific signature handling
func normalizeForProvider(req *anthropic.Request, provider string) {
    for i := range req.Messages {
        for j := range req.Messages[i].Content {
            block := &req.Messages[i].Content[j]
            if block.Type == "thinking" {
                switch provider {
                case "anthropic":
                    // Omit field when empty (nil pointer)
                    if block.Signature != nil && *block.Signature == "" {
                        block.Signature = nil
                    }
                case "openrouter", "glm":
                    // Must be present, but not just whitespace
                    if block.Signature == nil || *block.Signature == "" {
                        empty := ""
                        block.Signature = &empty
                    }
                }
            }
        }
    }
}
```

## Validation Checklist

- [ ] Test thinking blocks with each provider individually
- [ ] Test failover from GLM to OpenRouter with thinking content
- [ ] Verify signature field handling in both directions
- [ ] Check single-element array rejection
- [ ] Validate content block ordering (thinking must not be last)

## Related

- **Deep Copy Rule:** Always deep-copy before modifying for a provider
- **Failover Safety:** Each transformer normalizes for its own requirements

---

**Merged from:**
- `20260329-provider-validation-matrix.md` (experience)
- `provider-specific-signature-fix-2026-03-04.md` (plan)
