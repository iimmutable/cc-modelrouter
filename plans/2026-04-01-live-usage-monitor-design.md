# Live Usage Monitor Design

**Date:** 2026-04-01
**Author:** Claude Code (Brainstorming Skill)
**Status:** Approved

---

## Executive Summary

This document specifies the design for `ccrouter monitor` - a live terminal UI for monitoring usage statistics with real-time updates. The command provides an interactive dashboard displaying requests, tokens, fallbacks by route and model, with optional per-instance console log viewing.

**Key Features:**
- Real-time stats updates (1s default, configurable)
- Date range selection (TODAY/WEEK/MONTH/YTD/TTM)
- Instance filtering with live/stopped indicators
- Console log viewer with level filtering and tail/pause modes
- Full keyboard navigation
- Built with Bubble Tea + Lipgloss for rich terminal UI

---

## 1. High-Level Architecture

### Component Overview

The monitor is implemented as a new Cobra subcommand (`ccrouter monitor`) using the Bubble Tea framework for terminal UI rendering and event handling.

**Architecture Pattern:** Concurrent producer-consumer with channels for inter-goroutine communication.

```
┌─────────────────────────────────────────────────────────┐
│  Bubble Tea Application (main goroutine)                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Keyboard     │  │ State        │  │ Renderer     │  │
│  │ Handler      │→│ Manager      │→│ (Lipgloss)   │  │
│  └──────────────┘  └──────┬───────┘  └──────────────┘  │
└────────────────────────────┼────────────────────────────┘
                             │ ▲
                             │ │ channels (buffered)
                  ┌──────────┴─┴─────────┐
                  │                      │
         ┌────────▼─────────┐   ┌────────▼────────┐
         │ Stats Poller     │   │ Log Tailer      │
         │ (goroutine)      │   │ (goroutine)     │
         │                  │   │                  │
         │ - Ticker (1s)    │   │ - File watcher  │
         │ - Query SQLite   │   │ - Level filter  │
         │ - Aggregate      │   │ - Ring buffer   │
         │ - Backoff on err │   │ - Handle rotate │
         └──────────────────┘   └─────────────────┘
                │                       │
                ▼                       ▼
         ~/.cc-modelrouter/     ~/.cc-modelrouter/
              usage.db          logs/inst_*.log
```

### Core Components

1. **MonitorModel** - Root Bubble Tea model (state container)
2. **StatsPoller** - Background goroutine polling SQLite every N seconds
3. **LogTailer** - Background goroutine tailing instance log files
4. **UI Components** - Header, tables, instance list, console log, status bar

### Data Flow

1. User presses key → Bubble Tea `Update()` → State mutation → Re-render
2. Timer fires → StatsPoller queries DB → Send stats via channel → `Update()` receives → Re-render
3. Log line written → LogTailer reads → Parse & filter → Send via channel → Append to buffer → Re-render

---

## 2. UI Component Structure & Layout

### Component Hierarchy

```
MonitorModel (root Bubble Tea model)
├── HeaderBar (date range tabs + summary stats)
├── ContentArea (2-column layout)
│   ├── LeftPanel (~70% width)
│   │   ├── RouteTable (BY ROUTE section)
│   │   └── ModelTable (BY MODEL section)
│   └── RightPanel (~30% width)
│       └── InstanceList (scrollable instance selector)
├── ConsoleLog (conditional full-width section)
└── StatusBar (keyboard shortcuts, refresh indicator)
```

### Layout Behavior

**Console Log Disabled (default):**
```
┌─────────────────────────────────────────────────────┐
│ HeaderBar (full width)                              │
├─────────────────────────────────┬───────────────────┤
│ LeftPanel (70%)                 │ RightPanel (30%)  │
│ ┌─────────────────────────────┐ │ ┌───────────────┐ │
│ │ RouteTable                  │ │ │ InstanceList  │ │
│ └─────────────────────────────┘ │ │               │ │
│ ┌─────────────────────────────┐ │ │               │ │
│ │ ModelTable                  │ │ │               │ │
│ └─────────────────────────────┘ │ └───────────────┘ │
├─────────────────────────────────┴───────────────────┤
│ StatusBar                                           │
└─────────────────────────────────────────────────────┘
```

