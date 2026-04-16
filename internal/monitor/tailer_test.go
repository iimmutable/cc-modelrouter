package monitor

import (
	"fmt"
	"testing"
)

func TestParseLogLineRaw(t *testing.T) {
	t.Run("valid RFC3339 log line", func(t *testing.T) {
		raw := "[2026-04-01T16:32:22.519636+08:00] [ERROR] Router started"
		line := parseLogLineRaw(raw)

		if line.Level != LogLevelError {
			t.Errorf("expected LogLevelError, got %d", line.Level)
		}
		if line.Message != "Router started" {
			t.Errorf("expected 'Router started', got %q", line.Message)
		}
		if line.Raw != raw {
			t.Error("expected raw to be preserved")
		}
	})

	t.Run("WARN level", func(t *testing.T) {
		raw := "[2026-04-01T16:32:22.519636+08:00] [WARN] Something happened"
		line := parseLogLineRaw(raw)

		if line.Level != LogLevelWarn {
			t.Errorf("expected LogLevelWarn, got %d", line.Level)
		}
	})

	t.Run("WARNING level", func(t *testing.T) {
		raw := "[2026-04-01T16:32:22.519636+08:00] [WARNING] Something happened"
		line := parseLogLineRaw(raw)

		if line.Level != LogLevelWarn {
			t.Errorf("expected LogLevelWarn, got %d", line.Level)
		}
	})

	t.Run("malformed line", func(t *testing.T) {
		raw := "just some text without brackets"
		line := parseLogLineRaw(raw)

		if line.Level != LogLevelInfo {
			t.Errorf("expected default LogLevelInfo, got %d", line.Level)
		}
		if line.Message != "just some text without brackets" {
			t.Errorf("expected message to be full raw line")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		line := parseLogLineRaw("")
		if line.Message != "" {
			t.Errorf("expected empty message for empty input, got %q", line.Message)
		}
	})

	t.Run("two bracket groups only", func(t *testing.T) {
		raw := "[ts] [LEVEL]"
		line := parseLogLineRaw(raw)
		// SplitN(raw, "]", 3) returns ["[ts", " [LEVEL", ""] → message is ""
		if line.Message != "" {
			t.Errorf("expected empty message for two bracket groups only, got %q", line.Message)
		}
	})
}

func TestLogTailer_ShouldInclude(t *testing.T) {
	lt := &LogTailer{
		Filters: LogLevelSet(LogLevelDebug | LogLevelInfo | LogLevelError),
	}

	tests := []struct {
		level LogLevel
		want  bool
	}{
		{LogLevelVerbs, false},
		{LogLevelTrace, false},
		{LogLevelDebug, true},
		{LogLevelInfo, true},
		{LogLevelWarn, false},
		{LogLevelError, true},
		{LogLevelFatal, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("level_%d", tt.level), func(t *testing.T) {
			got := lt.shouldInclude(tt.level)
			if got != tt.want {
				t.Errorf("shouldInclude(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestLogTailer_UpdateFilters(t *testing.T) {
	lt := &LogTailer{
		Filters: LogLevelSet(LogLevelInfo),
	}

	lt.UpdateFilters(LogLevelSet(LogLevelError))
	if lt.Filters != LogLevelSet(LogLevelError) {
		t.Errorf("expected LogLevelError, got %d", lt.Filters)
	}
}
