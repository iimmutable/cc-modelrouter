# Testing Guide

This document describes the testing strategy, organization, and best practices for cc-modelrouter.

**Test Plan Documents:**
- [Unit Tests Plan](../../plans/unit-tests.md) - Comprehensive unit testing strategy
- [Integration Test Plans](../../plans/2025-02-24-integration-test-plans.md) - End-to-end testing strategy

---

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run Tests with Coverage

```bash
go test ./... -cover
```

### Run Tests with Coverage Report

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
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

### Run Integration Tests

```bash
# Requires test configuration file at .cc-modelrouter/test.config.json
go test -tags=integration ./test/... -v
```

### Run Tests for Specific Test Files

```bash
# Run individual test files directly (bypasses package tests)
go test ./internal/config/loader_test.go ./internal/config/loader.go
go test ./internal/daemon/pidfile_test.go ./internal/daemon/pidfile.go
go test ./internal/daemon/instance_test.go ./internal/daemon/instance.go
```

---

## Test Organization

```
cc-modelrouter/
├── internal/
│   ├── cli/
│   │   ├── adapters_test.go  # CLI adapter wrapper tests
│   │   └── root_test.go      # CLI root command tests
│   ├── config/
│   │   ├── loader_test.go    # Configuration loading tests
│   │   └── types_test.go     # Configuration type tests
│   ├── daemon/
│   │   ├── instance_test.go  # Instance management tests
│   │   └── pidfile_test.go   # PID file I/O tests
│   ├── provider/
│   │   ├── client_test.go    # Provider client tests
│   │   └── http_test.go      # HTTP client with retry logic tests
│   ├── proxy/
│   │   ├── handler_test.go    # HTTP request handler tests
│   │   ├── server_test.go     # HTTP server lifecycle tests
│   │   └── streaming_test.go # SSE streaming tests
│   ├── router/
│   │   ├── engine_test.go     # Route detection tests
│   │   └── failover_test.go   # Failover logic tests
│   ├── transformer/
│   │   ├── anthropic_test.go  # Anthropic transformer tests
│   │   ├── gemini_test.go     # Gemini transformer tests
│   │   ├── glm_test.go       # GLM transformer tests
│   │   ├── qwen_test.go      # Qwen transformer tests
│   │   ├── openrouter_test.go # OpenRouter transformer tests
│   │   └── registry_test.go   # Transformer registry tests
│   └── usage/
│       ├── db_test.go         # Database operations tests
│       ├── tracker_test.go    # Usage tracker tests
│       ├── period_test.go     # Period parsing tests
│       ├── stats_test.go      # Statistics aggregation tests
│       └── formatter_test.go   # Output formatting tests
├── pkg/
│   └── api/
│       └── anthropic/
│           └── types_test.go  # API type marshaling tests
└── test/
    └── integration_test.go  # Integration tests
```

---

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

**Prerequisites:**
- Test configuration file at `.cc-modelrouter/test.config.json`
- Valid API credentials for configured providers
- Network access to provider endpoints

**Running:**
```bash
go test -tags=integration ./test/... -v
```

#### Integration Test Scenarios

| Test | Description |
|------|-------------|
| `TestIntegrationBasicRequest` | End-to-end request through proxy with usage tracking verification |

The integration test validates:
1. Configuration loading
2. Transformer initialization
3. Provider client creation
4. Router engine setup
5. Proxy handler request processing
6. Usage tracking after successful response

---

## Current Test Coverage

| Package | Test Files | Test Count | Status |
|---------|------------|------------|--------|
| `internal/cli` | adapters_test.go, root_test.go | 34 | ✓ |
| `internal/config` | loader_test.go, types_test.go | 4 | ✓ |
| `internal/daemon` | instance_test.go, pidfile_test.go | 32 | ✓ |
| `internal/provider` | client_test.go, http_test.go | 23 | ✓ |
| `internal/proxy` | handler_test.go, server_test.go, streaming_test.go | 61 | ✓ |
| `internal/router` | engine_test.go, failover_test.go | 10 | ✓ |
| `internal/transformer` | anthropic_test.go, gemini_test.go, glm_test.go, qwen_test.go, openrouter_test.go, registry_test.go | ~20 | ✓ |
| `internal/usage` | db_test.go, tracker_test.go, period_test.go, stats_test.go, formatter_test.go | 19 | ✓ |
| `pkg/api/anthropic` | types_test.go | 3 | ✓ |

