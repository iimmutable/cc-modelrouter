# Transformer Re-architecture Implementation Plan

**Date:** 2026-02-26
**Status:** Ready for Execution
**Based on Design:** `2026-02-26-transformer-refactoring-design.md`

---

## Overview

This plan implements the transformer re-architecture using a **Unified Intermediate Format** that separates protocol conversion (Transformers) from cross-cutting concerns (Interceptors). The plan is divided into 6 phases with bite-sized tasks following TDD principles.

**Key Principles:**
- TDD: Write tests before implementation code
- DRY: Extract reusable conversion utilities
- YAGNI: Build only what's needed, avoid premature abstraction
- Atomic commits: One logical change per commit

---

## Phase 1: Core Infrastructure
**Branch:** `transformer-refactor-core`
**Estimated tasks:** 16 bite-sized steps

### 1.1 Create Unified Types Package
**Tasks:** 5

**Task 1.1.1:** Create `internal/transformer/unified/message.go`
```go
package unified

import "encoding/json"

// Role represents the role of a message sender.
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

// UnifiedMessage represents a message in unified format.
type UnifiedMessage struct {
    Role       string           `json:"role"`
    Content    []MessageContent `json:"content"`
    ToolCalls  []ToolCall       `json:"tool_calls,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
}

// MessageContent represents content blocks.
type MessageContent struct {
    Type         string            `json:"type"`
    Text         string            `json:"text,omitempty"`
    CacheControl *CacheControl     `json:"cache_control,omitempty"`
    ImageURL     *ImageURL         `json:"image_url,omitempty"`
    MediaType    string            `json:"media_type,omitempty"`
    Thinking     *ThinkingContent  `json:"thinking,omitempty"`
}

// CacheControl for caching prompts.
type CacheControl struct {
    Type string `json:"type"`
}

// ImageURL represents an image URL.
type ImageURL struct {
    URL string `json:"url"`
}

// ThinkingContent for extended thinking.
type ThinkingContent struct {
    Content   string `json:"content"`
    Signature string `json:"signature,omitempty"`
}
```

**Task 1.1.2:** Create `internal/transformer/unified/tool.go`
```go
package unified

// ToolCall represents a tool call in a response.
type ToolCall struct {
    ID       string          `json:"id"`
    Name     string          `json:"name"`
    Input    json.RawMessage `json:"input"`
}

// UnifiedTool represents a tool definition.
type UnifiedTool struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

// ToolFunction defines a tool's function.
type ToolFunction struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Parameters  any    `json:"parameters"`
}
```

**Task 1.1.3:** Create `internal/transformer/unified/request.go`
```go
package unified

// UnifiedChatRequest represents the unified request format.
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

// ReasoningConfig for extended thinking.
type ReasoningConfig struct {
    MaxTokens int    `json:"max_tokens,omitempty"`
    Effort    string `json:"effort,omitempty"`
}
```

**Task 1.1.4:** Create `internal/transformer/unified/response.go`
```go
package unified

