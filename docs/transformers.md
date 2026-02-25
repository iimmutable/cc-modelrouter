# Transformers Guide

Transformers convert requests and responses between Anthropic format and provider-specific API formats.

## Transformer Interface

```go
type Transformer interface {
    // Name returns the transformer name (used for registry lookup)
    Name() string

    // TransformRequest converts an Anthropic request to a provider-specific HTTP request
    TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

    // TransformResponse converts a provider response to Anthropic format
    TransformResponse(resp *http.Response) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming
    SupportsStreaming() bool

    // TransformStreamChunk transforms a streaming chunk to Anthropic format
    TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
```

## Built-in Transformers

### AnthropicTransformer

Pass-through transformer for Anthropic-native APIs.

**Use case:** Direct Anthropic API or Anthropic-compatible endpoints.

**Request format:** Unchanged (Anthropic native)
**Auth:** `x-api-key` header

```go
// Request
POST /v1/messages
Headers:
  x-api-key: <api-key>
  anthropic-version: 2023-06-01
```

### OpenRouterTransformer

Converts to OpenAI chat completions format.

**Use case:** OpenRouter API

**Request format:** OpenAI-compatible
**Auth:** `Authorization: Bearer` header

```go
// Request
POST /chat/completions
Headers:
  Authorization: Bearer <api-key>
Body:
{
  "model": "anthropic/claude-sonnet-4",
  "messages": [{"role": "user", "content": "..."}],
  "max_tokens": 4096
}
```

### GeminiTransformer

Converts to Google Gemini API format.

**Use case:** Google Gemini API

**Request format:** Gemini native (`contents`/`parts`)
**Auth:** Query parameter `key=`

```go
// Request
POST /models/{model}:generateContent?key={api-key}
Body:
{
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "..."}]
    }
  ],
  "systemInstruction": {
    "parts": [{"text": "..."}]
  },
  "tools": [...]
}
```

**Key transformations:**
- `messages` → `contents` with `parts`
- `system` → `systemInstruction`
- `assistant` role → `model` role
- Tools → `functionDeclarations`
- Images → `inlineData`

### QwenTransformer

Converts to OpenAI-compatible format for Alibaba Qwen.

**Use case:** DashScope Qwen API

**Request format:** OpenAI-compatible
**Auth:** `Authorization: Bearer` header

```go
// Request
POST /chat/completions
Headers:
  Authorization: Bearer <api-key>
Body:
{
  "model": "qwen-turbo",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."}
  ],
  "max_tokens": 4096
}
```

**Key transformations:**
- System prompt → first message with `role: "system"`
- Tools → OpenAI function format
- Tool calls in response → `tool_use` content blocks

### GLMTransformer

Pass-through for Anthropic-compatible GLM API.

**Use case:** Zhipu BigModel GLM

**Request format:** Anthropic-compatible
**Auth:** `Authorization: Bearer` header

```go
// Request
POST /v1/messages
Headers:
  Authorization: Bearer <api-key>
Body: (Anthropic format)
```

## Transformer Registry

Transformers are registered at startup:

```go
registry := transformer.NewRegistry()
registry.Register(transformer.NewAnthropicTransformer())
registry.Register(transformer.NewOpenRouterTransformer())
registry.Register(transformer.NewGeminiTransformer())
registry.Register(transformer.NewQwenTransformer())
registry.Register(transformer.NewGLMTransformer())
```

**Lookup:**
```go
t, err := registry.Get("gemini")
if err != nil {
    // Transformer not found
}
```

## Creating a Custom Transformer

### 1. Implement the Interface

```go
package transformer

type MyTransformer struct{}

func NewMyTransformer() *MyTransformer {
    return &MyTransformer{}
}

func (t *MyTransformer) Name() string {
    return "myprovider"
}

func (t *MyTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
    // Convert Anthropic request to provider format
    providerReq := convertRequest(req, model)

    body, err := json.Marshal(providerReq)
    if err != nil {
        return nil, err
    }

    httpReq, err := http.NewRequest("POST", baseURL+"/endpoint", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }

    // Set appropriate headers
    httpReq.Header.Set("Authorization", "Bearer "+apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    return httpReq, nil
}

func (t *MyTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
    }

    var providerResp ProviderResponse
    if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
        return nil, err
    }

    return convertToAnthropic(&providerResp), nil
}

func (t *MyTransformer) SupportsStreaming() bool {
    return true // or false
}

func (t *MyTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
    // Convert streaming chunk format
    return chunk, nil // or transform
}
```

