# Troubleshooting

Common issues and their solutions when using ccrouter.

## Port Conflicts

### Issue: Router appears to start but requests return unexpected responses

**Symptoms:**
- Running `ccrouter start` succeeds
- Requests to the router return unexpected responses (e.g., JSON errors from a different service)
- SSE events are malformed or missing event type lines

**Root Cause:**
Another service is already listening on the configured port (default: 8081). The router's HTTP server starts but can't bind to the port, so requests go to the other service.

**Diagnosis:**
```bash
# Check what process is listening on the port
lsof -i :8081
# or
netstat -an | grep 8081
# or
lsof -nP -iTCP -sTCP:LISTEN | grep 8081
```

If you see something like:
```
COMMAND    PID   USER   FD   TYPE  DEVICE SIZE/OFF NODE NAME
node     77308 avextk   13u  IPv6  ...   TCP *:8081 (LISTEN)
```

This indicates a Node.js server (or other service) is using the port.

**Solution:**
1. **Stop the conflicting service:**
   ```bash
   kill <PID>
   # e.g., kill 77308
   ```

2. **Or use a different port:**
   ```bash
   ccrouter start --port 8082
   ```

3. **Or configure a different default port in your config:**
   ```json
   {
     "server": {
       "port": 8082,
       "host": "localhost"
     }
   }
   ```

**Prevention:**
To avoid port conflicts in the future:
- Use a unique port for development environments
- Check for running instances with `ccrouter status` before starting
- Consider using environment-specific ports (e.g., 8081 for dev, 8082 for staging)

## Malformed SSE Events ("[object Object]" responses)

### Issue: Claude Code shows "[object Object]" instead of actual responses

**Symptoms:**
- Using `ccrouter code` with GLM or other providers
- Responses contain text like "[object Object]" instead of actual AI responses
- SSE events appear to be truncated or malformed

**Root Cause:**
This was caused by port conflicts (see above) where a different server was handling requests. The router code itself correctly handles SSE events with proper event type lines.

**Expected SSE Format:**
```
event: message_start
data: {"type":"message_start",...}

event: content_block_delta
data: {"type":"content_block_delta",...}

event: message_stop
data: {"type":"message_stop"}
```

**Diagnosis:**
```bash
# Test the router directly with curl
curl -s -N -X POST http://localhost:8081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-4.7","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"Say hello"}]}'
```

Look for proper `event:` lines preceding each `data:` line.

**Solution:**
Follow the port conflict resolution steps above. Once the correct router process is handling requests, SSE events will include proper event types.

## GLM Provider Issues

### Issue: "ping" events in GLM responses

**Information:**
GLM (BigModel) API sends periodic `ping` events to keep the connection alive. These are automatically filtered out by the router as they are not part of the Anthropic SSE format.

**Log message:**
```
[STREAM] Filtering out non-Anthropic event: ping
```

This is normal behavior and not an error. The router correctly filters these events before passing responses to clients.

## Debugging Mode

### Enable Request/Response Logging

The router includes a logging interceptor that logs all requests and responses. This is enabled by default when using the `start` command.

**View logs in real-time:**
```bash
# Logs are written to stdout/stderr of the router process
ccrouter start 2>&1 | tee router.log
```

**Common log patterns:**
```
[REQUEST] Model: glm-4.7, Stream: true, Messages: 1, MaxTokens: 50
[ROUTE] Detected: default, Targets: 2
[STREAM] Starting stream to bigmodel/glm-4.7
[STREAM] Filtering out non-Anthropic event: ping
[STREAM] Stream completed with bigmodel/glm-4.7, fallbacks: 0
```

### Check Router Status

```bash
# Show all running instances
ccrouter status

# Output:
# INSTANCE ID           PID    PORT   STATUS   CONFIG  UPTIME
# inst_20260224_233233  42381  8081   running  global  5m30s
```

### Clean Up Stale Instances

```bash
# Remove dead/stale instance files
ccrouter clean
```

## Configuration Issues

### Issue: Provider not found

**Symptoms:**
```
Error: provider not found: xyz
```

**Solution:**
Check your configuration file includes the provider:
```json
{
  "providers": {
    "bigmodel": {
      "apiKey": "...",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-4.7"]
    }
  }
}
```

