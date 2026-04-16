# SSE Streaming Implementation Checklist

**Date:** 2026-03-29
**Tags:** #streaming #sse #proxy #go

## Problem

SSE (Server-Sent Events) streaming has multiple pitfalls that caused recurring issues: `[object Object]` responses, truncated streams, buffer overflows.

## Checklist

1. **Buffer size** — Go's `bufio.Scanner` defaults to 64KB. Provider responses with large thinking blocks exceed this. Use a custom scanner with `bufio.MaxScanTokenSize` (64KB) or larger buffer.

2. **Event format** — Every SSE event must have both `event:` and `data:` lines:
   ```
   event: content_block_delta
   data: {"type":"content_block_delta",...}
   ```
   Missing `event:` lines cause "[object Object]" errors in clients.

3. **Ping filtering** — Some providers (GLM) send `event: ping` keep-alives. Filter these out — they're not part of the Anthropic SSE spec.

4. **Synthetic events** — When converting from non-streaming to streaming, generate the full event sequence: `message_start` → `content_block_delta` → `message_delta` (must include `usage.output_tokens`) → `message_stop`.

5. **GetBody for retries** — HTTP request bodies can only be read once. Set `GetBody` for retry support:
   ```go
   bodyCopy := make([]byte, len(body))
   copy(bodyCopy, body)
   httpReq.GetBody = func() (io.ReadCloser, error) {
       return io.NopCloser(bytes.NewReader(bodyCopy)), nil
   }
   ```

6. **Flush on every event** — Use `http.Flusher` interface to flush after each SSE event. Without flushing, events buffer until the stream ends.

7. **Timeout separation** — Streaming needs longer timeouts than non-streaming (10min vs 30s). A large thinking block response can take minutes.

8. **HTTP 200 is not success** — Some providers (BigModel/GLM) return HTTP 200 but send `event: error` SSE events mid-stream. Always inspect SSE event types after `TransformStreamEvent` — never rely on HTTP status codes alone for error detection. See [BigModel Error 1213 Root Cause Catalog](20260402-bigmodel-error-1213-root-cause-catalog.md) for details.
