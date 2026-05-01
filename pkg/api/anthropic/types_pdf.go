package anthropic

// PDFContent represents PDF content in message blocks.
// PDF files can be referenced by file ID after being uploaded via the Files API.
type PDFContent struct {
	Type     string     `json:"type"`     // "pdf"
	FileID   string     `json:"file_id"`  // ID of uploaded PDF file
	Source   *PDFSource `json:"source,omitempty"`
}

// PDFSource represents the source of PDF content.
type PDFSource struct {
	Type      string `json:"type"`       // "base64" or "file_id"
	Data      string `json:"data"`       // Base64 encoded PDF data (if type is "base64")
	FileID    string `json:"file_id"`    // File ID (if type is "file_id")
	MediaType string `json:"media_type"` // e.g., "application/pdf"
}

// PDFPage represents a single page in a PDF document.
type PDFPage struct {
	PageNumber int     `json:"page_number"`
	Image      string  `json:"image"`       // Base64 encoded page image
	Text       string  `json:"text"`        // Extracted text from page
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

// PDFDocument represents a PDF document with pages.
type PDFDocument struct {
	ID          string     `json:"id"`
	Filename    string     `json:"filename"`
	PageCount   int        `json:"page_count"`
	Pages       []PDFPage  `json:"pages,omitempty"`
	Metadata    PDFMetadata `json:"metadata"`
	CreatedAt   int64      `json:"created_at"` // Unix timestamp
}

// PDFMetadata contains metadata about a PDF document.
type PDFMetadata struct {
	Title      string  `json:"title,omitempty"`
	Author     string  `json:"author,omitempty"`
	Subject    string  `json:"subject,omitempty"`
	Keywords   string  `json:"keywords,omitempty"`
	Creator    string  `json:"creator,omitempty"`
	Producer   string  `json:"producer,omitempty"`
	Created    int64   `json:"created,omitempty"`
	Modified   int64   `json:"modified,omitempty"`
	PageCount  int     `json:"page_count"`
}

// PDFExtractionRequest represents a request to extract content from a PDF.
type PDFExtractionRequest struct {
	FileID      string   `json:"file_id"`
	PageNumbers []int    `json:"page_numbers,omitempty"` // null means all pages
	ExtractText bool     `json:"extract_text"`
	ExtractImages bool   `json:"extract_images"`
}

// PDFExtractionResponse represents the result of PDF content extraction.
type PDFExtractionResponse struct {
	ID       string        `json:"id"`
	FileID   string        `json:"file_id"`
	Pages    []PDFPage     `json:"pages"`
	Metadata PDFMetadata   `json:"metadata"`
	Status   string        `json:"status"`
}

// PDFPageContent represents content extracted from a single PDF page.
type PDFPageContent struct {
	PageNumber int      `json:"page_number"`
	Text       string   `json:"text,omitempty"`
	Images     []string `json:"images,omitempty"` // Base64 encoded images
	Tables     []string `json:"tables,omitempty"` // Extracted tables as JSON
}

// IsValidMediaType checks if the media type is a valid PDF type.
func IsValidPDFMediaType(mediaType string) bool {
	switch mediaType {
	case "application/pdf", "application/x-pdf", "text/pdf":
		return true
	default:
		return false
	}
}

// MaxPDFFileSize represents the maximum size for PDF uploads (100MB).
const MaxPDFFileSize int64 = 100 * 1024 * 1024

// MaxPDFPages represents the maximum number of pages allowed for processing.
const MaxPDFPages int = 1000
