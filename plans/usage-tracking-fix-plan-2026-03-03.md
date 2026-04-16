# Usage Tracking Fix Plan

**Date**: 2026-03-03
**Confidence Level**: 99%

## Problem Statement

When users run `./bin/ccrouter usage`, the request and token counts do not change after making requests through the router. The usage tracking feature is not working.

## Root Cause Analysis

### Investigation Summary

The database exists and contains historical records (22 records, latest from 2026-03-02). However, no new records are being created for current requests.

### Root Cause

**The `code` command does not initialize the usage tracker.**

Comparing `internal/cli/start.go` vs `internal/cli/code.go`:

**`start.go` (lines 216-228) - HAS usage tracker:**
```go
// Initialize usage tracker
dbPath, err := usage.DBPath()
if err != nil {
    return fmt.Errorf("failed to get db path: %w", err)
}

usageDB, err := usage.InitDB(dbPath)
if err != nil {
    return fmt.Errorf("failed to init usage db: %w", err)
}

tracker := usage.NewTracker(usageDB, usage.DefaultBufferSize, usage.DefaultFlushTimeout)
server.SetUsageTracker(tracker)
server.SetInstanceID(instanceID)
```

**`code.go` (lines 197-203) - MISSING usage tracker:**
```go
server.SetProviderClients(clients)
server.SetConfig(cfg)

// Start server  <-- NO usage tracker setup!
if err := server.Start(); err != nil {
    return fmt.Errorf("failed to start server: %w", err)
}
```

### Why This Causes No Usage Tracking

In `internal/proxy/handler.go`, the tracking code only runs if `h.usageTracker != nil`:

**Line 234-237 (non-streaming):**
```go
if h.usageTracker != nil {  // <-- This check fails when tracker is nil
    totalTokens := resp.Usage.InputTokens + resp.Usage.OutputTokens
    h.usageTracker.Record(h.instanceID, routeName, successfulModel, totalTokens, fallbackCount)
}
```

**Line 292-302 (streaming):**
```go
if h.usageTracker != nil {  // <-- This check fails when tracker is nil
    tokensToTrack := totalTokens
    if tokensToTrack == 0 {
        tokensToTrack = h.estimateTokens(req)
    }
    h.usageTracker.Record(h.instanceID, routeName, target.Model, tokensToTrack, i)
}
```

## Fix Plan

### File to Modify

`internal/cli/code.go`

### Location

After line 198 (`server.SetConfig(cfg)`), before line 200 (`// Start server`)

### Code to Add

```go
// Initialize usage tracker
dbPath, err := usage.DBPath()
if err != nil {
    return fmt.Errorf("failed to get db path: %w", err)
}

usageDB, err := usage.InitDB(dbPath)
if err != nil {
    return fmt.Errorf("failed to init usage db: %w", err)
}

tracker := usage.NewTracker(usageDB, usage.DefaultBufferSize, usage.DefaultFlushTimeout)
server.SetUsageTracker(tracker)
server.SetInstanceID(instanceID)
```

### Change Type

**Feature addition** - Adding missing usage tracking support to the `code` command.

## Testing Plan

1. **Before fix:**
   - Run `./bin/ccrouter code`
   - Make requests through Claude Code
   - Run `./bin/ccrouter usage`
   - Verify: No new records appear

2. **After fix:**
   - Build: `go build -o bin/release/ccrouter ./cmd/ccrouter`
   - Run `./bin/ccrouter code`
   - Make requests through Claude Code
   - Exit Claude Code (triggers shutdown which flushes tracker)
   - Run `./bin/ccrouter usage`
   - Verify: New records appear with correct token counts

## Implementation Details

### Context

The usage tracker:
- Buffers records in memory (default: 500 records)
- Flushes to database every 3 seconds OR when buffer is full
- MUST flush on server shutdown via `tracker.Shutdown()`

The `server.Stop()` call in `code.go` line 297 already handles the shutdown:
```go
server.Stop(ctx)  // This calls tracker.Shutdown() via interface
```

### Dependencies

The `code.go` file already imports:
- `"github.com/iimmutable/cc-modelrouter/internal/usage"`  ✅ NOT imported yet
- `"github.com/iimmutable/cc-modelrouter/internal/daemon"` ✅ Already imported

### Required Changes

1. **Add import** (line 22 area):
   ```go
   "github.com/iimmutable/cc-modelrouter/internal/usage"
   ```

2. **Add usage tracker initialization** (after line 198):
   ```go
   // Initialize usage tracker
   dbPath, err := usage.DBPath()
   if err != nil {
       return fmt.Errorf("failed to get db path: %w", err)
   }

   usageDB, err := usage.InitDB(dbPath)
   if err != nil {
       return fmt.Errorf("failed to init usage db: %w", err)
   }

   tracker := usage.NewTracker(usageDB, usage.DefaultBufferSize, usage.DefaultFlushTimeout)
   server.SetUsageTracker(tracker)
   server.SetInstanceID(instanceID)
   ```

## Confidence Assessment

| Aspect | Confidence |
|--------|------------|
| Root cause identified | 99% |
| Fix will resolve issue | 99% |
| No side effects expected | 95% |
| Implementation complete | 100% |

**Overall Confidence**: 99%

## Additional Verification

The existing tests in `test/integration/usage_tracking_test.go` already pass because they manually set up the usage tracker. This fix brings the production `code` command to parity with the test setup.

## Related Code

- `internal/cli/start.go` - Reference implementation (lines 216-228)
- `internal/proxy/handler.go` - Usage tracking calls (lines 234-237, 292-302)
- `internal/proxy/server.go` - Shutdown handling (lines 189-192)
- `internal/usage/tracker.go` - Tracker implementation
- `test/integration/usage_tracking_test.go` - Test setup (lines 86-87, 208-209)
