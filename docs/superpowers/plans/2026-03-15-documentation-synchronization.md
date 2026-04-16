# Documentation Synchronization Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Synchronize all documentation files with the current codebase state after recent transformer architecture refactoring.

**Architecture:** Update documentation to reflect the current Transformer interface (`PrepareRequest`, `ParseResponse`, `TransformStreamEvent`) and remove references to non-existent concepts ("Unified Intermediate Format") and directories (`converters/`, `unified/`).

**Tech Stack:** Go 1.22+, Markdown documentation

---

## Summary of Changes Needed

### Critical Issues Found

1. **CLAUDE.md** - Claims "Unified Intermediate Format" architecture which doesn't exist
2. **CLAUDE.md** - Transformer interface shows outdated method names
3. **README.md** - Project structure section references non-existent directories
4. **README.md** - Transformer interface shows outdated method names
5. **docs/transformers.md** - Interface documentation is outdated
6. **docs/architecture.md** - Interface documentation is outdated
7. **Memory file** - Does not exist (should be created for future sessions)

### Current Transformer Interface (from `internal/transformer/interface.go`)

```go
type Transformer interface {
    Name() string
    Endpoint() string
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

### Outdated Interface (currently in docs)

```go
type Transformer interface {
    Name() string
    TransformRequest(req *anthropic.Request, ...) (*http.Request, error)
    TransformResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}
```

### Architecture Reality

The transformers work **directly** with `anthropic.Request` and `anthropic.Response` types - there is NO "Unified Intermediate Format" or `unified/` package. Each transformer converts directly between Anthropic format and provider-specific formats.

---

## File Structure

**Files to modify:**
- `CLAUDE.md` - Fix architecture description and interface documentation (HIGH PRIORITY)
- `README.md` - Fix project structure and interface documentation
- `docs/transformers.md` - Update interface and file locations
- `docs/architecture.md` - Update interface documentation

**Files to create:**
- `memory/MEMORY.md` - Project memory for future sessions

---

## Chunk 0: CLAUDE.md Updates (HIGH PRIORITY)

### Task 0a: Fix Architecture Overview Section

**Files:**
- Modify: `CLAUDE.md:59-76`

- [ ] **Step 0a.1: Update Core Request Flow diagram**

The diagram is mostly correct but the description mentions "Unified Intermediate Format" which doesn't exist. Update the Architecture Overview section:

**Current (line 61):**
```
cc-modelrouter is a Go-based HTTP proxy that routes Claude Code requests to multiple LLM providers with automatic format transformation. The architecture uses a **Unified Intermediate Format** pattern to separate protocol conversion from routing logic.
```

**Corrected:**
```
cc-modelrouter is a Go-based HTTP proxy that routes Claude Code requests to multiple LLM providers with automatic format transformation. Transformers convert requests directly between Anthropic format and provider-specific formats.
```

### Task 0b: Fix Transformer Layer Section

**Files:**
- Modify: `CLAUDE.md:90-97`

- [ ] **Step 0b.1: Update Transformer Layer description**

**Current (lines 90-97):**
```markdown
**Transformer Layer (`internal/transformer/`)**
- **Unified Format**: Provider-agnostic intermediate representation
- **Interface**: All providers implement `Transformer` interface with:
  - `TransformRequest`: Anthropic → Provider HTTP Request
  - `TransformResponse`: Provider Response → Anthropic
  - `SupportsStreaming`: Whether the transformer supports streaming
  - `TransformStreamChunk`: Streaming event transformation
- **Providers**: anthropic, openai, openrouter, gemini, glm (Anthropic-compatible)
```

**Corrected:**
```markdown
**Transformer Layer (`internal/transformer/`)**
- **Direct Transformation**: Transformers convert directly between Anthropic format and provider-specific formats
- **Interface**: All providers implement `Transformer` interface with:
  - `Name()`: Transformer identifier
  - `Endpoint()`: API endpoint path for this transformer
  - `PrepareRequest`: Convert Anthropic request to provider HTTP request
  - `ParseResponse`: Convert provider HTTP response to Anthropic format
  - `SupportsStreaming`: Whether the transformer supports streaming
  - `TransformStreamEvent`: Convert provider SSE events to Anthropic format
- **Providers**: anthropic, openai, openrouter, gemini, glm-anthropic
```

- [ ] **Step 0b.2: Commit CLAUDE.md changes**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md to reflect current transformer architecture"
```

---

## Chunk 1: README.md Updates

### Task 1: Update Project Structure Section

**Files:**
- Modify: `README.md:367-414`

- [ ] **Step 1: Remove non-existent directories from project structure**

Remove the following entries from the project structure diagram:
- `internal/transformer/converters/` directory and all its files
- `internal/transformer/unified/` directory and all its files

