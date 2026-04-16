# BigModel Error 1213 Root Cause Catalog

**Date:** 2026-04-02
**Tags:** #composite #glm #bigmodel #error-1213 #provider #streaming #validation
**Severity:** high
**Scope:** project
**Captured:** composite (merged from provider-connection-pooling + glm-anthropic-endpoint-compatibility)

---

## Context

BigModel (GLM) returns error code 1213 for multiple independent issues. The same error code surfaces from at least 5 distinct root causes, making diagnosis difficult. This catalog consolidates all known triggers into one reference.

## Root Causes

### 1. HTTP Keep-Alive Connection Reuse (Transport Layer)

| Attribute | Value |
|-----------|-------|
| **Error code** | 1213 |
| **HTTP status** | 200 or non-200 |
| **Pattern** | Intermittent, appears on 2nd+ request |
| **Fix** | `disableKeepAlives: true` in provider config |

BigModel's API gateway doesn't support HTTP keep-alive. Go's default `http.Transport` reuses TCP connections, triggering 1213 on subsequent requests.

```json
{
  "bigmodel": {
    "apiKey": "${BIGMODEL_API_KEY}",
    "disableKeepAlives": true
  }
}
```

### 2. `stream: false` Rejection (API Layer)

| Attribute | Value |
|-----------|-------|
| **Error code** | 1213 |
| **HTTP status** | Non-200 |
| **Pattern** | Consistent, every non-streaming request |
| **Fix** | Force `reqCopy.Stream = true` in GLM transformer |

GLM only supports streaming for Anthropic-format requests. Sending `stream: false` always triggers 1213.

```go
reqCopy.Stream = true // GLM always streams
```

### 3. Thinking Blocks in Assistant Messages (Validation Layer)

| Attribute | Value |
|-----------|-------|
| **Error code** | 1213 |
| **HTTP status** | Non-200 |
| **Pattern** | When conversation history has thinking blocks |
| **Fix** | Strip `thinking` and `redacted_thinking` from assistant messages |

GLM rejects assistant messages containing thinking content blocks. This is a degradation — the model loses reasoning context from previous turns.

```go
func stripAssistantThinkingBlocks(req *anthropic.Request) {
    for i := range req.Messages {
        if req.Messages[i].Role != anthropic.RoleAssistant { continue }
        var filtered []anthropic.ContentBlock
        for _, block := range req.Messages[i].Content {
            if block.Type != "thinking" && block.Type != "redacted_thinking" {
                filtered = append(filtered, block)
            }
        }
        req.Messages[i].Content = filtered
    }
}
```

### 4. SSE Error Events on HTTP 200 (Streaming Layer)

| Attribute | Value |
|-----------|-------|
| **Error code** | 1213 |
| **HTTP status** | **200** (misleading!) |
| **Pattern** | Mid-stream, after initial successful events |
| **Fix** | Detect `event: error` SSE events and trigger failover |

BigModel returns HTTP 200 but sends `event: error` SSE events mid-stream. Without explicit handling, these errors are forwarded to Claude Code instead of triggering failover.

```go
// In handler.go tryStreamingTarget - after TransformStreamEvent:
for _, te := range transformedEvents {
    if te.EventType == "error" {
        return 0, fmt.Errorf("Provider stream error (code %v): %v", code, message)
    }
}
```

### 5. Tool Names Exceeding 64 Characters (Validation Layer)

| Attribute | Value |
|-----------|-------|
| **Error code** | 1213 |
| **HTTP status** | Non-200 |
| **Pattern** | When request contains MCP tools with long names |
| **Fix** | Truncate tool names to 64 chars with hash suffix |

GLM enforces a 64-character limit on tool names. Claude Code MCP tools (e.g., `mcp__plugin_everything-claude-code_github__create_pull_request_review`) exceed 70+ characters.

```go
const maxToolNameLength = 64

func truncateToolNames(req *anthropic.Request) {
    for i := range req.Tools {
        if len(req.Tools[i].Name) > maxToolNameLength {
            hash := fmt.Sprintf("%x", sha256.Sum256([]byte(originalName)))[:6]
            req.Tools[i].Name = originalName[:57] + "_" + hash
        }
    }
}
```

## Diagnosis Quick Reference

When you see error 1213 in logs:

| Symptom | Root Cause | Check |
|---------|------------|-------|
| Intermittent, 2nd+ request | Keep-alive reuse | Is `disableKeepAlives: true` set? |
| Every non-streaming request | `stream: false` | Does GLM transformer force `Stream = true`? |
| Has thinking in history | Thinking blocks | Does GLM transformer strip assistant thinking? |
| HTTP 200 but stream breaks | SSE error event | Does handler check for `event: error`? |
| Has long tool names | Tool name limit | Does GLM transformer truncate tool names? |

## Processing Order

In `glm_anthropic.go`, the order matters:

1. Convert user thinking blocks (to text format)
2. Strip assistant thinking blocks (remove entirely)
3. Normalize content (merge text blocks, fix arrays)
4. Validate blocks (check ordering)
5. Truncate tool names (enforce 64-char limit)

## Key Pitfalls

- **Do NOT assume HTTP 200 means success** — always check SSE event types
- **Do NOT send `stream: false` to GLM** — always force streaming
- **Error 1213 is NOT a single bug** — it has at least 5 independent root causes
- **The thinking block strip is a degradation** — document the tradeoff
- **Tool name truncation needs hash suffix** — avoid collisions when tools share prefixes

## References

- Source 1: `20260329-provider-connection-pooling.md` — Keep-alive root cause
- Source 2: `20260402-glm-anthropic-endpoint-compatibility.md` — API incompatibilities
- Commit: `dc92427` — fix(proxy): handle BigModel error 1213
- Files: `internal/transformer/transformers/glm_anthropic.go`, `internal/proxy/handler.go`
- Related: `20260330-provider-thinking-block-compatibility.md` — OpenRouter thinking block handling
