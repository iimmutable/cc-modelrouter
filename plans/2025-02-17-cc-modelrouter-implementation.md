# cc-modelrouter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go-based HTTP proxy that routes Claude Code requests to multiple LLM providers with transformer support and sequential failover.

**Architecture:** Layered architecture with clean separation: CLI → HTTP Server → Router Engine → Transformer → Provider Client. Configuration supports global and project-scoped files with complete override.

**Tech Stack:** Go 1.21+, net/http, encoding/json, sync package for concurrency

---

## Task 1: Project Setup and Module Initialization

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `.gitignore` (update)

**Step 1: Initialize Go module**

Run: `go mod init github.com/iimmutable/cc-modelrouter`
Expected: `go: creating new go.mod: module github.com/iimmutable/cc-modelrouter`

**Step 2: Create directory structure**

Run:
```bash
mkdir -p cmd/ccrouter internal/cli internal/daemon internal/proxy internal/config internal/router internal/transformer internal/provider internal/models pkg/api/anthropic
```
Expected: All directories created successfully

**Step 3: Update .gitignore**

Add to `.gitignore`:
```
# Binaries
ccrouter
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary
*.test

# Output
/bin/

# Go workspace
go.work

# IDE
.idea/
.vscode/

# OS
.DS_Store

# Instance files
~/.cc-modelrouter/instances/
```

**Step 4: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: initialize Go module and project structure"
```

---

## Task 2: Anthropic API Types

**Files:**
- Create: `pkg/api/anthropic/types.go`
- Create: `pkg/api/anthropic/types_test.go`

**Step 1: Write the failing test**

Create `pkg/api/anthropic/types_test.go`:

```go
package anthropic

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshaling(t *testing.T) {
	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.Model != req.Model {
		t.Errorf("expected model %s, got %s", req.Model, unmarshaled.Model)
	}
}

func TestContentBlockTypes(t *testing.T) {
	textBlock := ContentBlock{
		Type: "text",
		Text: "Hello",
	}

	imageBlock := ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "base64data",
		},
	}

	toolUseBlock := ContentBlock{
		Type:  "tool_use",
		ID:    "toolu_123",
		Name:  "get_weather",
		Input: json.RawMessage(`{"location": "SF"}`),
	}

	textData, _ := json.Marshal(textBlock)
	imageData, _ := json.Marshal(imageBlock)
	toolData, _ := json.Marshal(toolUseBlock)

	if string(textData) == "" {
		t.Error("text block should marshal to non-empty JSON")
	}
	if string(imageData) == "" {
		t.Error("image block should marshal to non-empty JSON")
	}
	if string(toolData) == "" {
		t.Error("tool_use block should marshal to non-empty JSON")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api/anthropic/... -v`
Expected: FAIL - undefined: Request, Message, ContentBlock, ImageSource

**Step 3: Write implementation**

Create `pkg/api/anthropic/types.go`:

```go
// Package anthropic defines the API types for Anthropic's Messages API.
// These types are used for request/response handling in the proxy.
package anthropic

import "encoding/json"

// Role represents the role of a message sender.
type Role string

const (
	RoleUser    Role = "user"
	RoleAssistant Role = "assistant"
)

// Request represents an Anthropic Messages API request.
type Request struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []Message       `json:"messages"`
	System    json.RawMessage `json:"system,omitempty"`
	Tools     []Tool          `json:"tools,omitempty"`
	ToolChoice any            `json:"tool_choice,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    Role          `json:"role"`
	Content MessageContent `json:"content"`
}

// MessageContent can be either a string or array of ContentBlocks.
type MessageContent []ContentBlock

// MarshalJSON implements custom marshaling for MessageContent.
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	// If single text block, marshal as string
	if len(mc) == 1 && mc[0].Type == "text" && mc[0].Text != "" {
		return json.Marshal(mc[0].Text)
	}
	// Otherwise marshal as array
	return json.Marshal([]ContentBlock(mc))
}

// UnmarshalJSON implements custom unmarshaling for MessageContent.
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// Try as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*mc = MessageContent{{Type: "text", Text: str}}
		return nil
	}

	// Try as array
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	*mc = blocks
	return nil
}

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text,omitempty"`
	Source  *ImageSource    `json:"source,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Content string          `json:"content,omitempty"`
}

// ImageSource represents the source of an image.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// Response represents an Anthropic Messages API response.
type Response struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage represents token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents a streaming event from the API.
type StreamEvent struct {
	Type         string          `json:"type"`
	Message      *Response       `json:"message,omitempty"`
	Index        int             `json:"index,omitempty"`
	ContentBlock *ContentBlock   `json:"content_block,omitempty"`
	Delta        *StreamDelta    `json:"delta,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
}

