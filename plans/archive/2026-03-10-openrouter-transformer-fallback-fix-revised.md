# Fix Plan: OpenRouter Transformer Fallback Issue - REVISED

**Date:** 2026-03-10
**Status:** Needs Clarification
**Confidence:** RECONSIDERING

---

## Critical Finding: Documentation vs. Reality

### Documentation Claims (docs/configuration.md:81-123)

The documentation states OpenRouter provides **two different endpoints**:

**Anthropic-Compatible Endpoint:**
```json
{
  "openrouter-anthropic": {
    "apiKey": "${OPENROUTER_API_KEY}",
    "baseURL": "https://openrouter.ai/api",
    "transformer": "anthropic",
    "models": ["anthropic/claude-sonnet-4", "anthropic/claude-opus-4"]
  }
}
```
- Endpoint: `/v1/messages`
- Transformer: `anthropic` (pass-through)
- Auth: `x-api-key: <key>`

**OpenAI-Compatible Endpoint:**
```json
{
  "openrouter-openai": {
    "apiKey": "${OPENROUTER_API_KEY}",
    "baseURL": "https://openrouter.ai/api/v1",
    "transformer": "openai",
    "models": ["google/gemini-2.5-flash", "openai/gpt-4o"]
  }
}
```
- Endpoint: `/chat/completions`
- Transformer: `openai` (OpenAI-compatible format)
- Auth: `Authorization: Bearer <key>`

### Current Reality (config.json)

```json
{
  "openrouter-anthropic": {
    "apiKey": "${OPENROUTER_API_KEY}",
    "baseURL": "https://openrouter.ai/api",
    "models": ["anthropic/claude-haiku-4.5", "anthropic/claude-sonnet-4.5", "anthropic/claude-opus-4.5"],
    "transformer": "openrouter",  // ← Was empty, now set to "openrouter"
    "timeout": "60s"
  },
  "openrouter-openai": {
    "apiKey": "${OPENROUTER_API_KEY}",
    "baseURL": "https://openrouter.ai/api",  // ← NOT /api/v1 as documented!
    "models": ["google/gemini-2.5-flash", "google/gemini-2.5-pro"],
    "transformer": "openrouter",  // ← Was empty, now set to "openrouter"
    "timeout": "60s"
  }
}
```

### Key Inconsistencies

| Aspect | Documentation | Current Config | Status |
|--------|---------------|----------------|--------|
| `openrouter-anthropic` transformer | `anthropic` | `openrouter` | ❌ Mismatch |
| `openrouter-openai` transformer | `openai` | `openrouter` | ❌ Mismatch |
| `openrouter-openai` base URL | `/api/v1` | `/api` | ❌ Mismatch |

---

## Root Cause Analysis - RECONSIDERED

### The Original Error

```
API error: 400 Bad Request
"Invalid input: expected string, received array" at path ["messages", 1, "content"]
"Invalid input: expected string, received undefined" at path [0, "signature"]
```

### What Actually Happened

1. Provider: `openrouter-anthropic`
2. Handler tried to get transformer for `openrouter-anthropic`
3. Not found in registry → fell back to `anthropic` transformer
4. `anthropic` transformer strips whitespace signatures
5. OpenRouter rejected the request due to missing signature

### Why Providers Are Split

**Based on the documentation:**
The split exists because OpenRouter supposedly provides **different API formats** for different model families:
- Anthropic models → Anthropic-compatible API → needs `anthropic` transformer
- Other models (Google, OpenAI) → OpenAI-compatible API → needs `openai` transformer

**But the current config doesn't match this!**
- Both use the same base URL (`/api`)
- Both now use the same transformer (`openrouter`)

---

## Critical Questions

### Question 1: Is the Documentation Outdated?

**Hypothesis:** OpenRouter now uses a single unified API format (Anthropic-compatible) for all models, regardless of the underlying model family.

**Evidence For:**
- Current config uses same base URL and transformer for both providers
- OpenRouterTransformer exists and works for both
- The split might be historical baggage

**Evidence Against:**
- Documentation explicitly describes two different endpoints
- Different transformers (`anthropic` vs `openai`) are registered

### Question 2: What Should the Fix Be?

**Option A: Follow Documentation (If Still Accurate)**
```json
{
  "openrouter-anthropic": {
    "transformer": "anthropic",  // As documented
    "baseURL": "https://openrouter.ai/api"
  },
  "openrouter-openai": {
    "transformer": "openai",  // As documented
    "baseURL": "https://openrouter.ai/api/v1"  // As documented
  }
}
```
**Problem:** `anthropic` transformer strips signatures → will cause the same error!

**Option B: Use OpenRouter Transformer (Current Fix)**
```json
{
  "openrouter-anthropic": {
    "transformer": "openrouter",  // Preserves signatures
    "baseURL": "https://openrouter.ai/api"
  },
  "openrouter-openai": {
    "transformer": "openrouter",  // Preserves signatures
    "baseURL": "https://openrouter.ai/api"
  }
}
```
**Problem:** Contradicts documentation, but fixes the immediate error.

**Option C: Create Anthropic-Preserving Transformer**
Create a new transformer that uses Anthropic format but preserves signatures (like OpenRouterTransformer but named differently).

### Question 3: Why Does OpenRouter Need Signatures Preserved?

Looking at the transformers:
- `AnthropicTransformer` (anthropic.go:82-95): **Strips** whitespace signatures because Anthropic's API rejects them
- `OpenRouterTransformer` (openrouter.go:78-111): **Preserves** signatures (sets to `" "`) because OpenRouter requires them

**The contradiction:** If `openrouter-anthropic` is supposed to use the Anthropic transformer (per documentation), why does OpenRouter require signatures when Anthropic doesn't?

**Possible explanation:** OpenRouter's "Anthropic-compatible" endpoint isn't 100% compatible - it has different validation rules.

---

## What I Need to Know

1. **Is the documentation accurate?** Does OpenRouter actually provide two different API formats?
2. **What was the original issue that caused the provider split?** (You mentioned it was for "fixing some issues")
3. **Which transformer should each provider use?**
   - If following docs: `openrouter-anthropic` → `anthropic`, `openrouter-openai` → `openai`
   - If based on current reality: both → `openrouter`
4. **Should the documentation be updated to match reality?**

---

## Pending Action

**NOT implementing any changes until clarification is received.**

The fix plan is on hold pending answers to the questions above.
