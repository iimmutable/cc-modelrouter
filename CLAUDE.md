# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
```bash
# Build all packages
go build ./...

# Build debug binary (default - when no "release"/"prod"/"production" specified)
go build -o bin/debug/ccrouter ./cmd/ccrouter

# Build release binary (when "release", "prod", or "production" specified)
go build -o bin/release/ccrouter ./cmd/ccrouter
```

### Testing
```bash
# Run all tests
go test ./...

# With coverage
go test ./... -cover

# Generate HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test ./internal/router -run TestThinkLevelDetection

# Run integration tests (requires test config)
go test -tags=integration ./test/... -v
```

### Running the Server
```bash
# Start router standalone
./bin/ccrouter start

# Start with debug logging
./bin/ccrouter start --log-level=debug --log-destination=file

# Start router and launch Claude Code (for development)
./bin/ccrouter code

# Show running instances
./bin/ccrouter status

# Stop specific instance
./bin/ccrouter stop <instance-id>

# Stop all instances
./bin/ccrouter stop --all
```

## Architecture Overview

cc-modelrouter is a Go-based HTTP proxy that routes Claude Code requests to multiple LLM providers with automatic format transformation. The architecture uses a **Unified Intermediate Format** pattern to separate protocol conversion from routing logic.

### Core Request Flow
```
Claude Code → HTTP Proxy → Router Engine → Transformer → Provider API
                  ↓              ↓             ↓
            (validate)     (detect      (convert
                           route type    request)
                           & select
                           provider)

Provider API → Transformer → HTTP Proxy → Claude Code
                  ↓              ↓
           (convert       (stream
            response)     response)
```

### Key Components

**HTTP Proxy Layer (`internal/proxy/`)**
- Implements Anthropic Messages API endpoint (`/v1/messages`)
- Validates requests (max 50MB), handles SSE streaming
- Integrates router, transformer, and usage tracker via adapter pattern

**Router Engine (`internal/router/`)**
- Detects route type based on request characteristics (thinking level, images, web search, etc.)
- Selects appropriate `provider:model` from route configuration
- Manages sequential failover with configurable retries

**Transformer Layer (`internal/transformer/`)**
- **Unified Format**: Provider-agnostic intermediate representation
- **Interface**: All providers implement `Transformer` interface with:
  - `TransformRequestIn/Out`: Anthropic ↔ Unified ↔ Provider HTTP Request
  - `TransformResponseIn/Out`: Provider Response ↔ Unified ↔ Anthropic
  - `TransformSSEEvent`: Streaming event transformation
- **Providers**: anthropic, openai, openrouter, gemini, qwen, glm, minimax

**Usage Tracking (`internal/usage/`)**
- SQLite database at `~/.cc-modelrouter/usage.db`
- Buffered writing with periodic flush
- Tracks actual provider-reported tokens (input + output)
- **Critical**: Extracts usage from `message_delta` events in streaming responses

**Daemon Management (`internal/daemon/`)**
- Instance isolation for concurrent project support
- Dynamic port allocation to avoid conflicts
- Metadata stored in `~/.cc-modelrouter/instances/<instance-id>.json`

### Route Detection Logic

The router automatically selects routes based on request characteristics:

| Route | Trigger | Detection |
|-------|---------|-----------|
| `background` | Claude Code background agent | Model contains "claude" + "haiku" |
| `ultrathink` | Maximum thinking | `budget_tokens >= 32,000` |
| `thinkMore` | Enhanced thinking | `budget_tokens >= 10,000` |
| `think` | Basic thinking | `budget_tokens >= 4,000` |
| `longContext` | Large context | Token count > 60,000 |
| `image` | Image content | Request contains image blocks |
| `webSearch` | Web search enabled | Tool names contain "web"/"search" |
| `default` | Fallback | All other requests |

**Thinking level fallback**: If `ultrathink` not configured, falls back to `thinkMore`, then `think`.

### Configuration

**File Locations:**
- Global: `~/.cc-modelrouter/config.json`
- Project: `<project>/.cc-modelrouter/config.json` (overrides global entirely)

**Environment Variables:**
- Use `${VAR_NAME}` syntax for API keys
- Required: `OPENROUTER_API_KEY`, `GEMINI_API_KEY`, `BIGMODEL_API_KEY`, etc.

**Config Structure:**
```json
{
  "server": {"port": 8081, "host": "localhost"},
  "providers": {
    "<name>": {
      "apiKey": "${API_KEY}",
      "baseURL": "https://api.example.com",
      "models": ["model-1", "model-2"]
    }
  },
  "router": {
    "routes": {
      "default": "provider:model;provider2:model2",
      "think": "provider:model"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

Routes are semicolon-separated lists of `provider:model` pairs for failover.

## Important Implementation Details

### Usage Tracking in Streaming Responses

**Critical for accurate token tracking**: Some providers (notably GLM) send both `input_tokens` and `output_tokens` in `message_delta` events during streaming. The handler must extract both:

```go
// In handler.go tryStreamingTarget:
var totalInputTokens int
var totalOutputTokens int

