# Invalid Anthropic Messages API Request - Root Cause Analysis & Fix

**Date:** 2026-03-11
**Status:** FIXED
**Severity:** CRITICAL

---

## Problem Statement

The "Invalid Anthropic Messages API request" error kept recurring despite multiple fix attempts:
- Error types: `invalid_union`, `invalid_type`, `invalid_input`
- Error path: `messages[X].content`
- Specific error: `"expected string, received undefined"` at path `[N, "data"]`

---

## Investigation Summary

### Phase 1: Log Analysis

**Error from `inst_20260311_155524.log`:**
```
"path": ["messages", 1, "content"],
"errors": [
  {"message": "Invalid input: expected string, received array"},
  {"message": "Invalid input: expected string, received undefined", "path": [3, "data"]}
]
```

Key observations:
- Route: `thinkMore` → OpenRouter → Anthropic
- First `thinkMore` succeeded (15:55:47)
- Second `thinkMore` failed (15:57:26) after many GLM responses in history

### Phase 2: Code Analysis

**Key files investigated:**
- `internal/transformer/transformers/anthropic.go` - validation functions
- `internal/transformer/transformers/openrouter.go` - OpenRouter transformer
- `pkg/api/anthropic/types.go` - ContentBlock and MessageContent types

### Phase 3: Root Cause Identification

**The Bug:** In `validateAndRepairBlocks()` function (line 148):

```go
// BUGGY CODE:
if block.Type == "image" && block.Source != nil {  // Skips when Source is nil!
    if block.Source.Data == "" {
        // fix
    }
}
```

**Problem:** When `block.Source` is `nil`, the condition evaluates to `false`, and validation is **completely skipped**. The nil Source then gets marshaled, causing OpenRouter to fail with `"expected string, received undefined"` at path `[3, "data"]`.

### Phase 4: Why Regression Happened

Previous fixes addressed:
- ✅ Thinking blocks (signature handling)
- ✅ Single-element content normalization
- ✅ Image blocks with empty Data string

**But missed:** Image blocks with **nil Source** (not empty - completely missing)

---

## The Fix

**File:** `internal/transformer/transformers/anthropic.go`

### Fix 1: Handle nil Source for images (lines 147-158)

```go
// BEFORE (buggy):
if block.Type == "image" && block.Source != nil {

// AFTER (fixed):
if block.Type == "image" {
    if block.Source == nil || block.Source.Data == "" {
```

### Fix 2: Add document block validation (new)

```go
// Validate document blocks - require document source
if block.Type == "document" {
    if block.DocumentSource == nil {
        block.Type = "text"
        block.Text = "[Document: source unavailable]"
        block.DocumentSource = nil
    }
}
```

---

## Verification

| Test | Result |
|------|--------|
| Build | ✅ Pass |
| All transformer tests | ✅ Pass |
| All project tests | ✅ Pass |
| nil Source image → text | ✅ Fixed |
| Empty Data image → text | ✅ Fixed |
| nil Source document → text | ✅ Fixed |

---

## Key Lessons

1. **Defensive validation must handle ALL cases**, not just the common ones. The condition `block.Source != nil` was meant to prevent nil pointer panic, but it also inadvertently skipped nil detection.

2. **Error path analysis is critical**. The `[3, "data"]` error path directly pointed to ImageSource structure, which led to the root cause.

3. **Multiple fixes over weeks** addressed symptoms (thinking blocks, empty Data) but not the underlying gap (nil Source).

4. **Testing in isolation passes but production fails** because the edge case (nil Source) wasn't in test scenarios.

---

## Prevention Recommendations

1. **Add edge case tests** for:
   - Image block with nil Source
   - Image block with empty Data
   - Document block with nil DocumentSource
   - Tool blocks with missing required fields

2. **Centralize validation** - Create a unified `ValidateAllBlocks()` function that validates ALL block types, not scattered individual checks.

3. **Add request validation logging** - Log content structure before sending to providers to enable faster debugging.

4. **Consider schema validation** - Use JSON Schema or similar to validate requests against expected format before sending.

---

## Related Commits

- `8a893d1` - fix(transformer): normalize ALL single-element content to prevent validation errors
- `c7068e1` - fix(transformer): expand normalization to handle multiple thinking blocks
- `1d93478` - fix(openrouter): always normalize thinking blocks for OpenRouter to fix validation errors
- `fa1d7ad` - fix(transformer): comprehensive fix for thinking block signature validation errors