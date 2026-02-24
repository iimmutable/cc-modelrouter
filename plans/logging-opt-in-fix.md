# Fix Plan: Logging Opt-In by Default

## Problem Statement

Currently, logging is enabled by default in cc-router. This causes issues:
1. Log messages crash Claude Code's terminal when using `ccrouter code` command
2. Logs are written even when users don't explicitly want them
3. Cannot easily disable logging without explicitly configuring it

## Solution

Make logging **opt-in** rather than opt-out:
- If `logging` section is missing in config → logging disabled
- If `logging.enabled` is not `true` → logging disabled
- Only when `logging.enabled: true` → logging enabled

## Changes Required

### 1. Add `Enabled` Property to LoggingConfig

**File**: `internal/config/types.go`

Add a new `Enabled` field to control whether logging is active:

```go
// LoggingConfig represents logging configuration.
type LoggingConfig struct {
    // Enabled controls whether logging is active.
    // If false or not specified, logging is disabled.
    // Default: false
    Enabled bool `json:"enabled,omitempty"`

    // Destination is where logs should be written.
    // Valid values: "stdout", "stderr", "file", or a file path.
    // If "file", uses the default log file path.
    Destination string `json:"destination,omitempty"`

    // FilePath is the specific file path when Destination is "file" or a custom path.
    // If empty, uses the default: ~/.cc-modelrouter/router.log
    FilePath string `json:"filePath,omitempty"`

    // AlsoConsole controls whether to also log to console when logging to a file.
    AlsoConsole bool `json:"alsoConsole,omitempty"`

    // Level controls log verbosity.
    // Valid values: "debug", "info", "warn", "error". Currently for future use.
    Level string `json:"level,omitempty"`
}
```

Add helper method:

```go
// IsEnabled returns true if logging is explicitly enabled.
func (lc *LoggingConfig) IsEnabled() bool {
    return lc.Enabled
}
```

### 2. Remove Logging from Default Configuration

**File**: `internal/config/types.go`

Remove the `Logging` field from the `Defaults()` function so that omitting the `logging` section in config results in disabled logging:

```go
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
        // Logging removed from defaults - opt-in only
    }
}
```

### 3. Update Logging Init to Check Enabled Flag

**File**: `internal/logging/logging.go`

Modify `Init()` to disable logging when config is nil or not explicitly enabled:

```go
// Init initializes logging based on the provided configuration.
func Init(cfg *config.LoggingConfig) (func(), error) {
    // If config is nil or logging is not enabled, disable logging
    if cfg == nil || !cfg.IsEnabled() {
        // Disable all logging output
        log.SetOutput(io.Discard)
        return func() {}, nil
    }

    // Get the primary log writer
    writer, err := cfg.GetLogWriter()
    if err != nil {
        return nil, fmt.Errorf("failed to create log writer: %w", err)
    }

    // ... rest of existing logic
}
```

### 4. Force Disable AlsoConsole in Code Command

**File**: `internal/cli/code.go`

Ensure console logging is always disabled for `code` command to prevent UI corruption:

```go
// Apply log destination overrides from flags
if logDestination != "" {
    cfg.Logging.Destination = logDestination
}
// Force disable console logging for code command
cfg.Logging.AlsoConsole = false
```

### 5. Update Tests

Update any tests that depend on default logging behavior to explicitly enable logging.

## Configuration Examples

### Before (Current Behavior)
```yaml
# No logging section - still logs to file by default
server:
  port: 8081
providers:
  bigmodel: ...
```
**Result**: Logs to `~/.cc-modelrouter/router.log`

### After (New Behavior)
```yaml
# No logging section - logging disabled
server:
  port: 8081
providers:
  bigmodel: ...
```
**Result**: No logging

### To Enable Logging (New)
```yaml
logging:
  enabled: true
  destination: file

server:
  port: 8081
providers:
  bigmodel: ...
```
**Result**: Logs to `~/.cc-modelrouter/router.log`

### To Enable Logging with Console Output
```yaml
logging:
  enabled: true
  destination: file
  alsoConsole: true
```
**Result**: Logs to both file and console

### Custom Log File
```yaml
logging:
  enabled: true
  destination: /var/log/cc-router.log
```
**Result**: Logs to custom path

## Migration Impact

### Breaking Change
- Existing users without `logging.enabled: true` will lose logging after upgrade
- This is intentional - logging should be opt-in

### Recommendations for Users
1. Add `logging: { enabled: true }` to config file to re-enable logging
2. Use `ccrouter logs` command to view logs (when logging is enabled)
3. Use `ccrouter start --log-also-console` for temporary console logging

## Implementation Order

1. [ ] Add `Enabled` field and `IsEnabled()` method to `LoggingConfig`
2. [ ] Remove `Logging` from `Defaults()` function
3. [ ] Update `Init()` in `logging/logging.go` to check `IsEnabled()`
4. [ ] Update `code.go` to force disable `AlsoConsole`
5. [ ] Update affected tests
6. [ ] Update documentation
7. [ ] Test all scenarios

## Verification

After implementation, verify:
1. [ ] No logging when `logging` section is missing
2. [ ] No logging when `logging.enabled: false`
3. [ ] Logging works when `logging.enabled: true`
4. [ ] `ccrouter code` command doesn't show logs in terminal
5. [ ] `ccrouter start` respects the logging config
6. [ ] All tests pass