// Extract from message_delta events:
if te.EventType == "message_delta" {
    if usage, ok := eventData["usage"].(map[string]interface{}); ok {
        if outputTokens, ok := usage["output_tokens"].(float64); ok {
            totalOutputTokens += int(outputTokens)
        }
        if inputTokens, ok := usage["input_tokens"].(float64); ok {
            totalInputTokens = int(inputTokens)  // Use actual, not estimate
        }
    }
}
```

If provider doesn't send `input_tokens` in streaming, the code falls back to `estimateTokens(req)` (÷4 character count).

### Transformer Implementation Pattern

When adding a new provider transformer:

1. Implement `Transformer` interface in `internal/transformer/providers/`
2. Add to registry in `internal/transformer/registry.go`
3. Create converter functions in `internal/transformer/converters/` if needed
4. Handle both non-streaming and streaming responses
5. For streaming: generate synthetic `message_delta` events with `output_tokens` if provider doesn't send them

**Streaming SSE Event Handling:**
- Use `transformer.SSEEvent` struct for event transformation
- Generate `message_start`, `content_block_delta`, `message_delta`, `message_stop` events
- `message_delta` should include `usage.output_tokens` for proper tracking

### Request Interceptors

Interceptors provide cross-cutting concerns at three points:
- **Request**: Before routing (validation, logging)
- **Response**: After provider response (metrics, validation)
- **Streaming**: Per SSE event (modification, filtering)

Add via handler methods: `AddRequestInterceptor`, `AddResponseInterceptor`, `AddStreamingInterceptor`

### Testing Patterns

- **Unit tests**: Alongside source files (`*_test.go`)
- **Table-driven tests**: For multiple scenarios
- **Integration tests**: In `test/integration/` with `-tags=integration`
- **Mock HTTP clients**: Use `httptest.NewServer` for provider testing

### Failover Strategy

The router implements **sequential failover**:
1. Try each `provider:model` in route configuration order
2. After exhausting list, loop back to beginning
3. Maximum attempts: `2 × number of providers`
4. Each attempt = 1 request (no per-provider retries by default)

### Error Handling

- Provider errors: Return to client with proper HTTP status codes
- Transform errors: Log and skip invalid SSE events, don't fail entire stream
- Timeout errors: Configurable per provider in config
- All errors logged with context (provider, model, route)

## Files API Status

**IMPORTANT: Claude Code does NOT use Anthropic's Files API.**

### What IS Implemented

- **API Endpoints**: `/v1/files` endpoints exist for Anthropic API completeness
- **Type Support**: ContentBlock types include `document` with `DocumentSource` for file_id references
- **Transformers**: Document blocks are recognized but use placeholder text for non-Anthropic providers

### What is NOT Implemented

- **File Storage**: Files are NOT actually stored (mock responses only)
- **File Resolution**: Document blocks with `file_id` are NOT resolved when routing to non-Anthropic providers
- **File Content Retrieval**: No mechanism exists to fetch file content by file_id

### Behavior

When a request contains a document block with file_id:
```json
{
  "type": "document",
  "source": {"type": "file", "file_id": "file-abc123"},
  "title": "Report.pdf"
}
```

**For Anthropic provider**: Passes through unchanged (but Anthropic won't recognize the file_id since we don't proxy to real Files API)

**For other providers (OpenAI, Gemini, GLM, etc.)**: A warning is logged and placeholder text is used:
```
[Document: Report.pdf - file_id: file-abc123]
```

### Why This Exists

The Files API handlers and document types exist for:
1. **API Specification Compliance**: To match Anthropic's published API
2. **Future Use**: If Files API support is needed for direct (non-Claude Code) usage
3. **Type Safety**: To properly parse and validate incoming requests

Since Claude Code doesn't use Files API, implementing file storage/resolution would be premature (YAGNI principle).

### If You Need Files API Support

If direct API usage requires working file upload/retrieval:
1. Implement a FileStore with SQLite or filesystem backend
2. Update Files API handlers to store actual files
3. Update transformers to resolve file_id and inline content for non-Anthropic providers
4. See `plans/2026-03-06-file-resolution-layer-implementation.md` for detailed implementation plan

## Log File Locations

- **Instance logs**: `~/.cc-modelrouter/logs/inst_YYYYMMDD_HHMMSS.log`
- **Per-instance logging**: Each `ccrouter code` instance gets its own log file
- **Log levels**: debug, info, warn, error (configurable via `--log-level`)
