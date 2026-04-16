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
		return os.Getenv("CCROUTER_OPENROUTER_API_KEY")
	case "bigmodel", "glm", "zhipu":
		return os.Getenv("CCROUTER_BIGMODEL_API_KEY")
	case "aliyun", "qwen", "dashscope":
		return os.Getenv("CCROUTER_ALIYUN_API_KEY")
	case "anthropic":
		return os.Getenv("CCROUTER_ANTHROPIC_API_KEY")
	case "openai":
		return os.Getenv("CCROUTER_OPENAI_API_KEY")
	case "gemini", "google":
		return os.Getenv("CCROUTER_GEMINI_API_KEY")
	case "minimax":
		return os.Getenv("CCROUTER_MINIMAX_API_KEY")
	default:
		return ""
	}
}

// skipIfNoKey skips the test if the API key is not available.
func skipIfNoKey(t *testing.T, provider string) {
	key := getAPIKey(provider)
	if key == "" {
		t.Skipf("CCROUTER_%s_API_KEY not set, skipping real API tests", strings.ToUpper(provider))
	}
}