### 2. Register the Transformer

```go
// In internal/cli/start.go and internal/cli/code.go
registry.Register(transformer.NewMyTransformer())
```

## Format Conversion Patterns

### Message Content

**Anthropic:**
```json
{
  "content": [
    {"type": "text", "text": "Hello"},
    {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "..."}}
  ]
}
```

**OpenAI-compatible:**
```json
{
  "content": "Hello"
}
```

**Gemini:**
```json
{
  "parts": [
    {"text": "Hello"},
    {"inlineData": {"mimeType": "image/png", "data": "..."}}
  ]
}
```

### Tools

**Anthropic:**
```json
{
  "tools": [
    {
      "name": "get_weather",
      "description": "Get weather",
      "input_schema": {"type": "object", "properties": {...}}
    }
  ]
}
```

**OpenAI-compatible:**
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather",
        "parameters": {"type": "object", "properties": {...}}
      }
    }
  ]
}
```

**Gemini:**
```json
{
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_weather",
          "description": "Get weather",
          "parameters": {...}
        }
      ]
    }
  ]
}
```

### Tool Use in Response

**Anthropic:**
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_123",
      "name": "get_weather",
      "input": {"location": "SF"}
    }
  ]
}
```

**OpenAI-compatible:**
```json
{
  "choices": [{
    "message": {
      "tool_calls": [{
        "id": "call_123",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\": \"SF\"}"
        }
      }]
    }
  }]
}
```

**Gemini:**
```json
{
  "candidates": [{
    "content": {
      "parts": [{
        "functionCall": {
          "name": "get_weather",
          "args": {"location": "SF"}
        }
      }]
    }
  }]
}
```

## File Locations

```
internal/transformer/
├── interface.go        # Transformer interface
├── registry.go         # Transformer registry
├── anthropic.go        # Anthropic transformer
├── openrouter.go       # OpenRouter transformer
├── gemini.go           # Gemini transformer
├── qwen.go             # Qwen transformer
├── glm_anthropic.go    # GLM Anthropic transformer
└── transformers/       # Transformer implementations
    ├── anthropic.go
    ├── openrouter.go
    ├── glm_anthropic.go
    └── ...
```

---

## Thinking Block Handling (CRITICAL)

### Overview

Thinking blocks are a special content block type used by Anthropic's extended thinking feature. Different providers have different validation requirements for thinking blocks, making correct handling critical.

### Provider Signature Requirements

| Provider | Signature Requirement | Notes |
|----------|---------------------|-------|
| **Direct Anthropic API** | Omit when empty | Rejects whitespace-only signatures like `" "` |
| **OpenRouter Anthropic models** | Must be present | Requires field even if empty string |
| **GLM providers (Aliyun, BigModel)** | Must be present | Accepts empty string value |
| **Other OpenRouter models** | Must be present | Accepts empty or whitespace values |

### The Signature Field Problem

The `ContentBlock.Signature` field has conflicting requirements:
- **Omit** the field for direct Anthropic API (or get 400 error)
- **Include** the field for OpenRouter Anthropic models (or get 400 error)

With a plain `string` type, we cannot distinguish between:
1. "Omit this field from JSON"
2. "Include this field with empty string value"

**Current Implementation (Bug):**
```go
type ContentBlock struct {
    Signature string `json:"signature,omitempty"`
}
```

When `Signature = ""`, the `omitempty` tag omits the field, causing OpenRouter to see it as "undefined".

**Required Fix:**
```go
type ContentBlock struct {
    Signature *string `json:"signature,omitempty"`
}
```

This allows:
- `nil` → omit field (for Anthropic API)
- `&""` → include empty string (for OpenRouter)
- `&"value"` → include actual value

### Content Normalization

**User Messages with Thinking Blocks:**
- Claude Code may resend previous assistant responses as user messages
- These can contain thinking blocks that need conversion
- **Solution:** Convert thinking blocks to text blocks wrapped in `<thinking>` tags

**Assistant Messages with Single Thinking Blocks:**
- Some providers reject single-element arrays for content
- **Solution:** Add a minimal text block with single space to make it multi-element

