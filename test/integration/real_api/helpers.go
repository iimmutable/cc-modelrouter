//go:build integration_real
// +build integration_real

package real_api

import (
	"os"
	"strings"
	"testing"
)

// getAPIKey retrieves an API key from environment variables with proper skipping.
func getAPIKey(provider string) string {
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

// skipIfNoKey skips the test if the API key is not available.
func skipIfNoKey(t *testing.T, provider string) {
	key := getAPIKey(provider)
	if key == "" {
		t.Skipf("%s_API_KEY not set, skipping real API tests", strings.ToUpper(provider))
	}
}