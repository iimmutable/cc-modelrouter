# cc-modelrouter Design Document

**Date:** 2025-02-16
**Author:** Design Team
**Status:** Approved

---

## Overview

cc-modelrouter is a Go-based HTTP proxy server that intercepts and transforms requests between Claude Code and multiple LLM providers. It enables routing to different models based on request type, with built-in transformers for provider compatibility and sequential failover with looping.

**Key Distinguishing Features:**
1. Global and project-scoped configuration (project overrides global)
2. Built-in transformers for Anthropic, OpenRouter, Gemini, Qwen, and GLM
3. Sequential failover with looping (max 2× model list size)
4. Isolated instance pairs (one router + one Claude Code per command)
5. Minimal CLI: `code`, `start`, `stop`, `restart`, `status`, `clean`, `config`, `logs`

---

## Architecture

### Layered Architecture

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
│ (anthropic, openrouter, gemini, qwen, glm transformers)    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Provider Client Layer                                       │
│ (HTTP clients, streaming, retry)                            │
└─────────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
cc-modelrouter/
├── cmd/
│   ├── ccrouter/                   # Main CLI
│   │   └── main.go
│   └── code/                       # "ccrouter code" subcommand
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── code.go                 # "code" command
│   │   ├── start.go                # "start" command
│   │   ├── stop.go                 # "stop" command
│   │   ├── restart.go              # "restart" command
│   │   ├── status.go               # "status" command
│   │   ├── clean.go                # "clean" command
│   │   ├── config.go               # "config" command
│   │   └── logs.go                 # "logs" command
│   ├── daemon/
│   │   ├── instance.go             # Instance lifecycle
│   │   ├── port.go                 # Dynamic port allocation
│   │   └── pidfile.go              # PID file management
│   ├── proxy/
│   │   ├── server.go               # HTTP proxy server
│   │   ├── handler.go              # Anthropic API handler
│   │   ├── interceptor.go          # Request/response interception
│   │   └── streaming.go            # Streaming response handling
│   ├── config/
│   │   ├── loader.go               # Config loading
│   │   ├── global.go               # Global config handler
│   │   └── project.go              # Project config handler
│   ├── router/
│   │   ├── engine.go               # Route matching
│   │   ├── failover.go             # Failover with looping
│   │   └── selector.go             # Model selection
│   ├── transformer/
│   │   ├── interface.go            # Transformer interface
│   │   ├── registry.go             # Transformer registry
│   │   ├── anthropic.go            # Anthropic/GLM transformer
│   │   ├── openrouter.go           # OpenRouter transformer
│   │   ├── gemini.go               # Gemini transformer
│   │   └── qwen.go                 # Qwen transformer
│   ├── provider/
│   │   ├── client.go               # Provider client interface
│   │   ├── http.go                 # HTTP client with streaming
│   │   └── retry.go                # Retry logic
│   └── models/
│       ├── request.go              # Request types
│       └── response.go             # Response types
├── pkg/
│   └── api/
│       └── anthropropic/           # Anthropic API types
│           └── types.go
├── go.mod
├── go.sum
└── README.md
```

---

## Configuration

### Config File Locations

- **Global:** `~/.cc-modelrouter/config.json`
- **Project:** `<project-root>/.cc-modelrouter/config.json`

### Merging Strategy

Project config completely overrides global config when present. No deep merging.

### Config Structure

```json
{
  "server": {
    "port": 8081,
    "host": "localhost"
  },
  "providers": {
    "bigmodel": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-5", "glm-4.7", "glm-4.6v", "glm-4.5-air"]
    },
    "openrouter": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api/v1/chat/completions",
      "models": [
        "anthropic/claude-haiku-4.5",
        "anthropic/claude-sonnet-4.5",
        "anthropic/claude-opus-4.5",
        "google/gemini-2.5-pro"
      ]
    }
  },
  "router": {
    "routes": {
      "default": "bigmodel:glm-4.7;openrouter:anthropic/claude-sonnet-4.5",
      "background": "bigmodel:glm-4.5-air",
      "think": "openrouter:anthropic/claude-sonnet-4.5",
      "thinkMore": "openrouter:anthropic/claude-sonnet-4.5",
      "ultrathink": "openrouter:anthropic/claude-opus-4.5",
      "longContext": "bigmodel:glm-4.7;openrouter:google/gemini-2.5-pro",
      "webSearch": "bigmodel:glm-4.7;openrouter:google/gemini-2.5-pro",
      "image": "bigmodel:glm-4.6v;openrouter:google/gemini-2.5-pro"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

### Environment Variable Interpolation

Supports both `${VAR_NAME}` and `$VAR_NAME` syntax for sensitive values like API keys.

---

## Instance Isolation

Each `ccrouter code` command creates an isolated pair:

- Unique instance ID: `inst_YYYYMMDD_HHMMSS`
- Dynamically allocated port (starting from base port)
- Separate PID file: `~/.cc-modelrouter/instances/<instance-id>.json`
- Explicit environment for child Claude Code process

### Instance Metadata

```json
{
  "id": "inst_20250216_143022",
  "port": 8081,
  "pid": 12345,
  "configType": "project",
  "configPath": "/path/to/project/.cc-modelrouter/config.json",
  "startTime": "2025-02-16T14:30:22Z",
  "projectRoot": "/path/to/project"
}
```

---

## Request Flow

1. **Claude Code** sends request to `http://localhost:<port>/v1/messages`
2. **Proxy Handler** validates Anthropic API request
3. **Router Engine** detects route type and selects provider:model
4. **Failover Manager** loops through providers with retry
5. **Transformer** converts request to provider format
6. **Provider Client** sends to provider API
7. **Transformer** converts response back to Anthropic format
8. **Response Writer** streams back to Claude Code

### Route Detection

| Route | Trigger Condition | Detection Method |
|-------|-------------------|------------------|
| `background` | Background agent requests | Model name contains "claude" and "haiku" |
| `ultrathink` | Highest thinking level | `budget_tokens >= 32,000` |
| `thinkMore` | Middle thinking level | `budget_tokens >= 10,000` |
| `think` | Basic thinking level | `budget_tokens >= 4,000` |
| `image` | Image processing requests | Request contains image content blocks |
| `webSearch` | Web search enabled | Tool names contain "web" or "search" |
| `longContext` | Large context requests | Token count > 60,000 |
| `default` | Standard requests | Fallback for all other requests |

#### Thinking Levels

Claude Code supports multiple thinking intensity levels through trigger phrases. These phrases are converted to numeric `budget_tokens` values before the request reaches our router:

| Level | Budget Tokens | Trigger Phrases |
|-------|---------------|-----------------|
| Basic | ~4,000 | "think", "思考" |
| Middle | ~10,000 | "think hard", "think more", "think deeply", "megathink" |
| Highest | ~32,000 | "ultrathink", "think harder", "think intensely", "think longer" |

**Fallback Behavior:**
- If `ultrathink` route is not configured, highest level falls back to `thinkMore`
- If `thinkMore` route is not configured, middle level falls back to `think`
- This allows flexible configuration with 1, 2, or 3 thinking tiers

---

## Transformers

### Transformer Interface

```go
type Transformer interface {
    Name() string
    TransformRequest(*anthropic.Request) (*http.Request, error)
    TransformResponse(*http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamChunk([]byte) ([]byte, error)
}
```

### Built-in Transformers

| Transformer | Purpose |
|-------------|---------|
| anthropic | Pass-through for Anthropic/GLM APIs |
| openrouter | OpenRouter API format with provider routing |
| gemini | Google Gemini API format |
| qwen | Alibaba Qwen API format |
| glm | Z-AI BigModel GLM specific |

### Transformer Resolution

1. If `transformer` field specified → use that transformer
2. If not specified → use transformer matching provider name
3. If no match found → use `anthropic` (pass-through)

---

## Failover Strategy

- **Sequential:** Try each provider:model in order
- **Looping:** After list exhausted, restart from beginning
- **Max Attempts:** 2 × number of providers in route
- **Retry Delay:** Configurable (default 500ms)

---

## CLI Commands

```
ccrouter code [claude-args...]  - Start router and launch Claude Code
ccrouter start [options]        - Start router server standalone
ccrouter restart [instance-id]  - Restart instance (always reloads config)
ccrouter stop [instance-id]     - Stop router instance (or all with --all)
ccrouter status                 - Show all running instances
ccrouter clean                  - Remove stale instance files
ccrouter config                 - Show active configuration
ccrouter logs [instance-id]     - Show logs for instance
```

---

## Security

- **File Permissions:** Config files set to 0600 (user read/write only)
- **Default Bind:** localhost (127.0.0.1) by default
- **Request Validation:** Max request size 50MB, token limits
- **API Key Logging:** Never logged, masked in error messages

---

## Performance

- **Connection Pooling:** MaxIdleConns: 100, MaxIdleConnsPerHost: 10
- **Max Request Size:** 50MB (supports large contexts)
- **Streaming:** Zero-copy streaming with minimal buffering

---

## Testing

> **See [docs/testing.md](../docs/testing.md) for detailed testing documentation.**

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./internal/router/... -v
```

### Test Coverage

| Package | Coverage | Description |
|---------|----------|-------------|
| `internal/router` | 83.7% | Route detection and failover |
| `internal/transformer` | 76.6% | Request/response transformation |
| `internal/provider` | 52.9% | HTTP client and retry logic |
| `internal/proxy` | 40.1% | HTTP server and request handling |
| `internal/config` | 46.8% | Configuration loading |

### Key Test Suites

#### Route Detection Tests

| Test | Description |
|------|-------------|
| `TestRouteDetection` | Basic route detection |
| `TestThinkLevelDetection` | Thinking level with full config |
| `TestThinkLevelFallback` | Fallback with partial config |
| `TestRoutePriority` | Route priority ordering |
| `TestGetTargets` | Target retrieval |

#### Handler Detection Tests

| Test | Description |
|------|-------------|
| `TestIsBackground` | Background agent detection |
| `TestGetThinkLevel` | Thinking level via budget_tokens |
| `TestHasWebSearch` | Web search via tool names |
| `TestHasImages` | Image content detection |

### Test Configuration

Location: `.cc-modelrouter/test.config.json`

```json
{
  "providers": {
    "bigmodel": {
      "enabled": true,
      "apiKey": "YOUR_API_KEY",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-4.7"]
    }
  },
  "testSettings": {
    "timeout": "5m",
    "retryAttempts": 3,
    "parallelTests": false,
    "verboseLogging": true
  }
}
```

### Test Categories

- **Unit Tests:** Logic without external dependencies
- **Integration Tests:** Real provider compatibility (`go test -tags=integration`)
- **E2E Tests:** Full request flow

### Test Scenarios

- Basic requests
- Streaming responses
- Tool use in streaming
- Long context handling
- Error handling
- Thinking level detection
- Route priority ordering
