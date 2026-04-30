# File Resolution Layer Implementation Plan

**Task #13**: Implement file resolution layer for transformers to resolve file_id references in document blocks.

**Status**: Research Complete - Awaiting Implementation Approval

**Created**: 2026-03-06

---

## Executive Summary

This document provides a comprehensive implementation plan for adding a file resolution layer to the cc-modelrouter. This layer will enable transformers to fetch actual file content referenced by `file_id` in document blocks and inline it for providers that don't support Anthropic's Files API.

### Problem Statement

Currently, when a Messages API request contains document blocks with `file_id` references:
```json
{
  "type": "document",
  "source": {
    "type": "file",
    "file_id": "file-abc123"
  },
  "title": "Report.pdf",
  "context": "Q3 financial report"
}
```

Transformers for non-Anthropic providers (OpenAI, Gemini, GLM, etc.) only insert placeholder text:
```
[Document: Report.pdf - file_id: file-abc123]
```

This prevents those providers from accessing the actual document content.

### Solution Overview

Implement a FileStore abstraction that:
1. Stores file content uploaded via Files API
2. Retrieves files by file_id for transformer use
3. Converts content to base64 for providers without file_id support
4. Maintains security with workspace-scoped access
5. Provides caching for frequently accessed files

---

## Phase 1: Architecture Design

### 1.1 Core Components

```
┌─────────────────────────────────────────────────────────────────┐
│                         Handler Layer                           │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │ Files API   │  │ Messages API │  │  Usage Tracker         │  │
│  │ Handlers    │  │ Handler      │  │  (existing pattern)    │  │
│  └──────┬──────┘  └──────┬───────┘  └────────────────────────┘  │
│         │                 │                                          │
│         ▼                 ▼                                          │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                    FileStore Interface                     │   │
│  │  - Store(file) -> file_id                                  │   │
│  │  - Get(file_id) -> FileContent                             │   │
│  │  - Delete(file_id) -> error                                │   │
│  │  - List(workspace) -> []FileObject                         │   │
│  └────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Storage Implementations                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ SQLite Store │  │ File System  │  │ S3/Object Store      │  │
│  │ (default)    │  │ Store        │  │ (future)             │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Transformer Integration                      │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  PrepareRequest(req, baseURL, apiKey, model, fileStore?)  │  │
│  │           - For each document block:                      │  │
│  │           - Check if source.type == "file"                │  │
│  │           - Fetch content from FileStore                  │  │
│  │           - Convert to provider-specific format           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 FileStore Interface

```go
// Package storage

// FileContent represents stored file content.
type FileContent struct {
    ID           string
    Filename     string
    MimeType     string
    SizeBytes    int64
    Content      []byte           // Raw bytes
    Base64Data   string           // Pre-computed base64 for performance
    WorkspaceID  string           // For scoping
    CreatedAt    time.Time
    ExpiresAt    *time.Time       // Optional expiration
    Metadata     map[string]string // Additional metadata
}

// FileStore defines the interface for file storage operations.
type FileStore interface {
    // Store stores a new file and returns its file_id.
    Store(ctx context.Context, file *FileContent) (string, error)

    // Get retrieves file content by file_id.
    Get(ctx context.Context, fileID string) (*FileContent, error)

    // Delete removes a file by file_id.
    Delete(ctx context.Context, fileID string) error

    // List returns all files in a workspace.
    List(ctx context.Context, workspaceID string, limit, offset int) ([]FileContent, error)

    // Exists checks if a file exists.
    Exists(ctx context.Context, fileID string) (bool, error)

    // Close closes any underlying connections.
    Close() error
}
```

### 1.3 Storage Options Analysis

| Option | Pros | Cons | Recommendation |
|--------|------|------|----------------|
| **SQLite** | • Single file<br>• ACID transactions<br>• Easy backup<br>• Proven pattern (usage tracker)<br>• Good for <500MB files | • Size limits (~2GB DB)<br>• Concurrency limits | ✅ **START HERE**<br>Good for MVP, proven pattern |
| **File System** | • No size limits<br>• Direct access<br>• Simple | • Cleanup complexity<br>• Permission issues<br>• Harder to backup | ⚠️ **Phase 2**<br>If SQLite limits hit |
| **S3/Object Store** | • Unlimited scale<br>• CDN integration<br>• Professional | • External dependency<br>• Network latency<br>• Cost | ❌ **Later**<br>Only if enterprise scale needed |

### 1.4 Transformer Integration Pattern

```go
// Updated Transformer interface (backward compatible)
type Transformer interface {
    Name() string
    Endpoint() string

    // Existing methods
    PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*anthropic.Response, error)
    SupportsStreaming() bool
    TransformStreamEvent(event *SSEEvent) ([]SSEEvent, error)
}

