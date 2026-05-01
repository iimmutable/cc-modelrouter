// Package provider handles communication with LLM providers.
package provider

import (
	"net/http"
	"time"
)

// ClientConfig represents client configuration.
type ClientConfig struct {
	BaseURL           string
	APIKey            string
	Timeout           string
	MaxIdleConns      int
	IdleConnTimeout   string
	MaxRetries        int
	RetryDelay        time.Duration
	DisableKeepAlives bool // Disable HTTP keep-alive for providers with connection issues (e.g., BigModel)
}

// Client is the interface for provider clients.
type Client interface {
	// Do executes an HTTP request.
	Do(req *http.Request) (*http.Response, error)
}

// Defaults returns default client configuration.
func Defaults() *ClientConfig {
	return &ClientConfig{
		Timeout:         "30s",
		MaxIdleConns:    100,
		IdleConnTimeout: "90s",
		MaxRetries:      2,
		RetryDelay:      500 * time.Millisecond,
	}
}