**Failover State Corruption:**
- Transformers must deep copy requests before modification
- Otherwise, modifications affect subsequent provider attempts
- **Solution:** JSON marshal/unmarshal to create independent copies

### Transformer-Specific Handling

#### AnthropicTransformer
```go
// Strip whitespace-only signatures (omit field)
if strings.TrimSpace(signature) == "" {
    signature = ""  // Will be omitted by MarshalJSON
}
// Do NOT normalize single thinking blocks (Anthropic accepts them)
```

#### OpenRouterTransformer

**CRITICAL: OpenRouter requires different handling than direct Anthropic API**

```go
// CRITICAL: Always normalize thinking blocks for OpenRouter
// Both Anthropic and non-Anthropic models via OpenRouter require multi-element arrays
// to prevent "expected string, received array" validation errors
normalizeThinkingBlockMessages(&reqCopy)

// Handle signatures based on target model
// OpenRouter Anthropic models require signature field to be PRESENT (not omitted)
// Existing signatures from previous provider responses MUST be cleared
if isAnthropicModel {
    // Always clear signature for OpenRouter Anthropic models
    // Setting to empty string ensures the field is present (required by OpenRouter)
    // Clears any existing signature from previous provider responses (GLM, OpenAI, etc.)
    if signature == nil || isWhitespacePtr(signature) {
        signature = strPtr("")  // Include empty string
    }
} else {
    // Non-Anthropic models: ensure signature field is present
    if signature == nil || isWhitespacePtr(signature) {
        signature = strPtr("")  // Include empty string
    }
}
```

**Key Differences from Direct Anthropic API:**

1. **Content Normalization:**
   - Direct Anthropic API: Accepts single thinking blocks without normalization
   - OpenRouter (all models): **Requires normalization** - single thinking blocks cause validation errors

2. **Signature Handling:**
   - Direct Anthropic API: Omit signature field when empty (set to `nil`)
   - OpenRouter Anthropic: Include signature field even when empty (set to `&""`)

3. **Failover Scenarios:**
   - When GLM or other providers return thinking blocks in assistant messages
   - These messages are sent to OpenRouter during failover
   - OpenRouter's strict validation rejects the single-element array format
   - Normalization adds a text block, creating a valid multi-element array

#### GLMAnthropicTransformer
```go
// Always include signature field
if signature == nil {
    signature = strPtr("")  // Include empty string
}
// Normalize thinking blocks
normalizeThinkingBlockMessages(&reqCopy)
```

### Testing Thinking Block Handling

```go
// Test case: OpenRouter Anthropic model with thinking block
req := &anthropic.Request{
    Messages: []anthropic.Message{
        {
            Role: "assistant",
            Content: anthropic.MessageContent{
                {Type: "thinking", Thinking: "..."},
            },
        },
    },
}

// After transformer processing:
// - Signature should be &"" (include empty string)
// - Content should remain single-element array (no normalization)
// - JSON should include "signature": ""
```

---

## Common Transformer Pitfalls

### 1. Modifying Requests In-Place

**Wrong:**
```go
func (t *MyTransformer) PrepareRequest(req *anthropic.Request, ...) {
    req.Messages[0].Content = append(req.Messages[0].Content, newBlock)
    // This modifies the original request!
}
```

**Right:**
```go
func (t *MyTransformer) PrepareRequest(req *anthropic.Request, ...) {
    var reqCopy anthropic.Request
    json.Unmarshal(json.Marshal(req), &reqCopy)
    reqCopy.Messages[0].Content = append(...)
    // Work with copy
}
```

### 2. Not Handling Provider-Specific Validation

**Wrong:**
```go
// Treat all providers the same
if block.Type == "thinking" {
    block.Signature = ""  // Will be omitted - breaks OpenRouter!
}
```

**Right:**
```go
// Handle provider-specific requirements
if isAnthropicModel {
    block.Signature = nil  // Omit for Anthropic API
} else {
    block.Signature = strPtr("")  // Include for others
}
```

### 3. Forgetting to Set GetBody for HTTP Requests

**Wrong:**
```go
httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
// Body can only be read once - retries will fail!
```

**Right:**
```go
bodyCopy := make([]byte, len(body))
copy(bodyCopy, body)
httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
httpReq.GetBody = func() (io.ReadCloser, error) {
    return io.NopCloser(bytes.NewReader(bodyCopy)), nil
}
```