// StreamDelta represents a delta in a streaming response.
type StreamDelta struct {
	Type        string          `json:"type,omitempty"`
	Text        string          `json:"text,omitempty"`
	StopReason  string          `json:"stop_reason,omitempty"`
	ToolUseID   string          `json:"tool_use_id,omitempty"`
	PartialJSON json.RawMessage `json:"partial_json,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api/anthropic/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/api/anthropic/
git commit -m "feat: add Anthropic API types"
```

---

## Task 3: Configuration Types and Loader

**Files:**
- Create: `internal/config/types.go`
- Create: `internal/config/types_test.go`
- Create: `internal/config/loader.go`
- Create: `internal/config/loader_test.go`

**Step 1: Write the failing tests**

Create `internal/config/types_test.go`:

```go
package config

import (
	"testing"
)

func TestProviderConfigValidation(t *testing.T) {
	pc := &ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
		Models:  []string{"model-1", "model-2"},
	}

	if err := pc.Validate(); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}

	pc.APIKey = ""
	if err := pc.Validate(); err == nil {
		t.Error("missing API key should fail validation")
	}
}

func TestRouteParsing(t *testing.T) {
	route := "bigmodel:glm-4.7;openrouter:anthropic/claude-sonnet-4.5"
	targets := ParseRoute(route)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Provider != "bigmodel" || targets[0].Model != "glm-4.7" {
		t.Errorf("unexpected first target: %+v", targets[0])
	}

	if targets[1].Provider != "openrouter" || targets[1].Model != "anthropic/claude-sonnet-4.5" {
		t.Errorf("unexpected second target: %+v", targets[1])
	}
}
```

Create `internal/config/loader_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"server": {
			"port": 8081,
			"host": "localhost"
		},
		"providers": {
			"test": {
				"apiKey": "test-key",
				"baseURL": "https://api.test.com",
				"models": ["model-1"]
			}
		},
		"router": {
			"routes": {
				"default": "test:model-1"
			}
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 8081 {
		t.Errorf("expected port 8081, got %d", cfg.Server.Port)
	}

	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
}

func TestEnvVarInterpolation(t *testing.T) {
	os.Setenv("TEST_API_KEY", "secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	input := "${TEST_API_KEY}"
	result := interpolateEnvVars(input)

	if result != "secret-key" {
		t.Errorf("expected 'secret-key', got '%s'", result)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/... -v`
Expected: FAIL - undefined types and functions

**Step 3: Write config types**

Create `internal/config/types.go`:

```go
// Package config handles configuration loading and management.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the complete configuration.
type Config struct {
	Server    ServerConfig             `json:"server"`
	Providers map[string]ProviderConfig `json:"providers"`
	Router    RouterConfig             `json:"router"`
}

// ServerConfig represents server configuration.
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// ProviderConfig represents a provider configuration.
type ProviderConfig struct {
	APIKey      string   `json:"apiKey"`
	BaseURL     string   `json:"baseURL"`
	Models      []string `json:"models"`
	Transformer string   `json:"transformer,omitempty"`
}

// Validate validates the provider configuration.
func (pc *ProviderConfig) Validate() error {
	if pc.APIKey == "" {
		return fmt.Errorf("apiKey is required")
	}
	if pc.BaseURL == "" {
		return fmt.Errorf("baseURL is required")
	}
	if len(pc.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	return nil
}

// RouterConfig represents router configuration.
type RouterConfig struct {
	Routes     map[string]string `json:"routes"`
	MaxRetries int               `json:"maxRetries"`
	RetryDelay string            `json:"retryDelay"`
}

// GetRetryDelay returns the retry delay as a duration.
func (rc *RouterConfig) GetRetryDelay() time.Duration {
	d, err := time.ParseDuration(rc.RetryDelay)
	if err != nil {
		return 500 * time.Millisecond
	}
	return d
}

// RouteTarget represents a parsed route target.
type RouteTarget struct {
	Provider string
	Model    string
}

// ParseRoute parses a route string into targets.
// Format: "provider1:model1;provider2:model2"
func ParseRoute(route string) []RouteTarget {
	var targets []RouteTarget
	parts := strings.Split(route, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		providerModel := strings.SplitN(part, ":", 2)
		if len(providerModel) != 2 {
			continue
		}

		targets = append(targets, RouteTarget{
			Provider: strings.TrimSpace(providerModel[0]),
			Model:    strings.TrimSpace(providerModel[1]),
		})
	}

	return targets
}

// Defaults returns the default configuration.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8081,
			Host: "localhost",
		},
		Providers: make(map[string]ProviderConfig),
		Router: RouterConfig{
			Routes:     make(map[string]string),
			MaxRetries: 2,
			RetryDelay: "500ms",
		},
	}
}
```

**Step 4: Write config loader**

Create `internal/config/loader.go`:

```go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobalConfigPath returns the global config file path.
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-modelrouter", "config.json")
}

// ProjectConfigPath returns the project config file path.
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".cc-modelrouter", "config.json")
}

// Load loads configuration from a file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Interpolate environment variables
	expanded := interpolateEnvVars(string(data))

	cfg := Defaults()
	if err := json.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// LoadWithOverride loads project config if exists, otherwise global.
func LoadWithOverride(projectRoot string) (*Config, string, error) {
	projectPath := ProjectConfigPath(projectRoot)
	if _, err := os.Stat(projectPath); err == nil {
		cfg, err := Load(projectPath)
		if err != nil {
			return nil, "", err
		}
		return cfg, "project", nil
	}

	globalPath := GlobalConfigPath()
	if _, err := os.Stat(globalPath); err == nil {
		cfg, err := Load(globalPath)
		if err != nil {
			return nil, "", err
		}
		return cfg, "global", nil
	}

	return nil, "", fmt.Errorf("no configuration found")
}

// Save saves configuration to a file.
func Save(cfg *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// interpolateEnvVars replaces ${VAR} and $VAR with environment variable values.
func interpolateEnvVars(s string) string {
	result := s

	// Replace ${VAR} patterns
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		varName := result[start+2 : end]
		varValue := os.Getenv(varName)
		result = result[:start] + varValue + result[end+1:]
	}

	// Replace $VAR patterns (word boundary)
	words := strings.Fields(result)
	for _, word := range words {
		if strings.HasPrefix(word, "$") && !strings.Contains(word, "{") {
			varName := word[1:]
			// Handle punctuation at end
			varName = strings.TrimRight(varName, ".,;:!?")
			varValue := os.Getenv(varName)
			if varValue != "" {
				result = strings.ReplaceAll(result, word, varValue)
			}
		}
	}

	return result
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat: add configuration types and loader"
```

---

## Task 4: Transformer Interface and Registry

**Files:**
- Create: `internal/transformer/interface.go`
- Create: `internal/transformer/registry.go`
- Create: `internal/transformer/registry_test.go`

**Step 1: Write the failing test**

Create `internal/transformer/registry_test.go`:

```go
package transformer

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Register a mock transformer
	mock := &mockTransformer{name: "test"}
	reg.Register(mock)

	// Retrieve it
	got, err := reg.Get("test")
	if err != nil {
		t.Fatalf("failed to get transformer: %v", err)
	}

	if got.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", got.Name())
	}

	// Test not found
	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent transformer")
	}
}

// mockTransformer for testing
type mockTransformer struct {
	name string
}

func (m *mockTransformer) Name() string { return m.name }
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/transformer/... -v`
Expected: FAIL - undefined: Registry, NewRegistry

**Step 3: Write transformer interface**

Create `internal/transformer/interface.go`:

```go
// Package transformer defines the transformer interface for request/response transformation.
package transformer

import (
	"net/http"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// Transformer transforms requests and responses between Anthropic format and provider format.
type Transformer interface {
	// Name returns the transformer name.
	Name() string

	// TransformRequest converts an Anthropic request to a provider-specific HTTP request.
	TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

	// TransformResponse converts a provider response to Anthropic format.
	TransformResponse(resp *http.Response) (*anthropic.Response, error)

	// SupportsStreaming returns true if this transformer supports streaming.
	SupportsStreaming() bool

	// TransformStreamChunk transforms a streaming chunk to Anthropic format.
	TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
```

**Step 4: Write transformer registry**

Create `internal/transformer/registry.go`:

```go
package transformer

import (
	"fmt"
	"sync"
)

// Registry manages transformer instances.
type Registry struct {
	mu           sync.RWMutex
	transformers map[string]Transformer
}

// NewRegistry creates a new transformer registry.
func NewRegistry() *Registry {
	return &Registry{
		transformers: make(map[string]Transformer),
	}
}

// Register adds a transformer to the registry.
func (r *Registry) Register(t Transformer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transformers[t.Name()] = t
}

// Get retrieves a transformer by name.
func (r *Registry) Get(name string) (Transformer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.transformers[name]
	if !ok {
		return nil, fmt.Errorf("transformer not found: %s", name)
	}
	return t, nil
}

// Has checks if a transformer exists.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.transformers[name]
	return ok
}

// Names returns all registered transformer names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.transformers))
	for name := range r.transformers {
		names = append(names, name)
	}
	return names
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/transformer/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/transformer/
git commit -m "feat: add transformer interface and registry"
```

---

## Task 5: Anthropic Pass-Through Transformer

**Files:**
- Create: `internal/transformer/anthropic.go`
- Create: `internal/transformer/anthropic_test.go`

**Step 1: Write the failing test**

Create `internal/transformer/anthropic_test.go`:

```go
package transformer

import (
	"testing"

	"github.com/musistudio/cc-modelouter/pkg/api/anthropic"
)

func TestAnthropicTransformerName(t *testing.T) {
	tr := NewAnthropicTransformer()
	if tr.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got '%s'", tr.Name())
	}
}

func TestAnthropicTransformRequest(t *testing.T) {
	tr := NewAnthropicTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://api.anthropic.com", "test-key", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	if httpReq.Header.Get("x-api-key") != "test-key" {
		t.Error("expected x-api-key header to be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/transformer/... -v`
Expected: FAIL - undefined: NewAnthropicTransformer

**Step 3: Write implementation**

Create `internal/transformer/anthropic.go`:

```go
package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// AnthropicTransformer is a pass-through transformer for Anthropic-compatible APIs.
type AnthropicTransformer struct{}

// NewAnthropicTransformer creates a new Anthropic transformer.
func NewAnthropicTransformer() *AnthropicTransformer {
	return &AnthropicTransformer{}
}

// Name returns the transformer name.
func (t *AnthropicTransformer) Name() string {
	return "anthropic"
}

// TransformRequest creates an HTTP request for the Anthropic API.
func (t *AnthropicTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	// Copy request and override model
	reqCopy := *req
	reqCopy.Model = model

	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	return httpReq, nil
}

// TransformResponse converts the HTTP response to Anthropic format.
func (t *AnthropicTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result anthropic.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SupportsStreaming returns true.
func (t *AnthropicTransformer) SupportsStreaming() bool {
	return true
}

// TransformStreamChunk passes through the chunk unchanged.
func (t *AnthropicTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return chunk, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/transformer/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/transformer/anthropic.go internal/transformer/anthropic_test.go
git commit -m "feat: add Anthropic pass-through transformer"
```

---

## Task 6: OpenRouter Transformer

**Files:**
- Create: `internal/transformer/openrouter.go`
- Create: `internal/transformer/openrouter_test.go`

**Step 1: Write the failing test**

Create `internal/transformer/openrouter_test.go`:

```go
package transformer

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestOpenRouterTransformerName(t *testing.T) {
	tr := NewOpenRouterTransformer()
	if tr.Name() != "openrouter" {
		t.Errorf("expected name 'openrouter', got '%s'", tr.Name())
	}
}

func TestOpenRouterTransformRequest(t *testing.T) {
	tr := NewOpenRouterTransformer()

	req := &anthropic.Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	httpReq, err := tr.TransformRequest(req, "https://openrouter.ai/api/v1", "test-key", "anthropic/claude-sonnet-4.5")
	if err != nil {
		t.Fatalf("failed to transform request: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST method, got %s", httpReq.Method)
	}

	if httpReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("expected Authorization header to be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/transformer/... -v`
Expected: FAIL - undefined: NewOpenRouterTransformer

**Step 3: Write implementation**

Create `internal/transformer/openrouter.go`:

```go
package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// OpenRouterTransformer transforms requests to OpenRouter API format.
type OpenRouterTransformer struct{}

// NewOpenRouterTransformer creates a new OpenRouter transformer.
func NewOpenRouterTransformer() *OpenRouterTransformer {
	return &OpenRouterTransformer{}
}

// Name returns the transformer name.
func (t *OpenRouterTransformer) Name() string {
	return "openrouter"
}

// OpenRouterRequest represents the OpenRouter chat completion format.
type OpenRouterRequest struct {
	Model       string           `json:"model"`
	Messages    []OpenRouterMsg  `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	Tools       []OpenRouterTool `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
}

// OpenRouterMsg represents a message in OpenRouter format.
type OpenRouterMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenRouterTool represents a tool in OpenRouter format.
type OpenRouterTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// TransformRequest creates an HTTP request for the OpenRouter API.
func (t *OpenRouterTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	orReq := t.convertRequest(req, model)

	body, err := json.Marshal(orReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// OpenRouter uses /chat/completions endpoint
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
	httpReq.Header.Set("HTTP-Referer", "https://github.com/iimmutable/cc-modelrouter")

	return httpReq, nil
}

// convertRequest converts Anthropic request to OpenRouter format.
func (t *OpenRouterTransformer) convertRequest(req *anthropic.Request, model string) *OpenRouterRequest {
	orReq := &OpenRouterRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	// Convert messages
	for _, msg := range req.Messages {
		content := t.extractTextContent(msg.Content)
		orReq.Messages = append(orReq.Messages, OpenRouterMsg{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	// Convert tools
	for _, tool := range req.Tools {
		orTool := OpenRouterTool{
			Type: "function",
		}
		orTool.Function.Name = tool.Name
		orTool.Function.Description = tool.Description
		orTool.Function.Parameters = tool.InputSchema
		orReq.Tools = append(orReq.Tools, orTool)
	}

	return orReq
}

// extractTextContent extracts text from message content.
func (t *OpenRouterTransformer) extractTextContent(content []anthropic.ContentBlock) string {
	var texts []string
	for _, block := range content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// OpenRouterResponse represents OpenRouter response format.
type OpenRouterResponse struct {
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

// TransformResponse converts OpenRouter response to Anthropic format.
func (t *OpenRouterTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var orResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return t.convertResponse(&orResp), nil
}

// convertResponse converts OpenRouter response to Anthropic format.
func (t *OpenRouterTransformer) convertResponse(orResp *OpenRouterResponse) *anthropic.Response {
	result := &anthropic.Response{
		ID:    orResp.ID,
		Type:  "message",
		Role:  anthropic.RoleAssistant,
		Model: orResp.Model,
		Usage: anthropic.Usage{
			InputTokens:  orResp.Usage.PromptTokens,
			OutputTokens: orResp.Usage.CompletionTokens,
		},
	}

	for _, choice := range orResp.Choices {
		result.Content = append(result.Content, anthropic.ContentBlock{
			Type: "text",
			Text: choice.Message.Content,
		})
		result.StopReason = choice.FinishReason
	}

	return result
}

// SupportsStreaming returns true.
func (t *OpenRouterTransformer) SupportsStreaming() bool {
	return true
}

// TransformStreamChunk transforms OpenRouter SSE to Anthropic format.
func (t *OpenRouterTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	// OpenRouter uses OpenAI-style streaming
	// For now, pass through - full implementation would convert format
	return chunk, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/transformer/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/transformer/openrouter.go internal/transformer/openrouter_test.go
git commit -m "feat: add OpenRouter transformer"
```

---

## Task 7: Provider Client Layer

**Files:**
- Create: `internal/provider/client.go`
- Create: `internal/provider/http.go`
- Create: `internal/provider/client_test.go`

**Step 1: Write the failing test**

Create `internal/provider/client_test.go`:

```go
package provider

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		Timeout: "30s",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/... -v`
Expected: FAIL - undefined: ClientConfig, NewClient

**Step 3: Write client interface**

Create `internal/provider/client.go`:

```go
// Package provider handles communication with LLM providers.
package provider

import (
	"net/http"
	"time"
)

// ClientConfig represents client configuration.
type ClientConfig struct {
	BaseURL           string
	APIKey            string
	Timeout           string
	MaxIdleConns      int
	IdleConnTimeout   string
	MaxRetries        int
	RetryDelay        time.Duration
}

// Client is the interface for provider clients.
type Client interface {
	// Do executes an HTTP request.
	Do(req *http.Request) (*http.Response, error)
}

// Defaults returns default client configuration.
func Defaults() *ClientConfig {
	return &ClientConfig{
		Timeout:         "30s",
		MaxIdleConns:    100,
		IdleConnTimeout: "90s",
		MaxRetries:      2,
		RetryDelay:      500 * time.Millisecond,
	}
}
```

**Step 4: Write HTTP client implementation**

Create `internal/provider/http.go`:

```go
package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HTTPClient wraps http.Client with provider-specific configuration.
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewClient creates a new provider client.
func NewClient(cfg *ClientConfig) (*HTTPClient, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	defaults := Defaults()
	if cfg.Timeout == "" {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = defaults.MaxIdleConns
	}
	if cfg.IdleConnTimeout == "" {
		cfg.IdleConnTimeout = defaults.IdleConnTimeout
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = defaults.RetryDelay
	}

	timeout, _ := time.ParseDuration(cfg.Timeout)
	idleTimeout, _ := time.ParseDuration(cfg.IdleConnTimeout)

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     idleTimeout,
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
	}, nil
}

// Do executes an HTTP request with retry logic.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(c.retryDelay):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 5xx errors
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// DoWithContext executes an HTTP request with context.
func (c *HTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.Do(req.WithContext(ctx))
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/provider/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/provider/
git commit -m "feat: add provider client layer"
```

---

## Task 8: Router Engine

**Files:**
- Create: `internal/router/engine.go`
- Create: `internal/router/engine_test.go`

**Step 1: Write the failing test**

Create `internal/router/engine_test.go`:

```go
package router

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
)

func TestRouteDetection(t *testing.T) {
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":      "bigmodel:glm-4.7;openrouter:claude-sonnet-4.5",
				"background":   "bigmodel:glm-4.5-air",
				"longContext":  "bigmodel:glm-4.7;openrouter:gemini-2.5-pro",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "default route",
			req:      RouteRequest{},
			expected: "default",
		},
		{
			name:     "background route",
			req:      RouteRequest{IsBackground: true},
			expected: "background",
		},
		{
			name:     "long context route",
			req:      RouteRequest{TokenCount: 70000},
			expected: "longContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/router/... -v`
Expected: FAIL - undefined: Engine, NewEngine, RouteRequest

**Step 3: Write implementation**

Create `internal/router/engine.go`:

```go
// Package router handles request routing and model selection.
package router

import (
	"github.com/iimmutable/cc-modelrouter/internal/config"
)

const (
	LongContextThreshold = 60000
)

// RouteRequest contains information for route detection.
type RouteRequest struct {
	IsBackground bool
	IsThink      bool
	TokenCount   int
	HasWebSearch bool
	HasImages    bool
}

// Engine handles route detection and target selection.
type Engine struct {
	config *config.Config
}

// NewEngine creates a new router engine.
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{config: cfg}
}

// DetectRoute determines which route to use based on request characteristics.
func (e *Engine) DetectRoute(req RouteRequest) string {
	// Priority order for route detection
	switch {
	case req.IsBackground:
		if route, ok := e.config.Router.Routes["background"]; ok && route != "" {
			return "background"
		}
	case req.IsThink:
		if route, ok := e.config.Router.Routes["think"]; ok && route != "" {
			return "think"
		}
	case req.HasImages:
		if route, ok := e.config.Router.Routes["image"]; ok && route != "" {
			return "image"
		}
	case req.HasWebSearch:
		if route, ok := e.config.Router.Routes["webSearch"]; ok && route != "" {
			return "webSearch"
		}
	case req.TokenCount > LongContextThreshold:
		if route, ok := e.config.Router.Routes["longContext"]; ok && route != "" {
			return "longContext"
		}
	}

	return "default"
}

// GetTargets returns the route targets for a given route name.
func (e *Engine) GetTargets(routeName string) []config.RouteTarget {
	route, ok := e.config.Router.Routes[routeName]
	if !ok {
		route = e.config.Router.Routes["default"]
	}
	return config.ParseRoute(route)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/router/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/router/
git commit -m "feat: add router engine with route detection"
```

---

## Task 9: Failover Logic

**Files:**
- Create: `internal/router/failover.go`
- Create: `internal/router/failover_test.go`

**Step 1: Write the failing test**

Create `internal/router/failover_test.go`:

```go
package router

import (
	"testing"
)

func TestFailoverMaxAttempts(t *testing.T) {
	targets := []mockTarget{
		{provider: "p1", model: "m1"},
		{provider: "p2", model: "m2"},
	}

	fo := NewFailover(targets)

	// Max attempts should be 2x the number of targets
	expected := len(targets) * 2
	if fo.MaxAttempts() != expected {
		t.Errorf("expected max attempts %d, got %d", expected, fo.MaxAttempts())
	}
}

func TestFailoverIteration(t *testing.T) {
	targets := []mockTarget{
		{provider: "p1", model: "m1"},
		{provider: "p2", model: "m2"},
	}

	fo := NewFailover(targets)

	// First iteration
	t1 := fo.Next()
	if t1.Provider() != "p1" {
		t.Errorf("expected p1, got %s", t1.Provider())
	}

	// Second
	t2 := fo.Next()
	if t2.Provider() != "p2" {
		t.Errorf("expected p2, got %s", t2.Provider())
	}

	// Loop back to first
	t3 := fo.Next()
	if t3.Provider() != "p1" {
		t.Errorf("expected p1 (loop), got %s", t3.Provider())
	}
}

type mockTarget struct {
	provider string
	model    string
}

func (m mockTarget) Provider() string { return m.provider }
func (m mockTarget) Model() string    { return m.model }
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/router/... -v`
Expected: FAIL - undefined: Failover, NewFailover

**Step 3: Write implementation**

Create `internal/router/failover.go`:

```go
package router

import (
	"github.com/iimmutable/cc-modelrouter/internal/config"
)

// Target represents a route target.
type Target interface {
	Provider() string
	Model() string
}

// routeTarget wraps config.RouteTarget to implement Target interface.
type routeTarget struct {
	config.RouteTarget
}

func (r routeTarget) Provider() string { return r.RouteTarget.Provider }
func (r routeTarget) Model() string    { return r.RouteTarget.Model }

// Failover manages sequential failover with looping.
type Failover struct {
	targets     []Target
	current     int
	attempts    int
	maxAttempts int
}

// NewFailover creates a new failover manager.
func NewFailover(targets []config.RouteTarget) *Failover {
	t := make([]Target, len(targets))
	for i, rt := range targets {
		t[i] = routeTarget{rt}
	}

	return &Failover{
		targets:     t,
		current:     0,
		attempts:    0,
		maxAttempts: len(targets) * 2, // 2x loop
	}
}

// Next returns the next target in the sequence.
// Returns nil if max attempts reached.
func (f *Failover) Next() Target {
	if f.attempts >= f.maxAttempts {
		return nil
	}

	target := f.targets[f.current]
	f.current = (f.current + 1) % len(f.targets)
	f.attempts++

	return target
}

// HasMore returns true if there are more attempts available.
func (f *Failover) HasMore() bool {
	return f.attempts < f.maxAttempts
}

// MaxAttempts returns the maximum number of attempts.
func (f *Failover) MaxAttempts() int {
	return f.maxAttempts
}

// Attempts returns the current attempt count.
func (f *Failover) Attempts() int {
	return f.attempts
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/router/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/router/failover.go internal/router/failover_test.go
git commit -m "feat: add failover logic with looping"
```

---

## Task 10: HTTP Proxy Server

**Files:**
- Create: `internal/proxy/server.go`
- Create: `internal/proxy/handler.go`
- Create: `internal/proxy/server_test.go`

**Step 1: Write the failing test**

Create `internal/proxy/server_test.go`:

```go
package proxy

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	cfg := &ServerConfig{
		Host: "localhost",
		Port: 8081,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Error("expected non-nil server")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/... -v`
Expected: FAIL - undefined: ServerConfig, NewServer

**Step 3: Write server implementation**

Create `internal/proxy/server.go`:

```go
// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ServerConfig represents server configuration.
type ServerConfig struct {
	Host          string
	Port          int
	MaxRequestSize int64
}

// Defaults returns default server configuration.
func Defaults() *ServerConfig {
	return &ServerConfig{
		Host:           "localhost",
		Port:           8081,
		MaxRequestSize: 50 * 1024 * 1024, // 50MB
	}
}

// Server is the HTTP proxy server.
type Server struct {
	config  *ServerConfig
	server  *http.Server
	handler *Handler
	mu      sync.Mutex
	running bool
}

// NewServer creates a new proxy server.
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg == nil {
		cfg = Defaults()
	}
	if cfg.MaxRequestSize == 0 {
		cfg.MaxRequestSize = Defaults().MaxRequestSize
	}

	handler := NewHandler(cfg.MaxRequestSize)

	return &Server{
		config:  cfg,
		handler: handler,
	}, nil
}

// SetRouter sets the router for the handler.
func (s *Server) SetRouter(router Router) {
	s.handler.SetRouter(router)
}

// SetTransformerRegistry sets the transformer registry.
func (s *Server) SetTransformerRegistry(reg TransformerRegistry) {
	s.handler.SetTransformerRegistry(reg)
}

// SetProviderClients sets the provider clients.
func (s *Server) SetProviderClients(clients map[string]HTTPClient) {
	s.handler.SetProviderClients(clients)
}

// Start starts the server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Long for streaming
	}

	s.running = true

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop stops the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	err := s.server.Shutdown(ctx)
	s.running = false
	return err
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
```

**Step 4: Write handler implementation**

Create `internal/proxy/handler.go`:

```go
package proxy

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// Router interface for handler dependency.
type Router interface {
	DetectRoute(req RouteRequest) string
	GetTargets(routeName string) []config.RouteTarget
}

// TransformerRegistry interface for handler dependency.
type TransformerRegistry interface {
	Get(name string) (Transformer, error)
}

// Transformer interface for handler dependency.
type Transformer interface {
	Name() string
	TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
	TransformResponse(resp *http.Response) (*anthropic.Response, error)
	SupportsStreaming() bool
	TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}

// HTTPClient interface for handler dependency.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RouteRequest for route detection.
type RouteRequest struct {
	IsBackground bool
	IsThink      bool
	TokenCount   int
	HasWebSearch bool
	HasImages    bool
}

// Handler handles HTTP requests.
type Handler struct {
	maxRequestSize      int64
	router              Router
	transformerRegistry TransformerRegistry
	providerClients     map[string]HTTPClient
	config              *config.Config
}

// NewHandler creates a new handler.
func NewHandler(maxRequestSize int64) *Handler {
	return &Handler{
		maxRequestSize:  maxRequestSize,
		providerClients: make(map[string]HTTPClient),
	}
}

// SetRouter sets the router.
func (h *Handler) SetRouter(router Router) {
	h.router = router
}

// SetTransformerRegistry sets the transformer registry.
func (h *Handler) SetTransformerRegistry(reg TransformerRegistry) {
	h.transformerRegistry = reg
}

// SetProviderClients sets the provider clients.
func (h *Handler) SetProviderClients(clients map[string]HTTPClient) {
	h.providerClients = clients
}

// SetConfig sets the configuration.
func (h *Handler) SetConfig(cfg *config.Config) {
	h.config = cfg
}

// ServeHTTP handles HTTP requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle POST to /v1/messages
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Read and parse request
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxRequestSize))
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	var req anthropic.Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Handle the request
	h.handleMessages(w, r, &req)
}

// handleMessages processes the messages request.
func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request, req *anthropic.Request) {
	// Detect route
	routeReq := RouteRequest{
		TokenCount:   h.estimateTokens(req),
		HasWebSearch: h.hasWebSearch(req),
		HasImages:    h.hasImages(req),
	}
	routeName := h.router.DetectRoute(routeReq)
	targets := h.router.GetTargets(routeName)

	// Try each target with failover
	for _, target := range targets {
		resp, err := h.tryTarget(r.Context(), req, target)
		if err != nil {
			continue
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	http.Error(w, "All providers failed", http.StatusBadGateway)
}

func (h *Handler) tryTarget(ctx context.Context, req *anthropic.Request, target config.RouteTarget) (*anthropic.Response, error) {
	// Get provider config
	providerCfg, ok := h.config.Providers[target.Provider]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", target.Provider)
	}

	// Get transformer
	transformerName := providerCfg.Transformer
	if transformerName == "" {
		transformerName = target.Provider
	}
	transformer, err := h.transformerRegistry.Get(transformerName)
	if err != nil {
		transformer, _ = h.transformerRegistry.Get("anthropic")
	}

	// Get client
	client, ok := h.providerClients[target.Provider]
	if !ok {
		return nil, fmt.Errorf("client not found: %s", target.Provider)
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(req, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
	if err != nil {
		return nil, err
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Transform response
	return transformer.TransformResponse(resp)
}

func (h *Handler) estimateTokens(req *anthropic.Request) int {
	// Rough estimation: ~4 chars per token
	total := 0
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				total += len(block.Text) / 4
			}
		}
	}
	return total
}

func (h *Handler) hasWebSearch(req *anthropic.Request) bool {
	for _, tool := range req.Tools {
		if strings.Contains(strings.ToLower(tool.Name), "web") ||
			strings.Contains(strings.ToLower(tool.Name), "search") {
			return true
		}
	}
	return false
}

func (h *Handler) hasImages(req *anthropic.Request) bool {
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "image" {
				return true
			}
		}
	}
	return false
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/proxy/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/proxy/
git commit -m "feat: add HTTP proxy server and handler"
```

---

## Task 11: Streaming Handler

**Files:**
- Create: `internal/proxy/streaming.go`
- Create: `internal/proxy/streaming_test.go`

**Step 1: Write the failing test**

Create `internal/proxy/streaming_test.go`:

```go
package proxy

import (
	"testing"
)

func TestSSEWriter(t *testing.T) {
	writer := NewSSEWriter(nil) // nil for testing

	if writer == nil {
		t.Error("expected non-nil SSE writer")
	}
}

func TestParseSSEEvent(t *testing.T) {
	line := "event: content_block_delta\ndata: {\"type\":\"text\",\"text\":\"Hello\"}"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("failed to parse SSE event: %v", err)
	}

	if event != "content_block_delta" {
		t.Errorf("expected event 'content_block_delta', got '%s'", event)
	}

	if string(data) == "" {
		t.Error("expected non-empty data")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/... -v`
Expected: FAIL - undefined: NewSSEWriter, ParseSSEEvent

**Step 3: Write implementation**

Create `internal/proxy/streaming.go`:

```go
package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEWriter handles Server-Sent Events writing.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	flusher, ok := w.(http.Flusher)
	if !ok {
		flusher = nil
	}
	return &SSEWriter{w: w, flusher: flusher}
}

// WriteEvent writes an SSE event.
func (s *SSEWriter) WriteEvent(event string, data []byte) error {
	if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// Flush flushes the response.
func (s *SSEWriter) Flush() {
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// ParseSSEEvent parses an SSE line into event type and data.
func ParseSSEEvent(line string) (event string, data []byte, err error) {
	if strings.HasPrefix(line, "event:") {
		event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
	} else if strings.HasPrefix(line, "data:") {
		data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
	}
	return event, data, nil
}

// SSEScanner scans SSE events from a reader.
type SSEScanner struct {
	scanner *bufio.Scanner
	event   string
	data    []byte
	err     error
}

// NewSSEScanner creates a new SSE scanner.
func NewSSEScanner(r io.Reader) *SSEScanner {
	return &SSEScanner{
		scanner: bufio.NewScanner(r),
	}
}

// Scan advances to the next event.
func (s *SSEScanner) Scan() bool {
	s.event = ""
	s.data = nil

	var eventData strings.Builder

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if line == "" {
			// Empty line marks end of event
			if eventData.Len() > 0 {
				s.data = []byte(eventData.String())
				return true
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			s.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	s.err = s.scanner.Err()
	return false
}

// Event returns the current event type.
func (s *SSEScanner) Event() string {
	return s.event
}

// Data returns the current event data.
func (s *SSEScanner) Data() []byte {
	return s.data
}

// Err returns any error encountered.
func (s *SSEScanner) Err() error {
	return s.err
}

// HandleStreaming handles a streaming request.
func (h *Handler) handleStreaming(w http.ResponseWriter, r *http.Request, req *anthropic.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sseWriter := NewSSEWriter(w)

	// Write message_start event
	startEvent := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":    generateMessageID(),
			"type":  "message",
			"role":  "assistant",
			"model": req.Model,
		},
	}
	startData, _ := json.Marshal(startEvent)
	sseWriter.WriteEvent("message_start", startData)

	// Detect route and try targets
	routeReq := RouteRequest{
		TokenCount:   h.estimateTokens(req),
		HasWebSearch: h.hasWebSearch(req),
		HasImages:    h.hasImages(req),
	}
	routeName := h.router.DetectRoute(routeReq)
	targets := h.router.GetTargets(routeName)

	for _, target := range targets {
		if err := h.streamFromTarget(r.Context(), req, target, sseWriter); err == nil {
			return
		}
	}

	// All failed
	errorEvent := map[string]any{
		"type":  "error",
		"error": map[string]string{"message": "All providers failed"},
	}
	errorData, _ := json.Marshal(errorEvent)
	sseWriter.WriteEvent("error", errorData)
}

func (h *Handler) streamFromTarget(ctx context.Context, req *anthropic.Request, target config.RouteTarget, sseWriter *SSEWriter) error {
	providerCfg, ok := h.config.Providers[target.Provider]
	if !ok {
		return fmt.Errorf("provider not found")
	}

	transformerName := providerCfg.Transformer
	if transformerName == "" {
		transformerName = target.Provider
	}
	transformer, err := h.transformerRegistry.Get(transformerName)
	if err != nil {
		transformer, _ = h.transformerRegistry.Get("anthropic")
	}

	client, ok := h.providerClients[target.Provider]
	if !ok {
		return fmt.Errorf("client not found")
	}

	// Ensure streaming is enabled
	req.Stream = true

	httpReq, err := transformer.TransformRequest(req, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("provider error: %d", resp.StatusCode)
	}

	// Stream response
	scanner := NewSSEScanner(resp.Body)
	for scanner.Scan() {
		chunk, err := transformer.TransformStreamChunk(scanner.Data(), scanner.Event())
		if err != nil {
			continue
		}
		sseWriter.WriteEvent(scanner.Event(), chunk)
	}

	return scanner.Err()
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/proxy/streaming.go internal/proxy/streaming_test.go
git commit -m "feat: add streaming handler support"
```

---

## Task 12: Daemon/Instance Management

**Files:**
- Create: `internal/daemon/instance.go`
- Create: `internal/daemon/instance_test.go`
- Create: `internal/daemon/pidfile.go`

**Step 1: Write the failing test**

Create `internal/daemon/instance_test.go`:

```go
package daemon

import (
	"testing"
)

func TestGenerateInstanceID(t *testing.T) {
	id := GenerateInstanceID()

	if !strings.HasPrefix(id, "inst_") {
		t.Errorf("expected ID to start with 'inst_', got '%s'", id)
	}
}

func TestInstanceMetadata(t *testing.T) {
	meta := &InstanceMetadata{
		ID:         "inst_20250217_143000",
		Port:       8081,
		PID:        12345,
		ConfigType: "project",
		StartTime:  time.Now(),
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	var unmarshaled InstanceMetadata
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if unmarshaled.ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, unmarshaled.ID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/... -v`
Expected: FAIL - undefined: GenerateInstanceID, InstanceMetadata

**Step 3: Write implementation**

Create `internal/daemon/instance.go`:

```go
// Package daemon manages router instances.
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstanceMetadata represents metadata for a running instance.
type InstanceMetadata struct {
	ID          string    `json:"id"`
	Port        int       `json:"port"`
	PID         int       `json:"pid"`
	ConfigType  string    `json:"configType"`
	ConfigPath  string    `json:"configPath"`
	ProjectRoot string    `json:"projectRoot"`
	StartTime   time.Time `json:"startTime"`
}

// GenerateInstanceID generates a unique instance ID.
func GenerateInstanceID() string {
	return fmt.Sprintf("inst_%s", time.Now().Format("20060102_150405"))
}

// InstancesDir returns the directory for instance files.
func InstancesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-modelrouter", "instances"), nil
}

// SaveInstance saves instance metadata to disk.
func SaveInstance(meta *InstanceMetadata) error {
	dir, err := InstancesDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, meta.ID+".json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadInstance loads instance metadata from disk.
func LoadInstance(id string) (*InstanceMetadata, error) {
	dir, err := InstancesDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta InstanceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// DeleteInstance removes instance metadata from disk.
func DeleteInstance(id string) error {
	dir, err := InstancesDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, id+".json")
	return os.Remove(path)
}

// ListInstances lists all instances.
func ListInstances() ([]*InstanceMetadata, error) {
	dir, err := InstancesDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var instances []*InstanceMetadata
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		meta, err := LoadInstance(id)
		if err != nil {
			continue
		}

		instances = append(instances, meta)
	}

	return instances, nil
}

// IsRunning checks if an instance is still running.
func IsRunning(meta *InstanceMetadata) bool {
	if meta.PID == 0 {
		return false
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to signal
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
```

**Step 4: Write PID file management**

Create `internal/daemon/pidfile.go`:

```go
package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WritePIDFile writes the current process PID to a file.
func WritePIDFile(path string) error {
	pid := os.Getpid()
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}

// ReadPIDFile reads a PID from a file.
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/daemon/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/daemon/
git commit -m "feat: add daemon instance management"
```

---

## Task 13: CLI Commands

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/code.go`
- Create: `internal/cli/start.go`
- Create: `internal/cli/stop.go`
- Create: `internal/cli/status.go`

**Step 1: Write the failing test**

Create a minimal test for the CLI structure. For now, we'll focus on implementation since CLI commands are hard to unit test in isolation.

**Step 2: Write root command**

Create `internal/cli/root.go`:

```go
// Package cli implements the command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the application version.
var Version = "0.1.0"

// NewRootCommand creates the root command.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ccrouter",
		Short:   "Claude Code Model Router",
		Version: Version,
	}

	cmd.AddCommand(NewCodeCommand())
	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStopCommand())
	cmd.AddCommand(NewRestartCommand())
	cmd.AddCommand(NewStatusCommand())
	cmd.AddCommand(NewCleanCommand())
	cmd.AddCommand(NewConfigCommand())
	cmd.AddCommand(NewLogsCommand())

	return cmd
}

