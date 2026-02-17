package cli

import (
	"net/http"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// RouterAdapter adapts router.Engine to proxy.Router interface.
type RouterAdapter struct {
	engine *router.Engine
}

// NewRouterAdapter creates a new router adapter.
func NewRouterAdapter(engine *router.Engine) *RouterAdapter {
	return &RouterAdapter{engine: engine}
}

// DetectRoute implements proxy.Router.
func (a *RouterAdapter) DetectRoute(req proxy.RouteRequest) string {
	routerReq := router.RouteRequest{
		IsBackground: req.IsBackground,
		IsThink:      req.IsThink,
		TokenCount:   req.TokenCount,
		HasWebSearch: req.HasWebSearch,
		HasImages:    req.HasImages,
	}
	return a.engine.DetectRoute(routerReq)
}

// GetTargets implements proxy.Router.
func (a *RouterAdapter) GetTargets(routeName string) []config.RouteTarget {
	return a.engine.GetTargets(routeName)
}

// TransformerAdapter adapts transformer.Transformer to proxy.Transformer interface.
type TransformerAdapter struct {
	t transformer.Transformer
}

// NewTransformerAdapter creates a new transformer adapter.
func NewTransformerAdapter(t transformer.Transformer) *TransformerAdapter {
	return &TransformerAdapter{t: t}
}

// Name implements proxy.Transformer.
func (a *TransformerAdapter) Name() string {
	return a.t.Name()
}

// TransformRequest implements proxy.Transformer.
func (a *TransformerAdapter) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	return a.t.TransformRequest(req, baseURL, apiKey, model)
}

// TransformResponse implements proxy.Transformer.
func (a *TransformerAdapter) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	return a.t.TransformResponse(resp)
}

// SupportsStreaming implements proxy.Transformer.
func (a *TransformerAdapter) SupportsStreaming() bool {
	return a.t.SupportsStreaming()
}

// TransformStreamChunk implements proxy.Transformer.
func (a *TransformerAdapter) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return a.t.TransformStreamChunk(chunk, eventType)
}

// RegistryAdapter adapts transformer.Registry to proxy.TransformerRegistry interface.
type RegistryAdapter struct {
	registry *transformer.Registry
}

// NewRegistryAdapter creates a new registry adapter.
func NewRegistryAdapter(registry *transformer.Registry) *RegistryAdapter {
	return &RegistryAdapter{registry: registry}
}

// Get implements proxy.TransformerRegistry.
func (a *RegistryAdapter) Get(name string) (proxy.Transformer, error) {
	t, err := a.registry.Get(name)
	if err != nil {
		return nil, err
	}
	return NewTransformerAdapter(t), nil
}
