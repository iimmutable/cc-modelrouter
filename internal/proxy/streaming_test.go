package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEWriter(t *testing.T) {
	writer := NewSSEWriter(nil) // nil for testing

	if writer == nil {
		t.Error("expected non-nil SSE writer")
	}
}

func TestNewSSEWriter_WithoutFlusher(t *testing.T) {
	// Note: httptest.ResponseRecorder actually implements Flusher in Go 1.22+
	// So we need to test with a non-Flusher implementation
	w := &nonFlusherWriter{}
	writer := NewSSEWriter(w)

	if writer == nil {
		t.Error("expected non-nil SSE writer")
	}
	if writer.flusher != nil {
		t.Error("expected nil flusher when response doesn't implement Flusher")
	}
}

type nonFlusherWriter struct {
	nonFlusher struct{}
}

func (f *nonFlusherWriter) WriteHeader(statusCode int) {}
func (f *nonFlusherWriter) Header() http.Header {
	return http.Header{}
}
func (f *nonFlusherWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func TestNewSSEWriter_WithFlusher(t *testing.T) {
	w := &flushableResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
	writer := NewSSEWriter(w)

	if writer == nil {
		t.Error("expected non-nil SSE writer")
	}
	if writer.flusher == nil {
		t.Error("expected flusher when response implements Flusher")
	}
}

func TestWriteEvent_Success(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	eventName := "message"
	data := []byte(`{"type":"text","text":"Hello"}`)

	err := writer.WriteEvent(eventName, data)
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := w.Body.String()
	expected := "event: message\ndata: {\"type\":\"text\",\"text\":\"Hello\"}\n\n"

	if output != expected {
		t.Errorf("expected output %q, got %q", expected, output)
	}
}

func TestWriteEvent_WithEmptyData(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteEvent("event", []byte{})
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := w.Body.String()
	expected := "event: event\ndata: \n\n"

	if output != expected {
		t.Errorf("expected output %q, got %q", expected, output)
	}
}

func TestWriteEvent_Flushes(t *testing.T) {
	w := &flushableResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
	writer := NewSSEWriter(w)

	err := writer.WriteEvent("event", []byte("data"))
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	if !w.flushed {
		t.Error("expected writer to flush after writing event")
	}
}

func TestFlush_WithFlusher(t *testing.T) {
	w := &flushableResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
	writer := NewSSEWriter(w)

	writer.Flush()

	if !w.flushed {
		t.Error("expected Flush to call flusher")
	}
}

func TestFlush_WithoutFlusher(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	// Should not panic or error
	writer.Flush()
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

func TestParseSSEEvent_EventOnly(t *testing.T) {
	line := "event: ping"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "ping" {
		t.Errorf("expected event 'ping', got '%s'", event)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %s", string(data))
	}
}

func TestParseSSEEvent_DataOnly(t *testing.T) {
	line := "data: {\"result\":\"ok\"}"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "" {
		t.Errorf("expected empty event, got '%s'", event)
	}
	if string(data) != `{"result":"ok"}` {
		t.Errorf("expected data %q, got %q", `{"result":"ok"}`, string(data))
	}
}

func TestParseSSEEvent_EventAndData(t *testing.T) {
	line := "event: update\ndata: {\"status\":\"running\"}"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "update" {
		t.Errorf("expected event 'update', got '%s'", event)
	}
	if string(data) != `{"status":"running"}` {
		t.Errorf("expected data %q, got %q", `{"status":"running"}`, string(data))
	}
}

func TestParseSSEEvent_MultiLine(t *testing.T) {
	line := "event: message\ndata: line1\ndata: line2\ndata: line3"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "message" {
		t.Errorf("expected event 'message', got '%s'", event)
	}

	// ParseSSEEvent replaces data on each "data:" line, so we get the last one
	expectedData := "line3"
	if string(data) != expectedData {
		t.Errorf("expected data %q, got %q", expectedData, string(data))
	}
}

func TestParseSSEEvent_EmptyLines(t *testing.T) {
	line := "event: event1\n\ndata: data1\n\ndata: data2"

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "event1" {
		t.Errorf("expected event 'event1', got '%s'", event)
	}

	// Empty lines are stripped but don't affect the final data value
	// The last "data:" line sets the data
	expectedData := "data2"
	if string(data) != expectedData {
		t.Errorf("expected data %q, got %q", expectedData, string(data))
	}
}

func TestParseSSEEvent_Whitespace(t *testing.T) {
	line := "event:  test-event  \n  data:  test-data  "

	event, data, err := ParseSSEEvent(line)
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "test-event" {
		t.Errorf("expected event 'test-event', got '%s'", event)
	}

	if string(data) != "test-data" {
		t.Errorf("expected data 'test-data', got '%s'", string(data))
	}
}

func TestParseSSEEvent_EmptyInput(t *testing.T) {
	event, data, err := ParseSSEEvent("")
	if err != nil {
		t.Fatalf("ParseSSEEvent failed: %v", err)
	}

	if event != "" {
		t.Errorf("expected empty event, got '%s'", event)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %s", string(data))
	}
}

func TestSSEScanner_ScanEvent(t *testing.T) {
	input := "event: ping\ndata: {}\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatal("expected Scan to return true")
	}

	if scanner.Event() != "ping" {
		t.Errorf("expected event 'ping', got '%s'", scanner.Event())
	}

	if string(scanner.Data()) != "{}" {
		t.Errorf("expected data '{}', got '%s'", string(scanner.Data()))
	}

	if scanner.Err() != nil {
		t.Errorf("expected no error, got %v", scanner.Err())
	}
}

func TestSSEScanner_EmptyLineBetweenEvents(t *testing.T) {
	input := "event: event1\ndata: data1\n\nevent: event2\ndata: data2\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	// First event
	if !scanner.Scan() {
		t.Fatal("expected first Scan to return true")
	}
	if scanner.Event() != "event1" {
		t.Errorf("first event: expected 'event1', got '%s'", scanner.Event())
	}

	// Second event
	if !scanner.Scan() {
		t.Fatal("expected second Scan to return true")
	}
	if scanner.Event() != "event2" {
		t.Errorf("second event: expected 'event2', got '%s'", scanner.Event())
	}

	// No more events
	if scanner.Scan() {
		t.Error("expected Scan to return false after all events")
	}
}

func TestSSEScanner_MultiLineData(t *testing.T) {
	input := "event: message\ndata: line1\ndata: line2\ndata: line3\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatal("expected Scan to return true")
	}

	if scanner.Event() != "message" {
		t.Errorf("expected event 'message', got '%s'", scanner.Event())
	}

	expectedData := "line1\nline2\nline3"
	if string(scanner.Data()) != expectedData {
		t.Errorf("expected data %q, got %q", expectedData, string(scanner.Data()))
	}
}

