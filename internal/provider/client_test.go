package provider

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		Timeout: "30s",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestNewClient_MissingBaseURL(t *testing.T) {
	cfg := &ClientConfig{
		APIKey:  "test-key",
		Timeout: "30s",
	}

	client, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error for missing baseURL")
	}
	if client != nil {
		t.Error("expected nil client for error case")
	}
}

func TestNewClient_Defaults(t *testing.T) {
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

	// Verify defaults were applied
	defaults := Defaults()
	if client.maxRetries != defaults.MaxRetries {
		t.Errorf("expected maxRetries %d, got %d", defaults.MaxRetries, client.maxRetries)
	}
	if client.retryDelay != defaults.RetryDelay {
		t.Errorf("expected retryDelay %v, got %v", defaults.RetryDelay, client.retryDelay)
	}
}

func TestDefaults(t *testing.T) {
	defaults := Defaults()

	if defaults.Timeout != "30s" {
		t.Errorf("expected Timeout '30s', got %s", defaults.Timeout)
	}
	if defaults.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns 100, got %d", defaults.MaxIdleConns)
	}
	if defaults.IdleConnTimeout != "90s" {
		t.Errorf("expected IdleConnTimeout '90s', got %s", defaults.IdleConnTimeout)
	}
	if defaults.MaxRetries != 2 {
		t.Errorf("expected MaxRetries 2, got %d", defaults.MaxRetries)
	}
	if defaults.RetryDelay != 500*time.Millisecond {
		t.Errorf("expected RetryDelay 500ms, got %v", defaults.RetryDelay)
	}
}

func TestNewClient_CustomConfig(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:         "https://api.example.com",
		Timeout:         "60s",
		MaxIdleConns:    50,
		IdleConnTimeout: "120s",
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify custom values were applied
	if client.maxRetries != 3 {
		t.Errorf("expected maxRetries 3, got %d", client.maxRetries)
	}
	if client.retryDelay != 1*time.Second {
		t.Errorf("expected retryDelay 1s, got %v", client.retryDelay)
	}
}
