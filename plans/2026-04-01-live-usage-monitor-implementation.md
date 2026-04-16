# Live Usage Monitor - Implementation Plan

**Date:** 2026-04-01
**Based on:** `2026-04-01-live-usage-monitor-design.md`
**Status:** Ready for Implementation

---

## Overview

This implementation plan details the steps to create `ccrouter monitor` - a live terminal UI dashboard for monitoring usage statistics with real-time updates.

---

## Phase 1: Core Infrastructure

### Task 1.1: Add Dependencies
**File:** `go.mod`

Add required Bubble Tea and Lipgloss packages:
```
github.com/charmbracelet/bubbletea v1.0.0
github.com/charmbracelet/lipgloss v0.11.0
github.com/charmbracelet/bubbles v0.20.0
```

### Task 1.2: Create Monitor Command
**File:** `internal/cli/monitor.go` (NEW)

Create Cobra subcommand with flags:
- `--refresh duration` - Stats refresh interval (default: 1s)
- `--db-path string` - Custom database path

```go
func NewMonitorCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "monitor",
        Short: "Live usage monitor with terminal UI",
        RunE:  runMonitor,
    }
    cmd.Flags().Duration("refresh", 1*time.Second, "Stats refresh interval")
    return cmd
}
```

### Task 1.3: Create Monitor Package Structure
**Directory:** `internal/monitor/`

Create:
- `internal/monitor/model.go` - MonitorModel
- `internal/monitor/poller.go` - StatsPoller
- `internal/monitor/tailer.go` - LogTailer
- `internal/monitor/buffer.go` - LogBuffer

### Task 1.4: Implement StatsPoller
**File:** `internal/monitor/poller.go`

- Thread-safe with mutex
- Exponential backoff (1s→2s→4s→max 30s)
- Date range calculation (TODAY/WEEK/MONTH/YTD/TTM)
- Graceful error handling

### Task 1.5: Implement LogTailer
**File:** `internal/monitor/tailer.go`

- File rotation detection (os.SameFile)
- Log level parsing
- Ring buffer integration
- Handle file deletion

### Task 1.6: Implement LogBuffer
**File:** `internal/monitor/buffer.go`

- Thread-safe ring buffer
- 1000 line capacity
- GetLines(), Append(), Clear()

---

## Phase 2: UI Components

### Task 2.1: Define Styles and Colors
**File:** `internal/monitor/styles.go` (NEW)

Define Lipgloss styles:
- Color palette
- Table styles
- Instance list styles
- Log level styles

### Task 2.2: Implement MonitorModel
**File:** `internal/monitor/model.go`

- Initialize Bubble Tea model
- Setup channels (buffered)
- Setup context and WaitGroup

### Task 2.3: Implement Header Rendering
**File:** `internal/monitor/view.go`

Render:
- Date range tabs (TODAY/WEEK/MONTH/YTD/TTM)
- Summary stats (Requests, Tokens, Fallbacks)

### Task 2.4: Implement Route Table
**File:** `internal/monitor/view.go`

Render BY ROUTE section:
- Columns: Route, Requests, Tokens, Fallbacks
- Sorted alphabetically
- Alternating row colors

### Task 2.5: Implement Model Table
**File:** `internal/monitor/view.go`

Render BY MODEL section:
- Columns: Model, Requests, Tokens
- Sorted by tokens descending

### Task 2.6: Implement Instance List
**File:** `internal/monitor/view.go`

Render INSTANCES panel:
- ** ALL ** option at top
- Running indicator (● green)
- Stopped indicator (○ gray)
- Highlight selected

### Task 2.7: Implement Console Log
**File:** `internal/monitor/view.go`

Render when enabled:
- Title bar with status
- Scrollable log content
- Level filter checkboxes

### Task 2.8: Implement Status Bar
**File:** `internal/monitor/view.go`

Render:
- Keyboard shortcuts
- Error messages
- Last update timestamp

---

## Phase 3: State & Interaction

### Task 3.1: Implement Update Method
**File:** `internal/monitor/model.go`

Handle messages:
- `tea.KeyMsg` - All keyboard input
- `tea.WindowSizeMsg` - Terminal resize
- `StatsUpdateMsg` - Stats from poller
- `LogMsg` - Log line from tailer
- `ErrorMsg` - Error from workers

### Task 3.2: Implement Date Range Navigation
**File:** `internal/monitor/model.go`

