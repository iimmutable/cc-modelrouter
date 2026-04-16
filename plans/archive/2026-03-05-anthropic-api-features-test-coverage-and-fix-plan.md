# Anthropic-Compatible API Features Test Coverage & Fix Plan

**Date:** 2026-03-05
**Author:** Investigation Summary
**Status:** Planning Phase

---

## Executive Summary

This document provides a comprehensive investigation of test coverage for Anthropic-compatible API features and a fix plan to improve coverage without breaking existing functionality.

### Investigation Scope
1. **Image Support** - Handling image blocks in message content
2. **PDF Support** - Handling PDF file uploads/processing
3. **Code Execution (Computer Use)** - Computer use/tool execution capabilities
4. **Files API** - File uploads, downloads, management

---

## Part 1: Current Test Coverage Analysis

### 1. Image Support

#### Existing Tests
| Location | Test | Coverage |
|----------|------|----------|
| `internal/proxy/handler_test.go:360-412` | `TestHasImages()` | Detects image blocks in requests |
| `pkg/api/anthropic/types_test.go:42-70` | Image block marshaling | Tests image source serialization |
| `test/integration/provider_quirks/qwen_quirks_test.go:485-563` | `TestQwenImageContent` | Tests Qwen's multimodal support |
| `internal/router/engine_test.go:207,242-245` | Route detection | Tests image routing priority |

#### Implementation Status
**Implemented:**
- `pkg/api/anthropic/types.go:338-343` - `ImageSource` struct
- `internal/router/engine.go:34,85-87` - `HasImages` field and detection
- `internal/transformer/transformers/gemini.go:202-210,266-275` - Gemini image conversion

#### Coverage Gaps
- No tests for different image formats (PNG, JPEG, WebP, GIF)
- No tests for image size limits or validation (max 50MB body)
- No tests for streaming responses with images
- Missing tests in OpenAI, OpenRouter, and GLM transformers
- No tests for base64 vs URL image sources
- No tests for image + text mixed content edge cases

**Coverage: ~30%**

---

### 2. PDF Support

#### Existing Tests
**NONE FOUND**

#### Implementation Status
**NOT IMPLEMENTED**
- No `/v1/files` endpoint
- No PDF processing logic
- No file upload handling

**Coverage: 0%**

**Note:** According to Anthropic API documentation, PDF support is part of the Files API, not directly in messages API.

---

### 3. Code Execution (Computer Use)

#### Existing Tests
**NONE FOUND**

#### Implementation Status
**NOT IMPLEMENTED**
- No computer use endpoint handlers
- No tool execution environment
- No computer use transformers

**Coverage: 0%**

**Note:** Computer Use API (beta) requires:
- `/v1/messages` with `computer_use` tools
- Environment sandbox
- Screenshot and coordinate handling
- File system access

---

### 4. Files API

#### Existing Tests
| Location | Test | Coverage |
|----------|------|----------|
| `internal/proxy/handler_test.go:1117` | Endpoint reference | Only mentions `/v1/files` path |

#### Implementation Status
**MINIMAL IMPLEMENTATION**
- No file upload/download endpoints
- No file storage management
- No multipart form data handling for files

**Coverage: ~10%**

---

## Part 2: Test Coverage Summary Table

| Feature | Test Coverage | Implementation | Major Gaps |
|---------|--------------|----------------|------------|
| **Image Support** | 30% | ✅ Partial | Format tests, size validation, streaming, provider tests |
| **PDF Support** | 0% | ❌ None | Complete feature missing |
| **Computer Use** | 0% | ❌ None | Complete feature missing |
| **Files API** | 10% | ⚠️ Minimal | Upload, download, file management |
| **Tool Use** | 40% | ✅ Implemented | Complex workflows, error handling |

---

## Part 3: Fix Plan

### Phase 1: Image Support Test Enhancement (Week 1)

**Goal:** Increase image support coverage from 30% to 80%

