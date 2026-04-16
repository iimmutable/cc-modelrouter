# Log Level System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a configurable log level system to reduce terminal flooding while maintaining debugging capability.

**Architecture:**
- Add log level enum (DEBUG, INFO, WARN, ERROR) to LoggingConfig
- Implement a leveled logger that filters messages based on configured level
- Replace direct `log.Printf` calls throughout the codebase with leveled logging
- Keep existing helper functions (Infof, Warnf, Errorf, Debugf) as the API

**Tech Stack:**
- Go standard library `log` package
- Thread-safe level filtering using atomic operations
- No external dependencies

---

## Task 1: Add LogLevel Type and Level Filtering

**Files:**
- Modify: `internal/config/types.go`
- Create: `internal/config/types_test.go` (if not exists, add tests)

**Step 1: Write the failing test**

Create `internal/config/loglevel_test.go`:

```go
package config

import (
	"testing"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARN", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"", LevelInfo}, // default
		{"invalid", LevelInfo}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLogLevel(tt.input); got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLogLevelShouldLog(t *testing.T) {
	tests := []struct {
		name      string
		setLevel  LogLevel
		msgLevel  LogLevel
		expected  bool
	}{
		{"debug logs debug at info level", LevelInfo, LevelDebug, false},
		{"debug logs debug at debug level", LevelDebug, LevelDebug, true},
		{"info logs info at info level", LevelInfo, LevelInfo, true},
		{"info logs info at error level", LevelError, LevelInfo, false},
		{"warn logs warn at info level", LevelInfo, LevelWarn, true},
		{"error logs error at info level", LevelInfo, LevelError, true},
		{"error logs error at error level", LevelError, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.setLevel.ShouldLog(tt.msgLevel); got != tt.expected {
				t.Errorf("%v.ShouldLog(%v) = %v, want %v", tt.setLevel, tt.msgLevel, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -v -run TestLogLevel
```

Expected: FAIL with "undefined: LogLevel"

**Step 3: Write minimal implementation**

Add to `internal/config/types.go` (after LoggingConfig struct, around line 78):

```go
// LogLevel represents the verbosity level for logging.
type LogLevel int

const (
	// LevelDebug enables all log messages including detailed debugging info.
	LevelDebug LogLevel = iota
	// LevelInfo enables informational messages and above (default).
	LevelInfo
	// LevelWarn enables warnings and errors only.
	LevelWarn
	// LevelError enables errors only.
	LevelError
)

// String returns the string representation of the log level.
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

// ShouldLog returns true if a message at the given level should be logged.
func (l LogLevel) ShouldLog(msgLevel LogLevel) bool {
	return msgLevel >= l
}

// ParseLogLevel parses a string to a LogLevel.
// Defaults to LevelInfo if the string is empty or invalid.
func ParseLogLevel(s string) LogLevel {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "WARN":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo // default
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -v -run TestLogLevel
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/types.go internal/config/loglevel_test.go
git commit -m "feat(config): add LogLevel type with parsing and filtering"
```

---

## Task 2: Add Level Field to LoggingConfig

**Files:**
- Modify: `internal/config/types.go`
- Modify: `internal/config/types_test.go` (add tests for new field)

**Step 1: Write the failing test**

Add to `internal/config/types_test.go` (in TestLoggingConfigMethods):

```go
{
	name: "with debug level",
	cfg: config.LoggingConfig{
		Enabled: true,
		Level:   "debug",
	},
	wantEnabled:   true,
	wantToFile:    true,
	wantToConsole: false,
},
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -v -run TestLoggingConfigMethods
```

Expected: Test should pass (Level field already exists), but we need to add parsing logic

**Step 3: Add GetLevel helper method**

Add to `internal/config/types.go` (after IsEnabled method, around line 107):

```go
// GetLevel returns the parsed log level, defaulting to LevelInfo.
func (lc *LoggingConfig) GetLevel() LogLevel {
	if lc.Level == "" {
		return LevelInfo // default
	}
	return ParseLogLevel(lc.Level)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/types.go internal/config/types_test.go
git commit -m "feat(config): add GetLevel helper to LoggingConfig"
```

---

## Task 3: Implement Leveled Logger