// Execute runs the CLI.
func Execute() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Write code command**

Create `internal/cli/code.go`:

```go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/spf13/cobra"
)

// NewCodeCommand creates the code command.
func NewCodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [claude-args...]",
		Short: "Start router and launch Claude Code",
		Args:  cobra.ArbitraryArgs,
		RunE:  runCode,
	}

	return cmd
}

func runCode(cmd *cobra.Command, args []string) error {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		projectRoot = "."
	}

	// Load configuration
	cfg, configType, err := config.LoadWithOverride(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create instance metadata
	instanceID := daemon.GenerateInstanceID()
	meta := &daemon.InstanceMetadata{
		ID:          instanceID,
		Port:        cfg.Server.Port,
		ConfigType:  configType,
		ProjectRoot: projectRoot,
		StartTime:   time.Now(),
	}

	// Initialize transformer registry
	registry := transformer.NewRegistry()
	registry.Register(transformer.NewAnthropicTransformer())
	registry.Register(transformer.NewOpenRouterTransformer())

	// Initialize provider clients
	clients := make(map[string]provider.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		client, err := provider.NewClient(&provider.ClientConfig{
			BaseURL: providerCfg.BaseURL,
			APIKey:  providerCfg.APIKey,
		})
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", name, err)
		}
		clients[name] = client
	}

	// Create router engine
	routerEngine := router.NewEngine(cfg)

	// Create and start proxy server
	server, err := proxy.NewServer(&proxy.ServerConfig{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server.SetRouter(routerEngine)
	server.SetTransformerRegistry(registry)
	server.SetProviderClients(clients)
	server.SetConfig(cfg)

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Save instance metadata
	meta.PID = os.Getpid()
	daemon.SaveInstance(meta)

	// Set environment for Claude Code
	anthropicURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)

	// Find claude executable
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		server.Stop(context.Background())
		return fmt.Errorf("claude executable not found: %w", err)
	}

	// Prepare command
	claudeArgs := append([]string{"claude"}, args...)
	env := os.Environ()
	env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", anthropicURL))

	// Execute claude
	execErr := syscall.Exec(claudePath, claudeArgs, env)

	// Cleanup on error
	server.Stop(context.Background())
	daemon.DeleteInstance(instanceID)

	return execErr
}

func findProjectRoot() (string, error) {
	// Look for .git directory or .cc-modelrouter directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".cc-modelrouter")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found")
}
```

