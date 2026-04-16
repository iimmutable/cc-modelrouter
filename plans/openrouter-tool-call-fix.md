# OpenRouter Tool Call Streaming Fix Plan

**Date**: 2026-02-25
**Status**: INVESTIGATION - Root Cause Identified
**Priority**: CRITICAL - Tool calls from OpenRouter providers not executing

---

## Executive Summary

When using `ccrouter code` with routes targeting OpenRouter-provided models (e.g., `minimax/minimax-m2.5`), tool calls are not being executed. The logs show `StopReason: tool_calls`, but Claude Code doesn't perform any tool actions.

**Root Cause:** The `transformToolCallChunks` function requires both `id` and `name` to arrive in the SAME SSE chunk. If mini-max sends these in separate chunks (valid streaming behavior), the `content_block_start` event is never emitted.

---

## Problem Analysis

### Observed Behavior

From log file `inst_20260225_231739.log`:
```
[STREAM] Stream completed with openrouter/minimax/minimax-m2.5, fallbacks: 0
[STREAM SUMMARY] Stream completed with 13 raw events processed
[RESPONSE] Success with openrouter/minimax/minimax-m2.5, StopReason: tool_calls, InputTokens: 30876, OutputTokens: 536
```

From log file `inst_20260225_174026.log`:
```
[STREAM SUMMARY] Stream completed with 116 raw events processed
[RESPONSE] Success with openrouter/minimax/minimax-m2.5, StopReason: tool_calls, InputTokens: 31073, OutputTokens: 118
```

Note: Only 118 output tokens is very small for a tool call response, suggesting the tool call content was not properly streamed.

### Code Analysis

**File:** `internal/transformer/openrouter.go:252`

```go
// Emit content_block_start on first chunk with id and name
if !state.started && hasID && functionName != "" {
    state.id = toolID
    state.name = functionName
    state.started = true

    contentBlockStart := map[string]any{
        "type":  "content_block_start",
        "index": toolIndex,
        "content_block": map[string]any{
            "type": "tool_use",
            "id":   toolID,
            "name": functionName,
        },
    }
    // ... emit event
}
```

**Problem:** The condition `!state.started && hasID && functionName != ""` requires:
1. State not yet started
2. Has ID
3. Has function name (not empty)

**All three must be true in the SAME chunk.**

### Why This Fails

If mini-max/OpenRouter sends tool calls in separate chunks:

| Chunk | Content | hasID | functionName | Condition Result | Emitted Events |
|-------|---------|-------|---------------|------------------|----------------|
| 1 | `{index: 0, id: "call_123"}` | true | "" | FALSE (no name) | None |
| 2 | `{index: 0, name: "web_search"}` | false | "web_search" | FALSE (no id) | None |
| 3 | `{index: 0, arguments: "{"query"}` | false | "" | FALSE | None |

**Result:** `content_block_start` is NEVER emitted, so Claude Code never receives the tool_use event.

---

## Investigation Tasks

### Task 1: Confirm Mini-Max/OpenRouter SSE Format

**Objective:** Capture the actual SSE events from mini-max to confirm whether id and name arrive in separate chunks.

**Steps:**
1. Enable debug level logging in config
2. Make a test request that triggers tool calls
3. Analyze the `[STREAM-DEBUG]` log entries for raw SSE events
4. Identify the exact chunk format for tool calls

**Expected Findings:**
- Either: id and name arrive together (current code works)
- Or: id and name arrive separately (root cause confirmed)

### Task 2: Check OpenAI API Documentation

**Objective:** Verify whether OpenAI's streaming format allows id and name to arrive separately.

