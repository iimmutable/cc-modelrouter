// Package config handles configuration loading and management.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel represents the severity level for logging.
type LogLevel int

const (
	// LevelDebug is the lowest level, most verbose logging.
	LevelDebug LogLevel = iota
	// LevelInfo is the default level for general information.
	LevelInfo
	// LevelWarn is for warning messages.
	LevelWarn
	// LevelError is for error messages only, least verbose.
	LevelError
)

// Config represents the complete configuration.
type Config struct {
	Server    ServerConfig              `json:"server"`
	Providers map[string]ProviderConfig `json:"providers"`
	Router    RouterConfig              `json:"router"`
	Logging   LoggingConfig             `json:"logging,omitempty"`
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

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	// Enabled controls whether logging is active.
	// If false or not specified, logging is disabled.
	// Default: false (opt-in only)
	Enabled bool `json:"enabled,omitempty"`

	// Destination is where logs should be written.
	// Valid values: "stdout", "stderr", "file", or a file path.
	// If "file", uses the default log file path.
	Destination string `json:"destination,omitempty"`

	// FilePath is the specific file path when Destination is "file" or a custom path.
	// If empty, uses the default: ~/.cc-modelrouter/router.log
	FilePath string `json:"filePath,omitempty"`

	// Level controls log verbosity.
	// Valid values: "debug", "info" (default), "warn", "error".
	// - debug: Shows all messages including detailed streaming events
	// - info: Shows request/response summaries and warnings
	// - warn: Shows only warnings and errors
	// - error: Shows only errors
	Level string `json:"level,omitempty"`
}

// GetLogPath returns the resolved log file path.
func (lc *LoggingConfig) GetLogPath() (string, error) {
	if lc.FilePath != "" {
		return lc.FilePath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".cc-modelrouter", "router.log"), nil
}

// GetLogPathWithInstance returns the resolved log file path with an instance ID.
// The log file will be named <instanceID>.log in the logs directory.
func (lc *LoggingConfig) GetLogPathWithInstance(instanceID string) (string, error) {
	if lc.FilePath != "" {
		return lc.FilePath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".cc-modelrouter", "logs", instanceID+".log"), nil
}

// ShouldLogToFile returns true if logs should go to a file.
func (lc *LoggingConfig) ShouldLogToFile() bool {
	return lc.Destination == "file" || (lc.Destination != "" && lc.Destination != "stdout" && lc.Destination != "stderr")
}

// ShouldLogToConsole returns true if logs should go to console.
func (lc *LoggingConfig) ShouldLogToConsole() bool {
	return lc.Destination == "stdout" || lc.Destination == "stderr"
}

// IsEnabled returns true if logging is explicitly enabled.
func (lc *LoggingConfig) IsEnabled() bool {
	return lc.Enabled
}

// GetLevel returns the parsed log level, defaulting to LevelInfo.
func (lc *LoggingConfig) GetLevel() LogLevel {
	if lc.Level == "" {
		return LevelInfo // default
	}
	return ParseLogLevel(lc.Level)
}

// GetLogWriter returns the appropriate writer for the log destination.
func (lc *LoggingConfig) GetLogWriter() (io.Writer, error) {
	switch lc.Destination {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "", "file":
		// Default to file
		logPath, err := lc.GetLogPath()
		if err != nil {
			return nil, err
		}
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return nil, err
		}
		return os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	default:
		// Treat as a custom file path
		if err := os.MkdirAll(filepath.Dir(lc.Destination), 0755); err != nil {
			return nil, err
		}
		return os.OpenFile(lc.Destination, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
}
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
		// Logging is opt-in - not included in defaults
	}
}

// String returns the string representation of the LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ShouldLog returns true if the given message level should be logged
// based on the current log level. Messages with a level equal to or
// higher than the current level will be logged.
func (l LogLevel) ShouldLog(msgLevel LogLevel) bool {
	return msgLevel >= l
}

// ParseLogLevel parses a string into a LogLevel.
// The comparison is case-insensitive. Empty or invalid strings
// default to LevelInfo.
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