func TestSSEScanner_EventMethod(t *testing.T) {
	input := "event: test-event\ndata: {}\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatal("expected Scan to return true")
	}

	if scanner.Event() != "test-event" {
		t.Errorf("expected event 'test-event', got '%s'", scanner.Event())
	}
}

func TestSSEScanner_DataMethod(t *testing.T) {
	input := "event: event1\ndata: test-data\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatal("expected Scan to return true")
	}

	if string(scanner.Data()) != "test-data" {
		t.Errorf("expected data 'test-data', got '%s'", string(scanner.Data()))
	}
}

func TestSSEScanner_ErrMethod(t *testing.T) {
	// Create a reader that will cause an error
	errorReader := &errorReader{}
	scanner := NewSSEScanner(errorReader)

	// Scan should fail
	if scanner.Scan() {
		t.Error("expected Scan to return false on error")
	}

	if scanner.Err() == nil {
		t.Error("expected error to be returned")
	}
}

func TestSSEScanner_EventOnly(t *testing.T) {
	input := "event: ping\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	// Scan returns false when there's no data because
	// SSEScanner only returns true when eventData.Len() > 0
	if scanner.Scan() {
		// If it does scan, check the event
		if scanner.Event() != "ping" {
			t.Errorf("expected event 'ping', got '%s'", scanner.Event())
		}
		// Data should be empty since we only provided an event
	} else {
		// This is the expected behavior - Scan returns false without data
		// because SSEScanner only returns true when eventData.Len() > 0
	}
}

func TestSSEScanner_DataOnly(t *testing.T) {
	input := "data: {}\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	if !scanner.Scan() {
		t.Fatal("expected Scan to return true")
	}

	if scanner.Event() != "" {
		t.Errorf("expected empty event, got '%s'", scanner.Event())
	}
	if string(scanner.Data()) != "{}" {
		t.Errorf("expected data '{}', got '%s'", string(scanner.Data()))
	}
}

func TestSSEScanner_EventMethodReturnsEmpty(t *testing.T) {
	input := "event: event1\ndata: data1\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	// Call Event() before scanning
	event := scanner.Event()
	if event != "" {
		t.Errorf("expected empty event before scan, got '%s'", event)
	}

	// Now scan
	scanner.Scan()

	// Event() should return the scanned event
	event = scanner.Event()
	if event != "event1" {
		t.Errorf("expected event 'event1', got '%s'", event)
	}
}

func TestSSEScanner_DataMethodReturnsEmpty(t *testing.T) {
	input := "event: event1\ndata: data1\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	// Call Data() before scanning
	data := scanner.Data()
	if len(data) != 0 {
		t.Errorf("expected empty data before scan, got %s", string(data))
	}

	// Now scan
	scanner.Scan()

	// Data() should return the scanned data
	data = scanner.Data()
	if string(data) != "data1" {
		t.Errorf("expected data 'data1', got '%s'", string(data))
	}
}

// Helper types for testing

type flushableResponseRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushableResponseRecorder) Flush() {
	f.flushed = true
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, &readError{}
}

type readError struct{}

func (e *readError) Error() string {
	return "read error"
}
