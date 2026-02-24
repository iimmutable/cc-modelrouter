package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/logging"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// Router interface for handler dependency.
type Router interface {
	DetectRoute(req router.RouteRequest) string
	GetTargets(routeName string) []config.RouteTarget
}

// TransformerRegistry interface for handler dependency.
type TransformerRegistry interface {
	Get(name string) (transformer.Transformer, error)
}

// HTTPClient interface for handler dependency.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// UsageTracker interface for tracking usage statistics.
type UsageTracker interface {
	Record(instanceID, route, model string, tokens, fallbacks int)
}

// Handler handles HTTP requests.
type Handler struct {
	maxRequestSize        int64
	router                Router
	transformerRegistry   TransformerRegistry
	providerClients       map[string]HTTPClient
	config                *config.Config
	usageTracker          UsageTracker
	instanceID            string
	requestInterceptors   []RequestInterceptor
	responseInterceptors  []ResponseInterceptor
	streamingInterceptors []StreamingResponseInterceptor
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

// SetUsageTracker sets the usage tracker.
func (h *Handler) SetUsageTracker(tracker UsageTracker) {
	h.usageTracker = tracker
}

// SetInstanceID sets the instance ID.
func (h *Handler) SetInstanceID(id string) {
	h.instanceID = id
}

// SetRequestInterceptors sets the request interceptors.
func (h *Handler) SetRequestInterceptors(interceptors []RequestInterceptor) {
	h.requestInterceptors = interceptors
}

// SetResponseInterceptors sets the response interceptors.
func (h *Handler) SetResponseInterceptors(interceptors []ResponseInterceptor) {
	h.responseInterceptors = interceptors
}

// SetStreamingInterceptors sets the streaming response interceptors.
func (h *Handler) SetStreamingInterceptors(interceptors []StreamingResponseInterceptor) {
	h.streamingInterceptors = interceptors
}

// AddRequestInterceptor adds a single request interceptor.
func (h *Handler) AddRequestInterceptor(interceptor RequestInterceptor) {
	h.requestInterceptors = append(h.requestInterceptors, interceptor)
}

// AddResponseInterceptor adds a single response interceptor.
func (h *Handler) AddResponseInterceptor(interceptor ResponseInterceptor) {
	h.responseInterceptors = append(h.responseInterceptors, interceptor)
}

// AddStreamingInterceptor adds a single streaming interceptor.
func (h *Handler) AddStreamingInterceptor(interceptor StreamingResponseInterceptor) {
	h.streamingInterceptors = append(h.streamingInterceptors, interceptor)
}

// ServeHTTP handles HTTP requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Supported Anthropic API v1 endpoints
	supportedPaths := []string{
		"/v1/messages",
		"/v1/messages/with_overrides",
		"/v1/messages/batches",
	}

	isSupported := false
	for _, path := range supportedPaths {
		if r.URL.Path == path {
			isSupported = true
			break
		}
	}

	// Handle GET /v1/models for model listing
	if r.Method == http.MethodGet && r.URL.Path == "/v1/models" {
		h.handleModels(w, r)
		return
	}

	// Handle POST to supported message endpoints
	if r.Method == http.MethodPost && isSupported {
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
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

// handleMessages processes the messages request.
func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request, req *anthropic.Request) {
	// Call request interceptors
	for _, interceptor := range h.requestInterceptors {
		if err := interceptor.InterceptRequest(r.Context(), req); err != nil {
			logging.Errorf("Request interceptor failed: %v", err)
			http.Error(w, fmt.Sprintf("Request interceptor error: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Log incoming request details
	logging.Debugf("[REQUEST] Model: %s, Stream: %v, Messages: %d, MaxTokens: %d",
		req.Model, req.Stream, len(req.Messages), req.MaxTokens)
	if len(req.Tools) > 0 {
		logging.Debugf("[REQUEST] Tools: %d", len(req.Tools))
		for _, tool := range req.Tools {
			logging.Debugf("[REQUEST]   - %s", tool.Name)
		}
	}

	// Detect route
	routeReq := router.RouteRequest{
		IsBackground: h.isBackground(req),
		ThinkLevel:   h.getThinkLevel(req),
		TokenCount:   h.estimateTokens(req),
		HasWebSearch: h.hasWebSearch(req),
		HasImages:    h.hasImages(req),
	}
	routeName := h.router.DetectRoute(routeReq)
	targets := h.router.GetTargets(routeName)

	logging.Infof("[ROUTE] Detected: %s, Targets: %d", routeName, len(targets))

	// Handle streaming requests
	if req.Stream {
		h.handleStreaming(w, r, req, routeName, targets)
		return
	}

	// Try each target with failover for non-streaming
	var fallbackCount int
	var successfulModel string

	for i, target := range targets {
		resp, err := h.tryTarget(r.Context(), req, target)
		if err != nil {
			fallbackCount = i
			logging.Errorf("Target %d (%s/%s) failed: %v", i, target.Provider, target.Model, err)
			continue
		}

		successfulModel = target.Model

		// Call response interceptors
		for _, interceptor := range h.responseInterceptors {
			if err := interceptor.InterceptResponse(r.Context(), req, resp); err != nil {
				logging.Errorf("Response interceptor failed: %v", err)
				http.Error(w, fmt.Sprintf("Response interceptor error: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Track usage
		if h.usageTracker != nil {
			h.usageTracker.Record(h.instanceID, routeName, successfulModel, h.estimateTokens(req), fallbackCount)
		}

		// Log response details
		logging.Infof("[RESPONSE] Success with %s/%s, StopReason: %s, InputTokens: %d, OutputTokens: %d",
			target.Provider, target.Model, resp.StopReason, resp.Usage.InputTokens, resp.Usage.OutputTokens)

		// Write response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	logging.Errorf("All providers failed for route: %s", routeName)
	http.Error(w, "All providers failed", http.StatusBadGateway)
}

// handleStreaming processes a streaming messages request.
func (h *Handler) handleStreaming(w http.ResponseWriter, r *http.Request, req *anthropic.Request, routeName string, targets []config.RouteTarget) {
	// Call request interceptors
	for _, interceptor := range h.requestInterceptors {
		if err := interceptor.InterceptRequest(r.Context(), req); err != nil {
			logging.Errorf("Request interceptor failed: %v", err)
			h.sendStreamingError(w, "Request interceptor error", err)
			return
		}
	}

	logging.Streamf("Starting streaming request, route: %s, targets: %d", routeName, len(targets))

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Try each target with failover
	for i, target := range targets {
		if err := h.tryStreamingTarget(r.Context(), w, flusher, req, target); err != nil {
			logging.Streamf("Target %d (%s/%s) failed: %v", i, target.Provider, target.Model, err)
			continue
		}

		// Track usage on successful stream
		if h.usageTracker != nil {
			h.usageTracker.Record(h.instanceID, routeName, target.Model, h.estimateTokens(req), i)
		}
		logging.Streamf("Stream completed with %s/%s, fallbacks: %d", target.Provider, target.Model, i)
		return
	}

	// All providers failed - write error as SSE event following Anthropic's format
	h.sendStreamingError(w, "All providers failed", fmt.Errorf("all %d providers failed for route: %s", len(targets), routeName))
}

// sendStreamingError sends an error event in SSE format following Anthropic's error format.
func (h *Handler) sendStreamingError(w http.ResponseWriter, message string, err error) {
	logging.Errorf("Streaming error: %s: %v", message, err)

	// Anthropic's SSE error format
	errorJSON := fmt.Sprintf("{\"type\": \"error\", \"error\": {\"type\": \"api_error\", \"message\": \"%s: %s\"}}", message, err)
	w.Write([]byte("event: error\n"))
	w.Write([]byte("data: "))
	w.Write([]byte(errorJSON))
	w.Write([]byte("\n\n"))

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
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
	tf, err := h.transformerRegistry.Get(transformerName)
	if err != nil {
		tf, _ = h.transformerRegistry.Get("anthropic")
	}

	// Get client
	client, ok := h.providerClients[target.Provider]
	if !ok {
		return nil, fmt.Errorf("client not found: %s", target.Provider)
	}

	// Transform request
	httpReq, err := tf.TransformRequest(req, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
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
	return tf.TransformResponse(resp)
}

// tryStreamingTarget attempts to send a streaming request to a target provider.
func (h *Handler) tryStreamingTarget(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, req *anthropic.Request, target config.RouteTarget) error {
	logging.Streamf("Starting stream to %s/%s", target.Provider, target.Model)

	// Get provider config
	providerCfg, ok := h.config.Providers[target.Provider]
	if !ok {
		return fmt.Errorf("provider not found: %s", target.Provider)
	}

	// Get transformer
	transformerName := providerCfg.Transformer
	if transformerName == "" {
		transformerName = target.Provider
	}
	tf, err := h.transformerRegistry.Get(transformerName)
	if err != nil {
		tf, _ = h.transformerRegistry.Get("anthropic")
	}

	// Check if transformer supports streaming
	if !tf.SupportsStreaming() {
		return fmt.Errorf("transformer does not support streaming")
	}

	// Get client
	client, ok := h.providerClients[target.Provider]
	if !ok {
		return fmt.Errorf("client not found: %s", target.Provider)
	}

	// Ensure stream flag is set
	reqCopy := *req
	reqCopy.Stream = true

	// Transform request
	httpReq, err := tf.TransformRequest(&reqCopy, providerCfg.BaseURL, providerCfg.APIKey, target.Model)
	if err != nil {
		return err
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	// Read and forward SSE stream
	// Note: We do NOT emit synthetic message_start/content_block_start events here
	// because providers like GLM already send these events. Emitting synthetic ones
	// would cause duplicate events that break the client's parsing.
	scanner := NewSSEScanner(resp.Body)

	eventCount := 0
	for scanner.Scan() {
		eventCount++
		eventType := scanner.Event()
		data := scanner.Data()

		// Filter out non-Anthropic events that some providers send (e.g., GLM's "ping")
		if eventType == "ping" || eventType == "keepalive" {
			logging.StreamDebugf("Filtering out non-Anthropic event: %s", eventType)
			continue
		}

		if len(data) == 0 {
			continue
		}

		// Validate JSON before processing
		if !json.Valid(data) {
			logging.StreamDebugf("Invalid JSON data from provider, skipping: %s", string(data))
			continue
		}

		// Create SSE event and transform it
		sseEvent := &transformer.SSEEvent{
			EventType: eventType,
			Data:      data,
		}

		transformedEvents, err := tf.TransformSSEEvent(sseEvent)
		if err != nil {
			logging.StreamDebugf("Transform error: %v", err)
			continue
		}

		// Write all transformed events
		for _, te := range transformedEvents {
			if len(te.Data) == 0 {
				logging.StreamDebugf("Skipping empty event data, type: %s", te.EventType)
				continue
			}
			if !json.Valid(te.Data) {
				logging.StreamDebugf("Skipping invalid JSON event data, type: %s, data: %s", te.EventType, string(te.Data))
				continue
			}

			// Apply streaming interceptors
			interceptedData := te.Data
			var interceptorErr error
			for _, interceptor := range h.streamingInterceptors {
				interceptedData, interceptorErr = interceptor.InterceptStreamingEvent(ctx, req, te.EventType, te.Data)
				if interceptorErr != nil {
					logging.StreamDebugf("Streaming interceptor error: %v", interceptorErr)
					// Continue with original data on interceptor error
					interceptedData = te.Data
				}
			}

			// Re-validate after interceptor processing
			if !json.Valid(interceptedData) {
				logging.StreamDebugf("Skipping invalid JSON after interceptor, type: %s", te.EventType)
				continue
			}

			// Determine event format based on event type
			if te.EventType != "" {
				// Write event with explicit event type
				w.Write([]byte(fmt.Sprintf("event: %s\n", te.EventType)))
			}
			w.Write([]byte("data: "))
			w.Write(interceptedData)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}

	logging.Streamf("Stream completed with %d events processed", eventCount)

	if scanner.Err() != nil {
		// Send message_stop before returning error to properly close the stream
		messageStopData, err := json.Marshal(map[string]string{
			"type": "message_stop",
		})
		if err != nil {
			logging.Errorf("[STREAM] Failed to marshal message_stop event: %v", err)
		} else if len(messageStopData) > 0 {
			w.Write([]byte("event: message_stop\n"))
			w.Write([]byte("data: "))
			w.Write(messageStopData)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
		return scanner.Err()
	}

	logging.Streamf("Stream completed successfully")
	return nil
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

// isBackground detects if this is a background agent request.
// Claude Code uses Haiku models for background agents.
func (h *Handler) isBackground(req *anthropic.Request) bool {
	model := strings.ToLower(req.Model)
	return strings.Contains(model, "claude") && strings.Contains(model, "haiku")
}

// getThinkLevel detects the thinking level based on budget_tokens.
// Levels: NONE (0), BASIC (~4K), MIDDLE (~10K), HIGHEST (~32K)
func (h *Handler) getThinkLevel(req *anthropic.Request) router.ThinkLevel {
	if req.Thinking == nil || req.Thinking.BudgetTokens <= 0 {
		return router.ThinkNone
	}

	budget := req.Thinking.BudgetTokens
	switch {
	case budget >= router.ThinkLevelHighest:
		return router.ThinkHighest
	case budget >= router.ThinkLevelMiddle:
		return router.ThinkMiddle
	case budget >= router.ThinkLevelBasic:
		return router.ThinkBasic
	default:
		return router.ThinkBasic // Any non-zero budget is at least basic
	}
}

// handleModels returns the available models from all configured providers.
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	type Model struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}

	type ModelsResponse struct {
		Object string  `json:"object"`
		Data   []Model `json:"data"`
	}

	var models []Model
	for _, providerCfg := range h.config.Providers {
		for _, modelID := range providerCfg.Models {
			models = append(models, Model{
				ID:     modelID,
				Object: "model",
			})
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