**Overall:** **206+ tests** passing across 9 packages

---

## Coverage Goals Achieved

| Module | Previous Coverage | Current Coverage | Target | Status |
|--------|------------------|------------------|--------|--------|
| `daemon` | ~2% | ~90%+ | 90%+ | ✓ |
| `provider` | ~53% | ~90%+ | 90%+ | ✓ |
| `proxy` | ~40% | ~90%+ | 90%+ | ✓ |
| `router` | ~84% | ~84% | 95% | ~ |
| `cli` | ~0% | ~50%+ | 50%+ | ✓ |
| `config` | ~47% | ~85% | 90% | ~ |
| `transformer` | ~77% | ~77% | 90% | ~ |
| `usage` | ~85% | ~95%+ | 95% | ✓ |

**Overall Target:** 85%+ test coverage across the codebase ✓ **ACHIEVED**

---

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
| `TestFailover*` | Failover iteration and exhaustion tests |

### Proxy Handler Tests (`internal/proxy/handler_test.go`)

Tests for HTTP handling and request analysis:

| Test | Description |
|------|-------------|
| `NewHandler*` | Handler creation with different request sizes |
| `ServeHTTP_*` | HTTP method/path validation and error handling |
| `handleMessages_*` | Message processing and failover |
| `tryTarget_*` | Provider/transformer/client error handling |
| `estimateTokens_*` | Token estimation from message content |
| `hasWebSearch_*` | Web search detection via tool names |
| `hasImages_*` | Image detection in message content |
| `isBackground_*` | Background agent detection via model name |
| `getThinkLevel_*` | Thinking level detection with edge cases |
| `Handler_Setters` | Dependency injection for router, registry, clients |

### Proxy Server Tests (`internal/proxy/server_test.go`)

Tests for HTTP server lifecycle:

| Test | Description |
|------|-------------|
| `Defaults` | Default server configuration values |
| `NewServer*` | Server creation with various configurations |
| `Server_Setters` | Dependency injection methods |
| `Server_TimeoutConfiguration` | Read/write timeout verification |
| `Server_HandlerCreated` | Handler initialization |
| `Server_HandlerMaxRequestSize` | Handler size configuration |
| `Server_ShutdownWithUsageTracker` | Usage tracker shutdown on stop |
| `Server_ConcurrentIsRunning` | Concurrent IsRunning() calls |
| `Server_PortZero` | Port zero handling |
| `Server_Start*` / `Server_Stop*` | Start/stop behavior |

### Proxy Streaming Tests (`internal/proxy/streaming_test.go`)

Tests for SSE streaming:

| Test | Description |
|------|-------------|
| `NewSSEWriter*` | Writer creation with/without Flusher |
| `WriteEvent*` | Event writing with various data |
| `Flush*` | Flushing behavior |
| `ParseSSEEvent*` | Event parsing for various formats |
| `SSEScanner_Scan*` | Scanner event scanning |
| `SSEScanner_*Method*` | Scanner method returns |
| `SSEScanner_*` | Scanner data/event handling |

### Daemon Tests (`internal/daemon/instance_test.go`, `pidfile_test.go`)

Tests for instance management and PID file operations:

| Test | Description |
|------|-------------|
| `GenerateInstanceID*` | ID generation and uniqueness |
| `InstanceMetadata*` | Metadata JSON marshaling/unmarshaling |
| `InstancesDir_Success` | Instances directory path retrieval |
| `IsRunning_*` | Process running status checks |
| `SaveAndLoadInstance*` | Instance persistence |
| `DeleteInstance*` | Instance cleanup |
| `LoadInstance*` | Instance loading errors |
| `ListInstances*` | Instance listing with filters |
| `WritePIDFile*` | PID file creation and permissions |
| `ReadPIDFile*` | PID file reading and parsing |

### Provider HTTP Client Tests (`internal/provider/http_test.go`)

Tests for HTTP client with retry logic:

| Test | Description |
|------|-------------|
| `NewClient_*` | Client creation with various configurations |
| `Do_Success` | Successful request without retries |
| `Do_RetryOn*` | Retry behavior for 5xx errors |
| `Do_NoRetryOn4xx` | No retry on 4xx errors |
| `Do_MaxRetriesExceeded` | Error after max retries |
| `Do_ContextCancellation` | Context cancellation handling |
| `Do_NetworkError` | Network error handling |
| `DoWithContext_*` | Context forwarding |
| `Do_Timeout` | Request timeout handling |
| `Do_ReadBody` | Response body reading |
| `Do_ErrorFormat` | Error message format |

### CLI Adapter Tests (`internal/cli/adapters_test.go`)

Tests for CLI adapter wrappers:

| Test | Description |
|------|-------------|
| `NewRouterAdapter*` | Router adapter creation and delegation |
| `NewTransformerAdapter*` | Transformer adapter creation and delegation |
| `NewRegistryAdapter*` | Registry adapter creation and delegation |
| `TransformerAdapter_*` | All interface method delegations |
| `RegistryAdapter_*` | Get success and not-found scenarios |

### CLI Root Command Tests (`internal/cli/root_test.go`)

Tests for CLI command structure:

| Test | Description |
|------|-------------|
| `NewRootCommand*` | Root command creation and properties |
| `NewRootCommand_*Command` | Subcommand verification |
| `Version` | Version constant verification |
| `Execute_*` | Execute function behavior |
| `NewRootCommand_PersistentFlags` | Flag configuration |

### Transformer Tests (`internal/transformer/*_test.go`)

Tests for each provider transformer:

| Test | Description |
|------|-------------|
| `TestAnthropicTransform*` | Anthropic format transformation |
| `TestGeminiTransform*` | Gemini API format transformation |
| `TestQwenTransform*` | Qwen/DashScope format transformation |
| `TestGLMTransform*` | Zhipu GLM format transformation |
| `TestOpenRouterTransform*` | OpenRouter API format transformation |
| `TestRegistry*` | Transformer registry management |

### Usage Tracking Tests (`internal/usage/*_test.go`)

Tests for usage tracking functionality:

| Test | Description |
|------|-------------|
| `TestDB*` | SQLite database initialization and operations |
| `TestTracker*` | Buffer flushing, concurrent access, shutdown |
| `TestPeriod*` | Period parsing for time ranges |
| `TestStats*` | Statistics aggregation |
| `TestFormatter*` | Token number formatting |

---

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

---

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

### Using Temporary Directories

For file I/O tests, use `t.TempDir()` for automatic cleanup:

```go
func TestFileOperations(t *testing.T) {
    tmpDir := t.TempDir()
    filePath := filepath.Join(tmpDir, "test.txt")

    err := os.WriteFile(filePath, []byte("content"), 0600)
    if err != nil {
        t.Fatalf("failed to write file: %v", err)
    }
    // File is automatically cleaned up after test
}
```

---

## Continuous Integration

Tests are run automatically on:
- Pull requests
- Pushes to main branch
- Release tags

---

## Test Best Practices

1. **Test behavior, not implementation** - Focus on what the code does, not how
2. **Use descriptive test names** - Names should describe the scenario being tested
3. **Test edge cases** - Include boundary values, empty inputs, and error conditions
4. **Keep tests independent** - Each test should run independently of others
5. **Avoid test interdependence** - Tests should not rely on execution order
6. **Use `t.Parallel()` for parallel tests** - Speed up test execution where safe
7. **Clean up resources** - Use `t.Cleanup()` for resource cleanup
8. **Use `t.TempDir()` for temporary files** - Automatic cleanup after test
9. **Use table-driven tests** - For multiple similar test cases
10. **Mock external dependencies** - Use interfaces for mocking HTTP clients, databases, etc.

---

## Test Documentation

For detailed test planning and testing strategies, refer to:

- **[Unit Tests Plan](../../plans/unit-tests.md)** - Comprehensive unit testing strategy with full test case coverage
- **[Integration Test Plans](../../plans/2025-02-24-integration-test-plans.md)** - End-to-end testing strategy

These documents provide:
- Detailed test scenarios for each package
- Test coverage goals and gaps
- Step-by-step test implementation guides
- Mock and fixture examples
- Integration test setup instructions
- Test case completion status tracking