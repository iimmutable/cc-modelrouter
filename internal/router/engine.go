// Package router handles request routing and model selection.
package router

import (
	"github.com/iimmutable/cc-modelrouter/internal/config"
)

const (
	LongContextThreshold = 60000
)

// RouteRequest contains information for route detection.
type RouteRequest struct {
	IsBackground bool
	IsThink      bool
	TokenCount   int
	HasWebSearch bool
	HasImages    bool
}

// Engine handles route detection and target selection.
type Engine struct {
	config *config.Config
}

// NewEngine creates a new router engine.
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{config: cfg}
}

// DetectRoute determines which route to use based on request characteristics.
func (e *Engine) DetectRoute(req RouteRequest) string {
	// Priority order for route detection
	switch {
	case req.IsBackground:
		if route, ok := e.config.Router.Routes["background"]; ok && route != "" {
			return "background"
		}
	case req.IsThink:
		if route, ok := e.config.Router.Routes["think"]; ok && route != "" {
			return "think"
		}
	case req.HasImages:
		if route, ok := e.config.Router.Routes["image"]; ok && route != "" {
			return "image"
		}
	case req.HasWebSearch:
		if route, ok := e.config.Router.Routes["webSearch"]; ok && route != "" {
			return "webSearch"
		}
	case req.TokenCount > LongContextThreshold:
		if route, ok := e.config.Router.Routes["longContext"]; ok && route != "" {
			return "longContext"
		}
	}

	return "default"
}

// GetTargets returns the route targets for a given route name.
func (e *Engine) GetTargets(routeName string) []config.RouteTarget {
	route, ok := e.config.Router.Routes[routeName]
	if !ok {
		route = e.config.Router.Routes["default"]
	}
	return config.ParseRoute(route)
}
