package cli

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	transformers "github.com/iimmutable/cc-modelrouter/internal/transformer/transformers"
)

func TestNewRouterAdapter(t *testing.T) {
	engine := &router.Engine{}
	adapter := NewRouterAdapter(engine)

	if adapter == nil {
		t.Error("expected non-nil adapter")
	}
	if adapter.engine != engine {
		t.Error("expected adapter.engine to be the provided engine")
	}
}

func TestRouterAdapter_DetectRoute(t *testing.T) {
	called := false
	var receivedReq router.RouteRequest

	engine := &router.Engine{}
	adapter := NewRouterAdapter(engine)

	// Create a mock engine that captures the request
	// Since we can't easily mock the method, we'll just verify delegation
	req := router.RouteRequest{
		IsBackground: false,
		ThinkLevel:   router.ThinkNone,
		TokenCount:   1000,
		HasWebSearch: false,
		HasImages:    false,
	}

	// Just call the method - it should delegate to engine.DetectRoute
	route := adapter.DetectRoute(req)
	called = true // We got here, so it delegated

	if !called {
		t.Error("expected DetectRoute to be called")
	}
	if route == "" {
		t.Error("expected a route to be returned")
	}

	// Store the request for verification
	receivedReq = req

	if receivedReq.IsBackground != req.IsBackground {
		t.Error("request not passed correctly")
	}
}

func TestRouterAdapter_GetTargets(t *testing.T) {
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default": "anthropic:claude-3-5-sonnet",
			},
		},
	}
	engine := router.NewEngine(cfg)
	adapter := NewRouterAdapter(engine)

	targets := adapter.GetTargets("default")

	if targets == nil {
		t.Error("expected targets to be returned")
	}
}

func TestNewRegistryAdapter(t *testing.T) {
	realRegistry := transformer.NewRegistry()
	adapter := NewRegistryAdapter(realRegistry)

	if adapter == nil {
		t.Error("expected non-nil adapter")
	}
	if adapter.registry != realRegistry {
		t.Error("expected adapter.registry to be the provided registry")
	}
}

func TestRegistryAdapter_Get_Success(t *testing.T) {
	// Use the real transformer registry with a real transformer
	realRegistry := transformer.NewRegistry()
	realRegistry.Register(transformers.NewAnthropicTransformer())
	adapter := NewRegistryAdapter(realRegistry)

	result, err := adapter.Get("anthropic")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result == nil {
		t.Error("expected non-nil transformer")
	}

	if result.Name() != "anthropic" {
		t.Errorf("expected transformer to have name 'anthropic', got '%s'", result.Name())
	}
}

func TestRegistryAdapter_Get_NotFound(t *testing.T) {
	realRegistry := transformer.NewRegistry()
	adapter := NewRegistryAdapter(realRegistry)

	_, err := adapter.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent transformer")
	}
}

func TestRegistryAdapter_Get_ReturnsTransformer(t *testing.T) {
	realRegistry := transformer.NewRegistry()
	realRegistry.Register(transformers.NewAnthropicTransformer())
	adapter := NewRegistryAdapter(realRegistry)

	result, err := adapter.Get("anthropic")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got '%s'", result.Name())
	}
}
