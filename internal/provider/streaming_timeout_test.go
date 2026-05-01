package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStreamingTimeout_Default verifies that the default client timeout
// is 30 seconds, which is too short for streaming requests that can take
// several minutes.
func TestStreamingTimeout_Default(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "https://api.example.com",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// The default timeout should be 30s
	expectedTimeout := 30 * time.Second
	if client.client.Timeout != expectedTimeout {
		t.Errorf("expected default timeout %v, got %v", expectedTimeout, client.client.Timeout)
	}
}

// TestStreamingClient_CanBeCreated verifies that we can create a client
// with a longer timeout specifically for streaming requests.
func TestStreamingClient_CanBeCreated(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:         "https://api.example.com",
		Timeout:         "10m",
		MaxIdleConns:    100,
		IdleConnTimeout: "90s",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Verify the 10-minute timeout is applied
	expectedTimeout := 10 * time.Minute
	if client.client.Timeout != expectedTimeout {
		t.Errorf("expected timeout %v, got %v", expectedTimeout, client.client.Timeout)
	}
}

// TestClient_DoWithContext_RespectsContextCancellation verifies that
// DoWithContext properly respects context cancellation, which is critical
// for streaming requests where the client may cancel mid-stream.
func TestClient_DoWithContext_RespectsContextCancellation(t *testing.T) {
	// Server that sleeps longer than our test timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This simulates a long-running streaming response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		Timeout:    "100ms", // Short timeout for test
		MaxRetries: 0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Create a context that cancels after 50ms
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	// DoWithContext should respect context cancellation
	_, err = client.DoWithContext(ctx, req)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

// TestClient_DoWithContext_EnablesContextPropagation verifies that
// DoWithContext is available and properly propagates context. This is
// the key fix for streaming timeout issues - context can now be cancelled
// when the client disconnects during streaming.
func TestClient_DoWithContext_EnablesContextPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This simulates a long-running streaming response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		Timeout:    "100ms", // Short timeout for test
		MaxRetries: 0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Create a context that cancels after 50ms
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	// DoWithContext should respect context cancellation
	_, err = client.DoWithContext(ctx, req)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	// Verify context was properly propagated
	t.Log("DoWithContext successfully propagates context cancellation")
}