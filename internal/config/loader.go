package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobalConfigPath returns the global config file path.
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-modelrouter", "config.json")
}

// ProjectConfigPath returns the project config file path.
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".cc-modelrouter", "config.json")
}

// Load loads configuration from a file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Interpolate environment variables and get warnings
	expanded, warnings := interpolateEnvVars(string(data))

	// Print warnings for missing environment variables
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}

	cfg := Defaults()
	if err := json.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// LoadWithOverride loads project config if exists, otherwise global.
func LoadWithOverride(projectRoot string) (*Config, string, error) {
	projectPath := ProjectConfigPath(projectRoot)
	if _, err := os.Stat(projectPath); err == nil {
		cfg, err := Load(projectPath)
		if err != nil {
			return nil, "", err
		}
		return cfg, "project", nil
	}

	globalPath := GlobalConfigPath()
	if _, err := os.Stat(globalPath); err == nil {
		cfg, err := Load(globalPath)
		if err != nil {
			return nil, "", err
		}
		return cfg, "global", nil
	}

	return nil, "", fmt.Errorf("no configuration found")
}

// Save saves configuration to a file.
func Save(cfg *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// interpolateEnvVars replaces ${VAR} and $VAR with environment variable values.
// Returns the expanded string and a list of warnings for missing environment variables.
func interpolateEnvVars(s string) (string, []string) {
	result := s
	var warnings []string

	// Replace ${VAR} patterns
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		varName := result[start+2 : end]
		varValue := os.Getenv(varName)
		if varValue == "" {
			warnings = append(warnings, fmt.Sprintf("environment variable '%s' is not set", varName))
		}
		result = result[:start] + varValue + result[end+1:]
	}

	// Replace $VAR patterns (word boundary)
	words := strings.Fields(result)
	for _, word := range words {
		if strings.HasPrefix(word, "$") && !strings.Contains(word, "{") {
			varName := word[1:]
			// Handle punctuation at end
			varName = strings.TrimRight(varName, ".,;:!?")
			varValue := os.Getenv(varName)
			if varValue != "" {
				result = strings.ReplaceAll(result, word, varValue)
			} else {
				warnings = append(warnings, fmt.Sprintf("environment variable '%s' is not set", varName))
			}
		}
	}

	return result, warnings
}
