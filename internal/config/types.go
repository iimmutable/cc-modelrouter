// Package config handles configuration loading and management.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the complete configuration.
type Config struct {
	Server    ServerConfig              `json:"server"`
	Providers map[string]ProviderConfig `json:"providers"`
	Router    RouterConfig              `json:"router"`
}

// ServerConfig represents server configuration.
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// ProviderConfig represents a provider configuration.
type ProviderConfig struct {
	APIKey      string   `json:"apiKey"`
	BaseURL     string   `json:"baseURL"`
	Models      []string `json:"models"`
	Transformer string   `json:"transformer,omitempty"`
}

// Validate validates the provider configuration.
func (pc *ProviderConfig) Validate() error {
	if pc.APIKey == "" {
		return fmt.Errorf("apiKey is required")
	}
	if pc.BaseURL == "" {
		return fmt.Errorf("baseURL is required")
	}
	if len(pc.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	return nil
}

// RouterConfig represents router configuration.
type RouterConfig struct {
	Routes     map[string]string `json:"routes"`
	MaxRetries int               `json:"maxRetries"`
	RetryDelay string            `json:"retryDelay"`
}

// GetRetryDelay returns the retry delay as a duration.
func (rc *RouterConfig) GetRetryDelay() time.Duration {
	d, err := time.ParseDuration(rc.RetryDelay)
	if err != nil {
		return 500 * time.Millisecond
	}
	return d
}

// RouteTarget represents a parsed route target.
type RouteTarget struct {
	Provider string
	Model    string
}

// ParseRoute parses a route string into targets.
// Format: "provider1:model1;provider2:model2"
func ParseRoute(route string) []RouteTarget {
	var targets []RouteTarget
	parts := strings.Split(route, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		providerModel := strings.SplitN(part, ":", 2)
		if len(providerModel) != 2 {
			continue
		}

		targets = append(targets, RouteTarget{
			Provider: strings.TrimSpace(providerModel[0]),
			Model:    strings.TrimSpace(providerModel[1]),
		})
	}

	return targets
}

// Defaults returns the default configuration.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8081,
			Host: "localhost",
		},
		Providers: make(map[string]ProviderConfig),
		Router: RouterConfig{
			Routes:     make(map[string]string),
			MaxRetries: 2,
			RetryDelay: "500ms",
		},
	}
}
