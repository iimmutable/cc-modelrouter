# Project Experience Index

Knowledge captured specifically for this project.

## How to Use
- Experience files follow the format: `YYYYMMDD-HHmmss-kebab-description.md`
- Browse by tag or date
- Archived entries are in `./archive/`

## Index

### Composite (Merged) Entries
- 20260402 — [BigModel Error 1213 Root Cause Catalog](20260402-bigmodel-error-1213-root-cause-catalog.md) #composite #glm #bigmodel #error-1213 #provider #streaming #validation — All 5 known root causes for error 1213 with diagnosis quick reference (merged from connection-pooling + endpoint-compatibility)
- 20260330 — [Provider Thinking Block Compatibility](20260330-provider-thinking-block-compatibility.md) #transformer #provider #validation #thinking-blocks #composite — Per-provider signature and content array requirements (merged from validation matrix + signature fix)
- 20260330 — [Failover Safety Patterns](20260330-failover-safety-patterns.md) #failover #transformer #architecture #composite — Deep copy rules, GetBody pattern, and failover safety (merged from deep-copy rule + comprehensive fix)

### Current Entries
- 20260402 — [GLM Anthropic Endpoint Compatibility](20260402-glm-anthropic-endpoint-compatibility.md) #provider #glm #bigmodel #streaming #validation #api-compat — Error 1213 has four root causes: stream=false, thinking blocks, SSE errors on HTTP 200, 64-char tool name limit
- 20260329 — [SSE Streaming Checklist](20260329-sse-streaming-checklist.md) #streaming #sse #proxy — Buffer sizes, event format, timeouts, and retry patterns
- 20260329 — [Provider Connection Pooling](20260329-provider-connection-pooling.md) #provider #http — BigModel error 1213 fix with disableKeepAlives
- 20260329 — [Go Header Logging Security](20260329-go-header-logging-security.md) #security #go #logging — Never use %v with http.Header

### Archived
- See `./archive/` for superseded entries
