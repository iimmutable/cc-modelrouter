# Transformer Re-architecture Design Document

**Date:** 2026-02-26
**Author:** Design Draft
**Status:** Draft - Pending Approval

## 1. Overview & Goals

**Objective:** Rework the transformer implementation to enable seamless communication between Claude Code and any supported LLM provider for all use cases: request/response, streaming, tool-use, thinking/reasoning, and multimodal content.

**Key Issues Addressed:**
1. **Streaming SSE events** - Proper event transformation for all providers
2. **Tool call format** - Correct conversion between Anthropic and provider formats
3. **Thinking/reasoning** - Support for extended thinking in Claude Code
4. **Multimodal content** - Images, files, and other non-text content

**Design Principle:** Keep the implementation in Go, adopt the **core patterns** from musistudio/claude-code-router, and maintain backward compatibility where possible.

**Role Separation:**
- **Transformers** = Protocol conversion (Anthropic ↔ Provider format only)
- **Interceptors** = Utility/cross-cutting concerns (token limits, tool modifications, thinking content)

## 2. Architecture

### 2.1 Core Concept: Unified Intermediate Format

The new architecture introduces a **Unified Chat Format** as an intermediate representation between Anthropic format and provider-specific formats:

```
Anthropic Request → UnifiedChatRequest → Provider Request
Provider Response → UnifiedChatResponse → Anthropic Response
```

This enables:
- Cleaner separation - each transformer handles conversion to/from Unified format
- Easier provider additions - only need to implement two conversions
- Better testability - each phase can be tested independently
- Reusable utilities for common transformations

### 2.2 New Transformer Interface

```go
// Transformer transforms requests between Anthropic and provider formats.
type Transformer interface {
    // Name returns the transformer name.
    Name() string

    // Endpoint returns the API endpoint path (e.g., "/v1/messages", "/chat/completions")
    Endpoint() string

    // Auth prepares authentication headers for the request
    Auth(req *UnifiedChatRequest, provider Provider) (map[string]string, error)

    // TransformRequestIn converts Anthropic format to Unified format
    TransformRequestIn(req *anthropic.Request) (*UnifiedChatRequest, error)

    // TransformRequestOut converts Unified format to provider-specific HTTP request
    TransformRequestOut(unified *UnifiedChatRequest, baseURL, apiKey, model string) (*http.Request, error)

    // TransformResponseIn converts provider response to Unified format
    TransformResponseIn(resp *http.Response) (*UnifiedChatResponse, error)

    // TransformResponseOut converts Unified format to Anthropic format
    TransformResponseOut(unified *UnifiedChatResponse) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming
    SupportsStreaming() bool

    // TransformSSEEvent transforms a provider SSE event to Anthropic format
    TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

### 2.3 Unified Types

```go
// UnifiedChatRequest represents the unified request format
type UnifiedChatRequest struct {
    Messages      []UnifiedMessage    `json:"messages"`
    Model         string              `json:"model"`
    MaxTokens     int                 `json:"max_tokens,omitempty"`
    Temperature   float64             `json:"temperature,omitempty"`
    Stream        bool                `json:"stream,omitempty"`
    Tools         []UnifiedTool       `json:"tools,omitempty"`
    ToolChoice    any                 `json:"tool_choice,omitempty"`
    System        string              `json:"system,omitempty"`
    Reasoning     *ReasoningConfig    `json:"reasoning,omitempty"`
    Metadata      map[string]any      `json:"metadata,omitempty"`
}