**Files:**
- Modify: `internal/logging/logging.go`
- Modify: `internal/logging/logging_test.go`

**Step 1: Write the failing test**

Add to `internal/logging/logging_test.go`:

```go
func TestInitWithLevel(t *testing.T) {
	tests := []struct {
		name           string
		cfg            config.LoggingConfig
		shouldLogDebug bool
		shouldLogInfo  bool
		shouldLogWarn  bool
		shouldLogError bool
	}{
		{
			name: "debug level logs everything",
			cfg: config.LoggingConfig{
				Enabled:     true,
				Destination: "stdout",
				Level:       "debug",
			},
			shouldLogDebug: true,
			shouldLogInfo:  true,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name: "info level excludes debug",
			cfg: config.LoggingConfig{
				Enabled:     true,
				Destination: "stdout",
				Level:       "info",
			},
			shouldLogDebug: false,
			shouldLogInfo:  true,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name: "warn level only logs warn and error",
			cfg: config.LoggingConfig{
				Enabled:     true,
				Destination: "stdout",
				Level:       "warn",
			},
			shouldLogDebug: false,
			shouldLogInfo:  false,
			shouldLogWarn:  true,
			shouldLogError: true,
		},
		{
			name: "error level only logs errors",
			cfg: config.LoggingConfig{
				Enabled:     true,
				Destination: "stdout",
				Level:       "error",
			},
			shouldLogDebug: false,
			shouldLogInfo:  false,
			shouldLogWarn:  false,
			shouldLogError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Redirect log output
			originalStderr := originalStderr
			originalStderr = &buf
			defer func() { originalStderr = originalStderr }()

			cleanup, err := Init(&tt.cfg)
			if err != nil {
				t.Fatalf("Init failed: %v", err)
			}
			defer cleanup()

			// Log at all levels
			Debugf("debug message")
			Infof("info message")
			Warnf("warn message")
			Errorf("error message")

			output := buf.String()

			// Check each level
			if tt.shouldLogDebug && !strings.Contains(output, "[DEBUG]") {
				t.Error("Expected debug message in output")
			}
			if !tt.shouldLogDebug && strings.Contains(output, "[DEBUG]") {
				t.Error("Did not expect debug message in output")
			}

			if tt.shouldLogInfo && !strings.Contains(output, "[INFO]") {
				t.Error("Expected info message in output")
			}
			if !tt.shouldLogInfo && strings.Contains(output, "[INFO]") {
				t.Error("Did not expect info message in output")
			}

			if tt.shouldLogWarn && !strings.Contains(output, "[WARN]") {
				t.Error("Expected warn message in output")
			}
			if !tt.shouldLogWarn && strings.Contains(output, "[WARN]") {
				t.Error("Did not expect warn message in output")
			}

			if tt.shouldLogError && !strings.Contains(output, "[ERROR]") {
				t.Error("Expected error message in output")
			}
			if !tt.shouldLogError && strings.Contains(output, "[ERROR]") {
				t.Error("Did not expect error message in output")
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/logging/... -v -run TestInitWithLevel
```

Expected: FAIL - current implementation doesn't filter by level

**Step 3: Implement leveled logger**

Replace the entire `internal/logging/logging.go` with:

```go
// Package logging handles setup and configuration of application logging.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/iimmutable/cc-modelrouter/internal/config"
)

var (
	// originalStderr is the original stderr before any redirection
	originalStderr io.Writer = os.Stderr
	// originalStdout is the original stdout before any redirection
	originalStdout io.Writer = os.Stdout

	// currentLevel holds the current log level (atomic for thread safety)
	currentLevel atomic.Int64
)

// Init initializes logging based on the provided configuration.
// It redirects the standard log package output to the configured destination.
// If AlsoConsole is true, logs are written to both the file and console.
//
// The function returns an error if the log destination cannot be initialized,
// or a cleanup function that should be called on shutdown to close any open files.
func Init(cfg *config.LoggingConfig) (func(), error) {
	// Set the log level atomically
	level := int64(config.LevelInfo) // default
	if cfg != nil && cfg.IsEnabled() {
		level = int64(cfg.GetLevel())
	}
	currentLevel.Store(level)

	// If config is nil or logging is not explicitly enabled, disable logging
	if cfg == nil || !cfg.IsEnabled() {
		log.SetOutput(io.Discard)
		return func() {}, nil
	}

	// Get the primary log writer
	writer, err := cfg.GetLogWriter()
	if err != nil {
		return nil, fmt.Errorf("failed to create log writer: %w", err)
	}

	// If also logging to console, use a multi-writer
	var logWriter io.Writer
	if cfg.ShouldLogToConsole() && cfg.ShouldLogToFile() {
		// Log to both file and stderr
		logWriter = io.MultiWriter(writer, originalStderr)
	} else {
		logWriter = writer
	}

	// Set the log output with the standard format
	log.SetOutput(logWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	// Return a cleanup function
	cleanup := func() {
		if closer, ok := writer.(io.Closer); ok && cfg.ShouldLogToFile() {
			closer.Close()
		}
		// Reset to stderr on cleanup
		log.SetOutput(originalStderr)
	}

	return cleanup, nil
}

// InitWithPath initializes logging with a specific file path.
// This is a convenience function for programmatic setup.
func InitWithPath(logPath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime)

	return func() {
		f.Close()
		log.SetOutput(originalStderr)
	}, nil
}

// InitToStdout redirects logs to stdout instead of stderr.
func InitToStdout() {
	log.SetOutput(originalStdout)
	log.SetFlags(log.Ldate | log.Ltime)
}

// InitToStderr redirects logs to stderr (default behavior).
func InitToStderr() {
	log.SetOutput(originalStderr)
	log.SetFlags(log.Ldate | log.Ltime)
}

// InitToSilent discards all log output.
func InitToSilent() {
	log.SetOutput(io.Discard)
}

// DefaultLogPath returns the default log file path.
func DefaultLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-modelrouter", "router.log"), nil
}

// shouldLog returns true if a message at the given level should be logged.
func shouldLog(level config.LogLevel) bool {
	current := config.LogLevel(currentLevel.Load())
	return current.ShouldLog(level)
}

// Infof logs an info message.
func Infof(format string, v ...interface{}) {
	if !shouldLog(config.LevelInfo) {
		return
	}
	log.Printf("[INFO] "+format, v...)
}

// Warnf logs a warning message.
func Warnf(format string, v ...interface{}) {
	if !shouldLog(config.LevelWarn) {
		return
	}
	log.Printf("[WARN] "+format, v...)
}

// Errorf logs an error message.
func Errorf(format string, v ...interface{}) {
	if !shouldLog(config.LevelError) {
		return
	}
	log.Printf("[ERROR] "+format, v...)
}

// Debugf logs a debug message.
func Debugf(format string, v ...interface{}) {
	if !shouldLog(config.LevelDebug) {
		return
	}
	log.Printf("[DEBUG] "+format, v...)
}

// Streamf logs a streaming-related message (INFO level by default).
// These are verbose per-event logs that should be filtered at DEBUG level.
func Streamf(format string, v ...interface{}) {
	if !shouldLog(config.LevelInfo) {
		return
	}
	log.Printf("[STREAM] "+format, v...)
}

// StreamDebugf logs detailed streaming debug information (DEBUG level).
func StreamDebugf(format string, v ...interface{}) {
	if !shouldLog(config.LevelDebug) {
		return
	}
	log.Printf("[STREAM-DEBUG] "+format, v...)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/logging/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/logging/logging.go internal/logging/logging_test.go
git commit -m "feat(logging): implement leveled logger with atomic level filtering"
```

---

