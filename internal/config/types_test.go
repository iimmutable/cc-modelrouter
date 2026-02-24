package config

import (
	"testing"
)

func TestProviderConfigValidation(t *testing.T) {
	pc := &ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
		Models:  []string{"model-1", "model-2"},
	}

	if err := pc.Validate(); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}

	pc.APIKey = ""
	if err := pc.Validate(); err == nil {
		t.Error("missing API key should fail validation")
	}
}

func TestRouteParsing(t *testing.T) {
	route := "bigmodel:glm-4.7;openrouter:anthropic/claude-sonnet-4.5"
	targets := ParseRoute(route)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Provider != "bigmodel" || targets[0].Model != "glm-4.7" {
		t.Errorf("unexpected first target: %+v", targets[0])
	}

	if targets[1].Provider != "openrouter" || targets[1].Model != "anthropic/claude-sonnet-4.5" {
		t.Errorf("unexpected second target: %+v", targets[1])
	}
}

func TestLoggingConfigMethods(t *testing.T) {
	tests := []struct {
		name          string
		cfg           LoggingConfig
		wantEnabled   bool
		wantLevel     LogLevel
	}{
		{
			name: "with debug level",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "debug",
			},
			wantEnabled: true,
			wantLevel:   LevelDebug,
		},
		{
			name: "disabled logging",
			cfg: LoggingConfig{
				Enabled: false,
			},
			wantEnabled: false,
			wantLevel:   LevelInfo,
		},
		{
			name: "with info level",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "info",
			},
			wantEnabled: true,
			wantLevel:   LevelInfo,
		},
		{
			name: "with warn level",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "warn",
			},
			wantEnabled: true,
			wantLevel:   LevelWarn,
		},
		{
			name: "with error level",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "error",
			},
			wantEnabled: true,
			wantLevel:   LevelError,
		},
		{
			name: "empty level defaults to info",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "",
			},
			wantEnabled: true,
			wantLevel:   LevelInfo,
		},
		{
			name: "case insensitive parsing",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   "DEBUG",
			},
			wantEnabled: true,
			wantLevel:   LevelDebug,
		},
		{
			name: "whitespace handling",
			cfg: LoggingConfig{
				Enabled: true,
				Level:   " info ",
			},
			wantEnabled: true,
			wantLevel:   LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsEnabled(); got != tt.wantEnabled {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.wantEnabled)
			}
			if got := tt.cfg.GetLevel(); got != tt.wantLevel {
				t.Errorf("GetLevel() = %v, want %v", got, tt.wantLevel)
			}
		})
	}
}