// UnifiedMessage represents a message in unified format
type UnifiedMessage struct {
    Role       string           `json:"role"`
    Content    []MessageContent `json:"content"`
    ToolCalls  []ToolCall       `json:"tool_calls,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
}

// MessageContent represents content blocks (text, image, etc.)
type MessageContent struct {
    Type         string            `json:"type"` // text, image_url, thinking
    Text         string            `json:"text,omitempty"`
    CacheControl *CacheControl     `json:"cache_control,omitempty"`
    ImageURL     *ImageURL         `json:"image_url,omitempty"`
    MediaType    string            `json:"media_type,omitempty"`
    Thinking     *ThinkingContent  `json:"thinking,omitempty"`
}

// UnifiedTool represents a tool definition
type UnifiedTool struct {
    Type     string       `json:"type"` // function
    Function ToolFunction `json:"function"`
}

// ReasoningConfig for extended thinking
type ReasoningConfig struct {
    MaxTokens int    `json:"max_tokens,omitempty"`
    Effort    string `json:"effort,omitempty"` // none, low, medium, high
}

// UnifiedChatResponse represents the unified response format
type UnifiedChatResponse struct {
    ID         string             `json:"id"`
    Model      string             `json:"model"`
    Content    []MessageContent   `json:"content"`
    ToolCalls  []ToolCall         `json:"tool_calls,omitempty"`
    Usage      *Usage             `json:"usage,omitempty"`
    StopReason string             `json:"stop_reason,omitempty"`
}
```

## 3. Data Flow

### 3.1 Request Flow

```
1. Client sends Anthropic Request
       ↓
2. Handler detects route & selects transformer
       ↓
3. TransformRequestIn: Anthropic → Unified
   - Parse Anthropic format (messages, tools, system)
   - Convert to UnifiedChatRequest
   - Handle multimodal content (images, cache_control)
       ↓
4. TransformRequestOut: Unified → Provider
   - Convert Unified format to provider-specific format
   - Build HTTP request with proper headers
   - Handle provider-specific quirks (endpoint, auth)
       ↓
5. Send to provider API
```

### 3.2 Response Flow (Non-Streaming)

```
1. Provider returns response
       ↓
2. TransformResponseIn: Provider → Unified
   - Parse provider-specific format
   - Convert to UnifiedChatResponse
   - Handle tool calls, usage, stop_reason
       ↓
3. TransformResponseOut: Unified → Anthropic
   - Convert to Anthropic response format
   - Map fields correctly
       ↓
4. Return to client
```

### 3.3 Streaming Flow

```
1. Provider sends SSE events
       ↓
2. Parse SSE events from provider stream
       ↓
3. TransformSSEEvent: Provider Event → Anthropic Event
   - Convert event types (delta → content_block_delta)
   - Handle tool calls incrementally
   - Handle thinking/reasoning chunks
   - Generate synthetic events (message_start, content_block_stop)
       ↓
4. Apply streaming interceptors
       ↓
5. Validate and write to client
```

## 4. Provider Transformers

### 4.1 Transformers to Implement

| Transformer | Provider | Priority | Notes |
|-------------|----------|----------|-------|
| `AnthropicTransformer` | Anthropic | P0 | Pass-through, baseline |
| `OpenRouterTransformer` | OpenRouter | P0 | OpenAI-compatible with tool calls |
| `OpenAITransformer` | OpenAI | P0 | Standard OpenAI format |
| `GeminiTransformer` | Gemini | P1 | Google-specific format |
| `GLMTransformer` | Zhipu GLM | P1 | Anthropic-compatible |
| `QwenTransformer` | Alibaba Qwen | P1 | OpenAI-compatible |
| `MiniMaxTransformer` | MiniMax | P1 | Via OpenRouter |
| `DeepSeekTransformer` | DeepSeek | P2 | OpenAI-compatible |

### 4.2 Transformers vs. Interceptors

**Transformers** handle protocol conversion between Anthropic and provider formats only.

**Interceptors** handle utility/cross-cutting concerns via existing interceptor architecture:

| Interceptor Type | Purpose | Example Use |
|------------------|---------|-------------|
| `RequestInterceptor` | Modify requests before routing | Adjust `max_tokens` for provider limits |
| `RequestInterceptor` | Add/modify tool definitions | Inject provider-specific tools |
| `ResponseInterceptor` | Modify responses after provider | Handle thinking/reasoning content |
| `StreamingResponseInterceptor` | Modify SSE events in-flight | Filter or transform streaming events |

The interceptor pattern is preferred over "special transformers" because:
- Simpler architecture - no chaining complexity
- Consistent with current implementation
- Interceptors are already tested and working

## 5. Key Implementation Details

### 5.1 Streaming State Management

Each streaming transformer maintains state for:
- **Tool calls**: Track `id`, `name`, `arguments` accumulation across chunks
- **Message start**: Track if `message_start` has been emitted
- **Thinking content**: Accumulate reasoning chunks
- **Content blocks**: Track block indices for proper event ordering

```go
type StreamingState struct {
    MessageStarted   bool
    ToolCallStates   map[int]*ToolCallState
    ThinkingBuffer   strings.Builder
    ContentBlocks    []ContentBlockState
}

type ToolCallState struct {
    ID       string
    Name     string
    Args     strings.Builder
    Started  bool
    HasID    bool
    HasName  bool
}
```

### 5.2 SSE Event Transformation

Key event mappings:

| Provider Event | Anthropic Event | Notes |
|----------------|-----------------|-------|
| (none) | `message_start` | Synthetic, required by Anthropic |
| `delta.content` | `content_block_delta` | Type: `text_delta` |
| `delta.tool_calls` | `content_block_start` | Type: `tool_use` |
| `delta.tool_calls` | `content_block_delta` | Type: `input_json_delta` |
| `finish_reason` | `content_block_stop` | End of content |
| `finish_reason` | `message_delta` | With stop_reason, usage |
| `finish_reason` | `message_stop` | Final event |

### 5.3 Tool Call Handling

**Request (Anthropic → Provider):**
```go
// Anthropic format
Tools: []Tool{
    {Name: "search", InputSchema: {...}},
}

// Unified format
Tools: []UnifiedTool{
    {Type: "function", Function: {...}},
}

// OpenAI format
Tools: []ChatCompletionTool{
    {Type: "function", Function: {...}},
}
```

**Response (Provider → Anthropic):**
- OpenAI sends `delta.tool_calls[]` with incremental `id`, `function.name`, `function.arguments`
- Must emit: `content_block_start` (when we have id+name), then `content_block_delta` for each argument chunk

### 5.4 Thinking/Reasoning Support

Extended thinking requires special handling:

```go
type ThinkingContent struct {
    Content   string `json:"content"`
    Signature string `json:"signature,omitempty"`
}

// In streaming:
// 1. Accumulate reasoning chunks
// 2. On first regular content, emit complete thinking as single event
// 3. Then continue with regular content
```

### 5.5 Multimodal Content

```go
type MessageContent struct {
    Type string // "text", "image_url", "thinking"

    // For text
    Text string

    // For images
    ImageURL *ImageURL
    MediaType string // "image/jpeg", "image/png"

    // For caching
    CacheControl *CacheControl
}
```

## 6. Migration Plan

### Phase 1: Core Infrastructure (Branch: `transformer-refactor-core`)
1. Add `unified/` package with `UnifiedChatRequest`, `UnifiedChatResponse` types
2. Update `Transformer` interface with new methods
3. Create base `BaseTransformer` with common utilities
4. Add conversion utilities (`anthropic_to_unified.go`, `unified_to_openai.go`)

### Phase 2: OpenAI Family (Branch: `transformer-openai`)
1. Refactor `OpenRouterTransformer` with new interface
2. Create `OpenAITransformer` (extracted from OpenRouter)
3. Add comprehensive streaming tests
4. Fix tool call streaming issues

### Phase 3: Gemini (Branch: `transformer-gemini`)
1. Refactor `GeminiTransformer` with new interface
2. Add proper streaming support
3. Handle function calling format

### Phase 4: Chinese Providers (Branch: `transformer-chinese`)
1. Refactor `GLMTransformer`
2. Refactor `QwenTransformer`
3. Add `MiniMaxTransformer` (via OpenRouter)

### Phase 5: Utility Interceptors (Branch: `utility-interceptors`)
1. Add `MaxTokenInterceptor` - Adjust `max_tokens` for provider limits
2. Add `ReasoningInterceptor` - Handle thinking/reasoning content extraction/formatting
3. Add `ToolEnhanceInterceptor` - Add/modify tool definitions for provider compatibility

### Phase 6: Integration & Testing (Branch: `transformer-integration`)
1. Update `handler.go` to use new interface
2. Add comprehensive integration tests
3. Fix any edge cases discovered
4. Update documentation

## 7. File Structure

```
internal/transformer/
├── interface.go              # Transformer interface
├── registry.go               # Transformer registry
├── base.go                   # BaseTransformer with utilities
├── sse.go                    # SSE event types and utilities
│
├── unified/                  # Unified format types
│   ├── request.go
│   ├── response.go
│   ├── message.go
│   ├── tool.go
│   └── reasoning.go
│
├── converters/               # Format converters
│   ├── anthropic_to_unified.go
│   ├── unified_to_openai.go
│   ├── unified_to_gemini.go
│   └── sse_converter.go
│
├── providers/                # Provider transformers
│   ├── anthropic.go
│   ├── openai.go
│   ├── openrouter.go
│   ├── gemini.go
│   ├── glm.go
│   ├── qwen.go
│   └── minimax.go
│
└── test/                     # Tests
    ├── unified_test.go
    ├── openai_test.go
    ├── streaming_test.go
    └── integration_test.go
```

## 8. Testing Strategy

### 8.1 Unit Tests
- Test each conversion phase independently
- Mock HTTP responses for provider-specific formats
- Test SSE event parsing and transformation

### 8.2 Integration Tests
- Test full request/response cycle with real providers
- Test streaming with various scenarios (text, tools, thinking)
- Test error handling and edge cases

### 8.3 Test Scenarios
1. Simple text completion (non-streaming)
2. Simple text completion (streaming)
3. Tool use with no arguments
4. Tool use with complex arguments
5. Multiple tools in single request
6. Thinking/reasoning content
7. Image content
8. Cache control headers
9. Error responses
10. Truncated streams

## 9. Success Criteria

- [ ] All existing providers work with new interface
- [ ] Streaming works correctly for all providers
- [ ] Tool calls work in both request and response directions
- [ ] Thinking/reasoning content is properly handled
- [ ] Multimodal content (images) works correctly
- [ ] All tests pass
- [ ] No regressions in existing functionality
- [ ] Documentation updated

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing functionality | High | Comprehensive tests, gradual migration |
| Performance regression | Medium | Benchmark before/after, optimize hot paths |
| Provider API changes | Low | Abstract provider quirks, version transformers |
| SSE edge cases | Medium | Extensive streaming tests with real providers |

## References

- [claude-code-router transformer reference](https://github.com/musistudio/claude-code-router/tree/main/packages/core/src/transformer)
- Current implementation: `internal/transformer/`
- Anthropic Messages API: https://docs.anthropic.com/en/api/messages
- OpenAI Chat Completions API: https://platform.openai.com/docs/api-reference/chat
