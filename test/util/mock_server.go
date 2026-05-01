// Package util provides testing utilities for integration tests.
package util

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockServer is a configurable mock HTTP server for testing.
type MockServer struct {
	server      *httptest.Server
	requests    []*http.Request
	mu          sync.Mutex
	responses   map[string]*MockResponse
	delay       time.Duration
	errorOn     int // Simulate error on Nth request (0-based)
	errorStatus int // Status code for error simulation
}

// MockResponse represents a mock response configuration.
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Delay      time.Duration
}

// NewMockServer creates a new mock HTTP server.
func NewMockServer() *MockServer {
	ms := &MockServer{
		responses:   make(map[string]*MockResponse),
		errorStatus: http.StatusInternalServerError,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ms.handleRequest)

	ms.server = httptest.NewServer(mux)
	return ms
}

// handleRequest handles incoming requests to the mock server.
func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Record the request
	ms.mu.Lock()
	ms.requests = append(ms.requests, r)
	requestNum := len(ms.requests) - 1
	ms.mu.Unlock()

	// Check if we should simulate an error
	ms.mu.Lock()
	errorOn := ms.errorOn
	errorStatus := ms.errorStatus
	ms.mu.Unlock()

	if errorOn >= 0 && requestNum == errorOn {
		w.WriteHeader(errorStatus)
		w.Write([]byte(fmt.Sprintf(`{"error":{"message":"Simulated error","type":"error"}}`)))
		return
	}

	// Apply delay if configured
	if ms.delay > 0 {
		time.Sleep(ms.delay)
	}

	// Find response for this path
	ms.mu.Lock()
	mockResp, ok := ms.responses[r.URL.Path]
	ms.mu.Unlock()

	if !ok {
		// Try to match by path prefix
		for path, resp := range ms.responses {
			if strings.HasPrefix(r.URL.Path, path) {
				mockResp = resp
				break
			}
		}
	}

	if mockResp != nil {
		// Apply response-specific delay
		if mockResp.Delay > 0 {
			time.Sleep(mockResp.Delay)
		}

		// Set headers
		for k, v := range mockResp.Headers {
			w.Header().Set(k, v)
		}

		w.WriteHeader(mockResp.StatusCode)
		io.WriteString(w, mockResp.Body)
		return
	}

	// Default response
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"id":"test-id","type":"message","role":"assistant","content":[{"type":"text","text":"Hello from mock server"}],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
}

// URL returns the mock server's URL.
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// SetResponse sets a mock response for a specific path.
func (ms *MockServer) SetResponse(path string, statusCode int, body string, headers map[string]string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.responses[path] = &MockResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers:    headers,
	}
}

// SetDelay sets a delay for all responses.
func (ms *MockServer) SetDelay(delay time.Duration) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.delay = delay
}

// SetErrorOn sets which request number should return an error.
func (ms *MockServer) SetErrorOn(n int, status int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.errorOn = n
	ms.errorStatus = status
}

// GetRequests returns all recorded requests.
func (ms *MockServer) GetRequests() []*http.Request {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.requests
}

// GetRequestCount returns the number of recorded requests.
func (ms *MockServer) GetRequestCount() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.requests)
}

// Reset clears all recorded requests and error state.
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.requests = make([]*http.Request, 0)
	ms.errorOn = -1
}

// ClearResponses clears all configured responses.
func (ms *MockServer) ClearResponses() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.responses = make(map[string]*MockResponse)
}

// Close closes the mock server.
func (ms *MockServer) Close() {
	ms.server.Close()
}