**Current (incorrect):**
```
│   ├── transformer/       # Request/response transformers
│   │   ├── converters/     # Format conversion utilities
│   │   │   ├── anthropic_to_unified.go
│   │   │   ├── unified_to_anthropic.go
│   │   │   ├── unified_to_openai.go
│   │   │   ├── openai_to_unified.go
│   │   │   ├── unified_to_gemini.go
│   │   │   └── gemini_to_unified.go
│   │   ├── providers/      # Provider transformer implementations
...
│   │   ├── unified/        # Unified intermediate format types
│   │   │   ├── message.go
│   │   │   ├── tool.go
│   │   │   ├── request.go
│   │   │   ├── response.go
│   │   │   └── reasoning.go
│   │   ├── test/           # Integration tests
│   │   │   └── integration_test.go
│   │   ├── base.go          # Base transformer utilities
│   │   ├── interface.go     # Transformer interface definition
│   │   └── registry.go      # Transformer registry
```

**Corrected:**
```
│   ├── transformer/       # Request/response transformers
│   │   ├── transformers/   # Provider transformer implementations
│   │   │   ├── anthropic.go
│   │   │   ├── openai.go
│   │   │   ├── openrouter.go
│   │   │   ├── gemini.go
│   │   │   └── glm_anthropic.go
│   │   ├── test/           # Integration tests
│   │   ├── base.go         # Base transformer utilities
│   │   ├── interface.go    # Transformer interface definition
│   │   └── registry.go     # Transformer registry
```

- [ ] **Step 2: Verify the change is correct**

Run: `ls -la internal/transformer/`
Expected: See only `base.go`, `interface.go`, `registry.go`, `test/`, `transformers/`

### Task 2: Update Transformer Interface Documentation

**Files:**
- Modify: `README.md:257-283`

- [ ] **Step 3: Update the Transformer interface definition**

Replace the old interface with the current one:

```go
type Transformer interface {
    // Name returns the transformer name (used for registry lookup)
    Name() string

    // Endpoint returns the API endpoint path for this transformer
    Endpoint() string

    // PrepareRequest converts an Anthropic request to a provider-specific HTTP request
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

    // ParseResponse converts a provider HTTP response to Anthropic format
    ParseResponse(resp *http.Response) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming
    SupportsStreaming() bool

    // TransformStreamEvent converts a provider SSE event to Anthropic SSE events
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

- [ ] **Step 4: Add SSEEvent type documentation**

Add after the interface:

```go
// SSEEvent represents a complete server-sent event with type and data.
type SSEEvent struct {
    EventType string  // SSE event type (e.g., "message_start", "content_block_delta")
    Data      []byte  // Raw JSON data payload
}
```

### Task 3: Remove Converter Utilities Section

**Files:**
- Modify: `README.md:285-294`

- [ ] **Step 5: Remove the Converter Utilities section**

The converters directory no longer exists. Remove the section:

```markdown
### Converter Utilities

The `converters` package provides reusable conversion functions:

- `AnthropicToUnified()` - Convert Anthropic to unified format
- `UnifiedToAnthropic()` - Convert unified to Anthropic format
- `UnifiedToOpenAIRequest()` - Convert unified to OpenAI HTTP request
- `OpenAIToUnified()` - Convert OpenAI response to unified format
- `UnifiedRequestToAnthropic()` - Convert unified request to Anthropic
```

- [ ] **Step 6: Commit README changes**

```bash
git add README.md
git commit -m "docs: update README to reflect current transformer architecture"
```

---

## Chunk 2: docs/transformers.md Updates

### Task 4: Update Transformer Interface Section

**Files:**
- Modify: `docs/transformers.md:7-24`

- [ ] **Step 7: Update the Transformer interface in transformers.md**

Replace lines 7-24 with:

```go
type Transformer interface {
    // Name returns the transformer name (used for registry lookup)
    Name() string

    // Endpoint returns the API endpoint path for this transformer
    Endpoint() string

    // PrepareRequest converts an Anthropic request to a provider-specific HTTP request
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)

    // ParseResponse converts a provider HTTP response to Anthropic format
    ParseResponse(resp *http.Response) (*anthropic.Response, error)

    // SupportsStreaming returns true if this transformer supports streaming
    SupportsStreaming() bool

    // TransformStreamEvent converts a provider SSE event to Anthropic SSE events
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

### Task 5: Update Creating a Custom Transformer Section

**Files:**
- Modify: `docs/transformers.md:186-242`

- [ ] **Step 8: Update the custom transformer example**

Replace method names in the example:

```go
func (t *MyTransformer) Name() string {
    return "myprovider"
}

func (t *MyTransformer) Endpoint() string {
    return "/v1/messages"  // or provider-specific endpoint
}

func (t *MyTransformer) PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
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

func (t *MyTransformer) ParseResponse(resp *http.Response) (*anthropic.Response, error) {
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

func (t *MyTransformer) TransformStreamEvent(event *transformer.SSEEvent) ([]transformer.SSEEvent, error) {
    // Convert streaming SSE event format
    return []transformer.SSEEvent{*event}, nil // or transform
}
```

### Task 6: Update File Locations Section

**Files:**
- Modify: `docs/transformers.md:380-396`

- [ ] **Step 9: Update file locations to remove non-existent directories**

Replace with:

```
## File Locations

```
internal/transformer/
├── interface.go        # Transformer interface
├── registry.go         # Transformer registry
├── base.go             # Base types and utilities
└── transformers/       # Transformer implementations
    ├── anthropic.go    # Anthropic (pass-through)
    ├── openai.go       # OpenAI-compatible
    ├── openrouter.go   # OpenRouter (handles signature preservation)
    ├── gemini.go       # Gemini native format
    └── glm_anthropic.go # GLM Anthropic-compatible
```

**Note:** Transformers are registered in `internal/cli/start.go` and `internal/cli/code.go`.
```

- [ ] **Step 10: Commit docs/transformers.md changes**

```bash
git add docs/transformers.md
git commit -m "docs: update transformers.md to reflect current interface"
```

---

## Chunk 3: docs/architecture.md Updates

### Task 7: Update Transformer Interface in Architecture Doc

**Files:**
- Modify: `docs/architecture.md:118-130`

- [ ] **Step 11: Update the Transformer interface in architecture.md**

Replace with:

```go
type Transformer interface {
    Name() string
    Endpoint() string
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

- [ ] **Step 12: Commit docs/architecture.md changes**

```bash
git add docs/architecture.md
git commit -m "docs: update architecture.md to reflect current transformer interface"
```

---

## Chunk 4: Create Memory File

### Task 8: Create Project Memory File

**Files:**
- Create: `memory/MEMORY.md`

- [ ] **Step 13: Create the memory directory and file**

```markdown
# cc-modelrouter Memory

> Project-specific context that persists across Claude Code sessions.

## Architecture Overview

cc-modelrouter is a Go HTTP proxy that routes Claude Code requests to multiple LLM providers with automatic format transformation. Transformers convert requests directly between Anthropic format and provider-specific formats.

### Transformer Architecture

**Current Interface** (defined in `internal/transformer/interface.go`):

```go
type Transformer interface {
    Name() string
    Endpoint() string
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}
```

**Key Files:**
- `internal/transformer/interface.go` - Transformer interface definition
- `internal/transformer/registry.go` - Transformer registry
- `internal/transformer/base.go` - Base utilities
- `internal/transformer/transformers/` - Provider implementations

**Built-in Transformers:**
- `anthropic` - Pass-through for Anthropic API
- `openai` - OpenAI-compatible format
- `openrouter` - OpenRouter (handles signature preservation)
- `gemini` - Gemini native format
- `glm-anthropic` - GLM Anthropic-compatible

### Key Patterns

**Thinking Block Handling:**
- OpenRouter requires `signature` field present (even if empty)
- Anthropic API rejects whitespace-only signatures (omit field)
- Transformers must deep-copy requests before modification for failover

**Usage Tracking:**
- Extract `input_tokens` and `output_tokens` from `message_delta` events in streaming
- Fallback to character count estimate if provider doesn't send tokens

**Security:**
- Always use `logging.SanitizeHeadersString()` for header logging
- Never use `%v` with `http.Header` - leaks API keys

## Configuration

**File Locations:**
- Global: `~/.cc-modelrouter/config.json`
- Project: `<project>/.cc-modelrouter/config.json`

**Environment Variables:**
- Use `${VAR_NAME}` syntax for API keys
- Required: `OPENROUTER_API_KEY`, `GEMINI_API_KEY`, `BIGMODEL_API_KEY`, etc.

## Build Commands

```bash
# Build debug binary
go build -o bin/debug/ccrouter ./cmd/ccrouter

# Build release binary
go build -o bin/release/ccrouter ./cmd/ccrouter

# Run tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Common Issues

1. **Port conflicts** - Use `lsof -i :8081` to check
2. **Streaming issues** - Check for proper SSE event format
3. **Thinking block errors** - Check signature field handling per provider
```

- [ ] **Step 14: Commit memory file**

```bash
git add memory/MEMORY.md
git commit -m "docs: add project memory file for cross-session context"
```

---

## Chunk 5: Verification and Final Review

### Task 9: Verify All Changes

- [ ] **Step 15: Build the project to ensure no code changes broke anything**

Run: `go build ./...`
Expected: Success (no errors)

- [ ] **Step 16: Run tests to verify nothing is broken**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 17: Review all changes**

Run: `git diff main --stat`
Expected: Only documentation files changed

- [ ] **Step 18: Final commit if needed**

If any additional fixes were needed:
```bash
git add -A
git commit -m "docs: final documentation synchronization fixes"
```

---

## Summary

| Task | File | Change |
|------|------|--------|
| 0a | CLAUDE.md | Remove "Unified Intermediate Format" from architecture description |
| 0b | CLAUDE.md | Update Transformer interface with correct method names |
| 1 | README.md | Remove non-existent directories from project structure |
| 2 | README.md | Update Transformer interface to current version |
| 3 | README.md | Remove Converter Utilities section |
| 4 | docs/transformers.md | Update Transformer interface |
| 5 | docs/transformers.md | Update custom transformer example |
| 6 | docs/transformers.md | Update file locations |
| 7 | docs/architecture.md | Update Transformer interface |
| 8 | memory/MEMORY.md | Create project memory file |
| 9 | - | Verify all changes |