# GLM Anthropic Endpoint Compatibility

**Date:** 2026-04-02
**Tags:** #provider #glm #bigmodel #streaming #validation #api-compat
**Severity:** high
**Scope:** project
**Captured:** manual

## Context
BigModel/GLM provides an Anthropic-compatible endpoint at `https://open.bigmodel.cn/api/anthropic`. Claude Code sends requests in Anthropic format, but the GLM endpoint has several incompatibilities that surface as error 1213 or silent request failures. This is separate from the HTTP keep-alive issue documented in `20260329-provider-connection-pooling.md`.

## Problem
Multiple GLM Anthropic endpoint incompatibilities cause requests to fail with error 1213, often with HTTP 200 status (making detection harder):

1. **`stream=false` causes error 1213** — GLM only supports streaming for Anthropic-format requests. Sending `stream: false` triggers error 1213 even though the Anthropic API supports non-streaming.

2. **Thinking blocks in assistant messages trigger error 1213** — When conversation history contains assistant messages with `thinking` or `redacted_thinking` content blocks, GLM rejects the entire request.

3. **SSE error events sent with HTTP 200** — BigModel returns HTTP 200 but sends `event: error` SSE events mid-stream instead of failing the HTTP request. Without explicit handling, these errors are forwarded to Claude Code instead of triggering failover.

4. **Tool names exceeding 64 characters** — GLM enforces a 64-character limit on tool names. Claude Code MCP tools (e.g., `mcp__plugin_everything-claude-code_github__create_pull_request_review`) exceed 70+ characters.

## Solution

### Force streaming for GLM
```go
reqCopy.Stream = true // GLM always streams; stream=false causes error 1213
```

### Strip thinking blocks from assistant messages
```go
func stripAssistantThinkingBlocks(req *anthropic.Request) {
    for i := range req.Messages {
        if req.Messages[i].Role != anthropic.RoleAssistant { continue }
        // Filter out thinking and redacted_thinking blocks
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
A degraded response (without thinking context) is better than 100% failure.

### Detect SSE error events and trigger failover
```go
// In handler.go tryStreamingTarget - after TransformStreamEvent:
for _, te := range transformedEvents {
    if te.EventType == "error" {
        // Parse error, log, and return error to trigger failover
        return 0, fmt.Errorf("Provider stream error (code %v): %v", code, message)
    }
}
```

### Truncate long tool names
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

## Key Findings
- Error 1213 from GLM has **four distinct root causes**, not just HTTP keep-alive (see `20260329-provider-connection-pooling.md`)
- GLM's HTTP 200 + SSE error pattern means you **cannot rely on HTTP status codes** for error detection
- The processing order in `glm_anthropic.go` matters: convert user thinking → strip assistant thinking → normalize content → validate blocks → truncate tools
- BigModel supports thinking via native `reasoning_content` format, but NOT via Anthropic thinking blocks on their Anthropic endpoint

## Pitfalls
- Do NOT assume HTTP 200 means the request succeeded — always check SSE event types
- Do NOT send `stream: false` to GLM Anthropic endpoint — always force `stream: true`
- When truncating tool names, include a hash suffix to avoid collisions when multiple tools share the same prefix
- The thinking block strip is a **degradation** — the model loses reasoning context from previous turns. Document this tradeoff.

## References
- Commit: `dc92427` — fix(proxy): handle BigModel error 1213 and improve monitor TUI layout
- Related: `20260329-provider-connection-pooling.md` — HTTP keep-alive root cause for same error code
- Related: `20260330-provider-thinking-block-compatibility.md` — OpenRouter thinking block handling
- Files: `internal/transformer/transformers/glm_anthropic.go`, `internal/proxy/handler.go`
