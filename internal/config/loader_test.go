package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"server": {
			"port": 8081,
			"host": "localhost"
		},
		"providers": {
			"test": {
				"apiKey": "test-key",
				"baseURL": "https://api.test.com",
				"models": ["model-1"]
			}
		},
		"router": {
			"routes": {
				"default": "test:model-1"
			}
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 8081 {
		t.Errorf("expected port 8081, got %d", cfg.Server.Port)
	}

	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
}

func TestEnvVarInterpolation(t *testing.T) {
	os.Setenv("TEST_API_KEY", "secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	input := "${TEST_API_KEY}"
	result, _ := interpolateEnvVars(input)

	if result != "secret-key" {
		t.Errorf("expected 'secret-key', got '%s'", result)
	}
}
