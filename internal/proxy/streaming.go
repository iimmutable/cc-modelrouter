package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

// ParseSSEEvent parses an SSE line (or multi-line string) into event type and data.
func ParseSSEEvent(line string) (event string, data []byte, err error) {
	// Handle multi-line input by splitting on newlines
	lines := strings.Split(line, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(l, "event:"))
		} else if strings.HasPrefix(l, "data:") {
			data = []byte(strings.TrimSpace(strings.TrimPrefix(l, "data:")))
		}
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
	scanner := bufio.NewScanner(r)
	// Increase max token size from default 64KB (bufio.MaxScanTokenSize) to 1MB
	// This ensures large SSE payloads (e.g., long AI responses) are handled correctly
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &SSEScanner{
		scanner: scanner,
	}
}

// Scan advances to the next event.
func (s *SSEScanner) Scan() bool {
	s.event = ""
	s.data = nil

	var eventData strings.Builder
	var hasData bool

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if line == "" {
			// Empty line marks end of event
			if hasData {
				s.data = []byte(eventData.String())
				hasData = false
				return true
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			s.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			// Accumulate data lines - trim leading space after "data:" prefix
			// but preserve internal whitespace for multi-line JSON
			dataContent := strings.TrimLeft(strings.TrimPrefix(line, "data:"), " ")
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(dataContent)
			hasData = true
		}
	}

	// Handle case where stream ends without trailing newline
	if hasData {
		s.data = []byte(eventData.String())
		return true
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

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// Note: json import is reserved for future use in streaming message parsing
var _ json.RawMessage
