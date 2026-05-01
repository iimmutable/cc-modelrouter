package anthropic

import "time"

// FileUploadRequest represents a file upload request.
// This follows the Anthropic Files API specification for uploading files.
type FileUploadRequest struct {
	File     interface{} // Can be io.Reader or string (file path)
	Filename string
	Purpose  string // e.g., "vision", "assistant", "fine-tune"
}

// FileUploadResponse represents a successful file upload response.
// Matches the official Anthropic Files API specification.
type FileUploadResponse struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // "file"
	CreatedAt   time.Time `json:"created_at"`  // RFC 3339 datetime string
	SizeBytes   int64     `json:"size_bytes"`  // Size of the file in bytes
	Filename    string    `json:"filename"`    // Original filename of the uploaded file
	MimeType    string    `json:"mime_type"`   // MIME type of the file
	Downloadable bool     `json:"downloadable"` // Whether the file can be downloaded
}

// FileListResponse represents a list of files.
type FileListResponse struct {
	Object  string       `json:"object"` // "list"
	Data    []FileObject `json:"data"`
	HasMore bool         `json:"has_more"`
}

// FileObject represents a file object in the Files API.
// Matches the official Anthropic Files API specification.
type FileObject struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // "file"
	CreatedAt   time.Time `json:"created_at"`  // RFC 3339 datetime string
	SizeBytes   int64     `json:"size_bytes"`  // Size of the file in bytes
	Filename    string    `json:"filename"`    // Original filename of the uploaded file
	MimeType    string    `json:"mime_type"`   // MIME type of the file
	Downloadable bool     `json:"downloadable"` // Whether the file can be downloaded
}

// FileDeleteResponse represents a file deletion response.
type FileDeleteResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`    // "file"
	Deleted bool   `json:"deleted"`
}

// FileContentResponse represents the content of a file.
type FileContentResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"` // "file.content"
	Content string `json:"content"` // Base64 encoded content
}

// FilePurpose defines valid file purposes.
const (
	FilePurposeVision     = "vision"
	FilePurposeAssistant  = "assistant"
	FilePurposeFineTune   = "fine-tune"
	FilePurposeAssistants = "assistants"
)

// IsValidFilePurpose checks if a purpose string is valid.
func IsValidFilePurpose(purpose string) bool {
	switch purpose {
	case FilePurposeVision, FilePurposeAssistant, FilePurposeFineTune, FilePurposeAssistants:
		return true
	default:
		return false
	}
}

// FileStatus represents the processing status of a file.
type FileStatus string

const (
	FileStatusUploaded   FileStatus = "uploaded"
	FileStatusProcessed  FileStatus = "processed"
	FileStatusError      FileStatus = "error"
	FileStatusPending    FileStatus = "pending"
)

// FileWithStatus extends FileObject with status information.
type FileWithStatus struct {
	FileObject
	Status    FileStatus `json:"status"`
	Error     *FileError `json:"error,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// FileError represents an error that occurred during file processing.
type FileError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
