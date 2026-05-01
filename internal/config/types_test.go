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

func TestGetActiveRoutes_Legacy(t *testing.T) {
	cfg := &Config{
		Router: RouterConfig{
			Routes: map[string]string{
				"default": "openrouter:claude-sonnet-4",
				"think":   "openrouter:claude-opus-4",
			},
			Profiles: map[string]ProfileConfig{}, // Empty
		},
	}

	routes := cfg.GetActiveRoutes("") // Empty profile name falls back to legacy

	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}
	if routes["default"] != "openrouter:claude-sonnet-4" {
		t.Errorf("unexpected default route: %s", routes["default"])
	}
	if routes["think"] != "openrouter:claude-opus-4" {
		t.Errorf("unexpected think route: %s", routes["think"])
	}
}

func TestGetActiveRoutes_Profile(t *testing.T) {
	cfg := &Config{
		Router: RouterConfig{
			Routes: map[string]string{
				"default": "openrouter:claude-sonnet-4", // Legacy, should be ignored
			},
			Profiles: map[string]ProfileConfig{
				"fast": {
					Name:        "Fast Profile",
					Description: "Fast models",
					Routes: map[string]string{
						"default": "openrouter:gpt-4-mini",
						"think":   "openrouter:gpt-4-mini",
					},
				},
				"quality": {
					Name:        "Quality Profile",
					Description: "Best models",
					Routes: map[string]string{
						"default": "openrouter:claude-opus-4",
						"think":   "openrouter:claude-opus-4",
					},
				},
			},
		},
	}

	routes := cfg.GetActiveRoutes("quality") // Specify profile name

	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}
	// Should use quality profile routes, not legacy or fast profile
	if routes["default"] != "openrouter:claude-opus-4" {
		t.Errorf("unexpected default route: %s (expected quality profile)", routes["default"])
	}
}

func TestGetActiveRoutes_MissingProfile(t *testing.T) {
	cfg := &Config{
		Router: RouterConfig{
			Routes: map[string]string{
				"default": "openrouter:claude-sonnet-4",
			},
			Profiles: map[string]ProfileConfig{
				"fast": {
					Name:   "Fast",
					Routes: map[string]string{"default": "openrouter:gpt-4-mini"},
				},
			},
		},
	}

	routes := cfg.GetActiveRoutes("nonexistent") // Profile doesn't exist

	// Should fall back to legacy routes since profile doesn't exist
	if routes["default"] != "openrouter:claude-sonnet-4" {
		t.Errorf("expected fallback to legacy routes, got: %s", routes["default"])
	}
}

func TestGetActiveRoutes_EmptyProfileName(t *testing.T) {
	cfg := &Config{
		Router: RouterConfig{
			Routes: map[string]string{
				"default": "openrouter:claude-sonnet-4",
			},
			Profiles: map[string]ProfileConfig{
				"fast": {
					Name:   "Fast",
					Routes: map[string]string{"default": "openrouter:gpt-4-mini"},
				},
			},
		},
	}

	routes := cfg.GetActiveRoutes("") // Empty profile name

	// Should fall back to legacy routes since profile name is empty
	if routes["default"] != "openrouter:claude-sonnet-4" {
		t.Errorf("expected fallback to legacy routes, got: %s", routes["default"])
	}
}

func TestHasProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles map[string]ProfileConfig
		want     bool
	}{
		{
			name:     "no profiles",
			profiles: map[string]ProfileConfig{},
			want:     false,
		},
		{
			name:     "nil profiles",
			profiles: nil,
			want:     false,
		},
		{
			name: "one profile",
			profiles: map[string]ProfileConfig{
				"default": {Name: "Default"},
			},
			want: true,
		},
		{
			name: "multiple profiles",
			profiles: map[string]ProfileConfig{
				"fast":    {Name: "Fast"},
				"quality": {Name: "Quality"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Router: RouterConfig{Profiles: tt.profiles}}
			if got := cfg.HasProfiles(); got != tt.want {
				t.Errorf("HasProfiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProfileNames(t *testing.T) {
	cfg := &Config{
		Router: RouterConfig{
			Profiles: map[string]ProfileConfig{
				"fast":    {Name: "Fast"},
				"quality": {Name: "Quality"},
				"default": {Name: "Default"},
			},
		},
	}

	names := cfg.GetProfileNames()

	if len(names) != 3 {
		t.Errorf("expected 3 profile names, got %d", len(names))
	}

	// Verify sorted order
	expected := []string{"default", "fast", "quality"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("GetProfileNames()[%d] = %q, want %q (should be sorted)", i, name, expected[i])
		}
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