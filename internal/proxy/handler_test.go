package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

// Mock implementations for testing

type mockRouter struct {
	detectRouteFunc func(req router.RouteRequest) string
	getTargetsFunc  func(routeName string) []config.RouteTarget
}

func (m *mockRouter) DetectRoute(req router.RouteRequest) string {
	if m.detectRouteFunc != nil {
		return m.detectRouteFunc(req)
	}
	return "default"
}

func (m *mockRouter) GetTargets(routeName string) []config.RouteTarget {
	if m.getTargetsFunc != nil {
		return m.getTargetsFunc(routeName)
	}
	return []config.RouteTarget{
		{Provider: "anthropic", Model: "claude-3-5-sonnet"},
	}
}

type mockTransformerRegistry struct {
	transformers map[string]transformer.Transformer
}

func (m *mockTransformerRegistry) Get(name string) (transformer.Transformer, error) {
	if t, ok := m.transformers[name]; ok {
		return t, nil
	}
	return &anthropicTransformer{}, fmt.Errorf("transformer not found")
}

func (m *mockTransformerRegistry) addTransformer(name string, t transformer.Transformer) {
	if m.transformers == nil {
		m.transformers = make(map[string]transformer.Transformer)
	}
	m.transformers[name] = t
}

type anthropicTransformer struct {
	baseURL string
}

func (m *anthropicTransformer) Name() string { return "anthropic" }
func (m *anthropicTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	// Create a proper request for the mock server
	endpoint := baseURL
	if !strings.HasSuffix(baseURL, "/v1/messages") {
		endpoint = strings.TrimSuffix(baseURL, "/") + "/v1/messages"
	}
	body, _ := json.Marshal(req)
	return http.NewRequest("POST", endpoint, bytes.NewReader(body))
}
func (m *anthropicTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	return &anthropic.Response{}, nil
}
func (m *anthropicTransformer) SupportsStreaming() bool {
	return true
}
func (m *anthropicTransformer) TransformSSEEvent(event *transformer.SSEEvent) ([]transformer.SSEEvent, error) {
	// Pass-through for Anthropic-compatible streams
	return []transformer.SSEEvent{*event}, nil
}
func (m *anthropicTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return chunk, nil
}

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

type mockUsageTracker struct {
	records []mockRecord
}

type mockRecord struct {
	instanceID string
	route      string
	model      string
	tokens     int
	fallbacks  int
}

func (m *mockUsageTracker) Record(instanceID, route, model string, tokens, fallbacks int) {
	m.records = append(m.records, mockRecord{
		instanceID: instanceID,
		route:      route,
		model:      model,
		tokens:     tokens,
		fallbacks:  fallbacks,
	})
}

func TestNewHandler(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	if handler == nil {
		t.Error("expected non-nil handler")
	}
	if handler.maxRequestSize != 1024*1024 {
		t.Errorf("expected maxRequestSize 1048576, got %d", handler.maxRequestSize)
	}
	if handler.providerClients == nil {
		t.Error("expected providerClients map to be initialized")
	}
}

func TestNewHandler_DefaultMaxRequestSize(t *testing.T) {
	handler := NewHandler(0)
	if handler.maxRequestSize != 0 {
		t.Errorf("expected maxRequestSize 0, got %d", handler.maxRequestSize)
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Not Found") {
		t.Error("expected 'Not Found' in response body")
	}
}

