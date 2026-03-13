# Transformer Testing Summary

> **Status:** OUTDATED - This document describes an older architecture. Current tests are in `internal/transformer/transformers/`.

The transformer system has tests covering all major functionality. The current test files are:

---

## Current Test Files

| File | Coverage |
|------|----------|
| `anthropic_transformer_test.go` | Anthropic transformer signature handling |
| `openrouter_transformer_test.go` | OpenRouter Anthropic model handling |
| `openai_test.go` | OpenAI-compatible format transformation |
| `gemini_test.go` | Gemini native format |
| `glm_test.go` | GLM Anthropic-compatible |
| `normalization_test.go` | Content block normalization |
| `anthropic_normalization_test.go` | Single-element content handling |
| `integration_test.go` | End-to-end transformations |
| `content_block_test.go` | Content block preservation |
| `image_streaming_test.go` | Image handling in streaming |
| `repro_test.go` | Bug reproduction tests |

---

## Key Test Categories

### 1. Signature Handling Tests

- `TestAnthropicTransformer_SignatureNormalization` - Empty signature handling for Anthropic API
- `TestOpenRouterTransformer_AnthropicModels_SignatureOmitted` - OpenRouter signature requirements
- `TestOpenRouterFix_InvalidSignatureError` - Fix for empty signature errors

### 2. Content Normalization Tests

- `TestNormalizeSingleElementContent_*` - Single thinking block normalization
- `TestNormalizeAssistantMessages_*` - Assistant message handling
- `TestConvertUserThinkingToText` - User thinking block conversion

### 3. Integration Tests

- `TestAnthropicTransformer_WithThinkingBlock` - Full transformation cycle
- `TestGLMAnthropicTransformer_WithThinkingBlock` - GLM pass-through
- `TestNormalizeAssistantMessages_ShallowCopyBehavior` - Deep copy verification

---

## Notes

- The old converter-based architecture with separate `converters/` and `providers/` directories has been replaced
- Transformers are now in `internal/transformer/transformers/`
- Tests verify provider-specific signature handling (Anthropic omit vs OpenRouter include)