### Issue: Transformer not found

**Symptoms:**
```
Error: transformer not found: xyz
```

**Solution:**
Transformers are registered based on provider names. Valid transformer names:
- `anthropic` (default)
- `openrouter`
- `gemini`
- `qwen`
- `glm`

Use a supported transformer name or omit the `transformer` field to use the default (provider name).

## Performance Issues

### Issue: Slow streaming responses

**Potential causes:**
1. **Network latency:** Check your connection to the provider's API
2. **Provider rate limits:** Some providers have rate limits that may cause delays
3. **Interceptor overhead:** Response interceptors add processing time

**Diagnosis:**
```bash
# Check timing in logs
[STREAM] Starting stream to bigmodel/glm-4.7
# ... time passes ...
[STREAM] Stream completed with bigmodel/glm-4.7
```

**Solution:**
- Use geographically closer provider endpoints
- Check provider status for outages
- Remove unnecessary interceptors

### Issue: High memory usage

**Potential causes:**
1. **Large streaming buffers:** Long-lived streaming connections
2. **Usage tracking:** Large buffers in the usage tracker

**Diagnosis:**
```bash
# Check memory usage
ps aux | grep ccrouter
```

**Solution:**
- Reduce usage tracker buffer size in configuration
- Regularly restart the router (`ccrouter restart`)

## ConnectionRefused Error

### Issue: `ccrouter code` fails with "Unable to connect to API (ConnectionRefused)"

**Symptoms:**
- Running `ccrouter code` immediately shows connection error
- Claude Code cannot connect to the router
- Error appears right after startup

**Status:** **Fixed in version 0.4.0+**

This was caused by a race condition in server startup where the `Start()` method returned before the server was actually listening on the port. This has been fixed by implementing a server readiness guarantee using explicit listener creation.

**If you still encounter this issue:**

1. **Check for port conflicts:**
   ```bash
   lsof -i :8081
   ```

2. **Try a different port:**
   ```bash
   ccrouter code --port 8082
   ```

3. **Check the router is actually starting:**
   ```bash
   ccrouter start
   ```

4. **See the detailed fix documentation:** [connection-refused-fix.md](connection-refused-fix.md)

## Thinking Block Validation Errors (CRITICAL)

### Issue: "Invalid input: expected string, received array" during OpenRouter failover

**Status:** **Fixed in commit fa1d7ad** (comprehensive fix for thinking block signature validation errors)

**Symptoms:**
- 400 Bad Request errors from OpenRouter when using Anthropic models with thinking blocks
- Error occurs during **failover** when switching from GLM to OpenRouter Anthropic models
- Error messages like:
  - `"Invalid input: expected string, received array"` at path `["messages", N, "content"]`
  - Multiple messages fail (messages 1, 3, 5, 7, etc.) - all assistant messages with thinking blocks
  - `"invalid_union"` errors from provider validation

**Root Cause:**

When GLM (or other providers) return assistant messages containing thinking blocks, and then those messages are sent to OpenRouter with an Anthropic model during failover, OpenRouter rejects the request because:

1. **GLM returns thinking blocks** in assistant messages as:
   ```json
   "content": [{"type": "thinking", "thinking": "..."}]
   ```

2. **These messages persist** in the conversation history

3. **OpenRouter Anthropic models** have strict validation that:
   - Rejects single-element content arrays for thinking blocks
   - Requires multi-element arrays or string format
   - Expects proper signature handling

4. **The previous code** skipped normalization for Anthropic models via OpenRouter, incorrectly assuming they would accept single thinking blocks like direct Anthropic API does.

**The Error Pattern:**
```
"expected string, received array" at messages[1, 3, 5, 7...].content
```

These are all **assistant messages** (odd-numbered in zero-indexed array with user message at 0) that contain thinking blocks returned by GLM.

**Diagnosis:**
Check the error messages in logs:
```bash
grep "expected string, received array" ~/.cc-modelrouter/logs/*.log
```

Look for validation errors at paths like:
- `["messages", 1, "content"]` - First assistant message
- `["messages", 3, "content"]` - Second assistant message
- Pattern continues for all assistant messages with thinking blocks

**Solution Implemented:**

The fix in `internal/transformer/transformers/openrouter.go`:

1. **Always normalize thinking blocks** for OpenRouter, regardless of target model type
2. This adds a text block with single space to assistant messages with only thinking blocks
3. The resulting multi-element array `[{thinking}, {text: " "}]` passes OpenRouter's validation

```go
// CRITICAL FIX: Always normalize thinking blocks for OpenRouter
// Both Anthropic and non-Anthropic models via OpenRouter require multi-element arrays
// to prevent "expected string, received array" validation errors
normalizeThinkingBlockMessages(&reqCopy)
```

**Why This Works:**

1. **Normalization adds a text block**: `normalizeThinkingBlockMessages` adds a text block with `" "` to assistant messages that have only a thinking block
2. **Multi-element array format**: The resulting content is `[{thinking}, {text: " "}]` which is a multi-element array
3. **OpenRouter accepts multi-element arrays**: The validation error occurs with single-element arrays, not multi-element ones
4. **Signature handling remains correct**: The signature field is set to `""` for Anthropic models as required

**Prevention:**

The fix is applied at the transformer level, so all requests to OpenRouter (both direct and failover) will have properly normalized thinking blocks.

**Related Issues:**
- State corruption across failover attempts - Fixed with deep copying
- Signature field validation - Fixed with `*string` pointer type
- User message thinking blocks - Fixed with `convertUserThinkingToText`

