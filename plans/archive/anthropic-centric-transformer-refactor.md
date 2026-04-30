# Anthropic-Centric Transformer Refactor Plan

**Date**: 2026-03-02
**Status**: Proposed
**Author**: Claude (with user guidance)

## Executive Summary

Refactor cc-modelrouter's transformer layer to eliminate the unified intermediate format and make Anthropic API the canonical standard. This reduces complexity from 7 provider implementations to 3 transformers while preserving all existing functionality (interception, usage tracking, failover).

**Key Insight**: Most Chinese providers (BigModel for GLM, OpenRouter/Aliyun for Qwen) now provide Anthropic-compatible endpoints. The provider itself handles the format conversion, not cc-modelrouter.

## Current Architecture Problems

### Unnecessary Complexity
- **Unified intermediate format** adds double transformation: Anthropic → Unified → Provider
- **7 provider implementations** (anthropic, glm, openai, gemini, qwen, openrouter, minimax)
- **2,300+ lines of converter code** in `internal/transformer/converters/`
- **Code duplication**: OpenAI-family providers all have similar tool call state management

### Reality Check
| Provider | Actual API Style | Current Transformer |
|----------|-----------------|---------------------|
| Anthropic | Native Anthropic | Pass-through (correct) |
| GLM (BigModel) | **Anthropic-compatible** | Over-engineered |
| Qwen (OpenRouter/Aliyun) | **Anthropic-compatible** | Over-engineered |
| MiniMax (OpenRouter) | **Anthropic-compatible** | Over-engineered |
| OpenAI | Native OpenAI | Needs conversion |
| Gemini | Native Gemini | Needs conversion |

**Conclusion**: Only 3 transformers are actually needed.

## Target Architecture

```
internal/
├── transformer/
│   ├── interface.go                 # REVISED: Anthropic-centric interface
│   ├── registry.go                  # PRESERVED: Same registration mechanism
│   ├── base.go                      # REVISED: Remove unified dependencies
│   └── transformers/                # NEW: Streamlined transformers
│       ├── anthropic.go             # Pass-through (identity)
│       ├── openai.go                # OpenAI ↔ Anthropic (both directions)
│       └── gemini.go                # Gemini ↔ Anthropic (both directions)
│
├── intercept/                       # PRESERVED: All interception logic
├── usage/                           # PRESERVED: Usage tracking
└── router/                          # PRESERVED: Routing logic
```

## Revised Transformer Interface

```go
// internal/transformer/interface.go
package transformer

import (
    "net/http"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// SSEEvent represents a server-sent event.
type SSEEvent struct {
    EventType string
    Data      []byte
}

// Transformer transforms between Anthropic and provider formats.
type Transformer interface {
    // Name returns the transformer name.
    Name() string

    // Endpoint returns the API endpoint path.
    Endpoint() string

    // PrepareRequest converts Anthropic request to provider HTTP request.
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

    // ParseResponse converts provider HTTP response to Anthropic response.
    ParseResponse(resp *http.Response) (*anthropic.Response, error)

    // SupportsStreaming returns true if transformer supports streaming.
    SupportsStreaming() bool

    // TransformStreamEvent converts provider SSE event to Anthropic SSE events.
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

**Key change**: Direct Anthropic ↔ Provider conversion, no unified intermediate format.

## Configuration Schema (No Changes)

Existing `ProviderConfig` structure is preserved:

```json
{
  "providers": {
    "anthropic": {
      "apiKey": "${ANTHROPIC_API_KEY}",
      "baseURL": "https://api.anthropic.com",
      "transformer": "anthropic",
      "models": ["claude-sonnet-4-20250514", "claude-opus-4-20250514"]
    },
    "glm": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://api.bigmodel.cn",
      "transformer": "anthropic",
      "models": ["glm-4.1-all", "glm-4-flash"]
    },
    "qwen": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api",
      "transformer": "anthropic",
      "models": ["qwen/qwen-2.5-72b-instruct"]
    },
    "openai": {
      "apiKey": "${OPENAI_API_KEY}",
      "baseURL": "https://api.openai.com",
      "transformer": "openai",
      "models": ["gpt-4o", "gpt-4o-mini"]
    },
    "gemini": {
      "apiKey": "${GEMINI_API_KEY}",
      "baseURL": "https://generativelanguage.googleapis.com",
      "adapter": "gemini",
      "models": ["gemini-2.0-flash-exp"]
    }
  }
}
```

## Transformer Implementations

### 1. Anthropic Transformer (Pass-through)

**File**: `internal/transformer/transformers/anthropic.go`

```go
package transformers