**Console Log Enabled:**
```
┌─────────────────────────────────────────────────────┐
│ HeaderBar                                           │
├─────────────────────────────────┬───────────────────┤
│ LeftPanel (compressed)          │ RightPanel        │
│ ┌─────────────────────────────┐ │ ┌───────────────┐ │
│ │ RouteTable                  │ │ │ InstanceList  │ │
│ └─────────────────────────────┘ │ └───────────────┘ │
│ ┌─────────────────────────────┐ │                   │
│ │ ModelTable                  │ │                   │
│ └─────────────────────────────┘ │                   │
├─────────────────────────────────┴───────────────────┤
│ ConsoleLog (full width, ~40% height)                │
│ ┌─CONSOLE LOG─[✔]enabled(d)───────────────────────┐ │
│ │ [scrollable log content]                        │ │
│ └─[ ]VERBS [✔]DEBUG [✔]INFO [✔]WARN [✔]ERROR────┘ │
├─────────────────────────────────────────────────────┤
│ StatusBar                                           │
└─────────────────────────────────────────────────────┘
```

**Height Distribution:**
- Console disabled: ContentArea ~90%, StatusBar ~4%, margins ~6%
- Console enabled: ContentArea ~50%, ConsoleLog ~40%, StatusBar ~4%, margins ~6%

### Component Responsibilities

**HeaderBar:**
- Renders date range tabs (TODAY/WEEK/MONTH/YTD/TTM)
- Highlights selected date range
- Displays summary metrics (total requests, tokens, fallbacks)
- Handles tab navigation (arrow keys)

**RouteTable:**
- Displays route statistics in tabular format
- Columns: Route name, Requests, Tokens, Fallbacks
- Sorted alphabetically by route name
- Auto-adjusts column widths based on content
- Alternating row colors for readability

**ModelTable:**
- Displays model statistics in tabular format
- Columns: Model name, Requests, Tokens
- Sorted by token count descending (highest usage first)
- Alternating row colors

**InstanceList:**
- Scrollable list with `** ALL **` at top
- Shows running instances (● green indicator) + stopped instances in date range (○ gray)
- Highlights selected instance
- Navigation: up/down arrows or vim-style j/k keys
- Sorted: running first, then by start time descending

**ConsoleLog (conditional):**
- Title bar: `─CONSOLE LOG─[✔]enabled(d)─` or `─CONSOLE LOG─[⏸]paused(space)─`
- Main area: Scrollable ring buffer (last 1000 lines)
- Footer: Level checkboxes with toggle via 1-7 keys
- Tail-follow mode (auto-scroll) vs paused mode (spacebar toggle)
- Real-time filtering by log level

**StatusBar:**
- Keyboard shortcuts (context-sensitive)
- Error display (if any, in red)
- Last update timestamp
- Refresh indicator

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` | Quit application |
| `d` | Toggle Console Log pane (only when single instance selected) |
| `←/→` or `Tab` | Navigate date range tabs |
| `↑/↓` or `j/k` | Navigate instance list |
| `Space` | Pause/resume log tail (when Console Log enabled) |
| `1-7` | Toggle log level filters (1=VERBS, 2=TRACE, 3=DEBUG, 4=INFO, 5=WARN, 6=ERROR, 7=FATAL) |
| `r` | Force refresh stats immediately |
| `?` | Show help overlay (future enhancement) |

---

## 3. State Management & Data Models

### Application State

```go
type MonitorModel struct {
    // Synchronization
    mu sync.RWMutex  // Protects all fields accessed from multiple goroutines

    // Configuration
    refreshInterval time.Duration  // Default 1s, configurable via --refresh flag
    dbPath          string

    // UI State
    selectedDateRange DateRange     // TODAY, WEEK, MONTH, YTD, TTM
    selectedInstance  string        // "" for ALL, or instance ID
    consoleLogEnabled bool          // Toggle with 'd' key
    consoleLogPaused  bool          // Toggle with spacebar
    logLevelFilters   LogLevelSet   // Bitmask: DEBUG|INFO|WARN|ERROR|FATAL
    lastError         error         // Display in status bar

    // Data State
    stats            *UsageStats    // Current aggregated stats
    instances        []InstanceInfo // Available instances
    logBuffer        *LogBuffer     // Ring buffer for console logs (last 1000 lines)

    // Channels for data updates (buffered)
    statsChan        chan *UsageStats  // Buffer: 10
    logChan          chan string       // Buffer: 1000
    errChan          chan error        // Buffer: 100

    // Component state
    viewport         viewport.Model    // Lipgloss viewport for scrolling
    instanceCursor   int              // Selected item in instance list
    windowSize       tea.WindowSizeMsg // Terminal dimensions

    // Lifecycle management
    ctx              context.Context
    cancel           context.CancelFunc
    wg               sync.WaitGroup

    // Log tailer separate context
    logCtx           context.Context
    logCancel        context.CancelFunc

    // Background workers
    statsPoller      *StatsPoller
    logTailer        *LogTailer
}
```

### Data Models

```go
// UsageStats represents aggregated usage data
type UsageStats struct {
    Summary      Summary                // Total requests, tokens, fallbacks
    ByRoute      map[string]*RouteStats // Route breakdown
    ByModel      map[string]*ModelStats // Model breakdown
    Timestamp    time.Time              // Snapshot timestamp
}