## Task 4: Replace Direct log.Printf Calls with Leveled Logging

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/interceptor.go`

**Step 1: Update handler.go streaming logs**

Replace streaming-related `log.Printf` calls in `internal/proxy/handler.go` with `logging.Streamf` and `logging.StreamDebugf`.

For each replacement:
- Lines 261, 279, 287, 297, 353, 418, 428, 440, 447, 451, 461, 469, 485, 493, 504

Change:
```go
log.Printf("[STREAM] Starting streaming request, route: %s, targets: %d", routeName, len(targets))
```

To:
```go
logging.Streamf("Starting streaming request, route: %s, targets: %d", routeName, len(targets))
```

For detailed per-event logs (lines 418, 428, 440, 447, 451, 461, 469), use `StreamDebugf`:
```go
logging.StreamDebugf("Filtering out non-Anthropic event: %s", eventType)
```

**Step 2: Update interceptor.go logs**

Replace `log.Printf` calls with appropriate leveled logging functions.

**Step 3: Run tests**

```bash
go test ./internal/proxy/... -v
go test ./internal/logging/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/interceptor.go
git commit -m "refactor(logging): replace direct log calls with leveled logging"
```

---

## Task 5: Update CLI to Set Default Log Level

**Files:**
- Modify: `internal/cli/start.go`
- Modify: `internal/cli/code.go`

**Step 1: Add log level flag to start command**

Add flag definition in `NewStartCommand()`:
```go
cmd.Flags().String("log-level", "info", "Log level: debug, info, warn, error")
```

**Step 2: Apply log level from flag**

In `runStart()`, add:
```go
logLevel, _ := cmd.Flags().GetString("log-level")
cfg.Logging.Level = logLevel
```

**Step 3: Set default level to info for code command**

In `runCode()`, add:
```go
// Default to info level for code command to reduce verbosity
if cfg.Logging.Level == "" {
	cfg.Logging.Level = "info"
}
```

**Step 4: Run tests**

```bash
go test ./internal/cli/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/start.go internal/cli/code.go
git commit -m "feat(cli): add log-level flag and default to info level"
```

---

## Task 6: Clean Up Stale Log Files

**Files:**
- Modify: `.gitignore`

**Step 1: Update .gitignore**

Add to `.gitignore`:
```
# Debug log files
debug.log
*.log
```

**Step 2: Remove stale debug.log**

```bash
rm -f debug.log
```

**Step 3: Commit**

```bash
git add .gitignore
git commit -m "chore: add log files to gitignore and remove stale debug.log"
```

---

## Task 7: Update Configuration Documentation

**Files:**
- Modify: `internal/config/types.go` (documentation)

**Step 1: Update LoggingConfig documentation**

Update the comment for `Level` field to be more descriptive:

```go
// Level controls log verbosity.
// Valid values: "debug", "info" (default), "warn", "error".
// - debug: Shows all messages including detailed streaming events
// - info: Shows request/response summaries and warnings
// - warn: Shows only warnings and errors
// - error: Shows only errors
Level string `json:"level,omitempty"`
```

**Step 2: Commit**

```bash
git add internal/config/types.go
git commit -m "docs(config): improve log level documentation"
```

---

## Task 8: Integration Test

**Step 1: Build and test**

```bash
go build -o ccrouter ./cmd/ccrouter

# Test with different log levels
./ccrouter start --log-level=debug &
./ccrouter start --log-level=info &
./ccrouter start --log-level=error &
```

**Step 2: Verify log output**

Check that:
- Debug level shows all streaming events
- Info level shows request/response summaries but not per-event details
- Error level shows only errors

**Step 3: Final commit if adjustments needed**

---

## Verification Checklist

After implementation, verify:

1. [ ] No logging when `logging.enabled: false`
2. [ ] No debug logs when `logging.level: info`
3. [ ] All logs shown when `logging.level: debug`
4. [ ] `ccrouter code` command doesn't show verbose streaming logs
5. [ ] `ccrouter start` respects the logging level config
6. [ ] All tests pass: `go test ./...`
7. [ ] `debug.log` is in `.gitignore`
8. [ ] Stale `debug.log` file is removed

---

## Configuration Examples

### Default (No logging)
```yaml
# No logging section - logging disabled
server:
  port: 8081
providers:
  bigmodel: ...
```

### Enable logging with INFO level (recommended for most use)
```yaml
logging:
  enabled: true
  level: info
  destination: file
```

### Enable logging with DEBUG level (for troubleshooting)
```yaml
logging:
  enabled: true
  level: debug
  destination: file
```

### Enable ERROR only (minimal output)
```yaml
logging:
  enabled: true
  level: error
  destination: file
```

### Override via command line
```bash
ccrouter start --log-level=debug
ccrouter start --log-level=error
```