import (
    "bytes"
    "encoding/json"
    "net/http"

    "github.com/iimmutable/cc-modelrouter/internal/transformer"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

type AnthropicTransformer struct{}

func NewAnthropicTransformer() *AnthropicTransformer {
    return &AnthropicTransformer{}
}

func (t *AnthropicTransformer) Name() string {
    return "anthropic"
}

func (t *AnthropicTransformer) Endpoint() string {
    return "/v1/messages"
}

func (t *AnthropicTransformer) PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
    req.Model = model
    body, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }

    endpoint := baseURL
    if !bytes.HasSuffix([]byte(baseURL), []byte("/v1/messages")) {
        endpoint = baseURL + "/v1/messages"
    }

    httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")
    httpReq.Header.Set("User-Agent", "cc-modelrouter/1.0")

    return httpReq, nil
}

func (t *AnthropicTransformer) ParseResponse(resp *http.Response) (*anthropic.Response, error) {
    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
    }

    var anthropicResp anthropic.Response
    if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
        return nil, err
    }
    return &anthropicResp, nil
}

func (t *AnthropicTransformer) SupportsStreaming() bool {
    return true
}

func (t *AnthropicTransformer) TransformStreamEvent(event *transformer.SSEEvent) ([]transformer.SSEEvent, error) {
    // Pass through unchanged - already in Anthropic format
    return []transformer.SSEEvent{*event}, nil
}
```

### 2. OpenAI Transformer

**File**: `internal/transformer/transformers/openai.go`

Consolidates all OpenAI ↔ Anthropic conversion logic in a single file:

```go
package transformers

type OpenAITransformer struct {
    toolCallStates map[int]*toolCallState
    messageStarted bool
}

// Contains all OpenAI ↔ Anthropic conversion logic:
// - PrepareRequest: Anthropic → OpenAI format
//   - Convert messages to OpenAI format
//   - Convert tools to OpenAI function format
//   - Set proper headers (Authorization Bearer)
//
// - ParseResponse: OpenAI → Anthropic format
//   - Convert choices to content blocks
//   - Convert tool calls to Anthropic format
//   - Map finish_reason
//
// - TransformStreamEvent: OpenAI SSE → Anthropic SSE
//   - Generate synthetic message_start event
//   - Manage tool call state across chunks
//   - Generate content_block_start/delta/stop events
//   - Generate message_delta with output_tokens
```

### 3. Gemini Transformer

**File**: `internal/transformer/transformers/gemini.go`

Consolidates all Gemini ↔ Anthropic conversion logic in a single file:

```go
package transformers

type GeminiTransformer struct {
    partsBuilder    *strings.Builder
    currentRole     string
    functionCallMap map[string]*functionCallState
}

