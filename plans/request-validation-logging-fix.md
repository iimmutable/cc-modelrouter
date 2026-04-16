# Fix Plan: Add Request Validation Logging

## Problem

When receiving malformed requests from Claude Code, the proxy returns a 400 Bad Request error but does NOT log:
1. The actual error message from JSON unmarshaling
2. The request body that failed to parse
3. Any context about what went wrong

This makes debugging impossible when errors occur.

## Root Cause

In `internal/proxy/handler.go` lines 154-158:

```go
var req anthropic.Request
if err := json.Unmarshal(body, &req); err != nil {
    http.Error(w, "Invalid request format", http.StatusBadRequest)
    return  // No logging - silent failure!
}
```

Similarly at lines 147-151:

```go
body, err := io.ReadAll(io.LimitReader(r.Body, h.maxRequestSize))
if err != nil {
    http.Error(w, "Failed to read request", http.StatusBadRequest)
    return  // No logging
}
```

## Solution

Add comprehensive logging for request validation failures:

1. **Log read errors** with context
2. **Log unmarshal errors** with the actual error message
3. **Log a snippet of the request body** for debugging (truncated to avoid logging sensitive data)
4. **Add request ID correlation** to track the full request lifecycle

## Implementation

### File: `internal/proxy/handler.go`

#### Change 1: Log body read errors (lines 147-151)

```go
body, err := io.ReadAll(io.LimitReader(r.Body, h.maxRequestSize))
if err != nil {
    logging.Errorf("[REQUEST VALIDATION] Failed to read request body: %v", err)
    http.Error(w, "Failed to read request", http.StatusBadRequest)
    return
}
```

#### Change 2: Log JSON unmarshal errors with context (lines 154-158)

```go
var req anthropic.Request
if err := json.Unmarshal(body, &req); err != nil {
    // Log the actual error and a snippet of the request body for debugging
    bodySnippet := string(body)
    if len(bodySnippet) > 500 {
        bodySnippet = bodySnippet[:500] + "..."
    }
    logging.Errorf("[REQUEST VALIDATION] Invalid request format: %v", err)
    logging.Errorf("[REQUEST VALIDATION] Request body snippet: %s", bodySnippet)
    http.Error(w, "Invalid request format", http.StatusBadRequest)
    return
}
```

#### Change 3: Add validation log for successful parsing

After successful unmarshal, add optional debug logging:

```go
// Log successful request parsing (debug level)
logging.Debugf("[REQUEST VALIDATION] Successfully parsed request: model=%s, messages=%d, stream=%v",
    req.Model, len(req.Messages), req.Stream)
```

## Testing

1. **Test with valid request** - should log successful parsing at debug level
2. **Test with invalid JSON** - should log error with body snippet
3. **Test with oversized request** - should log read error
4. **Verify log format** - ensure error messages are clear and actionable

## Benefits

1. **Debuggability**: Can see exactly what failed and why
2. **Troubleshooting**: Request body snippets help identify malformed requests
3. **Monitoring**: Can track validation failure rates
4. **Security**: Truncated body snippets avoid logging sensitive data

## Alternative Consideration

For production environments, consider adding a config option to control whether to log request bodies (for privacy/security).

## Related Issues

- Issue: Usage tracking tests are failing (may be related to request format issues)
- Issue: GLM 400 Bad Request errors (different - those are provider-side, but better logging helps distinguish)