#### 1.1 Image Format Tests
**File:** `pkg/api/anthropic/types_image_test.go` (NEW)

```go
// Test different image media types
func TestImageMediaTypes(t *testing.T) {
    formats := []string{
        "image/png",
        "image/jpeg",
        "image/webp",
        "image/gif",
    }
    // Test each format
}
```

#### 1.2 Image Size Validation Tests
**File:** `internal/proxy/handler_image_test.go` (NEW)

```go
// Test image size limits
func TestImageSizeValidation(t *testing.T) {
    // Test single large image
    // Test multiple small images
    // Test 50MB boundary
}
```

#### 1.3 Streaming with Images Tests
**File:** `internal/transformer/transformers/image_streaming_test.go` (NEW)

```go
// Test SSE streaming with image content
func TestImageStreamingResponse(t *testing.T) {
    // Test Gemini image streaming
    // Test OpenAI image streaming
    // Test OpenRouter image streaming
}
```

#### 1.4 Provider-Specific Image Tests
**File:** `test/integration/images/provider_image_test.go` (NEW)

```go
// Test each provider's image handling
func TestProviderImageHandling(t *testing.T) {
    // Test OpenAI GPT-4 Vision
    // Test OpenRouter multimodal
    // Test GLM image support
}
```

**Safety Measures:**
- All tests use mock HTTP servers
- No real API calls
- Isolated test fixtures
- Preserve existing transformer behavior

---

### Phase 2: Files API Implementation (Week 2-3)

**Goal:** Implement basic Files API with 70% test coverage

#### 2.1 Files API Types
**File:** `pkg/api/anthropic/types_files.go` (NEW)

```go
// File upload request/response types
type FileUploadRequest struct {
    File     io.Reader
    Filename string
    Purpose  string
}

type FileUploadResponse struct {
    ID       string
    Filename string
    Bytes    int64
    CreatedAt int64
}
```

#### 2.2 Files Handler
**File:** `internal/proxy/files_handler.go` (NEW)

```go
// Handle file uploads
func (h *Handler) HandleFileUpload(w http.ResponseWriter, r *http.Request)

// Handle file downloads
func (h *Handler) HandleFileDownload(w http.ResponseWriter, r *http.Request)

// List files
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request)

// Delete files
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request)
```

#### 2.3 Files API Tests
**File:** `internal/proxy/files_handler_test.go` (NEW)

```go
func TestFileUpload(t *testing.T)
func TestFileDownload(t *testing.T)
func TestFileList(t *testing.T)
func TestFileDelete(t *testing.T)
func TestFileSizeValidation(t *testing.T)
func TestFileMalformedResponse(t *testing.T)
```

**Safety Measures:**
- Separate handler from messages handler
- No impact on existing `/v1/messages` endpoint
- Configurable file storage location
- File size limits enforced before processing

---

### Phase 3: PDF Support (Week 4)

**Goal:** Add PDF file support via Files API

#### 3.1 PDF Processing Types
**File:** `pkg/api/anthropic/types_pdf.go` (NEW)

```go
// PDF-specific types
type PDFContent struct {
    Type     string
    PDFSource *PDFSource
}

type PDFSource struct {
    Type      string
    Data      string // base64 encoded
}
```

#### 3.2 PDF Tests
**File:** `test/integration/files/pdf_test.go` (NEW)

```go
func TestPDFUploadAndUse(t *testing.T)
func TestPDFSizeValidation(t *testing.T)
func TestPDFMalformedResponse(t *testing.T)
```

**Safety Measures:**
- Uses existing Files API infrastructure
- No changes to message routing
- Provider-specific handling isolated to transformers

---

### Phase 4: Computer Use (Future - Out of Scope for Initial Fix)

**Status:** DEFERRED
- Computer Use API is still in beta
- Requires significant architectural changes
- Needs sandbox environment
- Requires security review

**Recommendation:** Monitor Anthropic API for Computer Use stability before implementing.

---

