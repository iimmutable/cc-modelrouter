# Provider Validation Matrix for Thinking Blocks & Content

**Date:** 2026-03-29
**Tags:** #transformer #provider #validation #thinking-blocks

## Problem

Each LLM provider validates thinking blocks and content arrays differently. Code that works with one provider breaks with another, especially during failover.

## Provider Signature Requirements

| Provider | Signature Field | Thinking Block Format | Single-Element Array |
|----------|----------------|----------------------|---------------------|
| Direct Anthropic | Omit when empty (`nil`) | Accepts single `[{thinking}]` | Accepted |
| OpenRouter (Anthropic) | Must be present (`""`) | Rejects single, needs `[{thinking},{text}]` | Rejected |
| GLM (BigModel/Aliyun) | Must be present (`""`) | Rejects single, needs normalization | Rejected |
| OpenAI | N/A | Converts to text | Accepted (string) |
| Gemini | N/A | Converts to `thoughtPart` | Accepted |

## Key Lessons

1. **OpenRouter != Direct Anthropic** — OpenRouter adds its own validation layer that is stricter than Anthropic's native API. Never assume Anthropic-compatible means identical behavior.

2. **`*string` pointer type for signature** — Use `*string` to distinguish between omitting the field (`nil`) and including an empty string (`&""`).

3. **Always normalize for OpenRouter** — The OpenRouter transformer must normalize ALL content, not just thinking blocks. Single-element arrays of any type (image, tool_result, thinking) fail validation.

4. **Failover amplifies differences** — Content from GLM conversations gets sent to OpenRouter on failover. Each provider's quirks compound.

5. **Normalization must handle edge cases** — Multiple thinking blocks, thinking after text, nil Source on images — the real world produces all of these.