func TestServeHTTP_WrongPath(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	req := httptest.NewRequest(http.MethodPost, "/v1/invalid", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestServeHTTP_ValidRequest(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetRouter(&mockRouter{})
	handler.SetTransformerRegistry(&mockTransformerRegistry{})

	// Set up mock provider client
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			resp := &anthropic.Response{
				ID:      "msg_123",
				Type:    "message",
				Content: []anthropic.ContentBlock{{Type: "text", Text: "test"}},
			}
			body, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		},
	}
	handler.SetProviderClients(map[string]HTTPClient{"anthropic": mockClient})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	reqBody := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestServeHTTP_ExceedsMaxSize(t *testing.T) {
	handler := NewHandler(1024) // 1KB max

	largeBody := strings.Repeat("a", 2000)
	reqBody := `{"model": "test", "max_tokens": 100, "messages": [{"role": "user", "content": [{"type": "text", "text": "` + largeBody + `"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestEstimateTokens(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		name     string
		request  *anthropic.Request
		expected int
	}{
		{
			name: "single text block",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{{Type: "text", Text: "hello world"}}},
				},
			},
			expected: len("hello world") / 4,
		},
		{
			name: "multiple text blocks",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{{Type: "text", Text: "first"}}},
					{Content: []anthropic.ContentBlock{{Type: "text", Text: "second"}}},
				},
			},
			expected: len("first")/4 + len("second")/4,
		},
		{
			name: "empty messages",
			request: &anthropic.Request{
				Messages: []anthropic.Message{},
			},
			expected: 0,
		},
		{
			name: "image block ignored",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{
						{Type: "image", Source: &anthropic.ImageSource{}},
					}},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.estimateTokens(tt.request)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestHasWebSearch(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		name     string
		request  *anthropic.Request
		expected bool
	}{
		{
			name: "web tool name",
			request: &anthropic.Request{
				Tools: []anthropic.Tool{{Name: "web_search"}},
			},
			expected: true,
		},
		{
			name: "search tool name",
			request: &anthropic.Request{
				Tools: []anthropic.Tool{{Name: "search"}},
			},
			expected: true,
		},
		{
			name: "uppercase web tool",
			request: &anthropic.Request{
				Tools: []anthropic.Tool{{Name: "WEB_SEARCH"}},
			},
			expected: true,
		},
		{
			name: "no search tools",
			request: &anthropic.Request{
				Tools: []anthropic.Tool{{Name: "calculator"}},
			},
			expected: false,
		},
		{
			name: "empty tools",
			request: &anthropic.Request{
				Tools: []anthropic.Tool{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.hasWebSearch(tt.request)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasImages(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		name     string
		request  *anthropic.Request
		expected bool
	}{
		{
			name: "with image block",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{{Type: "image"}}},
				},
			},
			expected: true,
		},
		{
			name: "without images",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{{Type: "text"}}},
				},
			},
			expected: false,
		},
		{
			name: "mixed content with image",
			request: &anthropic.Request{
				Messages: []anthropic.Message{
					{Content: []anthropic.ContentBlock{{Type: "text"}, {Type: "image"}}},
				},
			},
			expected: true,
		},
		{
			name: "empty messages",
			request: &anthropic.Request{
				Messages: []anthropic.Message{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.hasImages(tt.request)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsBackground(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{"haiku model", "claude-3-5-haiku-20241022", true},
		{"haiku with dash", "claude-haiku", true},
		{"uppercase haiku", "CLAUDE-HAIKU", true},
		{"sonnet model", "claude-3-5-sonnet-20241022", false},
		{"opus model", "claude-3-5-opus-20241022", false},
		{"non-claude model", "gpt-4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{Model: tt.model}
			result := handler.isBackground(req)
			if result != tt.expected {
				t.Errorf("expected %v for model %s, got %v", tt.expected, tt.model, result)
			}
		})
	}
}

func TestGetThinkLevel(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		name         string
		budgetTokens int
		expected     router.ThinkLevel
	}{
		{"no thinking", 0, router.ThinkNone},
		{"negative budget", -1, router.ThinkNone},
		{"basic level", 2000, router.ThinkBasic},
		{"basic threshold", 4000, router.ThinkBasic},
		{"middle level", 5000, router.ThinkBasic}, // < 10000, so still basic
		{"middle threshold", 10000, router.ThinkMiddle},
		{"highest level", 40000, router.ThinkHighest},
		{"highest threshold", 32000, router.ThinkHighest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var thinking *anthropic.ThinkingConfig
			if tt.budgetTokens > 0 {
				thinking = &anthropic.ThinkingConfig{BudgetTokens: tt.budgetTokens}
			}
			req := &anthropic.Request{Thinking: thinking}
			result := handler.getThinkLevel(req)
			if result != tt.expected {
				t.Errorf("expected %v for budget %d, got %v", tt.expected, tt.budgetTokens, result)
			}
		})
	}
}

func TestGetThinkLevel_NilThinking(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	req := &anthropic.Request{Thinking: nil}
	result := handler.getThinkLevel(req)
	if result != router.ThinkNone {
		t.Errorf("expected ThinkNone for nil thinking, got %v", result)
	}
}

func TestTryTarget_ProviderNotFound(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetTransformerRegistry(&mockTransformerRegistry{})
	handler.SetProviderClients(map[string]HTTPClient{})
	handler.SetConfig(&config.Config{})

	target := config.RouteTarget{Provider: "unknown", Model: "test-model"}
	req := &anthropic.Request{}

	_, err := handler.tryTarget(context.Background(), req, target)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "provider not found") {
		t.Errorf("expected 'provider not found' error, got %v", err)
	}
}

func TestTryTarget_TransformerNotFound(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetTransformerRegistry(&mockTransformerRegistry{})
	handler.SetProviderClients(map[string]HTTPClient{"anthropic": &mockHTTPClient{}})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "unknown",
			},
		},
	})

	target := config.RouteTarget{Provider: "anthropic", Model: "claude-3"}
	req := &anthropic.Request{}

	// Should fall back to anthropic transformer and succeed
	resp, err := handler.tryTarget(context.Background(), req, target)
	if err != nil {
		t.Errorf("expected success with fallback transformer, got error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}
}

func TestTryTarget_ClientNotFound(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetTransformerRegistry(&mockTransformerRegistry{})
	handler.SetProviderClients(map[string]HTTPClient{})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	target := config.RouteTarget{Provider: "anthropic", Model: "claude-3"}
	req := &anthropic.Request{}

	_, err := handler.tryTarget(context.Background(), req, target)
	if err == nil {
		t.Error("expected error for missing client")
	}
	if !strings.Contains(err.Error(), "client not found") {
		t.Errorf("expected 'client not found' error, got %v", err)
	}
}

func TestTryTarget_TransformerError(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	// Create a transformer that always fails
	failTransformer := &failingTransformer{}

	reg := &mockTransformerRegistry{}
	reg.addTransformer("anthropic", failTransformer)

	handler.SetTransformerRegistry(reg)
	handler.SetProviderClients(map[string]HTTPClient{"anthropic": &mockHTTPClient{}})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	target := config.RouteTarget{Provider: "anthropic", Model: "claude-3"}
	req := &anthropic.Request{}

	_, err := handler.tryTarget(context.Background(), req, target)
	if err == nil {
		t.Error("expected error from transformer")
	}
}

// failingTransformer is a transformer that always fails
type failingTransformer struct{}

func (m *failingTransformer) Name() string { return "failing" }
func (m *failingTransformer) TransformRequest(req *anthropic.Request, baseURL, apiKey, model string) (*http.Request, error) {
	return nil, fmt.Errorf("transform failed")
}
func (m *failingTransformer) TransformResponse(resp *http.Response) (*anthropic.Response, error) {
	return nil, fmt.Errorf("transform failed")
}
func (m *failingTransformer) SupportsStreaming() bool {
	return false
}
func (m *failingTransformer) TransformSSEEvent(event *transformer.SSEEvent) ([]transformer.SSEEvent, error) {
	return nil, nil
}
func (m *failingTransformer) TransformStreamChunk(chunk []byte, eventType string) ([]byte, error) {
	return chunk, nil
}

func TestHandler_Setters(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	router := &mockRouter{}
	handler.SetRouter(router)
	if handler.router == nil {
		t.Error("expected router to be set")
	}

	reg := &mockTransformerRegistry{}
	handler.SetTransformerRegistry(reg)
	if handler.transformerRegistry == nil {
		t.Error("expected transformerRegistry to be set")
	}

	clients := map[string]HTTPClient{}
	handler.SetProviderClients(clients)
	if handler.providerClients == nil {
		t.Error("expected providerClients to be set")
	}

	cfg := &config.Config{}
	handler.SetConfig(cfg)
	if handler.config == nil {
		t.Error("expected config to be set")
	}

	tracker := &mockUsageTracker{}
	handler.SetUsageTracker(tracker)
	if handler.usageTracker == nil {
		t.Error("expected usageTracker to be set")
	}

	handler.SetInstanceID("test-instance")
	if handler.instanceID != "test-instance" {
		t.Errorf("expected instanceID 'test-instance', got %s", handler.instanceID)
	}
}

func TestHandleMessages_UsageTracking(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tracker := &mockUsageTracker{}
	handler.SetUsageTracker(tracker)
	handler.SetInstanceID("inst-123")

	handler.SetRouter(&mockRouter{
		detectRouteFunc: func(req router.RouteRequest) string {
			return "test-route"
		},
		getTargetsFunc: func(routeName string) []config.RouteTarget {
			return []config.RouteTarget{{Provider: "anthropic", Model: "claude-3"}}
		},
	})

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			resp := &anthropic.Response{
				ID:      "msg_123",
				Type:    "message",
				Content: []anthropic.ContentBlock{{Type: "text", Text: "test"}},
			}
			body, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		},
	}
	handler.SetProviderClients(map[string]HTTPClient{"anthropic": mockClient})
	handler.SetTransformerRegistry(&mockTransformerRegistry{})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL: "https://api.anthropic.com",
				APIKey:  "test-key",
			},
		},
	})

	reqBody := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	anthropicReq := &anthropic.Request{}
	json.Unmarshal([]byte(reqBody), anthropicReq)
	handler.handleMessages(w, req, anthropicReq)

	if len(tracker.records) != 1 {
		t.Errorf("expected 1 usage record, got %d", len(tracker.records))
	}

	record := tracker.records[0]
	if record.instanceID != "inst-123" {
		t.Errorf("expected instanceID 'inst-123', got %s", record.instanceID)
	}
	if record.route != "test-route" {
		t.Errorf("expected route 'test-route', got %s", record.route)
	}
	if record.model != "claude-3" {
		t.Errorf("expected model 'claude-3', got %s", record.model)
	}
}

func TestHandleMessages_AllProvidersFailed(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetRouter(&mockRouter{})
	handler.SetTransformerRegistry(&mockTransformerRegistry{})

	failClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("provider failed")
		},
	}

	handler.SetProviderClients(map[string]HTTPClient{"anthropic": failClient})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	reqBody := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	anthropicReq := &anthropic.Request{}
	json.Unmarshal([]byte(reqBody), anthropicReq)
	handler.handleMessages(w, req, anthropicReq)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", w.Code)
	}
}

func TestEstimateTokens_LargeText(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	largeText := strings.Repeat("a", 10000)
	req := &anthropic.Request{
		Messages: []anthropic.Message{
			{Content: []anthropic.ContentBlock{{Type: "text", Text: largeText}}},
		},
	}

	result := handler.estimateTokens(req)
	expected := len(largeText) / 4
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestHasWebSearch_CaseInsensitive(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []string{
		"web_search",
		"Web_Search",
		"WEB_SEARCH",
		"WebSearch",
		"weBseArch",
		"search_web",
		"Search",
		"SEARCH",
	}

	for _, toolName := range tests {
		req := &anthropic.Request{
			Tools: []anthropic.Tool{{Name: toolName}},
		}
		if !handler.hasWebSearch(req) {
			t.Errorf("expected true for tool name %s", toolName)
		}
	}
}

func TestGetThinkLevel_ExactThresholds(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	tests := []struct {
		budget   int
		expected router.ThinkLevel
	}{
		{0, router.ThinkNone},
		{1, router.ThinkBasic},
		{3999, router.ThinkBasic},
		{4000, router.ThinkBasic},
		{4001, router.ThinkBasic},
		{9999, router.ThinkBasic},
		{10000, router.ThinkMiddle},
		{10001, router.ThinkMiddle},
		{31999, router.ThinkMiddle},
		{32000, router.ThinkHighest},
		{100000, router.ThinkHighest},
	}

	for _, tt := range tests {
		req := &anthropic.Request{
			Thinking: &anthropic.ThinkingConfig{BudgetTokens: tt.budget},
		}
		result := handler.getThinkLevel(req)
		if result != tt.expected {
			t.Errorf("budget %d: expected %v, got %v", tt.budget, tt.expected, result)
		}
	}
}

func TestServeHTTP_SupportedEndpoints(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetRouter(&mockRouter{})
	handler.SetTransformerRegistry(&mockTransformerRegistry{})

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			resp := &anthropic.Response{
				ID:      "msg_123",
				Type:    "message",
				Content: []anthropic.ContentBlock{{Type: "text", Text: "test"}},
			}
			body, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		},
	}
	handler.SetProviderClients(map[string]HTTPClient{"anthropic": mockClient})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				BaseURL:     "https://api.anthropic.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	reqBody := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]
	}`

	endpoints := []string{
		"/v1/messages",
		"/v1/messages/with_overrides",
		"/v1/messages/batches",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(reqBody))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("endpoint %s: expected status 200, got %d", endpoint, w.Code)
			}
			if w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("endpoint %s: expected Content-Type application/json, got %s", endpoint, w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestServeHTTP_ModelsEndpoint(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"bigmodel": {
				Models: []string{"glm-4.7", "glm-4.5-air"},
			},
			"openrouter": {
				Models: []string{"anthropic/claude-sonnet-4", "google/gemini-2.5-pro"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("expected object 'list', got %s", response.Object)
	}

	expectedModels := 4
	if len(response.Data) != expectedModels {
		t.Errorf("expected %d models, got %d", expectedModels, len(response.Data))
	}

	// Verify each model has correct object type
	for _, model := range response.Data {
		if model.Object != "model" {
			t.Errorf("model %s: expected object 'model', got %s", model.ID, model.Object)
		}
	}
}

func TestServeHTTP_ModelsEndpoint_NoProviders(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 models with no providers, got %d", len(response.Data))
	}
}

func TestServeHTTP_UnsupportedEndpoint(t *testing.T) {
	handler := NewHandler(1024 * 1024)

	unsupportedEndpoints := []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings",
		"/v1/files",
		"/health",
	}

	for _, endpoint := range unsupportedEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("endpoint %s: expected status 404, got %d", endpoint, w.Code)
			}
		})
	}
}

