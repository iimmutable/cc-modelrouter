# cc-modelrouter

A Go-based HTTP proxy server that routes Claude Code requests to multiple LLM providers with format transformation.

---

## For Claude Code & AI Development

**Developers using Claude Code:** See [CLAUDE.md](CLAUDE.md) for AI-specific build commands, implementation patterns, critical warnings, and operational guidance optimized for AI assistants.

---

## Features

- **Multi-Provider Support**: Route to Anthropic, OpenRouter, Google Gemini, Alibaba Qwen, Zhipu GLM, OpenAI, and more
- **Smart Routing**: Automatic route detection based on request characteristics (think levels, images, web search, etc.)
- **Format Transformation**: Built-in transformers for provider API compatibility (Anthropic, OpenAI, Gemini, GLM formats)
- **Sequential Failover**: Loop through providers with automatic retry on failure
- **Instance Isolation**: Each `ccrouter code` command creates an isolated router instance
- **Configuration Override**: Project configs override global settings completely
- **Route Profiles**: Dynamic switching between different routing configurations without restart
- **Usage Tracking**: SQLite-based token usage tracking with live monitor
- **Request Compaction**: Automatic request reduction for providers with context window limits
- **Interactive Config Wizard**: Full-screen TUI for configuration management

## Installation

```bash
go install github.com/iimmutable/cc-modelrouter/cmd/ccrouter@latest
```

## Quick Start

### 1. Create Configuration

Create `~/.cc-modelrouter/config.json`:

```json
{
  "server": {
    "port": 8081,
    "host": "localhost"
  },
  "providers": {
    "openrouter": {
      "apiKey": "${CCROUTER_OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api",
      "transformer": "openrouter",
      "models": ["anthropic/claude-sonnet-4"]
    },
    "bigmodel": {
      "apiKey": "${CCROUTER_BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "transformer": "glm-anthropic",
      "models": ["glm-4.7", "glm-4.5-air"]
    }
  },
  "router": {
    "routes": {
      "default": "openrouter:anthropic/claude-sonnet-4",
      "background": "bigmodel:glm-4.5-air",
      "think": "openrouter:anthropic/claude-sonnet-4",
      "thinkMore": "openrouter:anthropic/claude-sonnet-4",
      "ultrathink": "openrouter:anthropic/claude-opus-4",
      "image": "bigmodel:glm-4.6v"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

**Note:** OpenRouter provides a unified API endpoint (`/v1/messages`) that works with both Anthropic-format models and OpenAI-format models. The key difference is the model identifier:
```json
"openrouter-openai": {
  "apiKey": "${CCROUTER_OPENROUTER_API_KEY}",
  "baseURL": "https://openrouter.ai/api",
  "transformer": "openai",
  "models": ["google/gemini-2.5-flash"]
}
```

### 2. Run with Claude Code

```bash
ccrouter code
```

This starts the router and launches Claude Code with the router configured as the API endpoint.

### 3. Or Start Standalone

```bash
ccrouter start
```

Then configure Claude Code:

```bash
export ANTHROPIC_BASE_URL=http://localhost:8081
claude
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `ccrouter code` | Start router and launch Claude Code with auto-configuration |
| `ccrouter start` | Start router server standalone |
| `ccrouter stop [id]` | Stop instance (or all if no ID given) |
| `ccrouter restart [id]` | Restart instance (or all if no ID given) |
| `ccrouter status` | Show all running instances |
| `ccrouter clean` | Remove stale instance files |
| `ccrouter config` | Interactive configuration wizard (TUI) |
| `ccrouter logs [id]` | Show logs for instance |
| `ccrouter monitor` | Live usage monitor with terminal UI |
| `ccrouter profile list` | List available route profiles |
| `ccrouter profile switch <profile>` | Switch to a different route profile |
| `ccrouter profile status` | Show currently active profile |

### Detailed Command Usage

**Start with specific configuration:**
```bash
# Start with custom config file
ccrouter code --config /path/to/custom-config.json

# Start on specific port
ccrouter code --port 9090

# Start with specific route profile
ccrouter code --profile cost-opt

# Enable debug logging
ccrouter start --log-level=debug --log-destination=file

# Start with specific route profile
ccrouter start --profile production
```