**Reference:** [OpenAI API Documentation](https://platform.openai.com/docs/api-reference/streaming)

---

## Fix Plan

### Option A: Accumulate State Before Emitting content_block_start

**Strategy:** Store partial information (id, name) as it arrives, then emit `content_block_start` when BOTH are available.

**Changes Required:**

```go
type toolCallState struct {
    id        string
    name      string
    arguments strings.Builder
    started   bool
    hasID     bool   // NEW: Track if we've seen the id
    hasName   bool   // NEW: Track if we've seen the name
}

func (t *OpenRouterTransformer) transformToolCallChunks(toolCalls []any) ([]SSEEvent, error) {
    // ... existing code ...

    // Extract tool call fields
    toolID, hasID := toolCall["id"].(string)

    // Extract function fields
    var functionName string
    if function, hasFunction := toolCall["function"].(map[string]any); hasFunction {
        if name, ok := function["name"].(string); ok {
            functionName = name
        }
    }

    // NEW: Update state with partial information
    if hasID && toolID != "" {
        state.id = toolID
        state.hasID = true
    }
    if functionName != "" {
        state.name = functionName
        state.hasName = true
    }

    // Emit content_block_start when we have BOTH id AND name
    if !state.started && state.hasID && state.hasName {
        state.started = true

        contentBlockStart := map[string]any{
            "type":  "content_block_start",
            "index": toolIndex,
            "content_block": map[string]any{
                "type": "tool_use",
                "id":   state.id,
                "name": state.name,
            },
        }
        // ... emit event
    }

    // Rest of the function...
}
```

### Option B: Emit content_block_start with Partial Data

**Strategy:** Emit `content_block_start` as soon as we see the id, then update with name when it arrives.

**Drawback:** Anthropic's format expects both id and name in `content_block_start`, so this would violate the spec.

### Option C: Synthetic content_block_start on First Tool Call Chunk

**Strategy:** Emit `content_block_start` with placeholder values when the first tool call chunk arrives, then update.

**Drawback:** More complex, may cause issues if the client validates the format strictly.

---

## Recommended Fix

**Use Option A** - Accumulate state before emitting `content_block_start`.

**Rationale:**
1. Correctly implements the Anthropic SSE format
2. Handles all streaming patterns (id+name together, id first, name first)
3. Minimal code changes
4. Maintains backward compatibility

---

## Testing Strategy

### Unit Test: Separate ID and Name Chunks

```go
func TestOpenRouterTransformSSEEvent_ToolCallIDThenName(t *testing.T) {
    tr := NewOpenRouterTransformer()

    // First chunk: ID only
    idChunk := map[string]any{
        "id":      "chatcmpl-123",
        "object":  "chat.completion.chunk",
        "created": 1234567890,
        "model":   "minimax-m2.5",
        "choices": []map[string]any{
            {
                "index": 0,
                "delta": map[string]any{
                    "tool_calls": []map[string]any{
                        {"index": float64(0), "id": "call_abc123"},
                    },
                },
                "finish_reason": nil,
            },
        },
    }
    idData, _ := json.Marshal(idChunk)
    idEvent := &SSEEvent{EventType: "", Data: idData}

    result, err := tr.TransformSSEEvent(idEvent)
    if err != nil {
        t.Fatalf("failed to transform id chunk: %v", err)
    }
    // Should NOT emit content_block_start yet (no name)
    if len(result) != 0 {
        t.Errorf("expected 0 events before name arrives, got %d", len(result))
    }

    // Second chunk: Name only
    nameChunk := map[string]any{
        "id":      "chatcmpl-123",
        "object":  "chat.completion.chunk",
        "created": 1234567890,
        "model":   "minimax-m2.5",
        "choices": []map[string]any{
            {
                "index": 0,
                "delta": map[string]any{
                    "tool_calls": []map[string]any{
                        {
                            "index": float64(0),
                            "function": map[string]any{"name": "web_search"},
                        },
                    },
                },
                "finish_reason": nil,
            },
        },
    }
    nameData, _ := json.Marshal(nameChunk)
    nameEvent := &SSEEvent{EventType: "", Data: nameData}

    result, err = tr.TransformSSEEvent(nameEvent)
    if err != nil {
        t.Fatalf("failed to transform name chunk: %v", err)
    }
    // NOW should emit content_block_start (has id AND name)
    if len(result) != 2 {
        t.Fatalf("expected 2 events (message_start, content_block_start), got %d", len(result))
    }
    if result[1].EventType != "content_block_start" {
        t.Errorf("expected content_block_start event, got '%s'", result[1].EventType)
    }
}
```

### Unit Test: Name Then ID Chunks

Similar test but with name arriving before id.

### Integration Test

Run `ccrouter code` with a request that triggers tool calls on mini-max, verify:
1. Tool call events are properly generated
2. Claude Code executes the tools
3. No errors in logs

---

## Files to Modify

| File | Changes | Lines |
|------|---------|-------|
| `internal/transformer/openrouter.go` | Update `toolCallState` struct and `transformToolCallChunks` function | ~20-30 |
| `internal/transformer/openrouter_test.go` | Add new test cases for separate id/name chunks | ~100 |

---

## Risk Assessment

### Low Risk
- Fix is isolated to OpenRouter transformer
- Doesn't affect working providers (GLM, direct Anthropic)
- Maintains backward compatibility

### Medium Risk
- If the fix has bugs, tool calls from ALL OpenRouter models could break
- Need thorough testing with multiple OpenRouter models

### Mitigation
- Add comprehensive unit tests
- Test with multiple OpenRouter models before deploying
- Keep existing tests passing

---

## Rollout Plan

1. **Phase 1**: Implement the fix
2. **Phase 2**: Add unit tests
3. **Phase 3**: Run existing tests to verify no regression
4. **Phase 4**: Manual testing with `ccrouter code` and mini-max
5. **Phase 5**: Deploy and monitor

---

## Related Documents

- [SSE Debug and Fix Plan](./sse-debug-and-fix-plan.md) - Previous SSE fixes
- [Transformer JSON Marshaling Fix](./transformer-json-marshaling-fix.md) - Related transformer issues
- [Request/Response Interception Fix](./request-response-interception-fix.md) - SSE event handling context