// TestTryStreamingTarget_InvalidJSONInStream tests that invalid JSON from provider
// doesn't crash the handler and is properly handled.
func TestTryStreamingTarget_InvalidJSONInStream(t *testing.T) {
	handler := NewHandler(1024 * 1024)
	handler.SetTransformerRegistry(&mockTransformerRegistry{})
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {
				BaseURL:     "https://test.example.com",
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	// Create a mock provider that sends invalid JSON
	invalidJSONCalled := false
	mockProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invalidJSONCalled = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Streaming not supported")
		}

		// Send Anthropic-compatible initialization events
		w.Write([]byte("event: message_start\n"))
		w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"test-model\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n"))
		flusher.Flush()

		w.Write([]byte("event: content_block_start\n"))
		w.Write([]byte("data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		flusher.Flush()

		// Send invalid JSON that would cause "[object Object]" if not handled
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte("data: [object Object]\n\n"))
		flusher.Flush()

		// Send proper stop events
		w.Write([]byte("event: content_block_stop\n"))
		w.Write([]byte("data: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		flusher.Flush()
		w.Write([]byte("event: message_stop\n"))
		w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
		flusher.Flush()
	}))
	defer mockProvider.Close()

	// Update config to use the mock server URL
	handler.SetConfig(&config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {
				BaseURL:     mockProvider.URL,
				APIKey:      "test-key",
				Transformer: "anthropic",
			},
		},
	})

	handler.SetProviderClients(map[string]HTTPClient{"test": &http.Client{}})

	target := config.RouteTarget{Provider: "test", Model: "test-model"}
	req := &anthropic.Request{
		Model:     "test-model",
		MaxTokens: 100,
		Stream:    true,
		Messages:  []anthropic.Message{{Role: "user", Content: []anthropic.ContentBlock{{Type: "text", Text: "hello"}}}},
	}

	w := httptest.NewRecorder()
	// Set SSE headers since we're calling tryStreamingTarget directly
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	err := handler.tryStreamingTarget(context.Background(), w, w, req, target)

	// The handler should handle the invalid JSON gracefully
	// Invalid JSON should be skipped (logged but not crash)
	if err != nil {
		t.Logf("Expected non-nil error for stream completion, got: %v", err)
	}

	// Verify the provider was called
	if !invalidJSONCalled {
		t.Error("Mock provider was not called")
	}

	// Verify response has SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", w.Header().Get("Content-Type"))
	}

	// Verify message_start and content_block_start were emitted
	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "event: message_start") {
		t.Error("Missing 'message_start' event in response")
	}
	if !strings.Contains(responseBody, "event: content_block_start") {
		t.Error("Missing 'content_block_start' event in response")
	}

	// Verify the invalid JSON is NOT in the response (should be skipped)
	if strings.Contains(responseBody, "[object Object]") {
		t.Error("Invalid JSON '[object Object]' should not appear in response")
	}
}