**Step 4: Write start command**

Create `internal/cli/start.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/spf13/cobra"
)

var startPort int
var startHost string

// NewStartCommand creates the start command.
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start router server standalone",
		RunE:  runStart,
	}

	cmd.Flags().IntVarP(&startPort, "port", "p", 0, "Port to listen on")
	cmd.Flags().StringVarP(&startHost, "host", "H", "", "Host to bind to")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, configType, err := config.LoadWithOverride(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override with flags
	if startPort > 0 {
		cfg.Server.Port = startPort
	}
	if startHost != "" {
		cfg.Server.Host = startHost
	}

	// Create instance metadata
	instanceID := daemon.GenerateInstanceID()
	meta := &daemon.InstanceMetadata{
		ID:          instanceID,
		Port:        cfg.Server.Port,
		ConfigType:  configType,
		ProjectRoot: ".",
		StartTime:   time.Now(),
	}

	// Initialize components
	registry := transformer.NewRegistry()
	registry.Register(transformer.NewAnthropicTransformer())
	registry.Register(transformer.NewOpenRouterTransformer())

	clients := make(map[string]provider.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		client, err := provider.NewClient(&provider.ClientConfig{
			BaseURL: providerCfg.BaseURL,
			APIKey:  providerCfg.APIKey,
		})
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", name, err)
		}
		clients[name] = client
	}

	routerEngine := router.NewEngine(cfg)

	// Create and start server
	server, err := proxy.NewServer(&proxy.ServerConfig{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server.SetRouter(routerEngine)
	server.SetTransformerRegistry(registry)
	server.SetProviderClients(clients)
	server.SetConfig(cfg)

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Save instance metadata
	meta.PID = os.Getpid()
	daemon.SaveInstance(meta)

	fmt.Printf("Router started on %s:%d (instance: %s)\n", cfg.Server.Host, cfg.Server.Port, instanceID)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Cleanup
	fmt.Println("\nShutting down...")
	server.Stop(context.Background())
	daemon.DeleteInstance(instanceID)

	return nil
}
```