// UnifiedChatResponse represents the unified response format.
type UnifiedChatResponse struct {
    ID         string           `json:"id"`
    Model      string           `json:"model"`
    Content    []MessageContent `json:"content"`
    ToolCalls  []ToolCall       `json:"tool_calls,omitempty"`
    Usage      *Usage           `json:"usage,omitempty"`
    StopReason string           `json:"stop_reason,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

**Task 1.1.5:** Create `internal/transformer/unified/reasoning.go`
```go
package unified

// ReasoningConfig for extended thinking.
type ReasoningConfig struct {
    MaxTokens int    `json:"max_tokens,omitempty"`
    Effort    string `json:"effort,omitempty"` // none, low, medium, high
}
```

**Task 1.1.6:** Write tests for unified types in `internal/transformer/unified/message_test.go`
- Test JSON marshaling/unmarshaling
- Test content block types
- Test tool calls
- Test thinking content

---

### 1.2 Create Converter Utilities
**Tasks:** 4

**Task 1.2.1:** Create `internal/transformer/converters/anthropic_to_unified.go`

```go
package converters

import (
    "github.com/iimmutable/cc-modelrouter/internal/transformer/unified"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// AnthropicToUnified converts Anthropic Request to UnifiedChatRequest.
func AnthropicToUnified(req *anthropic.Request) (*unified.UnifiedChatRequest, error) {
    unifiedReq := &unified.UnifiedChatRequest{
        Model:       req.Model,
        MaxTokens:   req.MaxTokens,
        Stream:      req.Stream,
        Temperature: 0, // Default, can be added if needed
    }

    // Convert messages
    for _, msg := range req.Messages {
        unifiedMsg := unified.UnifiedMessage{
            Role:    string(msg.Role),
            Content: convertMessageContent(msg.Content),
        }
        unifiedReq.Messages = append(unifiedReq.Messages, unifiedMsg)
    }

    // Convert tools
    for _, tool := range req.Tools {
        unifiedTool := unified.UnifiedTool{
            Type: "function",
            Function: unified.ToolFunction{
                Name:        tool.Name,
                Description: tool.Description,
                Parameters:  tool.InputSchema,
            },
        }
        unifiedReq.Tools = append(unifiedReq.Tools, unifiedTool)
    }

    // Convert tool_choice
    if req.ToolChoice != nil {
        unifiedReq.ToolChoice = req.ToolChoice
    }

    // Convert thinking config to reasoning config
    if req.Thinking != nil {
        unifiedReq.Reasoning = &unified.ReasoningConfig{
            MaxTokens: req.Thinking.BudgetTokens,
            Effort:    req.Thinking.Type,
        }
    }

    return unifiedReq, nil
}

func convertMessageContent(anthropicContent anthropic.MessageContent) []unified.MessageContent {
    var result []unified.MessageContent

    for _, block := range anthropicContent {
        switch block.Type {
        case "text":
            result = append(result, unified.MessageContent{
                Type: "text",
                Text: block.Text,
            })
        case "image":
            if block.Source != nil {
                result = append(result, unified.MessageContent{
                    Type:      "image_url",
                    MediaType: block.Source.MediaType,
                    ImageURL: &unified.ImageURL{
                        URL: "data:" + block.Source.MediaType + ";base64," + block.Source.Data,
                    },
                })
            }
        }
    }

    return result
}
```

**Task 1.2.2:** Create `internal/transformer/converters/unified_to_anthropic.go`

```go
package converters

import (
    "encoding/json"

    "github.com/iimmutable/cc-modelrouter/internal/transformer/unified"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// UnifiedToAnthropic converts UnifiedChatResponse to Anthropic Response.
func UnifiedToAnthropic(unified *unified.UnifiedChatResponse) (*anthropic.Response, error) {
    result := &anthropic.Response{
        ID:         unified.ID,
        Type:       "message",
        Role:       anthropic.RoleAssistant,
        Model:      unified.Model,
        StopReason: unified.StopReason,
    }

    // Convert content blocks
    for _, content := range unified.Content {
        block := anthropic.ContentBlock{Type: content.Type}

        switch content.Type {
        case "text":
            block.Text = content.Text
        case "tool_use":
            if content.Thinking != nil {
                block.Content = content.Thinking.Content
            }
        }

        result.Content = append(result.Content, block)
    }

    // Convert tool calls
    for _, toolCall := range unified.ToolCalls {
        block := anthropic.ContentBlock{
            Type: "tool_use",
            ID:   toolCall.ID,
            Name: toolCall.Name,
            Input: toolCall.Input,
        }
        result.Content = append(result.Content, block)
    }

    // Convert usage
    if unified.Usage != nil {
        result.Usage = anthropic.Usage{
            InputTokens:  unified.Usage.InputTokens,
            OutputTokens: unified.Usage.OutputTokens,
        }
    }

    return result, nil
}

// UnifiedRequestToAnthropic converts UnifiedChatRequest to Anthropic Request.
func UnifiedRequestToAnthropic(unified *unified.UnifiedChatRequest) (*anthropic.Request, error) {
    result := &anthropic.Request{
        Model:      unified.Model,
        MaxTokens:  unified.MaxTokens,
        Stream:     unified.Stream,
        ToolChoice: unified.ToolChoice,
    }

    // Convert messages
    for _, msg := range unified.Messages {
        anthropicMsg := anthropic.Message{
            Role:    anthropic.Role(msg.Role),
            Content: convertToAnthropicContent(msg.Content),
        }
        result.Messages = append(result.Messages, anthropicMsg)
    }

    // Convert tools
    for _, tool := range unified.Tools {
        anthropicTool := anthropic.Tool{
            Name:        tool.Function.Name,
            Description: tool.Function.Description,
            InputSchema: tool.Function.Parameters,
        }
        result.Tools = append(result.Tools, anthropicTool)
    }

    return result, nil
}

func convertToAnthropicContent(unifiedContent []unified.MessageContent) anthropic.MessageContent {
    var result anthropic.MessageContent

    for _, content := range unifiedContent {
        block := anthropic.ContentBlock{Type: content.Type}

        switch content.Type {
        case "text":
            block.Text = content.Text
        }

        result = append(result, block)
    }

    return result
}
```

**Task 1.2.3:** Create `internal/transformer/converters/unified_to_openai.go`

```go
package converters

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/iimmutable/cc-modelrouter/internal/transformer/unified"
)

// OpenAIRequest represents the OpenAI chat completion format.
type OpenAIRequest struct {
    Model      string         `json:"model"`
    Messages   []OpenAIMessage `json:"messages"`
    MaxTokens  int            `json:"max_tokens,omitempty"`
    Stream     bool           `json:"stream,omitempty"`
    Tools      []OpenAITool   `json:"tools,omitempty"`
    ToolChoice any            `json:"tool_choice,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format.
type OpenAIMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// OpenAITool represents a tool in OpenAI format.
type OpenAITool struct {
    Type     string `json:"type"`
    Function struct {
        Name        string `json:"name"`
        Description string `json:"description,omitempty"`
        Parameters  any    `json:"parameters"`
    } `json:"function"`
}

// UnifiedToOpenAIRequest converts UnifiedChatRequest to OpenAI HTTP request.
func UnifiedToOpenAIRequest(unified *unified.UnifiedChatRequest, baseURL, apiKey, model string) (*http.Request, error) {
    openaiReq := convertToOpenAI(unified, model)

    body, err := json.Marshal(openaiReq)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    endpoint := baseURL
    if !strings.HasSuffix(baseURL, "/chat/completions") {
        endpoint = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
    }

    httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+apiKey)

    return httpReq, nil
}

func convertToOpenAI(unified *unified.UnifiedChatRequest, model string) *OpenAIRequest {
    openaiReq := &OpenAIRequest{
        Model:     model,
        MaxTokens: unified.MaxTokens,
        Stream:    unified.Stream,
    }

    // Convert messages
    for _, msg := range unified.Messages {
        content := extractTextContent(msg.Content)
        openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
            Role:    msg.Role,
            Content: content,
        })
    }

    // Convert tools
    for _, tool := range unified.Tools {
        openaiTool := OpenAITool{Type: "function"}
        openaiTool.Function.Name = tool.Function.Name
        openaiTool.Function.Description = tool.Function.Description
        openaiTool.Function.Parameters = tool.Function.Parameters
        openaiReq.Tools = append(openaiReq.Tools, openaiTool)
    }

    return openaiReq
}

func extractTextContent(content []unified.MessageContent) string {
    var texts []string
    for _, block := range content {
        if block.Type == "text" {
            texts = append(texts, block.Text)
        }
    }
    return strings.Join(texts, "\n")
}

// OpenAIResponse represents OpenAI response format.
type OpenAIResponse struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Index   int `json:"index"`
        Message struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"message"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
}

// OpenAIToUnified converts OpenAI response to Unified format.
func OpenAIToUnified(resp *OpenAIResponse) (*unified.UnifiedChatResponse, error) {
    if len(resp.Choices) == 0 {
        return &unified.UnifiedChatResponse{}, nil
    }

    choice := resp.Choices[0]
    result := &unified.UnifiedChatResponse{
        ID:         resp.ID,
        Model:      resp.Model,
        StopReason: mapOpenAIFinishReason(choice.FinishReason),
        Content: []unified.MessageContent{
            {Type: "text", Text: choice.Message.Content},
        },
        Usage: &unified.Usage{
            InputTokens:  resp.Usage.PromptTokens,
            OutputTokens: resp.Usage.CompletionTokens,
        },
    }

    return result, nil
}

func mapOpenAIFinishReason(reason string) string {
    switch reason {
    case "stop":
        return "end_turn"
    case "length":
        return "max_tokens"
    case "tool_calls":
        return "tool_use"
    case "content_filter":
        return "stop_sequence"
    default:
        return "end_turn"
    }
}
```

**Task 1.2.4:** Write tests for converters in `internal/transformer/converters/converters_test.go`
- Test Anthropic to Unified conversion
- Test Unified to Anthropic conversion
- Test Unified to OpenAI conversion
- Test OpenAI to Unified conversion

---

### 1.3 Update Transformer Interface
**Tasks:** 2

**Task 1.3.1:** Update `internal/transformer/interface.go`

```go
// Package transformer defines the transformer interface for request/response transformation.
package transformer

import (
    "net/http"

    "github.com/iimmutable/cc-modelrouter/internal/transformer/unified"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// SSEEvent represents a complete server-sent event with type and data.
type SSEEvent struct {
    EventType string
    Data      []byte
}

// Provider represents a provider configuration.
type Provider struct {
    BaseURL string
    APIKey  string
    Model   string
}

// Transformer transforms requests between Anthropic and provider formats.
type Transformer interface {
    // Name returns the transformer name.
    Name() string

    // Endpoint returns the API endpoint path.
    Endpoint() string

    // Auth prepares authentication headers for the request.
    Auth(req *unified.UnifiedChatRequest, provider Provider) (map[string]string, error)

    // TransformRequestIn converts Anthropic format to Unified format.
    TransformRequestIn(req *anthropic.Request) (*unified.UnifiedChatRequest, error)

    // TransformRequestOut converts Unified format to provider-specific HTTP request.
    TransformRequestOut(unified *unified.UnifiedChatRequest, baseURL, apiKey, model string) (*http.Request, error)

    // TransformResponseIn converts provider response to Unified format.
    TransformResponseIn(resp *http.Response) (*unified.UnifiedChatResponse, error)

    // TransformResponseOut converts Unified format to Anthropic format.
    TransformResponseOut(unified *unified.UnifiedChatResponse) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming.
    SupportsStreaming() bool

    // TransformSSEEvent transforms a provider SSE event to Anthropic format.
    TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error)

    // TransformStreamChunk transforms a streaming chunk to Anthropic format.
    // Deprecated: Use TransformSSEEvent for proper SSE event handling.
    TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
```

**Task 1.3.2:** Create `internal/transformer/base.go` with common utilities

```go
package transformer

import (
    "encoding/json"
    "fmt"
)

// BaseTransformer provides common utilities for transformers.
type BaseTransformer struct {
    name string
}

// NewBaseTransformer creates a new base transformer.
func NewBaseTransformer(name string) *BaseTransformer {
    return &BaseTransformer{name: name}
}

// Name returns the transformer name.
func (b *BaseTransformer) Name() string {
    return b.name
}

// MarshalSSEEvent creates an SSEEvent from the provided data.
func (b *BaseTransformer) MarshalSSEEvent(eventType string, data any) (SSEEvent, error) {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return SSEEvent{}, fmt.Errorf("failed to marshal %s event: %w", eventType, err)
    }
    if len(jsonData) == 0 {
        return SSEEvent{}, fmt.Errorf("%s event marshaled to empty JSON", eventType)
    }
    return SSEEvent{EventType: eventType, Data: jsonData}, nil
}
```

**Task 1.3.3:** Write tests for base utilities

---

### 1.4 Update Existing Transformers
**Tasks:** 5

**Task 1.4.1:** Refactor `internal/transformer/anthropic.go` to use new interface

**Task 1.4.2:** Refactor `internal/transformer/openrouter.go` to use new interface

**Task 1.4.3:** Create `internal/transformer/providers/openai.go` (extracted from OpenRouter)

**Task 1.4.4:** Move existing transformer files to `providers/` subdirectory

**Task 1.4.5:** Update imports and package references across all transformers

---

**Phase 1 Completion Criteria:**
- [ ] All unified types defined and tested
- [ ] All converter utilities implemented and tested
- [ ] Transformer interface updated with new methods
- [ ] BaseTransformer with common utilities
- [ ] Anthropic transformer using new interface
- [ ] All Phase 1 tests passing
- [ ] Go modules updated

---

## Phase 2: OpenAI Family
**Branch:** `transformer-openai` (merge from `transformer-refactor-core`)
**Estimated tasks:** 12 bite-sized steps

### 2.1 OpenAI Transformer
**Tasks:** 6

**Task 2.1.1:** Create `internal/transformer/providers/openai.go` with complete implementation

```go
package providers

import (
    "io"
    "net/http"

    "github.com/iimmutable/cc-modelrouter/internal/transformer"
    "github.com/iimmutable/cc-modelrouter/internal/transformer/converters"
    "github.com/iimmutable/cc-modelrouter/internal/transformer/unified"
    "github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// OpenAITransformer transforms requests to OpenAI API format.
type OpenAITransformer struct {
    *transformer.BaseTransformer
    toolCallStates  map[int]*toolCallState
    messageStarted  bool
}

type toolCallState struct {
    id        string
    name      string
    arguments strings.Builder
    started   bool
    hasID     bool
    hasName   bool
}

// NewOpenAITransformer creates a new OpenAI transformer.
func NewOpenAITransformer() *OpenAITransformer {
    return &OpenAITransformer{
        BaseTransformer: transformer.NewBaseTransformer("openai"),
        toolCallStates:  make(map[int]*toolCallState),
    }
}

// Endpoint returns the API endpoint path.
func (t *OpenAITransformer) Endpoint() string {
    return "/v1/chat/completions"
}

// Auth prepares authentication headers.
func (t *OpenAITransformer) Auth(req *unified.UnifiedChatRequest, provider transformer.Provider) (map[string]string, error) {
    return map[string]string{
        "Content-Type":  "application/json",
        "Authorization": "Bearer " + provider.APIKey,
    }, nil
}

// TransformRequestIn converts Anthropic to Unified format.
func (t *OpenAITransformer) TransformRequestIn(req *anthropic.Request) (*unified.UnifiedChatRequest, error) {
    return converters.AnthropicToUnified(req)
}

// TransformRequestOut converts Unified to OpenAI HTTP request.
func (t *OpenAITransformer) TransformRequestOut(unified *unified.UnifiedChatRequest, baseURL, apiKey, model string) (*http.Request, error) {
    return converters.UnifiedToOpenAIRequest(unified, baseURL, apiKey, model)
}

// TransformResponseIn converts OpenAI response to Unified format.
func (t *OpenAITransformer) TransformResponseIn(resp *http.Response) (*unified.UnifiedChatResponse, error) {
    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
    }

    var openaiResp converters.OpenAIResponse
    if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return converters.OpenAIToUnified(&openaiResp)
}

// TransformResponseOut converts Unified to Anthropic format.
func (t *OpenAITransformer) TransformResponseOut(unified *unified.UnifiedChatResponse) (*anthropic.Response, error) {
    return converters.UnifiedToAnthropic(unified)
}

// SupportsStreaming returns true.
func (t *OpenAITransformer) SupportsStreaming() bool {
    return true
}

// TransformSSEEvent transforms OpenAI SSE events to Anthropic format.
// (Full implementation similar to existing OpenRouter)
func (t *OpenAITransformer) TransformSSEEvent(event *SSEEvent) ([]SSEEvent, error) {
    // ... (reusing OpenRouter's SSE transformation logic)
}

// TransformStreamChunk deprecated method.
func (t *OpenAITransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
    // ... (reusing OpenRouter's deprecated logic)
}
```

**Task 2.1.2:** Implement OpenAI SSE event transformation

**Task 2.1.3:** Implement OpenAI tool call state management

**Task 2.1.4:** Write unit tests for OpenAI transformer

**Task 2.1.5:** Write streaming tests for OpenAI transformer

**Task 2.1.6:** Fix any test failures

---

### 2.2 OpenRouter Transformer
**Tasks:** 6

**Task 2.2.1:** Refactor `internal/transformer/providers/openrouter.go` to use new interface

**Task 2.2.2:** Update OpenRouter to use converter utilities

**Task 2.2.3:** Verify OpenRouter SSE event transformation

**Task 2.2.4:** Write unit tests for OpenRouter transformer

**Task 2.2.5:** Write streaming tests for OpenRouter transformer

**Task 2.2.6:** Fix tool call streaming issues if any

---

**Phase 2 Completion Criteria:**
- [ ] OpenAI transformer fully implemented
- [ ] OpenRouter transformer refactored
- [ ] All OpenAI family tests passing
- [ ] Streaming works correctly for text and tools
- [ ] No regressions from existing functionality

---

## Phase 3: Gemini
**Branch:** `transformer-gemini` (merge from `transformer-openai`)
**Estimated tasks:** 8 bite-sized steps

### 3.1 Gemini Transformer
**Tasks:** 8

**Task 3.1.1:** Create `internal/transformer/converters/unified_to_gemini.go`

```go
package converters

import (
    // ... (Gemini-specific conversion logic)
)
```

**Task 3.1.2:** Create `internal/transformer/converters/gemini_to_unified.go`

**Task 3.1.3:** Refactor `internal/transformer/providers/gemini.go` to use new interface

**Task 3.1.4:** Implement Gemini streaming support

**Task 3.1.5:** Handle Gemini function calling format

**Task 3.1.6:** Write unit tests for Gemini transformer

**Task 3.1.7:** Write streaming tests for Gemini transformer

**Task 3.1.8:** Fix any test failures

---

**Phase 3 Completion Criteria:**
- [ ] Gemini transformer fully implemented
- [ ] All Gemini tests passing
- [ ] Streaming works correctly
- [ ] Function calling works correctly

---

## Phase 4: Chinese Providers
**Branch:** `transformer-chinese` (merge from `transformer-gemini`)
**Estimated tasks:** 12 bite-sized steps

### 4.1 GLM Transformer
**Tasks:** 4

**Task 4.1.1:** Refactor `internal/transformer/providers/glm.go` to use new interface

**Task 4.1.2:** GLM is Anthropic-compatible - minimal changes needed

**Task 4.1.3:** Write tests for GLM transformer

**Task 4.1.4:** Verify GLM passes through correctly

---

### 4.2 Qwen Transformer
**Tasks:** 4

**Task 4.2.1:** Refactor `internal/transformer/providers/qwen.go` to use new interface

**Task 4.2.2:** Qwen is OpenAI-compatible - use converter utilities

**Task 4.2.3:** Write tests for Qwen transformer

**Task 4.2.4:** Verify Qwen works correctly

---

### 4.3 MiniMax Transformer
**Tasks:** 4

**Task 4.3.1:** Create `internal/transformer/providers/minimax.go`

**Task 4.3.2:** MiniMax via OpenRouter - use OpenAI format

**Task 4.3.3:** Write tests for MiniMax transformer

**Task 4.3.4:** Verify MiniMax works correctly

---

**Phase 4 Completion Criteria:**
- [ ] All Chinese providers refactored
- [ ] All Chinese provider tests passing
- [ ] No regressions for Chinese providers

---

## Phase 5: Utility Interceptors
**Branch:** `utility-interceptors` (merge from `transformer-chinese`)
**Estimated tasks:** 6 bite-sized steps

### 5.1 Interceptor Implementations
**Tasks:** 6

**Task 5.1.1:** Create `internal/interceptor/max_token.go`
- Adjust max_tokens based on provider limits
- Prevent overflow errors

**Task 5.1.2:** Create `internal/interceptor/reasoning.go`
- Handle thinking/reasoning content extraction
- Format reasoning for Claude Code

**Task 5.1.3:** Create `internal/interceptor/tool_enhance.go`
- Add/modify tool definitions for provider compatibility
- Handle provider-specific tool quirks

**Task 5.1.4:** Write tests for MaxTokenInterceptor

**Task 5.1.5:** Write tests for ReasoningInterceptor

**Task 5.1.6:** Write tests for ToolEnhanceInterceptor

---

**Phase 5 Completion Criteria:**
- [ ] All utility interceptors implemented
- [ ] All interceptor tests passing
- [ ] Interceptors integrate properly with handlers

---

## Phase 6: Integration & Testing
**Branch:** `transformer-integration` (merge from `utility-interceptors`)
**Estimated tasks:** 10 bite-sized steps

### 6.1 Handler Update
**Tasks:** 4

**Task 6.1.1:** Update `internal/proxy/handler.go` to use new transformer interface

**Task 6.1.2:** Update request routing to call TransformRequestIn/TransformRequestOut

**Task 6.1.3:** Update response handling to call TransformResponseIn/TransformResponseOut

**Task 6.1.4:** Update streaming handling to use new SSE event transformation

---

### 6.2 Integration Tests
**Tasks:** 6

**Task 6.2.1:** Create `internal/transformer/test/integration_test.go`

```go
// Test full request/response cycle for each provider
// Test streaming with various scenarios
// Test error handling
```

**Task 6.2.2:** Write integration test for simple text completion (non-streaming)

**Task 6.2.3:** Write integration test for simple text completion (streaming)

**Task 6.2.4:** Write integration test for tool use with no arguments

**Task 6.2.5:** Write integration test for tool use with complex arguments

**Task 6.2.6:** Write integration test for thinking/reasoning content

---

### 6.3 Documentation & Cleanup
**Tasks:** 4

**Task 6.3.1:** Update README.md with new architecture

**Task 6.3.2:** Add inline documentation to all new types

**Task 6.3.3:** Remove deprecated TransformStreamChunk methods

**Task 6.3.4:** Final code review and cleanup

---

**Phase 6 Completion Criteria:**
- [ ] Handler fully updated
- [ ] All integration tests passing
- [ ] All scenarios tested
- [ ] Documentation updated
- [ ] No regressions

---

## Overall Success Criteria

- [ ] All existing providers work with new interface
- [ ] Streaming works correctly for all providers
- [ ] Tool calls work in both request and response directions
- [ ] Thinking/reasoning content is properly handled
- [ ] Multimodal content (images) works correctly
- [ ] All tests pass
- [ ] No regressions in existing functionality
- [ ] Documentation updated

---

## Commit Strategy

**Atomic Commits:**
- Each task should result in 1-2 commits
- Commit messages follow conventional commits format:
  - `feat(transformer): add unified message types`
  - `test(transformer): add converter tests`
  - `fix(transformer): resolve tool call streaming issue`

**Frequent Commits:**
- Commit after completing each bite-sized task
- Don't let work pile up before committing

---

## Rollback Plan

If issues arise:
1. Revert to last stable phase branch
2. Document the issue
3. Fix in new branch
4. Re-merge after verification

---

## Execution Notes

**Task Granularity:** Each task is designed to take 2-5 minutes.

**TDD Approach:**
1. Write the test first
2. Run test (it fails)
3. Implement minimum code to pass test
4. Run test (it passes)
5. Refactor if needed
6. Commit

**When to Ask:**
- If design assumptions are unclear
- If encountering unexpected behavior
- If a task seems more complex than described