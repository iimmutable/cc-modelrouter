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

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// Note: json import is reserved for future use in streaming message parsing
var _ json.RawMessage