**Manage instances:**
```bash
# Stop all instances
ccrouter stop

# Stop specific instance
ccrouter stop inst_20250216_143022

# Force stop stuck instance
ccrouter stop --force inst_20250216_143022
```

**View logs:**
```bash
# Show last 100 lines for specific instance
ccrouter logs inst_20250216_143022

# Follow logs in real-time
ccrouter logs -f inst_20250216_143022

# Show logs for all instances
ccrouter logs
```

## Configuration

### File Locations

- **Global**: `~/.cc-modelrouter/config.json`
- **Project**: `<project>/.cc-modelrouter/config.json`

Project config completely overrides global config when present.

### Environment Variables

Use `${VAR_NAME}` or `$VAR_NAME` syntax for sensitive values:

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "${CCROUTER_OPENROUTER_API_KEY}"
    }
  }
}
```

### Route Types

| Route | Trigger | Detection |
|-------|---------|-----------|
| `default` | Standard requests | Fallback |
| `background` | Background agent | Model contains "claude" + "haiku" |
| `think` | Basic thinking | `budget_tokens >= 4,000` |
| `thinkMore` | Enhanced thinking | `budget_tokens >= 10,000` |
| `ultrathink` | Maximum thinking | `budget_tokens >= 32,000` |
| `longContext` | Large context | Token count > 60,000 |
| `webSearch` | Web search enabled | Tool names contain "web"/"search" |
| `image` | Image processing | Request contains images |

### Provider Format

```json
{
  "providers": {
    "<name>": {
      "apiKey": "your-api-key",
      "baseURL": "https://api.example.com",
      "models": ["model-1", "model-2"]
    }
  }
}
```

### Provider Options

| Option | Type | Description |
|--------|------|-------------|
| `apiKey` | string | API key (supports `${VAR_NAME}` env var syntax) |
| `baseURL` | string | Provider API base URL |
| `models` | []string | List of available models |
| `transformer` | string | Transformer name (defaults to provider name) |
| `disableKeepAlives` | bool | Disable HTTP keep-alive for providers with connection issues |
| `maxRequestBodyBytes` | int64 | Maximum request body size in bytes (default: 0 = no limit) |
| `compaction` | object | Request compaction settings (method, summarizeProvider, summarizeModel) |

### Logging Configuration

```json
{
  "logging": {
    "enabled": true,
    "destination": "file",
    "filePath": "~/.cc-modelrouter/logs/router.log",
    "level": "debug"
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable/disable logging (opt-in) |
| `destination` | string | "file" | Where to write logs: "stdout", "stderr", "file", or a file path |
| `filePath` | string | ~/.cc-modelrouter/router.log | Specific log file path |
| `level` | string | "info" | Log level: "debug", "info", "warn", "error" |

**Log Levels:**
- `debug`: All messages including detailed streaming events
- `info`: Request/response summaries and warnings
- `warn`: Only warnings and errors
- `error`: Only errors

### Usage Tracking

The router tracks token usage statistics per model, route, and instance using SQLite. View live statistics via the `monitor` command.

```bash
ccrouter monitor
```

Data is stored in `~/.cc-modelrouter/usage.db` with buffered writes (500 records or 3 seconds).

## Supported Providers

| Provider | Transformer | API Format | Authentication | Compatible Models |
|----------|-------------|------------|----------------|-------------------|
| Anthropic | `anthropic` | Native Anthropic | `x-api-key` header | Claude Sonnet 4.5/Opus 4.5/Haiku 4.5 |
| OpenRouter | `openrouter` | Anthropic-compatible | `x-api-key` header | All OpenRouter models (Anthropic, Gemini, OpenAI, etc.) |
| OpenAI | `openai` | OpenAI-compatible | `Authorization: Bearer` header | GPT-4, GPT-4o, GPT-3.5 |
| Google Gemini | `gemini` | Gemini native | Query param `key=` | Gemini Pro/Flash/Exp |
| Zhipu GLM | `glm-anthropic` | Anthropic-compatible | `x-api-key` header | GLM-4/4.5 Air/4.6v/4.7 |
| Alibaba Qwen | `openai` | OpenAI-compatible | `Authorization: Bearer` | Qwen Turbo/Plus/Max |
| MiniMax | `anthropic` | Anthropic-compatible | `x-api-key` header | MiniMax models |

## Architecture

The router uses a **Unified Intermediate Format** architecture that separates protocol conversion from cross-cutting concerns.

```
┌─────────────────────────────────────────────────────────────┐
│ CLI / Config Layer                                          │
│ (start/stop/status, config loading/merging)                 │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ HTTP Server Layer                                           │
│ (request validation, Anthropic API endpoint)                │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Request Interceptors                                        │
│ (cross-cutting concerns: logging, validation, etc.)         │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Router Engine Layer                                         │
│ (route matching, failover logic, model selection)           │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Transformer Layer - Unified Intermediate Format            │
│                                                             │
│  Request Flow:                                             │
│  Anthropic → Unified → Provider HTTP Request               │
│                                                             │
│  Response Flow:                                            │
│  Provider Response → Unified → Anthropic                   │
│                                                             │
│  Provider Transformers:                                     │
│  • anthropic      (pass-through)                            │
│  • openai         (OpenAI-compatible)                      │
│  • openrouter     (Anthropic with signature handling)      │
│  • gemini         (Gemini native)                           │
│  • glm-anthropic  (Anthropic-compatible)                    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Response Interceptors                                       │
│ (response modification, logging, usage tracking)            │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Streaming Interceptors                                     │
│ (SSE event modification, thinking extraction)              │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Provider Client Layer                                       │
│ (HTTP clients, streaming, retry)                            │
└─────────────────────────────────────────────────────────────┘
```

### Unified Intermediate Format

The router uses a unified intermediate format that represents messages in a provider-agnostic way:

**Request Components:**
- `Messages[]` - Array of messages with role and content blocks
- `Model` - Target model identifier
- `MaxTokens` - Maximum tokens for response
- `Temperature` - Sampling temperature
- `Stream` - Whether to use streaming
- `Tools[]` - Tool definitions for function calling
- `ToolChoice` - Tool selection strategy
- `System` - System prompt
- `Reasoning` - Extended thinking configuration
- `Metadata` - Additional metadata

**Message Content Blocks:**
- `text` - Plain text content
- `image_url` - Image content with data URL
- `thinking` - Extended thinking/reasoning content

**Response Components:**
- `ID` - Response identifier
- `Model` - Model used
- `Content[]` - Response content blocks
- `ToolCalls[]` - Tool calls made
- `Usage` - Token usage statistics
- `StopReason` - Reason for stopping

### Transformer Interface

Each provider implements the `Transformer` interface:

```go
type Transformer interface {
    // Name returns the transformer name (used for registry lookup)
    Name() string

    // Endpoint returns the API endpoint path for this transformer
    Endpoint() string

    // PrepareRequest converts an Anthropic request to a provider-specific HTTP request
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

    // ParseResponse converts a provider HTTP response to Anthropic format
    ParseResponse(resp *http.Response) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming
    SupportsStreaming() bool

    // TransformStreamEvent converts a provider SSE event to Anthropic SSE events
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}

// SSEEvent represents a complete server-sent event with type and data.
type SSEEvent struct {
    EventType string  // SSE event type (e.g., "message_start", "content_block_delta")
    Data      []byte  // Raw JSON data payload
}
```

### Interceptors

Interceptors provide cross-cutting concerns that can be applied to requests and responses:

**Request Interceptors:**
- Validate request parameters
- Modify requests before routing
- Add logging context
- Adjust token limits based on provider capabilities
- Enhance tool definitions

**Response Interceptors:**
- Modify response content
- Extract and format thinking/reasoning
- Track usage statistics
- Validate response format

**Streaming Interceptors:**
- Modify SSE events during streaming
- Extract thinking content from streams
- Format tool call responses
- Track streaming usage in real-time

### Provider Format Compatibility

| Provider | Transformer | Request Format | Response Format | Streaming |
|----------|-------------|---------------|----------------|-----------|
| Anthropic | `anthropic` | Native Anthropic | Native Anthropic | Native events |
| OpenAI | `openai` | OpenAI-compatible | OpenAI-compatible | Transformed to Anthropic |
| OpenRouter | `openrouter` | Anthropic-compatible | Anthropic-compatible | Pass-through with validation |
| Google Gemini | `gemini` | Gemini native | Gemini native | Transformed to Anthropic |
| Alibaba Qwen | `openai` | OpenAI-compatible | OpenAI-compatible | Transformed to Anthropic |
| Zhipu GLM | `glm-anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through with token tracking |
| MiniMax | `anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through |

## Request Flow

1. **Claude Code** sends request to `http://localhost:<port>/v1/messages`
2. **Proxy Handler** validates Anthropic API request (max 50MB)
3. **Router Engine** detects route type and selects provider:model based on priority
4. **Failover Manager** loops through providers in route with retry on failure
5. **Transformer** converts request to provider format using appropriate transformer
6. **Provider Client** sends to provider API with retry logic
7. **Transformer** converts response back to Anthropic format
8. **Response Writer** streams back to Claude Code with usage tracking

## Instance Isolation

Each `ccrouter code` command creates an isolated environment:

- Unique instance ID: `inst_YYYYMMDD_HHMMSS`
- Dynamically allocated port (or specified port)
- Separate PID file: `~/.cc-modelrouter/instances/<instance-id>.json`
- Instance-specific log file: `~/.cc-modelrouter/logs/<instance-id>.log`
- Explicit environment for child Claude Code process
- Temporary configuration override in project `.claude/settings.local.json`

## Security Features

- **Header Sanitization**: All logging automatically redacts sensitive headers (API keys, auth tokens)
- **Environment Variable Support**: Secure API key storage using `${VAR_NAME}` syntax
- **Admin API Protection**: Runtime profile management secured with generated tokens
- **Request Size Limits**: Built-in protection against oversized requests

## Development

### Build

```bash
# Build debug binary
go build -o bin/debug/ccrouter ./cmd/ccrouter

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/linux-amd64/ccrouter ./cmd/ccrouter
GOOS=linux GOARCH=arm64 go build -o bin/linux-arm64/ccrouter ./cmd/ccrouter

# Build release binary
go build -ldflags="-s -w" -o bin/release/ccrouter ./cmd/ccrouter
```

### Run Tests

```bash
# Run all tests
go test ./...

# Test with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test ./internal/router -run TestThinkLevelDetection

# Run security tests
go test -v ./test/security
```

### Project Structure

```
cc-modelrouter/
├── bin/                        # Compiled binaries
├── cmd/
│   └── ccrouter/              # Main CLI entry point
├── docs/                      # Documentation (architecture, security, usage)
├── plans/                     # Development plans and specifications
├── internal/
│   ├── cli/                   # CLI commands and adapters
│   ├── config/                # Configuration loading and validation
│   ├── configwizard/          # Interactive TUI configuration wizard (Bubble Tea)
│   ├── daemon/                # Instance management (PID files, metadata)
│   ├── interceptor/           # Request/response/streaming interceptors
│   ├── logging/               # Logging utilities with header sanitization
│   ├── monitor/               # Live usage monitor (terminal UI)
│   ├── provider/              # HTTP clients for providers
│   ├── proxy/                 # HTTP server and request handling
│   ├── router/                # Routing engine and sequential failover
│   ├── transformer/           # Request/response format transformers
│   │   ├── transformers/      # Individual transformer implementations
│   │   │   ├── anthropic.go   # Direct Anthropic API transformer
│   │   │   ├── openai.go      # OpenAI-compatible transformer
│   │   │   ├── openrouter.go  # OpenRouter-specific transformer
│   │   │   ├── gemini.go      # Google Gemini transformer
│   │   │   └── glm_anthropic.go # Zhipu GLM Anthropic-compatible transformer
│   │   ├── interface.go       # Transformer interface definition
│   │   ├── registry.go        # Transformer registry
│   │   └── base.go            # Base transformer utilities
│   └── usage/                 # SQLite-based usage tracking
└── pkg/
    └── api/
        └── anthropic/         # Anthropic API type definitions with custom marshaling
```

## Troubleshooting

### Common Issues

**API Key Errors**: Ensure environment variables are properly set and exported:
```bash
export CCROUTER_OPENROUTER_API_KEY="your-key-here"
```

**Port Conflicts**: Use `ccrouter status` to check for running instances and `ccrouter stop` to free ports.

**Provider Authentication**: Most issues stem from incorrect API key format or insufficient permissions.

### Debugging

Enable debug logging to troubleshoot:
```bash
ccrouter start --log-level=debug --log-destination=file
```

Check instance logs:
```bash
ccrouter logs
```

View running instances:
```bash
ccrouter status
```

## License

MIT