## Part 4: Implementation Order & Dependencies

```
Phase 1: Image Tests
├── 1.1 Format tests (no dependencies)
├── 1.2 Size validation (no dependencies)
├── 1.3 Streaming tests (needs existing streaming code)
└── 1.4 Provider tests (needs provider transformers)

Phase 2: Files API
├── 2.1 Types (no dependencies)
├── 2.2 Handler (needs types)
└── 2.3 Tests (needs handler)

Phase 3: PDF Support
├── 3.1 Types (needs Files API)
└── 3.2 Tests (needs Files API + types)

Phase 4: Computer Use (DEFERRED)
```

---

## Part 5: Safety Checklist

### Before Implementation
- [ ] All existing tests pass
- [ ] No breaking changes to existing transformers
- [ ] No changes to message routing logic
- [ ] Configurable feature flags for new endpoints

### During Implementation
- [ ] Each feature tested in isolation
- [ ] Mock servers used for integration tests
- [ ] No real API calls in test suite
- [ ] Gradual rollout with feature flags

### After Implementation
- [ ] All existing tests still pass
- [ ] New tests have >80% coverage
- [ ] No regression in existing functionality
- [ ] Documentation updated

---

## Part 6: Testing Strategy

### Unit Tests
- Type marshaling/unmarshaling
- Validation logic
- Error handling

### Integration Tests
- End-to-end request/response
- Provider-specific quirks
- Error scenarios

### Regression Tests
- Existing functionality preserved
- No breaking changes
- Backward compatibility

---

## Part 7: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Breaking existing image handling | Low | High | Comprehensive regression tests |
| Files API conflicts with messages | Low | Medium | Separate handlers, distinct routes |
| PDF support exceeds scope | Medium | Medium | Phase-based approach, defer if needed |
| Computer use complexity | High | High | Deferred to future phase |

---

## Part 8: Success Criteria

### Phase 1 (Image)
- [ ] 80% test coverage for image handling
- [ ] All providers tested for image support
- [ ] Streaming with images tested
- [ ] Size validation tested

### Phase 2 (Files API)
- [ ] Basic file upload/download working
- [ ] 70% test coverage
- [ ] No impact on messages endpoint

### Phase 3 (PDF)
- [ ] PDF upload supported
- [ ] PDF content tested in messages
- [ ] Size validation tested

---

## Part 9: References

1. **Claude API Documentation:** https://platform.claude.com/docs/en/home
2. **OpenRouter API Documentation:** https://openrouter.ai/openapi.yaml
3. **BigModel API Documentation:** https://docs.z.ai/api-reference/introduction

---

## Appendix A: File Structure

```
cc-modelrouter/
├── pkg/api/anthropic/
│   ├── types.go (existing)
│   ├── types_test.go (existing)
│   ├── types_files.go (NEW)
│   └── types_image_test.go (NEW)
├── internal/proxy/
│   ├── handler.go (existing)
│   ├── handler_test.go (existing)
│   ├── files_handler.go (NEW)
│   ├── files_handler_test.go (NEW)
│   └── handler_image_test.go (NEW)
├── internal/transformer/transformers/
│   ├── image_streaming_test.go (NEW)
│   └── ... (existing transformers)
└── test/integration/
    ├── images/
    │   └── provider_image_test.go (NEW)
    └── files/
        ├── pdf_test.go (NEW)
        └── files_api_test.go (NEW)
```

---

## Appendix B: Key Implementation Notes

### Image Handling
- Images already supported via `ImageSource` struct
- Gemini transformer handles conversion
- Need to add OpenAI, OpenRouter, GLM tests

### Files API
- New endpoints: `/v1/files`, `/v1/files/{id}`
- Multipart form data for uploads
- Separate from messages endpoint

### PDF Support
- Extension of Files API
- Uses same upload mechanism
- Special content type in messages

### Computer Use
- Requires `computer_use` tool type
- Needs screenshot/coordinate handling
- Out of scope for initial fix
