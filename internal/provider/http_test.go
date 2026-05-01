package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient_EmptyBaseURL(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "",
		APIKey:  "test-key",
	}

	client, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error for empty BaseURL")
	}
	if client != nil {
		t.Error("expected nil client for error case")
	}
}

func TestNewClient_WithDefaults(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "https://api.example.com",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	defaults := Defaults()
	if client.maxRetries != defaults.MaxRetries {
		t.Errorf("expected maxRetries %d, got %d", defaults.MaxRetries, client.maxRetries)
	}
	if client.retryDelay != defaults.RetryDelay {
		t.Errorf("expected retryDelay %v, got %v", defaults.RetryDelay, client.retryDelay)
	}
}

func TestNewClient_TimeoutParsing(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    time.Duration
	}{
		{"30 seconds", "30s", 30 * time.Second},
		{"1 minute", "1m", 1 * time.Minute},
		{"500ms", "500ms", 500 * time.Millisecond},
		{"2 hours", "2h", 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ClientConfig{
				BaseURL: "https://api.example.com",
				Timeout: tt.timeout,
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			if client.client.Timeout != tt.want {
				t.Errorf("expected timeout %v, got %v", tt.want, client.client.Timeout)
			}
		})
	}
}

func TestNewClient_IdleConnTimeoutParsing(t *testing.T) {
	tests := []struct {
		name              string
		idleConnTimeout   string
		expectedDuration  time.Duration
	}{
		{"90 seconds", "90s", 90 * time.Second},
		{"2 minutes", "2m", 2 * time.Minute},
		{"5 minutes", "5m", 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ClientConfig{
				BaseURL:         "https://api.example.com",
				IdleConnTimeout: tt.idleConnTimeout,
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			transport := client.client.Transport.(*http.Transport)
			if transport.IdleConnTimeout != tt.expectedDuration {
				t.Errorf("expected idleConnTimeout %v, got %v", tt.expectedDuration, transport.IdleConnTimeout)
			}
		})
	}
}

func TestDo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDo_RetryOn5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDo_RetryOn502(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_RetryOn503(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_RetryOn504(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_NoRetryOn4xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestDo_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err == nil {
		t.Error("expected error after max retries exceeded")
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 10,
		RetryDelay: 100 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(ctx)

	_, err = client.Do(req)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestDo_NetworkError(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:    "http://localhost:99999",
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", "http://localhost:99999/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err == nil {
		t.Error("expected error for network failure")
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func TestDoWithContext_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
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

	ctx := context.Background()
	resp, err := client.DoWithContext(ctx, req)
	if err != nil {
		t.Fatalf("DoWithContext failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDoWithContext_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 10,
		RetryDelay: 100 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = client.DoWithContext(ctx, req)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestDo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		Timeout:    "50ms",
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

	resp, err := client.Do(req)
	if err == nil {
		t.Error("expected timeout error")
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func TestDo_ReadBody(t *testing.T) {
	expectedBody := `{"result":"test"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, string(body))
	}
}

func TestDo_UserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestDo_ErrorFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected error")
	}

	expected := "max retries exceeded"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}

func TestNewClient_CustomMaxIdleConns(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:      "https://api.example.com",
		MaxIdleConns: 50,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	transport := client.client.Transport.(*http.Transport)
	if transport.MaxIdleConns != 50 {
		t.Errorf("expected MaxIdleConns 50, got %d", transport.MaxIdleConns)
	}
}

func TestNewClient_DefaultMaxIdleConns(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "https://api.example.com",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	transport := client.client.Transport.(*http.Transport)
	if transport.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns 100, got %d", transport.MaxIdleConns)
	}
}

func TestNewClient_ZeroMaxIdleConns(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:      "https://api.example.com",
		MaxIdleConns: 0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	transport := client.client.Transport.(*http.Transport)
	if transport.MaxIdleConns != 100 {
		t.Errorf("expected default MaxIdleConns 100, got %d", transport.MaxIdleConns)
	}
}

func TestDo_BodyClosedOnRetry(t *testing.T) {
	bodyClosed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body.Close()
			bodyClosed = true
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Error("expected error")
	}

	if !bodyClosed {
		t.Error("expected body to be closed on retry")
	}
}