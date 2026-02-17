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
				"longContext": "bigmodel:glm-4.7;openrouter:gemini-2.5-pro",
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
