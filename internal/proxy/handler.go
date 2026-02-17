package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// Router interface for handler dependency.
type Router interface {
	DetectRoute(req RouteRequest) string
	GetTargets(routeName string) []config.RouteTarget
}

// TransformerRegistry interface for handler dependency.
type TransformerRegistry interface {
	Get(name string) (Transformer, error)
}

// Transformer interface for handler dependency.
type Transformer interface {
	Name() string
	TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error)
	TransformResponse(resp *http.Response) (*anthropic.Response, error)
	SupportsStreaming() bool
	TransformStreamChunk(chunk []byte, eventType string) ([]byte, error)
}

// HTTPClient interface for handler dependency.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RouteRequest for route detection.
type RouteRequest struct {
	IsBackground bool
	IsThink      bool
	TokenCount   int
	HasWebSearch bool
	HasImages    bool
}

// Handler handles HTTP requests.
type Handler struct {
	maxRequestSize      int64
	router              Router
	transformerRegistry TransformerRegistry
	providerClients     map[string]HTTPClient
	config              *config.Config
}

// NewHandler creates a new handler.
func NewHandler(maxRequestSize int64) *Handler {
	return &Handler{
		maxRequestSize:  maxRequestSize,
		providerClients: make(map[string]HTTPClient),
	}
}

// SetRouter sets the router.
func (h *Handler) SetRouter(router Router) {
	h.router = router
}

// SetTransformerRegistry sets the transformer registry.
func (h *Handler) SetTransformerRegistry(reg TransformerRegistry) {
	h.transformerRegistry = reg
}

// SetProviderClients sets the provider clients.
func (h *Handler) SetProviderClients(clients map[string]HTTPClient) {
	h.providerClients = clients
}

// SetConfig sets the configuration.
func (h *Handler) SetConfig(cfg *config.Config) {
	h.config = cfg
}

// ServeHTTP handles HTTP requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle POST to /v1/messages
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Read and parse request
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxRequestSize))
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	var req anthropic.Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Handle the request
	h.handleMessages(w, r, &req)
}

// handleMessages processes the messages request.
func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request, req *anthropic.Request) {
	// Detect route
	routeReq := RouteRequest{
		TokenCount:   h.estimateTokens(req),
		HasWebSearch: h.hasWebSearch(req),
		HasImages:    h.hasImages(req),
	}
	routeName := h.router.DetectRoute(routeReq)
	targets := h.router.GetTargets(routeName)

	// Try each target with failover
	for _, target := range targets {
		resp, err := h.tryTarget(r.Context(), req, target)
		if err != nil {
			continue
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	http.Error(w, "All providers failed", http.StatusBadGateway)
}

func (h *Handler) tryTarget(ctx context.Context, req *anthropic.Request, target config.RouteTarget) (*anthropic.Response, error) {
	// Get provider config
	providerCfg, ok := h.config.Providers[target.Provider]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", target.Provider)
	}

	// Get transformer
	transformerName := providerCfg.Transformer
	if transformerName == "" {
		transformerName = target.Provider
	}
	transformer, err := h.transformerRegistry.Get(transformerName)
	if err != nil {
		transformer, _ = h.transformerRegistry.Get("anthropic")
	}

	// Get client
	client, ok := h.providerClients[target.Provider]
	if !ok {
		return nil, fmt.Errorf("client not found: %s", target.Provider)
	}

	// Transform request
	httpReq, err := transformer.TransformRequest(req, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
	if err != nil {
		return nil, err
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Transform response
	return transformer.TransformResponse(resp)
}

func (h *Handler) estimateTokens(req *anthropic.Request) int {
	// Rough estimation: ~4 chars per token
	total := 0
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				total += len(block.Text) / 4
			}
		}
	}
	return total
}

func (h *Handler) hasWebSearch(req *anthropic.Request) bool {
	for _, tool := range req.Tools {
		if strings.Contains(strings.ToLower(tool.Name), "web") ||
			strings.Contains(strings.ToLower(tool.Name), "search") {
			return true
		}
	}
	return false
}

func (h *Handler) hasImages(req *anthropic.Request) bool {
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "image" {
				return true
			}
		}
	}
	return false
}