**Step 5: Write stop command**

Create `internal/cli/stop.go`:

```go
package cli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/spf13/cobra"
)

var stopAll bool

// NewStopCommand creates the stop command.
func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [instance-id]",
		Short: "Stop router instance",
		RunE:  runStop,
	}

	cmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all instances")

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	if stopAll {
		return stopAllInstances()
	}

	if len(args) == 0 {
		return fmt.Errorf("instance-id required (or use --all)")
	}

	return stopInstance(args[0])
}

func stopInstance(id string) error {
	meta, err := daemon.LoadInstance(id)
	if err != nil {
		return fmt.Errorf("instance not found: %s", id)
	}

	if !daemon.IsRunning(meta) {
		daemon.DeleteInstance(id)
		return fmt.Errorf("instance is not running: %s", id)
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	fmt.Printf("Stopped instance: %s\n", id)
	daemon.DeleteInstance(id)
	return nil
}

func stopAllInstances() error {
	instances, err := daemon.ListInstances()
	if err != nil {
		return err
	}

	for _, meta := range instances {
		if daemon.IsRunning(meta) {
			proc, _ := os.FindProcess(meta.PID)
			proc.Signal(syscall.SIGTERM)
			fmt.Printf("Stopped instance: %s\n", meta.ID)
		}
		daemon.DeleteInstance(meta.ID)
	}

	return nil
}
```

