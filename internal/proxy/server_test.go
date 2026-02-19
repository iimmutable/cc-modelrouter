package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/pkg/api/anthropic"
)

func TestNewServer(t *testing.T) {
	cfg := &ServerConfig{
		Host: "localhost",
		Port: 8081,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Error("expected non-nil server")
	}
}

func TestNewServer_NilConfig(t *testing.T) {
	server, err := NewServer(nil)
	if err != nil {
		t.Fatalf("failed to create server with nil config: %v", err)
	}

	if server == nil {
		t.Error("expected non-nil server")
	}

	// Should use defaults
	if server.config.Host != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", server.config.Host)
	}
	if server.config.Port != 8081 {
		t.Errorf("expected port 8081, got %d", server.config.Port)
	}
}

func TestNewServer_DefaultMaxRequestSize(t *testing.T) {
	cfg := &ServerConfig{
		Host: "localhost",
		Port: 8081,
		// MaxRequestSize not set, should use default
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	expectedMaxSize := int64(50 * 1024 * 1024) // 50MB
	if server.config.MaxRequestSize != expectedMaxSize {
		t.Errorf("expected max request size %d, got %d", expectedMaxSize, server.config.MaxRequestSize)
	}
}

func TestDefaults(t *testing.T) {
	defaults := Defaults()

	if defaults.Host != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", defaults.Host)
	}
	if defaults.Port != 8081 {
		t.Errorf("expected port 8081, got %d", defaults.Port)
	}
	if defaults.MaxRequestSize != 50*1024*1024 {
		t.Errorf("expected max request size 50MB, got %d", defaults.MaxRequestSize)
	}
}

func TestServer_Addr(t *testing.T) {
	cfg := &ServerConfig{
		Host: "127.0.0.1",
		Port: 9999,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	expected := "127.0.0.1:9999"
	if server.Addr() != expected {
		t.Errorf("expected addr '%s', got '%s'", expected, server.Addr())
	}
}

func TestServer_IsRunning(t *testing.T) {
	server, err := NewServer(&ServerConfig{Host: "localhost", Port: 8082})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server.IsRunning() {
		t.Error("expected server to not be running initially")
	}

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	if !server.IsRunning() {
		t.Error("expected server to be running after Start()")
	}

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}

	if server.IsRunning() {
		t.Error("expected server to not be running after Stop()")
	}
}

func TestServer_StartTwice(t *testing.T) {
	server, err := NewServer(&ServerConfig{Host: "localhost", Port: 8083})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop(context.Background())

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Try to start again
	err = server.Start()
	if err == nil {
		t.Error("expected error when starting already running server")
	}
}

func TestServer_StopWhenNotRunning(t *testing.T) {
	server, err := NewServer(&ServerConfig{Host: "localhost", Port: 8084})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Stop should not error when server is not running
	ctx := context.Background()
	if err := server.Stop(ctx); err != nil {
		t.Errorf("expected no error when stopping non-running server, got: %v", err)
	}
}

func TestNewHandler(t *testing.T) {
	handler := NewHandler(50 * 1024 * 1024)

	if handler == nil {
		t.Error("expected non-nil handler")
	}
	if handler.maxRequestSize != 50*1024*1024 {
		t.Errorf("expected max request size 50MB, got %d", handler.maxRequestSize)
	}
	if handler.providerClients == nil {
		t.Error("expected providerClients map to be initialized")
	}
}

func TestIsBackground(t *testing.T) {
	handler := NewHandler(50 * 1024 * 1024)

	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{"claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022", true},
		{"claude-3-5-haiku-latest", "claude-3-5-haiku-latest", true},
		{"claude-haiku-4-20250514", "claude-haiku-4-20250514", true},
		{"claude-sonnet-4-20250514", "claude-sonnet-4-20250514", false},
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022", false},
		{"claude-opus-4-20250514", "claude-opus-4-20250514", false},
		{"gpt-4o", "gpt-4o", false},
		{"deepseek-chat", "deepseek-chat", false},
		{"Haiku", "Haiku", false}, // Must contain "claude" and "haiku"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{Model: tt.model}
			result := handler.isBackground(req)
			if result != tt.expected {
				t.Errorf("isBackground(%s) = %v, expected %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetThinkLevel(t *testing.T) {
	handler := NewHandler(50 * 1024 * 1024)

	tests := []struct {
		name     string
		thinking *anthropic.ThinkingConfig
		expected router.ThinkLevel
	}{
		{"nil thinking", nil, router.ThinkNone},
		{"thinking with budget 0", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 0}, router.ThinkNone},
		{"thinking with budget 1000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 1000}, router.ThinkBasic},
		{"thinking with budget 4000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 4000}, router.ThinkBasic},
		{"thinking with budget 5000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 5000}, router.ThinkBasic}, // < 10000
		{"thinking with budget 10000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 10000}, router.ThinkMiddle},
		{"thinking with budget 20000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 20000}, router.ThinkMiddle}, // < 32000
		{"thinking with budget 32000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 32000}, router.ThinkHighest},
		{"thinking with budget 50000", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 50000}, router.ThinkHighest},
		{"thinking with budget 1", &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 1}, router.ThinkBasic}, // Any non-zero is basic
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{Thinking: tt.thinking}
			result := handler.getThinkLevel(req)
			if result != tt.expected {
				t.Errorf("getThinkLevel() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestHasWebSearch(t *testing.T) {
	handler := NewHandler(50 * 1024 * 1024)

	tests := []struct {
		name     string
		tools    []anthropic.Tool
		expected bool
	}{
		{"no tools", nil, false},
		{"empty tools", []anthropic.Tool{}, false},
		{"web_search tool", []anthropic.Tool{{Name: "web_search"}}, true},
		{"WebSearch tool", []anthropic.Tool{{Name: "WebSearch"}}, true},
		{"search tool", []anthropic.Tool{{Name: "search_files"}}, true},
		{"web tool", []anthropic.Tool{{Name: "web_fetch"}}, true},
		{"unrelated tool", []anthropic.Tool{{Name: "read_file"}}, false},
		{"multiple tools with web", []anthropic.Tool{{Name: "read_file"}, {Name: "web_search"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{Tools: tt.tools}
			result := handler.hasWebSearch(req)
			if result != tt.expected {
				t.Errorf("hasWebSearch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestHasImages(t *testing.T) {
	handler := NewHandler(50 * 1024 * 1024)

	tests := []struct {
		name     string
		messages []anthropic.Message
		expected bool
	}{
		{"no messages", nil, false},
		{"text only", []anthropic.Message{
			{Content: []anthropic.ContentBlock{{Type: "text", Text: "hello"}}},
		}, false},
		{"image content", []anthropic.Message{
			{Content: []anthropic.ContentBlock{{Type: "image"}}},
		}, true},
		{"mixed content", []anthropic.Message{
			{Content: []anthropic.ContentBlock{
				{Type: "text", Text: "hello"},
				{Type: "image"},
			}},
		}, true},
		{"image in second message", []anthropic.Message{
			{Content: []anthropic.ContentBlock{{Type: "text", Text: "hello"}}},
			{Content: []anthropic.ContentBlock{{Type: "image"}}},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{Messages: tt.messages}
			result := handler.hasImages(req)
			if result != tt.expected {
				t.Errorf("hasImages() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