Handle:
- Left/Right arrows to change tabs
- Tab key to cycle through
- Update stats poller immediately
- Refresh instance list

### Task 3.3: Implement Instance Selection
**File:** `internal/monitor/model.go`

Handle:
- Up/Down arrows or j/k
- Update selected instance
- Handle console log state changes
- Trigger stats refresh

### Task 3.4: Implement Console Log Toggle
**File:** `internal/monitor/model.go`

Handle 'd' key:
- Only when single instance selected
- Start/stop log tailer
- Clear buffer on enable
- Disable on instance change

### Task 3.5: Implement Log Level Filters
**File:** `internal/monitor/model.go`

Handle keys 1-7:
- Toggle bit in LogLevelSet
- Update log tailer filters
- Re-render console log

### Task 3.6: Implement Pause/Resume
**File:** `internal/monitor/model.go`

Handle Space:
- Toggle consoleLogPaused
- Auto-scroll vs manual scroll

---

## Phase 4: Testing & Integration

### Task 4.1: Unit Tests - Date Range
**File:** `internal/monitor/poller_test.go`

Test:
- TODAY calculation
- WEEK calculation (Monday start)
- MONTH calculation
- YTD calculation
- TTM calculation

### Task 4.2: Unit Tests - Log Buffer
**File:** `internal/monitor/buffer_test.go`

Test:
- Append within capacity
- Ring buffer wraparound
- Clear behavior
- Thread safety

### Task 4.3: Unit Tests - Log Parsing
**File:** `internal/monitor/tailer_test.go`

Test:
- Valid log line parsing
- Malformed line handling
- Level detection

### Task 4.4: Integration Test - Full Flow
**File:** `internal/monitor/integration_test.go`

Test:
- Start monitor with mock DB
- Verify stats display
- Test keyboard navigation

### Task 4.5: Error Scenario Tests
**File:** `internal/monitor/error_test.go`

Test:
- Database unavailable
- Log file rotation
- Rapid instance switching
- Terminal resize

### Task 4.6: Manual Testing
- Test on minimum terminal (80x24)
- Test on large terminal (200x60)
- Test all keyboard shortcuts
- Test error messages display

---

## Phase 5: Final Integration

### Task 5.1: Register Command
**File:** `internal/cli/root.go` or `start.go`

Add monitor command to CLI:
```go
rootCmd.AddCommand(NewMonitorCommand())
```

### Task 5.2: Verify Build
Run:
```bash
go build -o bin/debug/ccrouter ./cmd/ccrouter
```

### Task 5.3: Test Run
Run monitor:
```bash
./bin/ccrouter monitor
```

Verify:
- UI renders correctly
- Stats load from database
- Keyboard navigation works

---

## File Changes Summary

### New Files
| File | Description |
|------|-------------|
| `internal/cli/monitor.go` | Cobra command |
| `internal/monitor/model.go` | Bubble Tea model |
| `internal/monitor/poller.go` | Stats poller |
| `internal/monitor/tailer.go` | Log tailer |
| `internal/monitor/buffer.go` | Ring buffer |
| `internal/monitor/styles.go` | Lipgloss styles |
| `internal/monitor/view.go` | Rendering functions |
| `internal/monitor/poller_test.go` | Tests |
| `internal/monitor/buffer_test.go` | Tests |
| `internal/monitor/tailer_test.go` | Tests |
| `internal/monitor/integration_test.go` | Tests |

### Modified Files
| File | Changes |
|------|---------|
| `go.mod` | Add Bubble Tea dependencies |

---

## Implementation Order

1. **Day 1 Morning:**
   - Task 1.1: Add dependencies
   - Task 1.2: Create monitor command
   - Task 1.3: Create package structure

2. **Day 1 Afternoon:**
   - Task 1.4: StatsPoller
   - Task 1.5: LogTailer
   - Task 1.6: LogBuffer

3. **Day 2 Morning:**
   - Task 2.1: Styles
   - Task 2.2: MonitorModel
   - Task 2.3-2.8: All rendering functions

4. **Day 2 Afternoon:**
   - Task 3.1-3.6: All interaction handlers

5. **Day 3:**
   - Task 4.1-4.6: Testing
   - Task 5.1-5.3: Integration

---

## Notes

- Follow existing code patterns in project
- Use existing `usage.*` package for DB access
- Maintain error handling standards from CLAUDE.md
- Test on macOS (primary platform)