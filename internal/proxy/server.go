// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/config"
)

// ServerConfig represents server configuration.
type ServerConfig struct {
	Host           string
	Port           int
	MaxRequestSize int64
}

// Defaults returns default server configuration.
func Defaults() *ServerConfig {
	return &ServerConfig{
		Host:           "localhost",
		Port:           8081,
		MaxRequestSize: 50 * 1024 * 1024, // 50MB
	}
}

// Server is the HTTP proxy server.
type Server struct {
	config  *ServerConfig
	server  *http.Server
	handler *Handler
	mu      sync.Mutex
	running bool
}

// NewServer creates a new proxy server.
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg == nil {
		cfg = Defaults()
	}
	if cfg.MaxRequestSize == 0 {
		cfg.MaxRequestSize = Defaults().MaxRequestSize
	}

	handler := NewHandler(cfg.MaxRequestSize)

	return &Server{
		config:  cfg,
		handler: handler,
	}, nil
}

// SetRouter sets the router for the handler.
func (s *Server) SetRouter(router Router) {
	s.handler.SetRouter(router)
}

// SetTransformerRegistry sets the transformer registry.
func (s *Server) SetTransformerRegistry(reg TransformerRegistry) {
	s.handler.SetTransformerRegistry(reg)
}

// SetProviderClients sets the provider clients.
func (s *Server) SetProviderClients(clients map[string]HTTPClient) {
	s.handler.SetProviderClients(clients)
}

// SetConfig sets the configuration.
func (s *Server) SetConfig(cfg *config.Config) {
	s.handler.SetConfig(cfg)
}

// Start starts the server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Long for streaming
	}

	s.running = true

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop stops the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	err := s.server.Shutdown(ctx)
	s.running = false
	return err
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