// Contains all Gemini ↔ Anthropic conversion logic:
// - PrepareRequest: Anthropic → Gemini format
//   - Convert messages to parts-based format
//   - Convert tools to function declarations
//   - API key in query parameter
//
// - ParseResponse: Gemini → Anthropic format
//   - Convert parts to content blocks
//   - Convert function calls to tool use
//
// - TransformStreamEvent: Gemini SSE → Anthropic SSE
//   - Parse chunk-based streaming
//   - Generate Anthropic SSE events
```

## Preserved Components

All existing functionality remains unchanged:

| Component | Location | Status |
|-----------|----------|--------|
| **Configuration types** | `internal/config/types.go` | Unchanged |
| **Registry** | `internal/transformer/registry.go` | Unchanged |
| **Request interception** | `internal/proxy/handler.go` | Unchanged |
| **Response interception** | `internal/proxy/handler.go` | Unchanged |
| **Streaming interception** | `internal/proxy/handler.go` | Unchanged |
| **Usage tracking** | `internal/usage/tracker.go` | Unchanged |
| **Router engine** | `internal/router/` | Unchanged |
| **Failover logic** | `internal/router/failover.go` | Unchanged |

## Deleted Components

```
DELETED AFTER MIGRATION:
├── internal/transformer/unified/     # Entire unified format package
│   ├── request.go
│   ├── response.go
│   ├── message.go
│   ├── content.go
│   └── tool.go
├── internal/transformer/converters/  # All converter files
│   ├── anthropic_to_unified.go
│   ├── unified_to_anthropic.go
│   ├── unified_to_openai.go
│   ├── unified_to_gemini.go
│   └── gemini_to_unified.go
└── internal/transformer/providers/   # Old provider implementations
    ├── anthropic.go
    ├── glm.go
    ├── openai.go
    ├── gemini.go
    ├── qwen.go
    ├── openrouter.go
    └── minimax.go
```

## Migration Steps

### Phase 1: Create New Interface and Transformers
- [ ] Create revised `internal/transformer/interface.go`
- [ ] Create `internal/transformer/transformers/` directory
- [ ] Implement `anthropic.go` (pass-through)
- [ ] Implement `openai.go` (consolidate from existing)
- [ ] Implement `gemini.go` (consolidate from existing)
- [ ] Update `internal/transformer/registry.go` to register new transformers

### Phase 2: Update Handler Integration
- [ ] Modify `internal/proxy/handler.go` to use new interface
- [ ] Ensure request interception still works
- [ ] Ensure response interception still works
- [ ] Ensure streaming interception still works
- [ ] Ensure usage tracking still works

### Phase 3: Port Tests
- [ ] Port Anthropic transformer tests
- [ ] Port OpenAI transformer tests (especially streaming and tool calls)
- [ ] Port Gemini transformer tests
- [ ] Add integration tests for Anthropic-compatible providers (GLM, Qwen)
- [ ] Run full test suite

### Phase 4: Validation
- [ ] Test with Anthropic provider
- [ ] Test with GLM (BigModel) using anthropic transformer
- [ ] Test with Qwen (OpenRouter/Aliyun) using anthropic transformer
- [ ] Test with OpenAI using openai transformer
- [ ] Test with Gemini using gemini transformer
- [ ] Verify usage tracking accuracy for all providers
- [ ] Verify streaming works for all providers
- [ ] Verify tool use works for all providers

### Phase 5: Remove Old Code
- [ ] Delete `internal/transformer/unified/`
- [ ] Delete `internal/transformer/converters/`
- [ ] Delete `internal/transformer/providers/`
- [ ] Update documentation

## Benefits

| Metric | Current | After Refactor | Improvement |
|--------|---------|----------------|-------------|
| **Transformers** | 7 implementations | 3 transformers | 57% reduction |
| **Code lines** | ~10,000 | ~3,000 | 70% reduction |
| **Format conversions** | 2 (Anthropic→Unified→Provider) | 1 (Anthropic→Provider) | 50% faster |
| **Test lines** | ~5,000 | ~1,500 | 70% reduction |
| **Mental model** | Unified format | Anthropic standard | Simpler |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Streaming tool call state management complexity | Thorough testing of OpenAI streaming |
| Provider API changes | Keep adapter interface flexible |
| Usage tracking regression | Port all usage tracking tests |
| Breaking existing configurations | No config changes required |

## Success Criteria

- [ ] All existing tests pass
- [ ] Manual testing confirms all providers work
- [ ] Usage tracking accurate for all providers
- [ ] Streaming works correctly for all providers
- [ ] Tool use works correctly for all providers
- [ ] Code reduction of at least 60%
- [ ] No configuration changes required for users

## Notes

- **No configuration changes**: Users keep using `transformer` field as before
- **Backward compatible**: Old and new can coexist during migration
- **Provider flexibility**: Users can configure Anthropic-compatible providers to use `anthropic` transformer
- **Future-proof**: Easy to add new transformers if needed