// InstanceInfo represents an instance
type InstanceInfo struct {
    ID          string
    IsRunning   bool      // PID check result
    StartTime   time.Time
    LogPath     string
}

// DateRange enum
type DateRange int
const (
    DateRangeToday DateRange = iota
    DateRangeWeek      // This calendar week (Monday-Sunday)
    DateRangeMonth     // This calendar month
    DateRangeYTD       // Year to date (Jan 1 - now)
    DateRangeTTM       // Trailing 12 months
)

// LogLine represents a parsed log line
type LogLine struct {
    Timestamp time.Time
    Level     LogLevel
    Message   string
    Raw       string  // Original line for display
}

// LogLevel enum with bitmask values
type LogLevel int
const (
    LogLevelVerbs  LogLevel = 1 << iota
    LogLevelTrace
    LogLevelDebug
    LogLevelInfo
    LogLevelWarn
    LogLevelError
    LogLevelFatal
)

type LogLevelSet int  // Bitmask of LogLevel values

// LogBuffer is a thread-safe ring buffer
type LogBuffer struct {
    lines    []LogLine
    capacity int        // 1000 lines
    head     int        // Write position
    size     int        // Current size
    mu       sync.RWMutex
}
```

### State Transitions

**1. Date Range Change (arrow keys in header):**
- Update `selectedDateRange`
- Call `statsPoller.UpdateDateRange()` with mutex protection
- Trigger immediate stats refresh
- Update instances list (filter by new date range)
- Preserve console log state if still valid

**2. Instance Selection Change (up/down in instance list):**
- Update `selectedInstance` and `instanceCursor`
- **If changing from single instance A to single instance B:**
  - Disable console log (user must explicitly re-enable)
  - Stop old logTailer via `logCancel()`
  - Clear `logBuffer`
- **If changing from single to ALL:**
  - Disable and hide console log
  - Stop logTailer
- **If changing from ALL to single:**
  - Make console log available (show 'd' hint in status bar)
  - Don't auto-enable
- Call `statsPoller.UpdateInstance()` with mutex protection
- Trigger immediate stats refresh

**3. Console Log Toggle ('d' key):**
- Only allowed when single instance selected
- Toggle `consoleLogEnabled`
- **If enabling:**
  - Create separate `logCtx` with cancel
  - Start new logTailer goroutine with `wg.Add(1)`
  - Initialize/clear `logBuffer`
- **If disabling:**
  - Call `logCancel()`
  - Clear `logBuffer`
  - Don't wait for goroutine (non-blocking)

**4. Log Level Filter Toggle (1-7 keys):**
- Toggle bit in `logLevelFilters` bitmask
- Call `logTailer.UpdateFilters()` with mutex protection
- Re-render console log (client-side filtering)

**5. Pause/Resume Log Tail (spacebar):**
- Toggle `consoleLogPaused`
- **When paused:**
  - LogTailer continues buffering to `logBuffer`
  - Viewport stops auto-scrolling
  - User can scroll through buffer
- **When resumed:**
  - Viewport jumps to latest line
  - Resume auto-scroll behavior

**6. Shutdown ('q' or Ctrl+C):**
- Call `cancel()` to signal all goroutines
- Wait for `wg.Wait()` with 2-second timeout
- Force quit if timeout exceeded (prevent hang)
- Return `tea.Quit`

---

## 4. Background Workers & Data Fetching

### StatsPoller Implementation

```go
type StatsPoller struct {
    mu              sync.RWMutex
    interval        time.Duration
    dbPath          string
    dateRange       DateRange
    instanceID      string
    consecutiveErrors int
}

// UpdateDateRange safely updates date range
func (sp *StatsPoller) UpdateDateRange(dr DateRange) {
    sp.mu.Lock()
    defer sp.mu.Unlock()
    sp.dateRange = dr
}

