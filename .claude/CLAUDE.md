# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Build
go build -o bin/debug/ccrouter ./cmd/ccrouter          # debug binary
go build -o bin/release/ccrouter ./cmd/ccrouter         # release binary

# Test
go test ./...                                           # all tests
go test ./internal/router -run TestThinkLevelDetection   # single test
go test ./... -coverprofile=coverage.out                # with coverage
go test -v ./test/security                              # security tests

# Run
./bin/ccrouter code                                     # router + Claude Code (most common)
./bin/ccrouter start                                    # standalone
./bin/ccrouter start --log-level=debug --log-destination=file
./bin/ccrouter status                                   # check status
./bin/ccrouter stop --all                               # stop all instances
```

## Architecture

HTTP proxy that routes Claude Code requests to multiple LLM providers with format transformation.

**Request flow:**
```
Claude Code → HTTP Proxy → Router Engine → Transformer → Provider API
                  ↓              ↓              ↓
            (validate)    (detect route    (convert
                          & select         request)
                          provider)

Provider API → Transformer → HTTP Proxy → Claude Code
                  ↓              ↓
           (convert        (stream
            response)      response)
```

**Key packages:**
- `cmd/ccrouter/` — CLI entry point (delegates to `internal/cli/`)
- `pkg/api/anthropic/` — Anthropic API type definitions with custom marshaling
- `internal/proxy/` — HTTP proxy server, request handler (`handler.go` ~1000 lines), streaming utilities, interceptors
- `internal/router/` — Route detection (`engine.go`) and sequential failover (`failover.go`)
- `internal/transformer/` — `Transformer` interface, registry, base utilities, and provider implementations in `transformers/`
- `internal/provider/` — HTTP client for provider APIs
- `internal/usage/` — SQLite usage tracking with buffered writes (500 records or 3s)
- `internal/config/` — Config loading with `${VAR_NAME}` env var interpolation
- `internal/cli/` — Cobra CLI commands (start, code, stop, status, config, logs, usage, clean, restart)
- `internal/interceptor/` — Request/response interceptors (max tokens, reasoning, tool enhancement)
- `internal/daemon/` — Instance management (PID files, metadata)

## Transformer Interface

Defined in `internal/transformer/interface.go`:

```go
type Transformer interface {
    Name() string
    Endpoint() string
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

**Built-in transformers** (`internal/transformer/transformers/`):
- `anthropic.go` — passthrough
- `openrouter.go` — handles `signature` field requirements (must be present even if empty for Anthropic models)
- `openai.go` — OpenAI-compatible format
- `gemini.go` — Gemini native format
- `glm_anthropic.go` — GLM Anthropic-compatible

**Adding a new transformer:** Create in `internal/transformer/transformers/`, implement the interface, register in `internal/cli/start.go` or `internal/cli/code.go`. For streaming, generate synthetic SSE events: `message_start` → `content_block_delta` → `message_delta` (must include `usage.output_tokens`) → `message_stop`.

## Route Detection

Router selects routes automatically (`internal/router/engine.go`):

| Route | Trigger |
|-------|---------|
| `background` | Model contains "claude" + "haiku" |
| `ultrathink` | `budget_tokens >= 32,000` |
| `thinkMore` | `budget_tokens >= 10,000` |
| `think` | `budget_tokens >= 4,000` |
| `longContext` | Token count > 60,000 |
| `image` | Request contains image blocks |
| `webSearch` | Tool names contain "web"/"search" |
| `default` | Fallback |

Thinking level cascade: `ultrathink` → `thinkMore` → `think` if not configured.

## Failover

Sequential failover (`internal/router/failover.go`): tries each `provider:model` in config order, loops back, max attempts = `2 × number of providers`. Route format: `"openrouter:claude-sonnet-4;bigmodel:glm-4.7"`.

**Critical:** Transformers must deep-copy requests before modification — failover reuses the original request for subsequent providers.

## Critical Implementation Rules

### Secret Handling (SECURITY)

Never use `%v` with `http.Header` — it leaks API keys. Always use:
```go
logging.Debugf("[PROXY REQUEST] Headers: %s", logging.SanitizeHeadersString(req.Header))
```

Headers auto-redacted: `Authorization`, `X-Api-Key`, `X-Auth-Token`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-BigModel-Api-Key`, `X-OpenRouter-Api-Key`.

Verify: `go test -v ./test/security`

### Usage Tracking in Streaming

GLM sends both `input_tokens` and `output_tokens` in `message_delta` events. Extract from `eventData["usage"]` map. Fallback: `estimateTokens(req)` (÷4 character count) if provider doesn't send tokens.

### Test File Organization (ENFORCED BY PRE-COMMIT HOOK)

| Type | Location | Package |
|------|----------|---------|
| Black-box | `<module>/test/` | `package <module>_test` |
| White-box | `<module>/` (alongside source) | `package <module>` |
| Cross-module | `test/` (root) | varies |

Test files with no corresponding source file must go in `test/` subfolder. See `.githooks/pre-commit`.

### Files API — NOT IMPLEMENTED

Claude Code doesn't use Files API. `/v1/files` endpoints exist for spec compliance only. `file_id` references get placeholder text for non-Anthropic providers. Do not implement file storage/resolution — YAGNI.

## Configuration

- **Global:** `~/.cc-modelrouter/config.json`
- **Project:** `<project>/.cc-modelrouter/config.json` (completely overrides global, not merged)
- **Env vars:** `${VAR_NAME}` syntax for API keys
- **Instance files:** `~/.cc-modelrouter/instances/*.json`, logs: `~/.cc-modelrouter/logs/inst_*.log`
- **Usage DB:** `~/.cc-modelrouter/usage.db` (SQLite)

## Interceptors

Three interception points in `internal/proxy/interceptor.go`:
- **Request interceptors** — before routing (validation, logging)
- **Response interceptors** — after provider response (metrics)
- **Streaming interceptors** — per SSE event (modification, filtering)

Built-in: `MaxTokensInterceptor`, `ReasoningInterceptor`, `ToolEnhanceInterceptor` in `internal/interceptor/`.

## Error Handling

| Error Type | Behavior |
|------------|----------|
| Provider errors | Return to client with proper HTTP status |
| Transform errors | Log and skip invalid SSE events (don't fail stream) |
| Timeout errors | Configurable per provider in config |
| All errors | Log with context (provider, model, route) |
