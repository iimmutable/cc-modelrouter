# Thinking Field Data Flow Analysis

> **Status:** OUTDATED - This document describes an older architecture. The transformer system has been refactored and now uses a different file structure.

## Executive Summary

The `thinking` field (Extended Thinking configuration) handling has been significantly improved. The current implementation properly handles thinking blocks in the transformer layer with provider-specific normalization.

**Current Architecture:**
- Transformers are located in `internal/transformer/transformers/`
- Thinking block normalization is handled per-provider
- Signature field handling differs by provider (Anthropic vs OpenRouter vs GLM)

---

## Current Transformers

| Provider | File | Notes |
|----------|------|-------|
| Anthropic | `internal/transformer/transformers/anthropic.go` | Strips empty signatures |
| OpenAI | `internal/transformer/transformers/openai.go` | OpenAI-compatible |
| OpenRouter | `internal/transformer/transformers/openrouter.go` | Requires signature field present |
| Gemini | `internal/transformer/transformers/gemini.go` | Native format |
| GLM | `internal/transformer/transformers/glm_anthropic.go` | Anthropic-compatible |

---

## Key Implementation Details

### Thinking Block Normalization

The current system normalizes thinking blocks to prevent validation errors:

1. **Single-element content arrays** are expanded to multi-element arrays
2. **User messages** with thinking blocks are converted to text wrapped in `<thinking>` tags
3. **Signature handling** differs by provider:
   - **Anthropic**: Omit signature when empty (uses `nil`)
   - **OpenRouter**: Always include signature (even empty, uses `&""`)
   - **GLM**: Always include signature

### Failover Considerations

When providers fail over, the transformer must handle thinking blocks from different provider responses:
- GLM responses may include thinking blocks with signatures
- These must be normalized before sending to OpenRouter
- Deep copy is used to avoid modifying the original request

---

## Summary Table (Current)

| Provider | Thinking Support | Signature Handling |
|----------|-----------------|-------------------|
| Anthropic | YES | Omit when empty (nil) |
| GLM | YES | Always include (empty ok) |
| OpenRouter | YES (Anthropic models) | Always include |
| OpenAI | NO | N/A |
| Gemini | NO | N/A |

---

## Historical Notes

This document previously analyzed a converter-based architecture. The current implementation uses direct transformer implementations in `internal/transformer/transformers/` directory, registered in `internal/cli/start.go` and `internal/cli/code.go`.
