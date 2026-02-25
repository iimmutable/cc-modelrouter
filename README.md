# cc-modelrouter

A Go-based HTTP proxy server that routes Claude Code requests to multiple LLM providers with format transformation.

## Features

- **Multi-Provider Support**: Route to Anthropic, OpenRouter, Google Gemini, Alibaba Qwen, and Zhipu GLM
- **Smart Routing**: Automatic route detection based on request characteristics
- **Format Transformation**: Built-in transformers for provider API compatibility
- **Sequential Failover**: Loop through providers with automatic retry
- **Instance Isolation**: Each `ccrouter code` command creates an isolated router instance
- **Configuration Override**: Project configs override global settings

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
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api",
      "transformer": "anthropic",
      "models": ["anthropic/claude-sonnet-4"]
    },
    "bigmodel": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "transformer": "anthropic",
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

**Note:** OpenRouter provides two endpoints:
- **Anthropic-compatible** (`https://openrouter.ai/api`): For Anthropic Claude models only
- **OpenAI-compatible** (`https://openrouter.ai/api/v1`): For Google, OpenAI, and other models

To use non-Anthropic models via OpenRouter, add a second provider entry:
```json
"openrouter-openai": {
  "apiKey": "${OPENROUTER_API_KEY}",
  "baseURL": "https://openrouter.ai/api/v1",
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
| `ccrouter code` | Start router and launch Claude Code |
| `ccrouter start` | Start router server standalone |
| `ccrouter stop [id]` | Stop router instance (or all with `--all`) |
| `ccrouter restart [id]` | Restart instance |
| `ccrouter status` | Show all running instances |
| `ccrouter clean` | Remove stale instance files |
| `ccrouter config` | Show active configuration |
| `ccrouter logs [id]` | Show logs for instance |

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
      "apiKey": "${OPENROUTER_API_KEY}"
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

## Supported Providers

| Provider | Transformer | API Format | Compatible Models |
|----------|-------------|------------|-------------------|
| Anthropic | `anthropic` | Native Anthropic | Claude 3.5 Sonnet, Haiku, Opus |
| OpenAI | `openai` | OpenAI-compatible | GPT-4, GPT-4 Turbo |
| OpenRouter (Anthropic) | `anthropic` | Anthropic-compatible | Anthropic Claude models only |
| OpenRouter (OpenAI) | `openai` | OpenAI-compatible | Google, OpenAI, other models |
| Google Gemini | `gemini` | Gemini native | Gemini Pro, Ultra, Flash |
| Alibaba Qwen | `anthropic` | Anthropic-compatible | Qwen Turbo, Plus, Max |
| Zhipu GLM | `anthropic` | Anthropic-compatible | GLM-4, GLM-4 Air |
| MiniMax | `anthropic` | Anthropic-compatible | MiniMax models |

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
│  • gemini         (Gemini native)                           │
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
    // Metadata
    Name() string
    Endpoint() string

    // Authentication
    Auth(req *UnifiedChatRequest, provider Provider) (map[string]string, error)

    // Request transformation: Anthropic → Unified → Provider HTTP Request
    TransformRequestIn(req *anthropic.Request) (*UnifiedChatRequest, error)
    TransformRequestOut(unified *UnifiedChatRequest, baseURL, apiKey, model string) (*http.Request, error)

    // Response transformation: Provider Response → Unified → Anthropic
    TransformResponseIn(resp *http.Response) (*UnifiedChatResponse, error)
    TransformResponseOut(unified *UnifiedChatResponse) (*anthropic.Response, error)

    // Streaming
    SupportsStreaming() bool
    TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error)
    TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) // Deprecated
}
```

### Converter Utilities

The `converters` package provides reusable conversion functions:

- `AnthropicToUnified()` - Convert Anthropic to unified format
- `UnifiedToAnthropic()` - Convert unified to Anthropic format
- `UnifiedToOpenAIRequest()` - Convert unified to OpenAI HTTP request
- `OpenAIToUnified()` - Convert OpenAI response to unified format
- `UnifiedRequestToAnthropic()` - Convert unified request to Anthropic

