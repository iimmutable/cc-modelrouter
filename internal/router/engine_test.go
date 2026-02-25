package router

import (
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
)

func TestRouteDetection(t *testing.T) {
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":     "bigmodel:glm-4.7;openrouter:claude-sonnet-4.5",
				"background":  "bigmodel:glm-4.5-air",
				"longContext": "bigmodel:glm-4.7;gemini:gemini-2.5-pro",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "default route",
			req:      RouteRequest{},
			expected: "default",
		},
		{
			name:     "background route",
			req:      RouteRequest{IsBackground: true},
			expected: "background",
		},
		{
			name:     "long context route",
			req:      RouteRequest{TokenCount: 70000},
			expected: "longContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestThinkLevelDetection(t *testing.T) {
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":    "bigmodel:glm-4.7",
				"think":      "openrouter:claude-sonnet-4.5",
				"thinkMore":  "openrouter:claude-sonnet-4.5",
				"ultrathink": "openrouter:claude-opus-4.5",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "no thinking",
			req:      RouteRequest{ThinkLevel: ThinkNone},
			expected: "default",
		},
		{
			name:     "basic thinking",
			req:      RouteRequest{ThinkLevel: ThinkBasic},
			expected: "think",
		},
		{
			name:     "middle thinking",
			req:      RouteRequest{ThinkLevel: ThinkMiddle},
			expected: "thinkMore",
		},
		{
			name:     "highest thinking",
			req:      RouteRequest{ThinkLevel: ThinkHighest},
			expected: "ultrathink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestThinkLevelFallback(t *testing.T) {
	// Test fallback when only "think" route is configured
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default": "bigmodel:glm-4.7",
				"think":   "openrouter:claude-sonnet-4.5",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "basic thinking uses think route",
			req:      RouteRequest{ThinkLevel: ThinkBasic},
			expected: "think",
		},
		{
			name:     "middle thinking falls back to think",
			req:      RouteRequest{ThinkLevel: ThinkMiddle},
			expected: "think",
		},
		{
			name:     "highest thinking falls back to think",
			req:      RouteRequest{ThinkLevel: ThinkHighest},
			expected: "think",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestThinkLevelPartialConfig(t *testing.T) {
	// Test when thinkMore is configured but ultrathink is not
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":   "bigmodel:glm-4.7",
				"think":     "openrouter:claude-sonnet-4.5",
				"thinkMore": "openrouter:claude-sonnet-4.5",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "basic thinking uses think route",
			req:      RouteRequest{ThinkLevel: ThinkBasic},
			expected: "think",
		},
		{
			name:     "middle thinking uses thinkMore",
			req:      RouteRequest{ThinkLevel: ThinkMiddle},
			expected: "thinkMore",
		},
		{
			name:     "highest thinking falls back to thinkMore",
			req:      RouteRequest{ThinkLevel: ThinkHighest},
			expected: "thinkMore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestRoutePriority(t *testing.T) {
	// Test that route detection follows correct priority order:
	// background > ultrathink > thinkMore > think > image > webSearch > longContext > default
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":     "provider:default-model",
				"background":  "provider:background-model",
				"think":       "provider:think-model",
				"thinkMore":   "provider:thinkmore-model",
				"ultrathink":  "provider:ultrathink-model",
				"image":       "provider:image-model",
				"webSearch":   "provider:websearch-model",
				"longContext": "provider:longcontext-model",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "background takes priority over think",
			req:      RouteRequest{IsBackground: true, ThinkLevel: ThinkHighest},
			expected: "background",
		},
		{
			name:     "background takes priority over image",
			req:      RouteRequest{IsBackground: true, HasImages: true},
			expected: "background",
		},
		{
			name:     "highest think takes priority over image",
			req:      RouteRequest{ThinkLevel: ThinkHighest, HasImages: true},
			expected: "ultrathink",
		},
		{
			name:     "basic think takes priority over image",
			req:      RouteRequest{ThinkLevel: ThinkBasic, HasImages: true},
			expected: "think",
		},
		{
			name:     "image takes priority over webSearch",
			req:      RouteRequest{HasImages: true, HasWebSearch: true},
			expected: "image",
		},
		{
			name:     "webSearch takes priority over longContext",
			req:      RouteRequest{HasWebSearch: true, TokenCount: 100000},
			expected: "webSearch",
		},
		{
			name:     "longContext when no other flags",
			req:      RouteRequest{TokenCount: 100000},
			expected: "longContext",
		},
		{
			name:     "default when no flags",
			req:      RouteRequest{},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestThinkLevelNoRouteConfigured(t *testing.T) {
	// Test when no thinking routes are configured - should fall back to default
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default": "provider:default-model",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name     string
		req      RouteRequest
		expected string
	}{
		{
			name:     "basic thinking falls back to default",
			req:      RouteRequest{ThinkLevel: ThinkBasic},
			expected: "default",
		},
		{
			name:     "middle thinking falls back to default",
			req:      RouteRequest{ThinkLevel: ThinkMiddle},
			expected: "default",
		},
		{
			name:     "highest thinking falls back to default",
			req:      RouteRequest{ThinkLevel: ThinkHighest},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := engine.DetectRoute(tt.req)
			if route != tt.expected {
				t.Errorf("expected route %s, got %s", tt.expected, route)
			}
		})
	}
}

func TestGetTargets(t *testing.T) {
	cfg := &config.Config{
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":    "provider1:model1;provider2:model2",
				"think":      "provider3:model3",
				"ultrathink": "provider4:model4",
			},
		},
	}

	engine := NewEngine(cfg)

	tests := []struct {
		name          string
		routeName     string
		expectedCount int
		firstProvider string
		firstModel    string
	}{
		{
			name:          "default route with multiple targets",
			routeName:     "default",
			expectedCount: 2,
			firstProvider: "provider1",
			firstModel:    "model1",
		},
		{
			name:          "think route with single target",
			routeName:     "think",
			expectedCount: 1,
			firstProvider: "provider3",
			firstModel:    "model3",
		},
		{
			name:          "unknown route falls back to default",
			routeName:     "unknown",
			expectedCount: 2,
			firstProvider: "provider1",
			firstModel:    "model1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := engine.GetTargets(tt.routeName)
			if len(targets) != tt.expectedCount {
				t.Errorf("expected %d targets, got %d", tt.expectedCount, len(targets))
			}
			if len(targets) > 0 {
				if targets[0].Provider != tt.firstProvider {
					t.Errorf("expected first provider %s, got %s", tt.firstProvider, targets[0].Provider)
				}
				if targets[0].Model != tt.firstModel {
					t.Errorf("expected first model %s, got %s", tt.firstModel, targets[0].Model)
				}
			}
		})
	}
}
