package config

import (
	"testing"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected string
	}{
		{"Debug level", LevelDebug, "DEBUG"},
		{"Info level", LevelInfo, "INFO"},
		{"Warn level", LevelWarn, "WARN"},
		{"Error level", LevelError, "ERROR"},
		{"Unknown level", LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected LogLevel
	}{
		{"Lowercase debug", "debug", LevelDebug},
		{"Uppercase DEBUG", "DEBUG", LevelDebug},
		{"Mixed case DeBuG", "DeBuG", LevelDebug},
		{"Lowercase info", "info", LevelInfo},
		{"Uppercase INFO", "INFO", LevelInfo},
		{"Lowercase warn", "warn", LevelWarn},
		{"Uppercase WARN", "WARN", LevelWarn},
		{"Lowercase error", "error", LevelError},
		{"Uppercase ERROR", "ERROR", LevelError},
		{"Empty string defaults to INFO", "", LevelInfo},
		{"Invalid string defaults to INFO", "invalid", LevelInfo},
		{"Whitespace defaults to INFO", "   ", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLogLevel(tt.input); got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLogLevelShouldLog(t *testing.T) {
	tests := []struct {
		name      string
		current   LogLevel
		msgLevel  LogLevel
		expected  bool
	}{
		{"Debug level logs debug", LevelDebug, LevelDebug, true},
		{"Debug level logs info", LevelDebug, LevelInfo, true},
		{"Debug level logs warn", LevelDebug, LevelWarn, true},
		{"Debug level logs error", LevelDebug, LevelError, true},
		{"Info level skips debug", LevelInfo, LevelDebug, false},
		{"Info level logs info", LevelInfo, LevelInfo, true},
		{"Info level logs warn", LevelInfo, LevelWarn, true},
		{"Info level logs error", LevelInfo, LevelError, true},
		{"Warn level skips debug", LevelWarn, LevelDebug, false},
		{"Warn level skips info", LevelWarn, LevelInfo, false},
		{"Warn level logs warn", LevelWarn, LevelWarn, true},
		{"Warn level logs error", LevelWarn, LevelError, true},
		{"Error level skips debug", LevelError, LevelDebug, false},
		{"Error level skips info", LevelError, LevelInfo, false},
		{"Error level skips warn", LevelError, LevelWarn, false},
		{"Error level logs error", LevelError, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.current.ShouldLog(tt.msgLevel); got != tt.expected {
				t.Errorf("LogLevel(%v).ShouldLog(%v) = %v, want %v", tt.current, tt.msgLevel, got, tt.expected)
			}
		})
	}
}