// UpdateInstance safely updates instance filter
func (sp *StatsPoller) UpdateInstance(instanceID string) {
    sp.mu.Lock()
    defer sp.mu.Unlock()
    sp.instanceID = instanceID
}

// getParams safely reads current parameters
func (sp *StatsPoller) getParams() (DateRange, string) {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    return sp.dateRange, sp.instanceID
}

// Run executes polling loop with exponential backoff
func (sp *StatsPoller) Run(ctx context.Context, statsChan chan<- *UsageStats, errChan chan<- error) {
    ticker := time.NewTicker(sp.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            dateRange, instanceID := sp.getParams()
            stats, err := sp.fetchUsageStats(dateRange, instanceID)

            if err != nil {
                sp.consecutiveErrors++

                select {
                case errChan <- fmt.Errorf("stats fetch failed (attempt %d): %w", sp.consecutiveErrors, err):
                case <-ctx.Done():
                    return
                }

                // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
                if sp.consecutiveErrors > 0 {
                    backoff := time.Duration(1<<uint(sp.consecutiveErrors-1)) * time.Second
                    if backoff > 30*time.Second {
                        backoff = 30 * time.Second
                    }
                    time.Sleep(backoff)
                }
                continue
            }

            // Success - reset error count
            if sp.consecutiveErrors > 0 {
                sp.consecutiveErrors = 0
                select {
                case errChan <- nil:  // nil signals recovery
                case <-ctx.Done():
                    return
                }
            }

            select {
            case statsChan <- stats:
            case <-ctx.Done():
                return
            }

        case <-ctx.Done():
            return
        }
    }
}

// fetchUsageStats queries database and aggregates
func (sp *StatsPoller) fetchUsageStats(dateRange DateRange, instanceID string) (*UsageStats, error) {
    db, err := usage.InitDB(sp.dbPath)
    if err != nil {
        // Return empty stats on error (graceful degradation)
        return &UsageStats{
            Summary:   usage.Summary{},
            ByRoute:   make(map[string]*usage.RouteStats),
            ByModel:   make(map[string]*usage.ModelStats),
            Timestamp: time.Now(),
        }, fmt.Errorf("database unavailable: %w", err)
    }
    defer db.Close()

    start, end := sp.calculateDateRange(dateRange)
    records, err := usage.GetRecordsByPeriod(db, instanceID, start, end)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }

    stats := &UsageStats{
        Summary:   usage.AggregateSummary(records),
        ByRoute:   usage.AggregateByRoute(records),
        ByModel:   usage.AggregateByModel(records),
        Timestamp: time.Now(),
    }

    return stats, nil
}

// calculateDateRange converts DateRange enum to start/end times
func (sp *StatsPoller) calculateDateRange(dateRange DateRange) (time.Time, time.Time) {
    now := time.Now()

    switch dateRange {
    case DateRangeToday:
        start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
        end := start.Add(24 * time.Hour)
        return start, end

    case DateRangeWeek:
        // This calendar week (Monday to Sunday)
        weekday := int(now.Weekday())
        if weekday == 0 {
            weekday = 7
        }
        daysFromMonday := weekday - 1
        start := time.Date(now.Year(), now.Month(), now.Day()-daysFromMonday, 0, 0, 0, 0, now.Location())
        end := start.Add(7 * 24 * time.Hour)
        return start, end

    case DateRangeMonth:
        start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
        end := start.AddDate(0, 1, 0)
        return start, end

    case DateRangeYTD:
        start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
        end := now
        return start, end

    case DateRangeTTM:
        start := now.AddDate(-1, 0, 0)
        end := now
        return start, end

    default:
        return now, now
    }
}
```

### LogTailer Implementation

```go
type LogTailer struct {
    mu         sync.RWMutex
    logPath    string
    filters    LogLevelSet
    bufferSize int
}

// UpdateFilters safely updates log level filters
func (lt *LogTailer) UpdateFilters(filters LogLevelSet) {
    lt.mu.Lock()
    defer lt.mu.Unlock()
    lt.filters = filters
}

// shouldInclude checks if level passes filter
func (lt *LogTailer) shouldInclude(level LogLevel) bool {
    lt.mu.RLock()
    defer lt.mu.RUnlock()
    return LogLevelSet(level)&lt.filters != 0
}