**Step 6: Write status command**

Create `internal/cli/status.go`:

```go
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/spf13/cobra"
)

// NewStatusCommand creates the status command.
func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show all running instances",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	instances, err := daemon.ListInstances()
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		fmt.Println("No instances found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPORT\tPID\tSTATUS\tCONFIG\tSTARTED")

	for _, meta := range instances {
		status := "stopped"
		if daemon.IsRunning(meta) {
			status = "running"
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\n",
			meta.ID,
			meta.Port,
			meta.PID,
			status,
			meta.ConfigType,
			meta.StartTime.Format("2006-01-02 15:04:05"),
		)
	}

	w.Flush()
	return nil
}
```

**Step 7: Write remaining commands**

Create `internal/cli/restart.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRestartCommand creates the restart command.
func NewRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [instance-id]",
		Short: "Restart instance (always reloads config)",
		RunE:  runRestart,
	}
}

func runRestart(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("instance-id required")
	}

	// Stop the instance
	if err := stopInstance(args[0]); err != nil {
		return err
	}

	// Note: Starting a new instance requires the user to run start/code again
	fmt.Println("Instance stopped. Run 'ccrouter start' or 'ccrouter code' to start a new instance.")
	return nil
}
```

Create `internal/cli/clean.go`:

```go
package cli

import (
	"fmt"

	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/spf13/cobra"
)

// NewCleanCommand creates the clean command.
func NewCleanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove stale instance files",
		RunE:  runClean,
	}
}

func runClean(cmd *cobra.Command, args []string) error {
	instances, err := daemon.ListInstances()
	if err != nil {
		return err
	}

	cleaned := 0
	for _, meta := range instances {
		if !daemon.IsRunning(meta) {
			daemon.DeleteInstance(meta.ID)
			fmt.Printf("Cleaned stale instance: %s\n", meta.ID)
			cleaned++
		}
	}

	if cleaned == 0 {
		fmt.Println("No stale instances to clean")
	} else {
		fmt.Printf("Cleaned %d stale instance(s)\n", cleaned)
	}

	return nil
}
```

