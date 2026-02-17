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
