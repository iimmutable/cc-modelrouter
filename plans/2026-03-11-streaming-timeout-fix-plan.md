# Fix Plan: Context Deadline Exceeded in Streaming Requests

**Issue:** HTTP streaming requests to Anthropic API via OpenRouter fail with "context deadline exceeded" timeout errors.

**Root Cause:** 30-second HTTP client timeout applies to all requests, but streaming responses can take several minutes.

---

## 1. Root Cause Identification

### Exact Location
- **File:** `internal/provider/client.go`
- **Line:** 29
- **Code:** `Timeout: "30s"`

### Why Current Configuration Is Insufficient
- The Go `http.Client.Timeout` applies to the entire request/response cycle including body reads
- Streaming responses can take 5-10+ minutes for complex reasoning/thinking
- No distinction between streaming and non-streaming request handling

### Additional Issues
1. **No context propagation**: `HTTPClient` interface lacks `DoWithContext()` method
2. **No streaming-specific client**: Single client used for all request types
3. **No configurable timeouts**: Timeout not exposed in provider config

---

## 2. Proposed Solution Architecture

### Architecture Overview
```
┌─────────────────────────────────────────────────────────────┐
│                        Request Flow                         │
├─────────────────────────────────────────────────────────────┤
│  Claude Code                                                │
│       │                                                     │
│       ▼                                                     │
│  ┌─────────────────┐     ┌────────────────────────────────┐ │
│  │  HTTP Request   │────▶│  Handler.detectStreamType()  │ │
│  │  (has context)  │     └────────────────────────────────┘ │
│  └─────────────────┘                │                       │
│                                     ▼                       │
│                          ┌──────────────────────────────┐   │
│                          │  Select HTTP Client         │   │
│                          │  ├─ Non-streaming: 30s      │   │
│                          │  └─ Streaming: 10min        │   │
│                          └──────────────────────────────┘   │
│                                     │                       │
│                                     ▼                       │
│                          ┌──────────────────────────────┐   │
│                          │  client.DoWithContext(ctx)  │   │
│                          └──────────────────────────────┘   │
```

### Key Components

#### 1. Extended HTTPClient Interface
```go
// internal/proxy/handler.go - Add context-aware method
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
    DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error)  // NEW
}
```

#### 2. Streaming Client Factory
```go
// internal/provider/http.go - Create separate streaming client
func (c *HTTPClient) CreateStreamingClient(streamingTimeout time.Duration) *http.Client {
    // Same transport but with extended/no timeout for streaming
    return &http.Client{
        Timeout:   streamingTimeout,  // 10 minutes for streaming
        Transport: c.client.Transport,
    }
}
```

#### 3. Configurable Timeouts
```go
// internal/config/types.go - Add to ProviderConfig
type ProviderConfig struct {
    APIKey           string   `json:"apiKey"`
    BaseURL          string   `json:"baseURL"`
    Models           []string `json:"models"`
    Transformer      string   `json:"transformer,omitempty"`
    Timeout          string   `json:"timeout,omitempty"`          // NEW: Non-streaming
    StreamingTimeout string   `json:"streamingTimeout,omitempty"` // NEW: Streaming
}
```

---

## 3. Implementation Plan

### Files to Modify

| File | Changes |
|------|---------|
| `internal/provider/client.go` | Add `StreamingTimeout` to config, update Defaults() |
| `internal/provider/http.go` | Add `CreateStreamingClient()`, implement DoWithContext() |
| `internal/proxy/handler.go` | Add `detectStreamType()`, update interface, use context-aware calls |
| `internal/config/types.go` | Add timeout fields to ProviderConfig |
| `internal/cli/start.go` | Pass timeout configs to client creation |

### Implementation Steps

#### Step 1: Update ProviderConfig to support timeouts
- Add `Timeout` field (non-streaming, default 30s)
- Add `StreamingTimeout` field (streaming, default 10m)

#### Step 2: Update HTTPClient interface
- Add `DoWithContext(ctx, req)` method
- Update implementations

#### Step 3: Implement streaming client creation
- Create new HTTP client with extended timeout
- Use `10 minutes` for streaming (sufficient for most use cases)

#### Step 4: Update handler to use context-aware calls
- Detect if request is streaming
- Select appropriate client (streaming vs non-streaming)
- Pass context to `DoWithContext()`

#### Step 5: Add logging for timeout configuration
- Log which client/timeout is being used
- Debug logging for streaming requests

### Backward Compatibility
- If `Timeout` not specified: use current default (30s)
- If `StreamingTimeout` not specified: use 10 minutes
- Existing configs continue to work unchanged

---

## 4. Recommended Timeout Values

| Request Type | Timeout | Rationale |
|--------------|---------|------------|
| **Non-streaming** | 30 seconds | Sufficient for sync API calls |
| **Streaming** | 10 minutes | Handles complex reasoning, large outputs |

**Note:** The 10-minute streaming timeout is a safety net. For truly long-running streams, the client's context (from the original HTTP request) should drive cancellation, not the HTTP client timeout. This allows:
- User cancels → context cancels → stream stops
- Network issues → context expires → stream stops
- No artificial timeout fires during normal long operations

---

## 5. Risk Analysis

### Potential Issues

| Risk | Mitigation |
|------|------------|
| **Memory leaks from hanging connections** | Context cancellation will terminate idle connections; transport settings handle cleanup |
| **Infinite hangs if stream never completes** | Context from original request provides timeout; client timeout as safety net |
| **Breaking non-streaming requests** | Separate clients ensure non-streaming keeps 30s limit |
| **Security implications** | No security impact - only timeout duration changes |

### Edge Cases Handled

1. **User cancels request** → Context cancels → Stream terminates cleanly
2. **Network interruption** → Context cancels → Proper cleanup
3. **Very long response (10+ min)** → Uses client timeout as safety net
4. **Provider goes unresponsive** → Context and client timeout both trigger

---

## 6. Testing Strategy

### Unit Tests
- Test streaming client creation with custom timeouts
- Test context cancellation propagation
- Test interface compliance

### Integration Tests
- Test streaming request with artificial delay
- Test timeout when response exceeds threshold
- Test context cancellation mid-stream

### Manual Testing
- Trigger real streaming request to OpenRouter
- Verify no timeout errors for long responses
- Verify cancellation works properly

---

## 7. Success Criteria

- [ ] Streaming requests complete for responses up to 10 minutes
- [ ] No "context deadline exceeded" errors during normal streaming
- [ ] Non-streaming requests maintain 30-second timeout
- [ ] User cancellation works properly
- [ ] Memory cleanup works (no leaked connections)
- [ ] Configuration is backward compatible

---

## 8. Configuration Example

```json
{
  "providers": {
    "openrouter-anthropic": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api/v1",
      "models": ["anthropic/claude-sonnet-4-20250514"],
      "timeout": "30s",
      "streamingTimeout": "10m"
    }
  }
}
```

---

*Plan created: 2026-03-11*
*Issue: Context deadline exceeded in streaming requests*