Create `internal/cli/config.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigCommand creates the config command.
func NewConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show active configuration",
		RunE:  runConfig,
	}
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, configType, err := config.LoadWithOverride(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Configuration source: %s\n\n", configType)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
```

Create `internal/cli/logs.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewLogsCommand creates the logs command.
func NewLogsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [instance-id]",
		Short: "Show logs for instance",
		RunE:  runLogs,
	}
}

func runLogs(cmd *cobra.Command, args []string) error {
	// TODO: Implement log viewing
	// For now, logs go to stdout/stderr
	fmt.Println("Log viewing not yet implemented. Logs are written to stdout/stderr.")
	return nil
}
```

**Step 8: Add cobra dependency and run tests**

Run: `go get github.com/spf13/cobra`
Run: `go test ./internal/cli/... -v`
Expected: PASS (or no tests to run)

**Step 9: Commit**

```bash
git add internal/cli/
git commit -m "feat: add CLI commands"
```

---

## Task 14: Main Entry Point

**Files:**
- Create: `cmd/ccrouter/main.go`

**Step 1: Write main entry point**

Create `cmd/ccrouter/main.go`:

```go
// Package main is the entry point for ccrouter.
package main

import (
	"github.com/iimmutable/cc-modelrouter/internal/cli"
)

func main() {
	cli.Execute()
}
```

