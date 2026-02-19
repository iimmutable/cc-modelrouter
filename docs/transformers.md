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
└── glm.go              # GLM transformer
```