// Run tails log file with rotation detection
func (lt *LogTailer) Run(ctx context.Context, logChan chan<- string, errChan chan<- error) {
    file, err := os.Open(lt.logPath)
    if err != nil {
        select {
        case errChan <- fmt.Errorf("failed to open log: %w", err):
        case <-ctx.Done():
        }
        return
    }
    defer file.Close()

    // Track file info for rotation detection (cross-platform)
    initialStat, _ := file.Stat()

    // Start reading from near end (show context)
    if info, err := file.Stat(); err == nil {
        seekPos := info.Size() - 20*1024  // ~20KB back ≈ 100 lines
        if seekPos < 0 {
            seekPos = 0
        }
        file.Seek(seekPos, io.SeekStart)

        // Discard partial first line
        reader := bufio.NewReader(file)
        reader.ReadString('\n')
    }

    reader := bufio.NewReader(file)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Check for rotation (os.SameFile works on Unix and Windows)
            if currentStat, err := os.Stat(lt.logPath); err == nil {
                if initialStat != nil && !os.SameFile(initialStat, currentStat) {
                    // File rotated - reopen
                    file.Close()

                    newFile, err := os.Open(lt.logPath)
                    if err != nil {
                        select {
                        case errChan <- fmt.Errorf("log rotated, reopen failed: %w", err):
                        case <-ctx.Done():
                            return
                        }
                        return
                    }

                    file = newFile
                    reader = bufio.NewReader(file)
                    initialStat, _ = file.Stat()
                }
            } else if os.IsNotExist(err) {
                // Log file deleted - wait for recreation
                select {
                case errChan <- fmt.Errorf("log file deleted, waiting"):
                case <-ctx.Done():
                    return
                }
                time.Sleep(1 * time.Second)
                continue
            }

            // Read available lines
            for {
                line, err := reader.ReadString('\n')
                if err != nil {
                    if err == io.EOF {
                        break  // No more data
                    }
                    select {
                    case errChan <- fmt.Errorf("log read error: %w", err):
                    case <-ctx.Done():
                        return
                    }
                    break
                }

                // Parse and filter
                if parsedLine := lt.parseLine(line); parsedLine != nil {
                    if lt.shouldInclude(parsedLine.Level) {
                        select {
                        case logChan <- line:
                        case <-ctx.Done():
                            return
                        default:
                            // Backpressure: drop line
                        }
                    }
                }
            }

        case <-ctx.Done():
            return
        }
    }
}

// parseLine extracts timestamp, level, message from log line
func (lt *LogTailer) parseLine(raw string) *LogLine {
    // Parse format: [TIMESTAMP] [LEVEL] message
    // Example: [2026-04-01T16:32:22.519636+08:00] [INFO] Router started

    parts := strings.SplitN(raw, "]", 3)
    if len(parts) < 3 {
        // Malformed line - return minimal LogLine
        return &LogLine{
            Timestamp: time.Now(),
            Level:     LogLevelInfo,
            Message:   strings.TrimSpace(raw),
            Raw:       raw,
        }
    }

    // Parse timestamp
    timestampStr := strings.TrimPrefix(parts[0], "[")
    timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
    if err != nil {
        timestamp = time.Now()  // Fallback
    }

    // Parse level
    levelStr := strings.TrimSpace(strings.TrimPrefix(parts[1], "["))
    level := parseLogLevel(levelStr)

    // Message
    message := strings.TrimSpace(parts[2])

    return &LogLine{
        Timestamp: timestamp,
        Level:     level,
        Message:   message,
        Raw:       raw,
    }
}

func parseLogLevel(s string) LogLevel {
    switch strings.ToUpper(s) {
    case "VERBS":
        return LogLevelVerbs
    case "TRACE":
        return LogLevelTrace
    case "DEBUG":
        return LogLevelDebug
    case "INFO":
        return LogLevelInfo
    case "WARN", "WARNING":
        return LogLevelWarn
    case "ERROR":
        return LogLevelError
    case "FATAL":
        return LogLevelFatal
    default:
        return LogLevelInfo
    }
}
```

### LogBuffer Implementation (Thread-Safe Ring Buffer)

```go
type LogBuffer struct {
    lines    []LogLine
    capacity int
    head     int
    size     int
    mu       sync.RWMutex
}

func NewLogBuffer(capacity int) *LogBuffer {
    return &LogBuffer{
        lines:    make([]LogLine, 0, capacity),
        capacity: capacity,
    }
}

func (lb *LogBuffer) Append(line LogLine) {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    if lb.size < lb.capacity {
        lb.lines = append(lb.lines, line)
        lb.size++
    } else {
        // Ring buffer: overwrite oldest
        lb.lines[lb.head] = line
        lb.head = (lb.head + 1) % lb.capacity
    }
}