**See Also:**
- [Transformer documentation](transformers.md#thinking-block-handling-critical)
- Commit fa1d7ad: "fix(transformer): comprehensive fix for thinking block signature validation errors"

---

## State Corruption in Failover Scenarios

### Issue: Request modifications affect subsequent provider attempts

**Status:** **Fixed with Deep Copying**

**Symptoms:**
- First provider attempt succeeds but modifies the request
- Second provider attempt fails with validation errors
- Errors like "expected string, received array" on second attempt

**Root Cause:**
When transformers modify the request in-place (e.g., adding text blocks to thinking blocks), these modifications persist across failover attempts. The second provider receives a modified request that doesn't match its expected format.

**Solution:**
Transformers now create deep copies of the request before modification:

```go
var reqCopy anthropic.Request
reqJSON, err := json.Marshal(req)
if err != nil {
    return nil, fmt.Errorf("failed to marshal request for deep copy: %w", err)
}
if err := json.Unmarshal(reqJSON, &reqCopy); err != nil {
    return nil, fmt.Errorf("failed to unmarshal request deep copy: %w", err)
}
```

This ensures each provider receives an independent copy of the original request.

---

## Recurring Thinking Block Validation Errors (March 2026)

### Issue: "Invalid input: expected string, received array" - INCOMPLETE NORMALIZATION

**Status:** **FIXED in commit (March 2026)** - Expanded normalization to handle multiple thinking blocks

**Symptoms:**
- 400 Bad Request errors from OpenRouter when using Anthropic models
- Error occurs intermittently after multiple GLM responses accumulate in conversation history
- Error pattern:
  ```
  "code": "invalid_union",
  "path": ["messages", 1, "content"],
  "message": "Invalid input"
  ```

**Root Cause (March 2026):**

The `normalizeThinkingBlockMessages()` function in `anthropic.go` originally only added a text block when there was **exactly one thinking block** at index 0:

```go
// OLD (BUGGY) LOGIC
if len(content) == 1 && content[0].Type == "thinking" {
    // Add text block
}
```

This failed for:
1. **Multiple thinking blocks**: `[{thinking}, {thinking}, {thinking}]` - len > 1, condition fails
2. **Thinking after text**: `[{text}, {thinking}]` - content[0].Type != "thinking", condition fails

**Fix Applied (March 2026):**

Expanded normalization to check if content has ONLY thinking blocks (regardless of count or position), then add text block:

```go
// Check if content has ONLY thinking blocks (no text blocks)
hasOnlyThinkingBlocks := len(content) > 0
hasTextBlock := false

for _, block := range content {
    if block.Type == "text" {
        hasTextBlock = true
        break
    }
    if block.Type != "thinking" {
        hasOnlyThinkingBlocks = false
        break
    }
}

if hasOnlyThinkingBlocks && !hasTextBlock {
    // Add text block
}
```

**Files Changed:**
- `internal/transformer/transformers/anthropic.go` - Fixed normalization logic
- `internal/transformer/transformers/normalization_test.go` - Added test cases

---

## Single Content Block Validation Errors (March 2026 - UPDATED)

### Issue: "Invalid input: expected string, received array" - INCOMPLETE NORMALIZATION

**Status:** **Requires additional fix** - Current normalization only handles thinking-only content

**Symptoms:**
- 400 Bad Request errors from OpenRouter when using Anthropic models
- Error occurs intermittently after conversation history accumulates diverse content types
- Error pattern:
  ```
  "code": "invalid_union",
  "errors": [
    {"expected": "string", "code": "invalid_type", "path": [], "message": "Invalid input: expected string, received array"},
    {"expected": "string", "code": "invalid_type", "path": [3, "data"], "message": "Invalid input: expected string, received undefined"}
  ],
  "path": ["messages", 1, "content"]
  ```

**Root Cause Analysis (March 2026):**

The `normalizeThinkingBlockMessages()` function **only normalizes content with ONLY thinking blocks**. It does NOT handle:

1. **Single image blocks** → remains single-element array → fails validation
2. **Single tool_result blocks** → remains single-element array → fails validation
3. **Mixed non-text content** (e.g., thinking + image) → not normalized → fails validation

**Why the Error Path [3]["data"]:**

The path `[3]["data"]` indicates there's a content block at index 3 with a missing `data` field:
- Likely an image block with corrupted/missing `source.data`
- Or a transformed response that lost required fields during conversion

**MessageContent Marshaling Behavior:**

In `pkg/api/anthropic/types.go`, the `MarshalJSON()` method:
```go
// If single text block, marshal as string
if len(merged) == 1 && merged[0].Type == "text" {
    return json.Marshal(merged[0].Text)
}
// Otherwise marshal as array
return json.Marshal(merged)
```

**Only single text blocks become strings**. All other single-element content (thinking, image, tool_result) become arrays and fail OpenRouter validation.

**Fix Required:**

Replace thinking-specific normalization with generic single-element content handling:

```go
// normalizeSingleElementContent ensures ANY single-element content array
// (not just thinking) is normalized to prevent provider validation errors.
func normalizeSingleElementContent(req *anthropic.Request) {
    for i := range req.Messages {
        if req.Messages[i].Role != anthropic.RoleAssistant {
            continue
        }

        content := req.Messages[i].Content
        if len(content) == 0 {
            continue
        }

        // Check if content has a text block
        hasTextBlock := false
        for _, block := range content {
            if block.Type == "text" {
                hasTextBlock = true
                break
            }
        }

        // If single-element array WITHOUT text block, add text block
        if len(content) == 1 && !hasTextBlock {
            req.Messages[i].Content = append(content, anthropic.ContentBlock{
                Type: "text",
                Text: " ",
            })
        }
    }
}
```

**Additionally, add block validation to repair corrupted blocks:**

```go
// validateAndRepairBlocks checks for blocks with missing required fields
func validateAndRepairBlocks(req *anthropic.Request) {
    for i := range req.Messages {
        for j := range req.Messages[i].Content {
            block := &req.Messages[i].Content[j]

            // Validate image blocks
            if block.Type == "image" && block.Source != nil {
                if block.Source.Data == "" {
                    block.Type = "text"
                    block.Text = "[Image: data unavailable]"
                    block.Source = nil
                }
            }

            // Validate thinking blocks
            if block.Type == "thinking" && block.Thinking == "" {
                block.Type = "text"
                block.Text = "[Thinking: content unavailable]"
            }
        }
    }
}
```

**Files to Modify:**
- `internal/transformer/transformers/anthropic.go` - Replace normalization function
- `internal/transformer/transformers/openrouter.go` - Update call sites

---

## Getting Help

If you encounter issues not covered here:

1. **Enable debug logging:** Look for log messages that indicate where the failure occurs
2. **Test the provider directly:** Use curl to test the provider API without the router
3. **Check the logs:** Router logs often contain detailed error messages
4. **Review configuration:** Ensure all required fields are present and correct
5. **Check for thinking blocks:** If using extended thinking, be aware of provider-specific validation requirements
6. **File an issue:** Include logs, configuration, and steps to reproduce