### Interceptors

Interceptors provide cross-cutting concerns that can be applied to requests and responses:

**Request Interceptors:**
- Validate request parameters
- Modify requests before routing
- Add logging context

**Response Interceptors:**
- Modify response content
- Extract and format thinking/reasoning
- Track usage statistics

**Streaming Interceptors:**
- Modify SSE events during streaming
- Extract thinking content from streams
- Format tool call responses

### Provider Format Compatibility

| Provider | Transformer | Request Format | Response Format | Streaming |
|----------|-------------|---------------|----------------|-----------|
| Anthropic | `anthropic` | Native | Native | Native events |
| OpenAI | `openai` | OpenAI-compatible | OpenAI-compatible | Transformed |
| OpenRouter | `anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through |
| Google Gemini | `gemini` | Gemini native | Gemini native | Transformed |
| Alibaba Qwen | `anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through |
| Zhipu GLM | `anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through |
| MiniMax | `anthropic` | Anthropic-compatible | Anthropic-compatible | Pass-through |

## Request Flow

1. **Claude Code** sends request to `http://localhost:<port>/v1/messages`
2. **Proxy Handler** validates Anthropic API request
3. **Router Engine** detects route type and selects provider:model
4. **Failover Manager** loops through providers with retry
5. **Transformer** converts request to provider format
6. **Provider Client** sends to provider API
7. **Transformer** converts response back to Anthropic format
8. **Response Writer** streams back to Claude Code

## Instance Isolation

Each `ccrouter code` command creates an isolated pair:

- Unique instance ID: `inst_YYYYMMDD_HHMMSS`
- Dynamically allocated port
- Separate PID file: `~/.cc-modelrouter/instances/<instance-id>.json`
- Explicit environment for child Claude Code process

## Development

### Build

```bash
go build ./...
```

### Run Tests

```bash
go test ./...

# With coverage
go test ./... -cover
```

See [docs/testing.md](docs/testing.md) for detailed testing documentation.

### Project Structure

```
cc-modelrouter/
├── bin/                   # Compiled binaries
├── cmd/
│   └── ccrouter/          # Main CLI entry point
├── docs/                  # Documentation
├── plans/                 # Implementation plans and design docs
├── internal/
│   ├── cli/               # CLI commands and adapters
│   ├── config/            # Configuration loading and validation
│   ├── daemon/            # Instance management (PID, status)
│   ├── interceptor/       # Request/response interceptors
│   ├── logging/           # Logging utilities
│   ├── provider/          # HTTP clients for providers
│   ├── proxy/             # HTTP server and request handling
│   ├── router/            # Routing engine and failover logic
│   ├── transformer/       # Request/response transformers
│   │   ├── converters/     # Format conversion utilities
│   │   │   ├── anthropic_to_unified.go
│   │   │   ├── unified_to_anthropic.go
│   │   │   ├── unified_to_openai.go
│   │   │   ├── openai_to_unified.go
│   │   │   ├── unified_to_gemini.go
│   │   │   └── gemini_to_unified.go
│   │   ├── providers/      # Provider transformer implementations
│   │   │   ├── anthropic.go
│   │   │   ├── openai.go
│   │   │   ├── openrouter.go
│   │   │   ├── gemini.go
│   │   │   ├── qwen.go
│   │   │   ├── glm.go
│   │   │   └── minimax.go
│   │   ├── unified/        # Unified intermediate format types
│   │   │   ├── message.go
│   │   │   ├── tool.go
│   │   │   ├── request.go
│   │   │   ├── response.go
│   │   │   └── reasoning.go
│   │   ├── test/           # Integration tests
│   │   │   └── integration_test.go
│   │   ├── base.go          # Base transformer utilities
│   │   ├── interface.go     # Transformer interface definition
│   │   └── registry.go      # Transformer registry
│   └── usage/              # Usage tracking and statistics
└── pkg/
    └── api/
        └── anthropic/      # Anthropic API types
```

## License

MIT