// New optional interface for file-aware transformers
type FileAwareTransformer interface {
    Transformer
    SetFileStore(store FileStore)
}

// Handler integration
func (h *Handler) SetFileStore(store storage.FileStore) {
    h.fileStore = store

    // Inject into transformers that support it
    for _, transformer := range h.transformerRegistry.GetAll() {
        if fa, ok := transformer.(FileAwareTransformer); ok {
            fa.SetFileStore(store)
        }
    }
}
```

---

## Phase 2: Configuration

### 2.1 File Storage Configuration

```json
{
  "fileStorage": {
    "backend": "sqlite",           // "sqlite" | "filesystem" | "s3"
    "sqlite": {
      "path": "${HOME}/.cc-modelrouter/files.db",
      "maxFileSizeBytes": 524288000  // 500MB per Anthropic spec
    },
    "filesystem": {
      "directory": "${HOME}/.cc-modelrouter/files",
      "maxFileSizeBytes": 524288000
    },
    "retention": {
      "enabled": true,
      "maxAgeHours": 720,           // 30 days
      "cleanupIntervalHours": 24
    }
  }
}
```

### 2.2 Environment Variables

```
CC_MODELROUTER_FILE_STORAGE_PATH=~/.cc-modelrouter/files.db
CC_MODELROUTER_FILE_RETENTION_HOURS=720
CC_MODELROUTER_MAX_FILE_SIZE_BYTES=524288000
```

---

## Phase 3: Security Considerations

### 3.1 Workspace Scoping

Files MUST be scoped to workspace to prevent cross-workspace access:

```go
type FileContent struct {
    // ... other fields
    WorkspaceID  string    // REQUIRED: Claude Code workspace
    // ...
}
```

### 3.2 Access Control

1. **File ID format**: `file-{workspace_short_id}-{random}` for validation
2. **Get operations**: Always verify workspace ID matches
3. **List operations**: Only return files for requesting workspace
4. **Delete operations**: Workspace-scoped only

### 3.3 Validation

```go
// Validate file before storage
func ValidateFile(file *FileContent) error {
    // Size limit
    if file.SizeBytes > MaxFileSize {
        return fmt.Errorf("file exceeds %d byte limit", MaxFileSize)
    }

    // MIME type whitelist
    if !IsAllowedMimeType(file.MimeType) {
        return fmt.Errorf("unsupported MIME type: %s", file.MimeType)
    }

    // Filename sanitization
    if !IsValidFilename(file.Filename) {
        return fmt.Errorf("invalid filename: %s", file.Filename)
    }

    return nil
}
```

### 3.4 MIME Type Whitelist

```go
var AllowedMimeTypes = map[string]bool{
    // Documents
    "application/pdf":           true,
    "text/plain":               true,
    "text/markdown":            true,
    "application/msword":       true,
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,

    // Images
    "image/jpeg":               true,
    "image/png":                true,
    "image/gif":                true,
    "image/webp":               true,

    // Data
    "application/json":         true,
    "text/csv":                 true,
    "application/xml":          true,
}
```

---

## Phase 4: Implementation Phases (TDD Approach)

### Phase 4.1: SQLite Storage Implementation

**Test First:**

```go
// internal/storage/sqlite_store_test.go
func TestSQLiteStore_StoreAndGet(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    defer store.Close()

    file := &FileContent{
        Filename:    "test.pdf",
        MimeType:    "application/pdf",
        Content:     []byte("test content"),
        WorkspaceID: "ws-123",
    }

    fileID, err := store.Store(ctx, file)
    require.NoError(t, err)
    assert.NotEmpty(t, fileID)

    retrieved, err := store.Get(ctx, fileID)
    require.NoError(t, err)
    assert.Equal(t, file.Filename, retrieved.Filename)
    assert.Equal(t, file.Content, retrieved.Content)
}
```

**Then Implement:**

```go
// internal/storage/sqlite_store.go
type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    if err := createSchema(db); err != nil {
        return nil, err
    }

    return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Store(ctx context.Context, file *FileContent) (string, error) {
    fileID := generateFileID(file.WorkspaceID)

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return "", err
    }
    defer tx.Rollback()

    _, err = tx.ExecContext(ctx, `
        INSERT INTO files (id, filename, mime_type, content, base64_data,
                          workspace_id, created_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, fileID, file.Filename, file.MimeType, file.Content,
       file.Base64Data, file.WorkspaceID, time.Now(), file.ExpiresAt)

    if err != nil {
        return "", err
    }

    return fileID, tx.Commit()
}
```

### Phase 4.2: Handler Integration

**Test First:**

```go
// internal/proxy/files_handler_integration_test.go
func TestFileUploadAndStorage(t *testing.T) {
    store := setupTestStore(t)
    defer store.Close()

    handler := NewHandler(50 * 1024 * 1024)
    handler.SetFileStore(store)
    setupHandler(t, handler, "test-file-storage")

    // Upload file
    req := httptest.NewRequest("POST", "/v1/files",
        strings.NewReader(`{"filename":"test.pdf","purpose":"vision"}`))
    req.Header.Set("anthropic-beta", FilesAPIBetaVersion)

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var resp anthropic.FileUploadResponse
    json.Unmarshal(w.Body.Bytes(), &resp)

    // Verify file is retrievable
    stored, err := store.Get(context.Background(), resp.ID)
    require.NoError(t, err)
    assert.Equal(t, "test.pdf", stored.Filename)
}
```

### Phase 4.3: Transformer Integration

**Test First:**

```go
// internal/transformer/transformers/file_resolution_test.go
func TestOpenAITransformer_WithFileResolution(t *testing.T) {
    store := setupTestStore(t)
    defer store.Close()

    // Store a test file
    file := &FileContent{
        Filename:    "report.pdf",
        MimeType:    "application/pdf",
        Content:     []byte("test pdf content"),
        Base64Data:  base64.StdEncoding.EncodeToString([]byte("test pdf content")),
        WorkspaceID: "ws-123",
    }
    fileID, _ := store.Store(context.Background(), file)

    transformer := NewOpenAITransformer()
    transformer.SetFileStore(store)

    req := &anthropic.Request{
        Messages: []anthropic.Message{
            {
                Role: "user",
                Content: []anthropic.ContentBlock{
                    {
                        Type: "document",
                        Title: "Q3 Report",
                        DocumentSource: &anthropic.DocumentSource{
                            Type:   "file",
                            FileID: fileID,
                        },
                    },
                },
            },
        },
    }

    httpReq, err := transformer.PrepareRequest(
        req, "https://api.openai.com/v1/chat/completions",
        "test-key", "gpt-4")

    require.NoError(t, err)

    // Verify document content is inlined
    var openAIReq struct {
        Messages []struct {
            Content []interface{} `json:"content"`
        } `json:"messages"`
    }
    json.Unmarshal(httpReq.Body, &openAIReq)

    // Should contain actual content, not placeholder
    content := openAIReq.Messages[0].Content[0].(map[string]interface{})
    assert.NotContains(t, content["text"], "[Document:")
    assert.Contains(t, content["text"], "test pdf content")
}
```

**Then Implement:**

```go
// internal/transformer/transformers/openai.go
type OpenAITransformer struct {
    *transformer.BaseTransformer
    fileStore storage.FileStore
}

func (t *OpenAITransformer) SetFileStore(store storage.FileStore) {
    t.fileStore = store
}

func (t *OpenAITransformer) PrepareRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
    // Resolve file_id references in document blocks
    if t.fileStore != nil {
        t.resolveDocumentBlocks(req)
    }

    // ... existing code
}

func (t *OpenAITransformer) resolveDocumentBlocks(req *anthropic.Request) {
    for msgIdx := range req.Messages {
        for blockIdx := range req.Messages[msgIdx].Content {
            block := &req.Messages[msgIdx].Content[blockIdx]

            if block.Type == "document" && block.DocumentSource != nil {
                if block.DocumentSource.Type == "file" && block.DocumentSource.FileID != "" {
                    // Fetch file content
                    file, err := t.fileStore.Get(context.Background(), block.DocumentSource.FileID)
                    if err != nil {
                        // Log warning, keep placeholder
                        log.Printf("Failed to resolve file_id %s: %v",
                            block.DocumentSource.FileID, err)
                        continue
                    }

                    // Inline content for provider
                    block.CachedContent = fmt.Sprintf(
                        "File: %s\n\nContent:\n%s",
                        file.Filename,
                        string(file.Content),
                    )
                }
            }
        }
    }
}
```

### Phase 4.4: Cleanup and Retention

**Test First:**

```go
func TestFileCleanup(t *testing.T) {
    store := setupTestStore(t)
    defer store.Close()

    // Store old file
    oldFile := &FileContent{
        Filename:    "old.pdf",
        CreatedAt:   time.Now().Add(-48 * time.Hour),
        ExpiresAt:   timePtr(time.Now().Add(-1 * time.Hour)),
        WorkspaceID: "ws-123",
    }
    store.Store(context.Background(), oldFile)

    // Run cleanup
    err := store.CleanupExpired(context.Background())
    require.NoError(t, err)

    // Verify old file is gone
    exists, _ := store.Exists(context.Background(), oldFile.ID)
    assert.False(t, exists)
}
```

---

## Phase 5: Performance Optimization

### 5.1 Caching Strategy

```go
type CachedFileStore struct {
    underlying FileStore
    cache      *lru.Cache  // github.com/hashicorp/golang-lru
}

func NewCachedFileStore(underlying FileStore, cacheSize int) *CachedFileStore {
    cache, _ := lru.New(cacheSize)
    return &CachedFileStore{
        underlying: underlying,
        cache:      cache,
    }
}

func (c *CachedFileStore) Get(ctx context.Context, fileID string) (*FileContent, error) {
    // Check cache first
    if cached, ok := c.cache.Get(fileID); ok {
        return cached.(*FileContent), nil
    }

    // Fetch from underlying store
    file, err := c.underlying.Get(ctx, fileID)
    if err != nil {
        return nil, err
    }

    // Cache for future requests
    c.cache.Add(fileID, file)
    return file, nil
}
```

### 5.2 Lazy Base64 Encoding

```go
type FileContent struct {
    // ... other fields
    Content      []byte
    base64Data   string  // Lazy computed
    base64Cached bool
}

func (f *FileContent) Base64Data() string {
    if !f.base64Cached {
        f.base64Data = base64.StdEncoding.EncodeToString(f.Content)
        f.base64Cached = true
    }
    return f.base64Data
}
```

---

## Phase 6: Risk Analysis

### 6.1 Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|-----------|
| SQLite file size limit | Medium | Medium | Monitor DB size; implement filesystem fallback |
| Concurrent write conflicts | Low | Medium | Use proper transactions; retry logic |
| Memory exhaustion with large files | Medium | High | Stream large files; size limits |
| File ID collisions | Very Low | Critical | Use crypto-random; validation |

### 6.2 Security Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|-----------|
| Cross-workspace file access | Low | Critical | Workspace ID enforcement |
| Malicious file upload | Medium | High | MIME type whitelist; validation |
| DoS via large files | Medium | Medium | Size limits; rate limiting |
| Path traversal in filenames | Low | High | Filename sanitization |

### 6.3 Migration Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|-----------|
| Breaking existing transformers | Low | High | Backward compatible; optional interface |
| Performance regression | Medium | Medium | Benchmarking; caching |
| Data loss during migration | Very Low | Critical | Backup before migration |

---

## Phase 7: Implementation Checklist

### Phase 1: Core Storage (Week 1)
- [ ] Define FileStore interface
- [ ] Implement SQLiteStore with schema
- [ ] Add FileContent types
- [ ] Write unit tests for all operations
- [ ] Add configuration support

### Phase 2: Handler Integration (Week 1)
- [ ] Add FileStore to Handler dependencies
- [ ] Update Files API handlers to use real storage
- [ ] Add workspace ID extraction from context
- [ ] Write integration tests

### Phase 3: Transformer Integration (Week 2)
- [ ] Define FileAwareTransformer interface
- [ ] Update OpenAI transformer
- [ ] Update Gemini transformer
- [ ] Update GLM transformer
- [ ] Write transformer tests with file resolution

### Phase 4: Performance (Week 2)
- [ ] Implement LRU caching
- [ ] Add lazy base64 encoding
- [ ] Benchmark file operations
- [ ] Add metrics/observability

### Phase 5: Production Readiness (Week 3)
- [ ] Add cleanup/retention jobs
- [ ] Implement proper error handling
- [ ] Add security validation
- [ ] Write documentation
- [ ] Load testing

---

## Phase 8: Success Criteria

### Functional Requirements
- [ ] Files uploaded via Files API are stored persistently
- [ ] Transformers can resolve file_id references
- [ ] Document content is properly inlined for non-Anthropic providers
- [ ] Workspace scoping prevents cross-workspace access
- [ ] File size limits are enforced (500MB)

### Non-Functional Requirements
- [ ] File retrieval latency < 50ms (p95) with caching
- [ ] Storage supports at least 10,000 files per workspace
- [ ] Cleanup job runs without performance impact
- [ ] All operations are backward compatible

### Testing Requirements
- [ ] Unit test coverage > 90% for storage layer
- [ ] Integration tests for all transformers
- [ ] Load tests with 1,000 concurrent file operations
- [ ] Security tests for workspace isolation

---

## Appendix A: Database Schema

```sql
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    content BLOB NOT NULL,
    base64_data TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    workspace_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    metadata TEXT,  -- JSON

    CONSTRAINT valid_mime_type CHECK (mime_type IN (
        'application/pdf',
        'text/plain',
        'image/jpeg',
        'image/png',
        -- ... other allowed types
    )),
    CONSTRAINT max_size CHECK (size_bytes <= 524288000)  -- 500MB
);

CREATE INDEX IF NOT EXISTS idx_files_workspace ON files(workspace_id);
CREATE INDEX IF NOT EXISTS idx_files_expires_at ON files(expires_at);

CREATE TABLE IF NOT EXISTS file_usage_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id TEXT NOT NULL,
    accessed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    workspace_id TEXT NOT NULL,

    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_usage_log_file ON file_usage_log(file_id);
CREATE INDEX IF NOT EXISTS idx_usage_log_workspace ON file_usage_log(workspace_id);
```

---

## Appendix B: Migration Path

### From Mock to Real Storage

1. **Deploy with mock storage still active** (current state)
2. **Add SQLite implementation** (non-breaking)
3. **Update handlers to use real storage** (feature flag)
4. **Migrate existing in-memory files** (if any)
5. **Enable real storage** (remove mock)
6. **Monitor metrics** (performance, errors)

### Rollback Plan

If issues occur:
1. Disable file resolution (fallback to placeholders)
2. Keep storage but mark as read-only
3. Revert transformers to placeholder behavior

---

**End of Plan**

---

## Next Steps

Upon approval, proceed with:
1. Create git branch `feature/file-resolution-layer`
2. Implement Phase 4.1 (SQLite Storage) following TDD
3. Proceed through remaining phases
4. Regular status updates after each phase

**Estimated Timeline**: 3 weeks
**Risk Level**: Medium (well-contained, backward compatible)
**Priority**: High (enables full Files API functionality)
