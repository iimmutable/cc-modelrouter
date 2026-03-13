# ConnectionRefused Error Fix

**Issue:** `ccrouter code` command fails with "Unable to connect to API (ConnectionRefused)" error

**Date:** 2025-02-24

**Status:** Fixed

---

## Problem Description

When running `ccrouter code`, Claude Code would immediately fail with a `ConnectionRefused` error when trying to connect to the router. This was an intermittent issue that depended on timing and system load.

### Symptoms

```
Unable to connect to API (ConnectionRefused)
```

The error occurred immediately after launching `ccrouter code`, before any API requests could be made.

---

## Root Cause Analysis

### Race Condition in Server Startup

The issue was caused by a race condition in `internal/proxy/server.go`:

1. `server.Start()` launched the HTTP server in a goroutine
2. `Start()` returned **immediately** without waiting for the server to be ready
3. `runCode()` immediately launched Claude Code
4. Claude Code tried to connect before the server had finished binding to the port
5. **Result:** `ConnectionRefused` error

### Original Problematic Code

```go
// internal/proxy/server.go - Start() method (OLD)
func (s *Server) Start() error {
    // ... setup code ...

    go func() {
        if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
            // Log error
        }
    }()

    return nil  // Returns immediately!
}
```

The `ListenAndServe()` method is called in a goroutine, and `Start()` returns without waiting for the server to actually be ready to accept connections.

---

## Solution

### Implementation: Server Readiness Channel

The fix implements a server readiness guarantee using explicit listener creation:

1. Create the listener explicitly using `net.Listen()`
2. Only return from `Start()` after the listener is accepting connections
3. Signal readiness through a channel to the goroutine

### New Implementation

```go
// internal/proxy/server.go - Start() method (NEW)
func (s *Server) Start() error {
    // ... setup code ...

    // Create readiness channel before starting
    ready := make(chan struct{})
    s.ready = ready

    s.running = true
    s.mu.Unlock()

    // Create listener explicitly to know when we're ready
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        s.mu.Lock()
        s.running = false
        s.ready = nil
        s.mu.Unlock()
        return fmt.Errorf("failed to listen on %s: %w", addr, err)
    }

    // Launch server in goroutine
    go func() {
        // Signal readiness - listener is already accepting connections
        close(ready)
        if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
            // Log error
        }
    }()

    return nil  // Server is guaranteed to be ready
}
```

### Key Changes

| Change | Description |
|--------|-------------|
| Added `ready chan struct{}` field | Channel to signal server readiness |
| Use `net.Listen()` explicitly | Creates listener before launching goroutine |
| Close channel in goroutine | Signals that listener is accepting connections |
| Local `ready` variable | Avoids race condition with `Stop()` |
| Clean up in `Stop()` | Sets `s.ready = nil` when stopping |

---

## Files Modified

### `internal/proxy/server.go`

**Lines changed:** 30-40 (struct), 120-165 (Start method), 148-165 (Stop method)

- Added `ready chan struct{}` field to `Server` struct
- Modified `Start()` to use explicit listener creation
- Modified `Stop()` to clean up readiness channel

### `internal/proxy/server_test.go`

**Changes:**
- Added `TestServer_StartWithFixedPortIsReady` test
- Updated `TestServer_IsRunning` to remove `time.Sleep()`
- Updated `TestServer_StartTwice` to remove `time.Sleep()`
- Updated `TestServer_TimeoutConfiguration` to remove `time.Sleep()`
- Updated `TestServer_ShutdownWithUsageTracker` to remove `time.Sleep()`

---

## Testing

### New Test: `TestServer_StartWithFixedPortIsReady`

This test verifies that the server is ready immediately after `Start()` returns:

```go
func TestServer_StartWithFixedPortIsReady(t *testing.T) {
    serverCfg := &ServerConfig{
        Host: "127.0.0.1",
        Port: 19101,
    }

    server, err := NewServer(serverCfg)
    // ... setup config ...

    err = server.Start()  // Blocks until ready
    if err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Make HTTP request WITHOUT any sleep
    client := &http.Client{Timeout: 100 * time.Millisecond}
    req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:19101/v1/models", nil)
    resp, err := client.Do(req)  // This now works reliably
    if err != nil {
        t.Fatalf("Server not ready after Start returned: %v", err)
    }
    // ...
}
```

### Test Results

```bash
$ go test ./internal/proxy/...
ok      github.com/iimmutable/cc-modelrouter/internal/proxy    0.482s

$ go test ./...
ok      github.com/iimmutable/cc-modelrouter/internal/cli       0.653s
ok      github.com/iimmutable/cc-modelrouter/internal/config    1.435s
ok      github.com/iimmutable/cc-modelrouter/internal/daemon    1.183s
ok      github.com/iimmutable/cc-modelrouter/internal/provider  2.263s
ok      github.com iimmutable/cc-modelrouter/internal/proxy     0.482s
ok      github.com/iimmutable/cc-modelrouter/internal/router    0.311s
ok      github.com/iimmutable/cc-modelrouter/internal/transformer   1.299s
ok      github.com/iimmutable/cc-modelrouter/internal/usage    1.641s
```

All tests pass.

---

## Verification

To verify the fix works:

1. **Normal usage:**
   ```bash
   ccrouter code
   ```
   Should no longer show `ConnectionRefused` errors.

2. **Stress test (multiple rapid starts):**
   ```bash
   for i in {1..10}; do
     timeout 5 ccrouter code --help
   done
   ```
   Should complete without connection errors.

3. **Direct server test:**
   ```bash
   ccrouter start &
   sleep 0.1  # Minimal sleep now
   curl http://localhost:8081/v1/models
   ```
   Should work reliably even with minimal sleep.

---

## Related Issues

This fix also resolves:
- Intermittent failures in automated tests that used `time.Sleep()` to wait for server startup
- Race conditions in any code that depends on `server.Start()` guaranteeing readiness
- Potential issues with health check endpoints

---

## Future Considerations

### Potential Enhancements

1. **Health Check Endpoint:** Add `/health` endpoint that returns 200 when server is ready
2. **Startup Timeout:** Add configurable timeout for server startup
3. **Graceful Degradation:** If port is in use, provide clearer error messages

### Monitoring

The readiness channel could be exposed for monitoring purposes:
```go
// Check if server is ready
select {
case <-s.ready:
    // Server is ready
default:
    // Server not ready yet
}
```

---

## References

- Original issue: "ConnectionRefused when running ccrouter code"
- Fix commit: (add commit hash when merged)
- Related files: `internal/cli/code.go`, `internal/proxy/server.go`
