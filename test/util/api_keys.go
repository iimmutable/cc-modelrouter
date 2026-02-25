// Package util provides testing utilities for integration tests.
package util

import (
	"fmt"
	"os"
	"strings"
)

// GetAPIKey retrieves an API key from environment variables.
func GetAPIKey(provider string) string {
	switch strings.ToLower(provider) {
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "bigmodel", "glm", "zhipu":
		return os.Getenv("BIGMODEL_API_KEY")
	case "aliyun", "qwen", "dashscope":
		return os.Getenv("ALIYUN_API_KEY")
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "gemini", "google":
		return os.Getenv("GEMINI_API_KEY")
	case "minimax":
		return os.Getenv("MINIMAX_API_KEY")
	default:
		return ""
	}
}

// HasAPIKey checks if an API key is available for a provider.
func HasAPIKey(provider string) bool {
	return GetAPIKey(provider) != ""
}

// SkipIfNoKey skips the test if the API key is not available.
func SkipIfNoKey(provider string) SkipFunc {
	return func() (skip bool, reason string) {
		key := GetAPIKey(provider)
		if key == "" {
			return true, fmt.Sprintf("%s_API_KEY not set, skipping test", strings.ToUpper(provider))
		}
		return false, ""
	}
}

// SkipFunc is a function that determines whether to skip a test.
type SkipFunc func() (skip bool, reason string)

// Provider represents an LLM provider configuration for testing.
type Provider struct {
	Name        string
	APIKeyEnv   string
	BaseURL     string
	Model       string
	Transformer string
}

// AvailableProviders returns a list of providers with available API keys.
func AvailableProviders() []Provider {
	providers := []Provider{
		{
			Name:        "OpenRouter (Anthropic)",
			APIKeyEnv:   "OPENROUTER_API_KEY",
			BaseURL:     "https://openrouter.ai/api",
			Model:       "anthropic/claude-3.5-sonnet",
			Transformer: "anthropic",
		},
		{
			Name:        "BigModel (GLM)",
			APIKeyEnv:   "BIGMODEL_API_KEY",
			BaseURL:     "https://open.bigmodel.cn/api/anthropic",
			Model:       "glm-4",
			Transformer: "anthropic",
		},
		{
			Name:        "Aliyun (Qwen)",
			APIKeyEnv:   "ALIYUN_API_KEY",
			BaseURL:     "https://coding.dashscope.aliyuncs.com/apps/anthropic",
			Model:       "qwen-plus",
			Transformer: "anthropic",
		},
		{
			Name:        "Anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			BaseURL:     "https://api.anthropic.com",
			Model:       "claude-3-5-sonnet-20241022",
			Transformer: "anthropic",
		},
		{
			Name:        "OpenAI",
			APIKeyEnv:   "OPENAI_API_KEY",
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-4o",
			Transformer: "openai",
		},
		{
			Name:        "Gemini",
			APIKeyEnv:   "GEMINI_API_KEY",
			BaseURL:     "https://generativelanguage.googleapis.com/v1beta",
			Model:       "gemini-2.0-flash-exp",
			Transformer: "gemini",
		},
		{
			Name:        "MiniMax",
			APIKeyEnv:   "MINIMAX_API_KEY",
			BaseURL:     "https://api.minimax.chat/v1",
			Model:       "abab6.5s-chat",
			Transformer: "minimax",
		},
	}

	var available []Provider
	for _, p := range providers {
		if os.Getenv(p.APIKeyEnv) != "" {
			available = append(available, p)
		}
	}
	return available
}

// GetProviderInfo returns provider information by name.
func GetProviderInfo(name string) *Provider {
	nameLower := strings.ToLower(name)
	for _, p := range AvailableProviders() {
		if strings.ToLower(p.Name) == nameLower || strings.ToLower(p.Transformer) == nameLower {
			return &p
		}
	}
	return nil
}

// MaskAPIKey masks an API key for logging (shows first 8 and last 4 chars).
func MaskAPIKey(key string) string {
	if len(key) < 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}