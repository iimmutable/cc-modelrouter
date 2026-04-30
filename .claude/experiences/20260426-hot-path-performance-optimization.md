# Hot Path Performance Optimization Patterns

**Date:** 2026-04-26
**Tags:** #performance #go #optimization #sync #pooling #streaming

## Problem

As request volume grew, hot paths in the proxy showed contention from mutex locks and excessive allocations during SSE streaming. Usage tracking submissions also blocked on writes.

## Optimizations Applied

### 1. Lock-Free Transformer Registry (`sync.Map`)

Replaced `sync.RWMutex` with `sync.Map` for transformer lookups. Every request hits the registry, so even minor contention adds up.

```go
// Before: RWMutex contention on every request
registry.mu.RLock()
t := registry.transformers[name]
registry.mu.RUnlock()

// After: Lock-free lookup
t, ok := registry.store.Load(name)
```

**When to use:** Read-heavy maps where writes are rare (registration at startup only).

### 2. Buffered Channel Usage Tracking

Usage records submitted via buffered channel instead of direct mutex-protected writes.

```go
// Non-blocking submit
select {
case tracker.ch <- record:
default:
    // Channel full — drop and count overflow
    atomic.AddInt64(&tracker.dropped, 1)
}
```

**Trade-off:** Records may be dropped under extreme load, but tracked via `dropped` counter.

### 3. `sync.Pool` for SSE Scanner Buffers

SSE streaming allocates large buffers (64KB+) per request. Pooling reduces GC pressure.

```go
var scannerBufPool = sync.Pool{
    New: func() any { return make([]byte, bufio.MaxScanTokenSize) },
}

buf := scannerBufPool.Get().([]byte)
defer scannerBufPool.Put(buf)
```

**Important:** Always `Put()` back in the buffer, even on error paths.

### 4. Pre-Allocated SSE Prefixes

Direct byte slice operations instead of `fmt.Sprintf` for SSE event formatting.

```go
// Before: allocates new string every event
eventStr := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)

// After: pre-allocated byte slices, append only
var eventPrefix = []byte("event: ")
var dataPrefix = []byte("data: ")
```

### 5. Flush Timeout Reduction

Reduced from 3s to 1s for better responsiveness when backpressure occurs.

## Key Insights

- **Benchmark before optimizing** — JSON vs gob deep copy was benchmarked; gob was ~30% slower despite being "native Go" serialization.
- **Profile the actual bottleneck** — The biggest wins came from `sync.Map` (transformer registry) and `sync.Pool` (SSE buffers), not from micro-optimizations like string formatting.
- **Track dropped records** — When using fire-and-forget patterns (buffered channels), always track overflow so you know when to tune buffer size.

## Files

- `internal/transformer/registry.go` — `sync.Map` transformer registry
- `internal/usage/tracker.go` — Buffered channel usage tracking
- `internal/proxy/streaming.go` — `sync.Pool` scanner buffers, SSE prefix optimization
- `internal/proxy/deepcopy_test.go` — JSON vs gob benchmarks
