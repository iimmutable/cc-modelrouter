package provider

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTPClient wraps http.Client with provider-specific configuration.
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewClient creates a new provider client.
func NewClient(cfg *ClientConfig) (*HTTPClient, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	defaults := Defaults()
	if cfg.Timeout == "" {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = defaults.MaxIdleConns
	}
	if cfg.IdleConnTimeout == "" {
		cfg.IdleConnTimeout = defaults.IdleConnTimeout
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = defaults.RetryDelay
	}

	timeout, _ := time.ParseDuration(cfg.Timeout)
	idleTimeout, _ := time.ParseDuration(cfg.IdleConnTimeout)

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		IdleConnTimeout:     idleTimeout,
		MaxIdleConnsPerHost: 10,
		DisableKeepAlives:   cfg.DisableKeepAlives, // For providers with connection issues
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
	}, nil
}

// NewStreamingClient creates an HTTP client optimized for streaming requests.
// It has no Timeout (relies on context cancellation) but maintains
// connection timeouts for robustness. Use this for SSE streaming requests.
func NewStreamingClient(cfg *ClientConfig) (*HTTPClient, error) {
	defaults := Defaults()

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaults.MaxRetries
	}
	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = defaults.RetryDelay
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second, // Wait up to 60s for headers
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		DisableKeepAlives:    cfg.DisableKeepAlives, // For providers with connection issues
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout:   0, // NO timeout for streaming - use context cancellation
			Transport: transport,
		},
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}, nil
}

// Do executes an HTTP request with retry logic.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// CRITICAL: Restore body using GetBody for retries
			// After the first request, the body is consumed. GetBody allows
			// getting a fresh reader for the same content.
			if req.GetBody != nil {
				newBody, err := req.GetBody()
				if err != nil {
					lastErr = fmt.Errorf("failed to get fresh body for retry: %w", err)
					continue
				}
				req.Body = newBody
			}

			select {
			case <-time.After(c.retryDelay):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 5xx errors
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// DoWithContext executes an HTTP request with context.
func (c *HTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.Do(req.WithContext(ctx))
}