**Step 2: Build and verify**

Run: `go build -o bin/ccrouter ./cmd/ccrouter`
Expected: Binary created successfully

**Step 3: Test the binary**

Run: `./bin/ccrouter --version`
Expected: Version output

Run: `./bin/ccrouter --help`
Expected: Help output showing all commands

**Step 4: Commit**

```bash
git add cmd/ccrouter/main.go bin/
git commit -m "feat: add main entry point"
```

---

## Task 15: Integration Test Setup

**Files:**
- Create: `test/integration_test.go`
- Create: `.cc-modelrouter/test.config.json`

**Step 1: Create test configuration**

Create `.cc-modelrouter/test.config.json`:

```json
{
  "server": {
    "port": 18081,
    "host": "localhost"
  },
  "providers": {
    "bigmodel": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-4.7"]
    }
  },
  "router": {
    "routes": {
      "default": "bigmodel:glm-4.7"
    },
    "maxRetries": 1,
    "retryDelay": "500ms"
  }
}
```

**Step 2: Write integration test**

Create `test/integration_test.go`:

```go
//go:build integration
// +build integration

package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
)

func TestIntegrationBasicRequest(t *testing.T) {
	// Load test configuration
	cfg, err := config.Load(".cc-modelrouter/test.config.json")
	if err != nil {
		t.Skipf("Test config not found: %v", err)
	}

	// Initialize components
	registry := transformer.NewRegistry()
	registry.Register(transformer.NewAnthropicTransformer())

	clients := make(map[string]provider.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		client, _ := provider.NewClient(&provider.ClientConfig{
			BaseURL: providerCfg.BaseURL,
			APIKey:  providerCfg.APIKey,
		})
		clients[name] = client
	}

	routerEngine := router.NewEngine(cfg)

	// Create handler
	handler := proxy.NewHandler(50 * 1024 * 1024)
	handler.SetRouter(routerEngine)
	handler.SetTransformerRegistry(registry)
	handler.SetProviderClients(clients)
	handler.SetConfig(cfg)

	// Create test request
	reqBody := map[string]any{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": []map[string]any{
			{"role": "user", "content": "Say 'Hello'"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Logf("Response: %s", w.Body.String())
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
```

**Step 3: Run integration test**

Run: `go test -tags=integration ./test/... -v -timeout 5m`
Expected: Test runs against real provider (requires API key)

**Step 4: Commit**

```bash
git add test/ .cc-modelrouter/
git commit -m "feat: add integration test setup"
```

---

## Summary

This implementation plan covers all components needed for cc-modelrouter:

| Task | Component | Files |
|------|-----------|-------|
| 1 | Project Setup | go.mod, directories |
| 2 | API Types | pkg/api/anthropic/types.go |
| 3 | Configuration | internal/config/ |
| 4 | Transformer Interface | internal/transformer/interface.go, registry.go |
| 5 | Anthropic Transformer | internal/transformer/anthropic.go |
| 6 | OpenRouter Transformer | internal/transformer/openrouter.go |
| 7 | Provider Client | internal/provider/ |
| 8 | Router Engine | internal/router/engine.go |
| 9 | Failover Logic | internal/router/failover.go |
| 10 | HTTP Server | internal/proxy/server.go, handler.go |
| 11 | Streaming | internal/proxy/streaming.go |
| 12 | Daemon/Instance | internal/daemon/ |
| 13 | CLI Commands | internal/cli/ |
| 14 | Main Entry | cmd/ccrouter/main.go |
| 15 | Integration Tests | test/integration_test.go |

---

## Additional Transformers (Optional Future Tasks)

After the core implementation is complete, additional transformers can be added:

- **Gemini Transformer** - internal/transformer/gemini.go
- **Qwen Transformer** - internal/transformer/qwen.go

Each follows the same pattern as the OpenRouter transformer.

---

## Missing Imports to Add During Implementation

The following imports are required but may not be shown in all code snippets. Add them as needed:

- `context` - for context.Context
- `encoding/json` - for JSON marshaling/unmarshaling
- `fmt` - for formatting
- `strings` - for string operations
- `sync` - for sync.RWMutex
- `syscall` - for syscall.Signal
- `time` - for time.Time and time.Now()
- `os` - for file and process operations
- `os/signal` - for signal.Notify
- `path/filepath` - for filepath operations
- `io` - for io.Reader/io.Writer
- `bufio` - for bufio.Scanner
- `net/http` - for HTTP types
