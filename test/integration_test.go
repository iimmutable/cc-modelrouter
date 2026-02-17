//go:build integration
// +build integration

package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// routerAdapter adapts router.Engine to proxy.Router interface
type routerAdapter struct {
	engine *router.Engine
}

func (a *routerAdapter) DetectRoute(req proxy.RouteRequest) string {
	return a.engine.DetectRoute(router.RouteRequest{
		IsBackground: req.IsBackground,
		IsThink:      req.IsThink,
		TokenCount:   req.TokenCount,
		HasWebSearch: req.HasWebSearch,
		HasImages:    req.HasImages,
	})
}

func (a *routerAdapter) GetTargets(routeName string) []config.RouteTarget {
	return a.engine.GetTargets(routeName)
}

// transformerAdapter adapts transformer.Transformer to proxy.Transformer interface
type transformerAdapter struct {
	t transformer.Transformer
}

func (a *transformerAdapter) Name() string {
	return a.t.Name()
}

func (a *transformerAdapter) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	return a.t.TransformRequest(req, baseURL, apiKey, model)
}

func (a *transformerAdapter) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	return a.t.TransformResponse(resp)
}

func (a *transformerAdapter) SupportsStreaming() bool {
	return a.t.SupportsStreaming()
}

func (a *transformerAdapter) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return a.t.TransformStreamChunk(chunk, eventType)
}

// registryAdapter adapts transformer.Registry to proxy.TransformerRegistry interface
type registryAdapter struct {
	registry *transformer.Registry
}

func (a *registryAdapter) Get(name string) (proxy.Transformer, error) {
	t, err := a.registry.Get(name)
	if err != nil {
		return nil, err
	}
	return &transformerAdapter{t: t}, nil
}

func TestIntegrationBasicRequest(t *testing.T) {
	// Load test configuration
	cfg, err := config.Load(".cc-modelrouter/test.config.json")
	if err != nil {
		t.Skipf("Test config not found: %v", err)
	}

	// Initialize components
	registry := transformer.NewRegistry()
	registry.Register(transformer.NewAnthropicTransformer())

	clients := make(map[string]proxy.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		client, _ := provider.NewClient(&provider.ClientConfig{
			BaseURL: providerCfg.BaseURL,
			APIKey:  providerCfg.APIKey,
		})
		clients[name] = client
	}

	routerEngine := router.NewEngine(cfg)

	// Create handler with adapters
	handler := proxy.NewHandler(50 * 1024 * 1024)
	handler.SetRouter(&routerAdapter{engine: routerEngine})
	handler.SetTransformerRegistry(&registryAdapter{registry: registry})
	handler.SetProviderClients(clients)
	handler.SetConfig(cfg)

	// Create test request
	reqBody := map[string]any{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": []map[string]any{
			{"role": "user", "content": "Say 'Hello'"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Logf("Response: %s", w.Body.String())
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