func (lb *LogBuffer) GetLines() []LogLine {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    if lb.size < lb.capacity {
        // Not full yet
        result := make([]LogLine, lb.size)
        copy(result, lb.lines)
        return result
    }

    // Ring buffer full: return in order
    result := make([]LogLine, lb.capacity)
    for i := 0; i < lb.capacity; i++ {
        result[i] = lb.lines[(lb.head+i)%lb.capacity]
    }
    return result
}

func (lb *LogBuffer) Clear() {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    lb.lines = make([]LogLine, 0, lb.capacity)
    lb.head = 0
    lb.size = 0
}
```

### Instance Discovery

```go
func discoverInstances(dateRange DateRange) ([]InstanceInfo, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("failed to get home dir: %w", err)
    }

    instancesDir := filepath.Join(homeDir, ".cc-modelrouter", "instances")

    files, err := os.ReadDir(instancesDir)
    if err != nil {
        if os.IsNotExist(err) {
            return []InstanceInfo{}, nil  // No instances yet
        }
        return nil, fmt.Errorf("failed to read instances dir: %w", err)
    }

    start, end := calculateDateRangeForInstances(dateRange)
    var instances []InstanceInfo
    var parseErrors int

    for _, file := range files {
        if !strings.HasSuffix(file.Name(), ".json") {
            continue
        }

        path := filepath.Join(instancesDir, file.Name())
        data, err := os.ReadFile(path)
        if err != nil {
            parseErrors++
            continue
        }

        var meta struct {
            ID        string    `json:"id"`
            PID       int       `json:"pid"`
            StartTime time.Time `json:"startTime"`
        }

        if err := json.Unmarshal(data, &meta); err != nil {
            parseErrors++
            continue
        }

        // Filter by date range
        if meta.StartTime.Before(start) || meta.StartTime.After(end) {
            continue
        }

        isRunning := processExists(meta.PID)
        logPath := filepath.Join(homeDir, ".cc-modelrouter", "logs", meta.ID+".log")

        instances = append(instances, InstanceInfo{
            ID:        meta.ID,
            IsRunning: isRunning,
            StartTime: meta.StartTime,
            LogPath:   logPath,
        })
    }

    // Log warning if many parse errors
    if parseErrors > 0 && parseErrors > len(instances)/2 {
        return instances, fmt.Errorf("warning: failed to parse %d instance files", parseErrors)
    }

    // Sort: running first, then by start time descending
    sort.Slice(instances, func(i, j int) bool {
        if instances[i].IsRunning != instances[j].IsRunning {
            return instances[i].IsRunning
        }
        return instances[i].StartTime.After(instances[j].StartTime)
    })

    return instances, nil
}

func processExists(pid int) bool {
    if pid <= 0 {
        return false
    }

    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }

    // Send signal 0 to test (Unix and Windows compatible)
    err = process.Signal(syscall.Signal(0))
    return err == nil
}
```

---

## 5. Rendering & Styling with Lipgloss

### Color Palette

```go
var (
    primaryColor   = lipgloss.Color("#7D56F4")  // Purple
    secondaryColor = lipgloss.Color("#6C5CE7")  // Purple variant
    accentColor    = lipgloss.Color("#00D9FF")  // Cyan
    errorColor     = lipgloss.Color("#FF5555")  // Red
    successColor   = lipgloss.Color("#50FA7B")  // Green
    mutedColor     = lipgloss.Color("#6272A4")  // Gray-blue
)
```

**Accessibility Notes:**
- High contrast ratios for readability
- Colorblind-friendly (distinct hues)
- Fallback to basic 16 colors if needed

### Component Styles

```go
// Borders
borderStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(mutedColor)

// Header
headerStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#FAFAFA")).
    Background(primaryColor).
    Padding(0, 1)

// Tabs
activeTabStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(accentColor).
    Background(lipgloss.Color("#2E3440")).
    Padding(0, 2)

inactiveTabStyle = lipgloss.NewStyle().
    Foreground(mutedColor).
    Background(lipgloss.Color("#1E1E1E")).
    Padding(0, 2)

// Tables
tableHeaderStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(accentColor).
    BorderStyle(lipgloss.NormalBorder()).
    BorderBottom(true).
    BorderForeground(mutedColor)

tableRowStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#F8F8F2"))

tableRowAltStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#E0E0E0")).
    Background(lipgloss.Color("#282A36"))

