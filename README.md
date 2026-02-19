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
      "baseURL": "https://openrouter.ai/api/v1",
      "models": ["anthropic/claude-sonnet-4", "google/gemini-2.5-pro"]
    },
    "bigmodel": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-4.7", "glm-4.5-air"]
    }
  },
  "router": {
    "routes": {
      "default": "openrouter:anthropic/claude-sonnet-4",
      "background": "bigmodel:glm-4.5-air",
      "think": "openrouter:anthropic/claude-sonnet-4",
      "longContext": "openrouter:google/gemini-2.5-pro",
      "webSearch": "openrouter:google/gemini-2.5-pro",
      "image": "bigmodel:glm-4.6v"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
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

| Route | Trigger |
|-------|---------|
| `default` | Standard requests |
| `background` | Background agent requests |
| `think` | Plan mode requests |
| `longContext` | Requests with >60K tokens |
| `webSearch` | Web search enabled requests |
| `image` | Image processing requests |

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

| Provider | Transformer | API Format |
|----------|-------------|------------|
| Anthropic | `anthropic` | Native Anthropic |
| OpenRouter | `openrouter` | OpenAI-compatible |
| Google Gemini | `gemini` | Gemini native |
| Alibaba Qwen | `qwen` | OpenAI-compatible |
| Zhipu GLM | `glm` | Anthropic-compatible |

## Architecture

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
│ Router Engine Layer                                         │
│ (route matching, failover logic, model selection)           │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Transformer Layer                                           │
│ (anthropic, openrouter, gemini, qwen, glm)                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Provider Client Layer                                       │
│ (HTTP clients, streaming, retry)                            │
└─────────────────────────────────────────────────────────────┘
```

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
```

### Project Structure

```
cc-modelrouter/
├── cmd/
│   └── ccrouter/           # Main CLI entry point
├── internal/
│   ├── cli/                # CLI commands
│   ├── config/             # Configuration loading
│   ├── daemon/             # Instance management
│   ├── provider/           # HTTP clients
│   ├── proxy/              # HTTP server
│   ├── router/             # Routing engine
│   └── transformer/        # Request/response transformers
├── pkg/
│   └── api/anthropic/      # Anthropic API types
├── plans/                  # Design documents
└── test/                   # Integration tests
```

## License

MIT
