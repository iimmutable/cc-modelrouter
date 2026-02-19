# Testing Guide

This document describes the testing strategy, organization, and best practices for cc-modelrouter.

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run Tests with Coverage

```bash
go test ./... -cover
```

### Run Tests with Verbose Output

```bash
go test ./... -v
```

### Run Specific Package Tests

```bash
go test ./internal/router/... -v
go test ./internal/proxy/... -v
```

### Run Specific Test

```bash
go test ./internal/router/... -v -run TestThinkLevelDetection
```

## Test Organization

```
cc-modelrouter/
├── internal/
│   ├── config/
│   │   └── *_test.go      # Configuration loading tests
│   ├── daemon/
│   │   └── *_test.go      # Instance management tests
│   ├── provider/
│   │   └── *_test.go      # Provider client tests
│   ├── proxy/
│   │   └── *_test.go      # HTTP server and handler tests
│   ├── router/
│   │   └── *_test.go      # Route detection and failover tests
│   └── transformer/
│       └── *_test.go      # Transformer tests
└── pkg/
    └── api/
        └── anthropic/
            └── *_test.go  # API types tests
```

## Test Categories

### Unit Tests

Unit tests verify individual functions and methods in isolation.

**Location:** `*_test.go` files alongside source code

**Example - Testing thinking level detection:**
```go
func TestGetThinkLevel(t *testing.T) {
    handler := NewHandler(50 * 1024 * 1024)

    tests := []struct {
        name     string
        thinking *anthropic.ThinkingConfig
        expected router.ThinkLevel
    }{
        {"nil thinking", nil, router.ThinkNone},
        {"thinking with budget 32000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 32000}, router.ThinkHighest},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &anthropic.Request{Thinking: tt.thinking}
            result := handler.getThinkLevel(req)
            if result != tt.expected {
                t.Errorf("getThinkLevel() = %v, expected %v", result, tt.expected)
            }
        })
    }
}
```

### Integration Tests

Integration tests verify multiple components working together.

**Location:** `test/integration_test.go`

**Running:**
```bash
go test -tags=integration ./test/...
```

## Current Test Coverage

| Package | Coverage | Description |
|---------|----------|-------------|
| `internal/config` | 46.8% | Configuration loading and validation |
| `internal/daemon` | 1.5% | Instance lifecycle management |
| `internal/provider` | 52.9% | HTTP client and retry logic |
| `internal/proxy` | 40.1% | HTTP server and request handling |
| `internal/router` | 83.7% | Route detection and failover |
| `internal/transformer` | 76.6% | Request/response transformation |
| `pkg/api/anthropic` | 50.0% | API type marshaling |

## Key Test Suites

### Router Engine Tests (`internal/router/engine_test.go`)

Tests for route detection logic:

| Test | Description |
|------|-------------|
| `TestRouteDetection` | Basic route detection (default, background, longContext) |
| `TestThinkLevelDetection` | Thinking level detection with full config |
| `TestThinkLevelFallback` | Fallback behavior with partial config |
| `TestThinkLevelPartialConfig` | Middle tier fallback (ultrathink → thinkMore → think) |
| `TestRoutePriority` | Route priority ordering |
| `TestThinkLevelNoRouteConfigured` | Fallback to default when no think routes |
| `TestGetTargets` | Target retrieval and default fallback |

### Proxy Handler Tests (`internal/proxy/server_test.go`)

Tests for HTTP handling and request analysis:

| Test | Description |
|------|-------------|
| `TestIsBackground` | Background agent detection via model name |
| `TestGetThinkLevel` | Thinking level detection via budget_tokens |
| `TestHasWebSearch` | Web search detection via tool names |
| `TestHasImages` | Image detection in message content |

### Transformer Tests (`internal/transformer/*_test.go`)

Tests for each provider transformer:

| Test | Description |
|------|-------------|
| `TestAnthropicTransform*` | Anthropic/GLM format transformation |
| `TestGeminiTransform*` | Gemini API format transformation |
| `TestQwenTransform*` | Qwen/DashScope format transformation |
| `TestGLMTransform*` | Zhipu GLM format transformation |

### API Types Tests (`pkg/api/anthropic/types_test.go`)

Tests for request/response types:

| Test | Description |
|------|-------------|
| `TestRequestMarshaling` | Request JSON marshaling |
| `TestContentBlockTypes` | Content block type handling |
| `TestThinkingConfig` | Thinking config marshaling |
| `TestRequestWithThinking` | Full request with thinking parameter |

## Testing Route Detection

### Thinking Levels

The thinking level detection is tested with various budget_tokens values:

```go
// Budget thresholds
ThinkLevelBasic   = 4000   // "think"
ThinkLevelMiddle  = 10000  // "think more", "think hard"
ThinkLevelHighest = 32000  // "ultrathink"
```

**Test cases:**
- `budget_tokens = 0` → `ThinkNone`
- `budget_tokens = 1000` → `ThinkBasic`
- `budget_tokens = 4000` → `ThinkBasic`
- `budget_tokens = 10000` → `ThinkMiddle`
- `budget_tokens = 20000` → `ThinkMiddle`
- `budget_tokens = 32000` → `ThinkHighest`
- `budget_tokens = 50000` → `ThinkHighest`

### Route Priority

Routes are checked in this priority order:

1. `background` - Claude Code uses Haiku models for background agents
2. `ultrathink` - Highest thinking level (budget_tokens >= 32000)
3. `thinkMore` - Middle thinking level (budget_tokens >= 10000)
4. `think` - Basic thinking level (budget_tokens >= 4000)
5. `image` - Request contains image content
6. `webSearch` - Tool names contain "web" or "search"
7. `longContext` - Token count > 60,000
8. `default` - Fallback for all other requests

### Fallback Behavior

Thinking routes support flexible configuration:

```
ultrathink → thinkMore → think → default
thinkMore → think → default
think → default
```

## Writing New Tests

### Table-Driven Tests

Use table-driven tests for testing multiple scenarios:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case 1", "input1", "output1"},
        {"case 2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Testing HTTP Handlers

For HTTP handler tests, use `httptest`:

```go
import "net/http/httptest"

func TestHandler(t *testing.T) {
    req := httptest.NewRequest("POST", "/v1/messages", body)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", rec.Code)
    }
}
```

### Mocking Interfaces

Use interfaces for mocking external dependencies:

```go
type MockTransformer struct {
    transformErr error
}

func (m *MockTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
    if m.transformErr != nil {
        return nil, m.transformErr
    }
    return httptest.NewRequest("POST", baseURL, nil), nil
}
```

## Continuous Integration

Tests are run automatically on:
- Pull requests
- Pushes to main branch
- Release tags

## Test Best Practices

1. **Test behavior, not implementation** - Focus on what the code does, not how
2. **Use descriptive test names** - Names should describe the scenario being tested
3. **Test edge cases** - Include boundary values, empty inputs, and error conditions
4. **Keep tests independent** - Each test should run independently of others
5. **Avoid test interdependence** - Tests should not rely on execution order
6. **Use `t.Parallel()` for parallel tests** - Speed up test execution where safe
7. **Clean up resources** - Use `t.Cleanup()` for resource cleanup
