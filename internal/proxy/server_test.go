package proxy

import (
	"context"
	"testing"
	"time"
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