// Instance list
selectedInstanceStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(accentColor).
    Background(lipgloss.Color("#44475A")).
    Padding(0, 1)

unselectedInstanceStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#F8F8F2")).
    Padding(0, 1)

runningIndicator = lipgloss.NewStyle().
    Foreground(successColor).
    SetString("●")

stoppedIndicator = lipgloss.NewStyle().
    Foreground(mutedColor).
    SetString("○")

// Log levels (color-coded)
logLevelStyles = map[LogLevel]lipgloss.Style{
    LogLevelVerbs:  lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")),
    LogLevelTrace:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")),
    LogLevelDebug:  lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")),
    LogLevelInfo:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")),
    LogLevelWarn:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")),
    LogLevelError:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")),
    LogLevelFatal:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true),
}
```

### View() Implementation Pattern

```go
func (m MonitorModel) View() string {
    m.mu.RLock()
    defer m.mu.RUnlock()

    if m.windowSize.Width == 0 {
        return "Initializing..."
    }

    var sections []string

    // 1. Header Bar
    sections = append(sections, m.renderHeader())

    // 2. Content Area
    sections = append(sections, m.renderContent())

    // 3. Console Log (conditional)
    if m.consoleLogEnabled && m.selectedInstance != "" {
        sections = append(sections, m.renderConsoleLog())
    }

    // 4. Status Bar
    sections = append(sections, m.renderStatusBar())

    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
```

**Rendering Principles:**
- **Thread-safe**: All shared state reads protected by `mu.RLock()`
- **Responsive**: Adjust layout based on `windowSize`
- **Graceful degradation**: Show "Loading..." or empty states
- **Performance**: Minimize allocations, reuse builders

---

## 6. Error Handling & Edge Cases

### Critical Error Scenarios

**1. Database Unavailable**
- Return empty stats with error message
- Continue polling (with backoff)
- Display error in status bar
- Don't crash UI

**2. Log File Rotation/Deletion**
- Detect rotation via `os.SameFile()` (cross-platform)
- Automatically reopen new file
- Handle ENOENT gracefully (wait for recreation)
- Don't lose tail position

**3. Terminal Resize**
- Handle `tea.WindowSizeMsg`
- Recalculate all component dimensions
- Adjust viewport height/width
- Re-render immediately

**4. No Data Scenarios**
- No instances: Show "(no instances in date range)"
- No usage data: Show "No usage data for this period"
- Loading: Show "Loading..."
- All are visually distinct from errors

**5. Unparseable Log Lines**
- Fallback to minimal `LogLine` with raw content
- Use current timestamp
- Default to INFO level
- Never skip lines

**6. Rapid Instance Switching**
- Cancel old log tailer context
- Don't wait (non-blocking)
- Clear log buffer immediately
- Disable console log (explicit re-enable required)

**7. Goroutine Lifecycle**
- Use `sync.WaitGroup` to track goroutines
- Use `context.Context` for cancellation
- Separate contexts for stats poller and log tailer
- Timeout on shutdown (2 seconds max)

### Resource Management

**Buffered Channels:**
```go
statsChan := make(chan *UsageStats, 10)   // 10 updates
logChan   := make(chan string, 1000)      // 1000 log lines
errChan   := make(chan error, 100)        // 100 errors
```

**Backpressure Handling:**
- Stats: Block sender (wait for UI to catch up)
- Logs: Drop oldest (default case in select)
- Errors: Drop if buffer full (non-critical)

**Exponential Backoff (Stats Poller):**
- First error: Continue immediately
- Subsequent errors: 1s → 2s → 4s → 8s → max 30s
- Reset on success

**Ring Buffer (Log Buffer):**
- Fixed capacity: 1000 lines
- Overwrites oldest when full
- Thread-safe with mutex
- Bounded memory usage

### Shutdown Sequence

```go
func (m *MonitorModel) shutdown() tea.Cmd {
    return func() tea.Msg {
        // 1. Cancel all contexts
        m.cancel()
        if m.logCancel != nil {
            m.logCancel()
        }

        // 2. Wait for goroutines with timeout
        done := make(chan struct{})
        go func() {
            m.wg.Wait()
            close(done)
        }()

        select {
        case <-done:
            // Clean shutdown
        case <-time.After(2 * time.Second):
            // Force quit after timeout
        }

        return tea.Quit()
    }
}
```

---

## 7. Implementation Checklist

### Phase 1: Core Infrastructure
- [ ] Create `internal/cli/monitor.go` with Cobra command
- [ ] Create `internal/monitor/model.go` with MonitorModel
- [ ] Create `internal/monitor/poller.go` with StatsPoller
- [ ] Create `internal/monitor/tailer.go` with LogTailer
- [ ] Create `internal/monitor/buffer.go` with LogBuffer
- [ ] Add Bubble Tea and Lipgloss dependencies to `go.mod`

### Phase 2: UI Components
- [ ] Implement `renderHeader()` with date range tabs
- [ ] Implement `renderRouteTable()` with sorting
- [ ] Implement `renderModelTable()` with sorting
- [ ] Implement `renderInstanceList()` with indicators
- [ ] Implement `renderConsoleLog()` with level filters
- [ ] Implement `renderStatusBar()` with context shortcuts

### Phase 3: State Management
- [ ] Implement `Update()` method with all key handlers
- [ ] Implement date range navigation
- [ ] Implement instance selection logic
- [ ] Implement console log toggle
- [ ] Implement log level filter toggle
- [ ] Implement pause/resume logic

### Phase 4: Background Workers
- [ ] Implement stats polling loop with backoff
- [ ] Implement log tailing with rotation detection
- [ ] Implement instance discovery
- [ ] Implement process PID check
- [ ] Test goroutine lifecycle and cleanup

### Phase 5: Error Handling
- [ ] Add mutex protection to all shared state
- [ ] Implement graceful degradation for all errors
- [ ] Add error display in status bar
- [ ] Test database unavailable scenario
- [ ] Test log file rotation scenario
- [ ] Test rapid instance switching

### Phase 6: Testing & Polish
- [ ] Unit tests for date range calculations
- [ ] Unit tests for log parsing
- [ ] Unit tests for ring buffer
- [ ] Integration test for concurrent access
- [ ] Manual testing on different terminal sizes (80x24 to 200x60)
- [ ] Test on macOS, Linux, Windows
- [ ] Performance profiling (CPU, memory)

---

## 8. Future Enhancements (Out of Scope)

These features are not part of the initial implementation but could be added later:

1. **Help Overlay** - Press '?' to show keyboard shortcuts
2. **Search/Filter** - Filter tables by route or model name
3. **Export to CSV** - Save current stats to CSV file
4. **Sparklines** - Show token usage trends over time
5. **Alerts** - Highlight when fallback rate exceeds threshold
6. **Custom Date Range** - Allow user to enter YYYYMMDD-YYYYMMDD
7. **Multi-Instance Log View** - Show logs from multiple instances interleaved
8. **Cost Estimation** - Show estimated API costs per route/model
9. **Real-Time Graphs** - ASCII charts showing usage over time
10. **Configuration File** - Save user preferences (default date range, log levels, etc.)

---

## 9. Dependencies

**Go Libraries (to be added to `go.mod`):**
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/bubbles/viewport` - Scrollable viewport component

**Existing Dependencies:**
- `github.com/spf13/cobra` - CLI framework (already present)
- `modernc.org/sqlite` - SQLite driver (already present)

**Standard Library:**
- `context` - Cancellation
- `sync` - Mutexes, WaitGroup
- `time` - Timers, periods
- `os` - File operations
- `bufio` - Log reading
- `syscall` - PID checking

---

## 10. Open Questions (Resolved)

All questions have been answered during the brainstorming session:

1. ✅ **Refresh Rate:** 1 second default, configurable via `--refresh` flag
2. ✅ **Console Log Streaming:** Hybrid tail-follow with pause/scroll, log level filtering
3. ✅ **Date Range Navigation:** TODAY/WEEK/MONTH/YTD/TTM tabs
4. ✅ **Week/Month Definition:** Calendar-based, Monday start
5. ✅ **TUI Library:** Bubble Tea + Lipgloss
6. ✅ **Instance List Behavior:** Running + recent stopped instances in date range
7. ✅ **Multi-Instance Logs:** Not supported (single-instance only)

---

## Conclusion

This design provides a comprehensive blueprint for implementing the `ccrouter monitor` live usage dashboard. The architecture prioritizes:

- **Simplicity** - Straightforward polling approach over complex event-driven systems
- **Reliability** - Graceful error handling, never crashes
- **Performance** - Buffered channels, bounded memory, efficient rendering
- **User Experience** - Responsive UI, clear keyboard shortcuts, meaningful feedback
- **Maintainability** - Clean separation of concerns, well-defined interfaces

The implementation is estimated at **2-3 days** for an experienced Go developer familiar with Bubble Tea.
