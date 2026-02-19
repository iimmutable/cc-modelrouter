# Architecture Overview

cc-modelrouter is built as a layered architecture with clear separation of concerns.

## Layered Architecture

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

## Component Overview

### CLI Layer (`internal/cli/`)

Handles command-line interface using Cobra framework.

**Commands:**
- `code` - Start router and launch Claude Code
- `start` - Start router server standalone
- `stop` - Stop router instance
- `restart` - Restart instance
- `status` - Show running instances
- `clean` - Remove stale instance files
- `config` - Show active configuration
- `logs` - Show instance logs

### Configuration Layer (`internal/config/`)

Manages configuration loading with support for:
- Global configuration (`~/.cc-modelrouter/config.json`)
- Project configuration (`<project>/.cc-modelrouter/config.json`)
- Environment variable interpolation (`${VAR_NAME}` syntax)
- Configuration validation

**Key Types:**
```go
type Config struct {
    Server    ServerConfig
    Providers map[string]ProviderConfig
    Router    RouterConfig
}
```

### Proxy Server Layer (`internal/proxy/`)

HTTP server implementing the Anthropic Messages API endpoint.

**Features:**
- Request validation (max 50MB)
- SSE streaming support
- Integration adapters for router and transformer layers

**Key Components:**
- `Server` - HTTP server with graceful shutdown
- `Handler` - Request handler for `/v1/messages`
- `SSEWriter` - Server-Sent Events streaming

### Router Engine Layer (`internal/router/`)

Determines which provider and model to use for each request.

**Route Detection:**
| Route | Trigger Condition | Detection Method |
|-------|-------------------|------------------|
| `background` | Background agent | Model contains "claude" + "haiku" |
| `ultrathink` | Highest thinking | `budget_tokens >= 32,000` |
| `thinkMore` | Middle thinking | `budget_tokens >= 10,000` |
| `think` | Basic thinking | `budget_tokens >= 4,000` |
| `image` | Image content | Request contains image blocks |
| `webSearch` | Web search enabled | Tool names contain "web"/"search" |
| `longContext` | Large context | Token count > 60,000 |
| `default` | Fallback | All other requests |

**Thinking Levels:**

Claude Code converts trigger phrases to numeric `budget_tokens` before sending requests. Our router detects these levels:

| Level | Budget Threshold | Trigger Phrases |
|-------|-----------------|-----------------|
| `ThinkNone` | 0 | (no thinking) |
| `ThinkBasic` | >= 4,000 | "think", "思考" |
| `ThinkMiddle` | >= 10,000 | "think hard", "think more", "megathink" |
| `ThinkHighest` | >= 32,000 | "ultrathink", "think harder" |

**Thinking Route Fallback:**
- `ultrathink` → `thinkMore` → `think` (if not configured)
- `thinkMore` → `think` (if not configured)

**Failover Strategy:**
- Sequential: Try each provider:model in order
- Looping: After list exhausted, restart from beginning
- Max Attempts: 2 × number of providers in route

### Transformer Layer (`internal/transformer/`)

Converts requests and responses between Anthropic format and provider-specific formats.

**Interface:**
```go
type Transformer interface {
    Name() string
    TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    TransformResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
```

**Built-in Transformers:**

| Transformer | Format | Authentication |
|-------------|--------|----------------|
| `anthropic` | Anthropic native | `x-api-key` header |
| `openrouter` | OpenAI-compatible | `Authorization: Bearer` |
| `gemini` | Gemini native (`contents`/`parts`) | Query param `key=` |
| `qwen` | OpenAI-compatible | `Authorization: Bearer` |
| `glm` | Anthropic-compatible | `Authorization: Bearer` |

### Provider Client Layer (`internal/provider/`)

HTTP client for communicating with provider APIs.

**Features:**
- Connection pooling
- Retry with configurable delay
- Streaming support
- Timeout handling

### Daemon Management (`internal/daemon/`)

Manages router instance lifecycle.

**Features:**
- Instance ID generation
- PID file management
- Instance metadata storage
- Dynamic port allocation

## Request Flow

```
Claude Code                    cc-modelrouter                    Provider
    │                               │                               │
    │  POST /v1/messages            │                               │
    │──────────────────────────────>│                               │
    │                               │                               │
    │                    Validate request                            │
    │                               │                               │
    │                    Detect route type                          │
    │                               │                               │
    │                    Select provider:model                       │
    │                               │                               │
    │                    Transform request                           │
    │                               │                               │
    │                               │  POST /chat/completions        │
    │                               │──────────────────────────────>│
    │                               │                               │
    │                               │                    Process request
    │                               │                               │
    │                               │  Response (or streaming)       │
    │                               │<──────────────────────────────│
    │                               │                               │
    │                    Transform response                          │
    │                               │                               │
    │  Response (or streaming)      │                               │
    │<──────────────────────────────│                               │
    │                               │                               │
```

## Instance Isolation

Each `ccrouter code` command creates an isolated environment:

```
~/.cc-modelrouter/
├── config.json                    # Global configuration
└── instances/
    ├── inst_20250216_143022.json  # Instance metadata
    ├── inst_20250216_150033.json
    └── ...
```

**Instance Metadata:**
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

## Design Decisions

### Why Go?

- Single binary distribution
- Excellent HTTP server performance
- Strong typing for API contracts
- Built-in concurrency support

### Why Registry Pattern for Transformers?

- Easy to add new transformers
- Runtime transformer selection
- Clean separation of concerns

### Why Instance Isolation?

- Multiple projects can run simultaneously
- Each project uses its own configuration
- No port conflicts with dynamic allocation

### Why Sequential Failover?

- Simple and predictable behavior
- Easy to reason about retry logic
- Matches common load balancer patterns
