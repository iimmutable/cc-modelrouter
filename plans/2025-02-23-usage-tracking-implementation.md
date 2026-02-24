# Usage Tracking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add token usage tracking feature with SQLite persistence and CLI query interface for `cc-modelrouter`.

**Architecture:** In-memory buffer with auto-flush to SQLite database. Usage data collected in proxy handler, aggregated for CLI display with period filtering.

**Tech Stack:** Go 1.21+, modernc.org/sqlite (pure Go, no CGO), time package for period parsing

---

## Task 1: Add SQLite Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add modernc.org/sqlite dependency**

Run: `go get modernc.org/sqlite`
Expected: `go: added modernc.org/sqlite v1.x.x`

**Step 2: Tidy go.mod**

Run: `go mod tidy`
Expected: No errors

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(usage): add sqlite dependency"
```

---

## Task 2: Create Database Layer

**Files:**
- Create: `internal/usage/db.go`
- Test: `internal/usage/db_test.go`

**Step 1: Write the failing test**

Create `internal/usage/db_test.go`:

```go
package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDBInit(t *testing.T) {
	// Use temp file for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Check table exists
	if !tableExists(db, "usage_records") {
		t.Error("usage_records table was not created")
	}
}

func TestInsertRecord(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	record := &Record{
		InstanceID: "test_inst",
		Route:      "/think",
		Model:      "claude-sonnet-4",
		Tokens:     1000,
		Fallbacks:  1,
		Timestamp:  time.Now(),
	}

	if err := InsertRecord(db, record); err != nil {
		t.Fatalf("InsertRecord failed: %v", err)
	}

	// Verify insertion
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM usage_records")
	row.Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}
}

func TestGetRecordsByPeriod(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	// Insert records at different times
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0, Timestamp: yesterday},
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 200, Fallbacks: 0, Timestamp: now},
	}

	for _, r := range records {
		if err := InsertRecord(db, r); err != nil {
			t.Fatalf("InsertRecord failed: %v", err)
		}
	}

	// Query today's records only
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	results, err := GetRecordsByPeriod(db, "", startTime, time.Now())
	if err != nil {
		t.Fatalf("GetRecordsByPeriod failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 record for today, got %d", len(results))
	}
}

func tableExists(db *SQLiteConn, tableName string) bool {
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName)
	row.Scan(&count)
	return count == 1
}
```

Run: `go test ./internal/usage/... -v`
Expected: FAIL with "undefined: InitDB" and other undefined errors

**Step 2: Create the usage package directory**

Run: `mkdir -p internal/usage`
Expected: Directory created

**Step 3: Write minimal implementation**

Create `internal/usage/db.go`:

```go
package usage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Record represents a usage record.
type Record struct {
	ID         int64
	InstanceID string
	Route      string
	Model      string
	Tokens     int
	Fallbacks  int
	Timestamp  time.Time
}

// DBPath returns the path to the usage database.
func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-modelrouter", "usage.db"), nil
}

// InitDB initializes the usage database.
func InitDB(path string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Create table
	query := `
	CREATE TABLE IF NOT EXISTS usage_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		instance_id TEXT NOT NULL,
		route TEXT NOT NULL,
		model TEXT NOT NULL,
		tokens INTEGER NOT NULL,
		fallbacks INTEGER NOT NULL DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_instance_route ON usage_records(instance_id, route);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON usage_records(timestamp);
	`

	if _, err := db.Exec(query); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// InsertRecord inserts a usage record.
func InsertRecord(db *sql.DB, r *Record) error {
	query := `
	INSERT INTO usage_records (instance_id, route, model, tokens, fallbacks, timestamp)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, r.InstanceID, r.Route, r.Model, r.Tokens, r.Fallbacks, r.Timestamp)
	return err
}

// GetRecordsByPeriod retrieves records within a time range, optionally filtered by instance.
func GetRecordsByPeriod(db *sql.DB, instanceID string, start, end time.Time) ([]*Record, error) {
	query := `
	SELECT id, instance_id, route, model, tokens, fallbacks, timestamp
	FROM usage_records
	WHERE timestamp >= ? AND timestamp <= ?
	`
	args := []any{start, end}

	if instanceID != "" {
		query += " AND instance_id = ?"
		args = append(args, instanceID)
	}

	query += " ORDER BY timestamp DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*Record
	for rows.Next() {
		var r Record
		err := rows.Scan(&r.ID, &r.InstanceID, &r.Route, &r.Model, &r.Tokens, &r.Fallbacks, &r.Timestamp)
		if err != nil {
			return nil, err
		}
		records = append(records, &r)
	}

	return records, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/usage/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/usage/
git commit -m "feat(usage): add database layer with sqlite"
```

---

## Task 3: Create In-Memory Tracker

**Files:**
- Create: `internal/usage/tracker.go`
- Test: `internal/usage/tracker_test.go`

**Step 1: Write the failing test**

Create `internal/usage/tracker_test.go`:

```go
package usage

import (
	"testing"
	"time"
)

func TestTrackerRecord(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	tracker := NewTracker(db, 2, 100*time.Millisecond) // Small buffer for testing

	// Record below buffer size
	tracker.Record("inst1", "/think", "model1", 100, 0)

	// Should not be flushed yet (buffer not full)
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 0 {
		t.Error("records flushed before buffer full")
	}

	// Fill buffer
	tracker.Record("inst1", "/think", "model1", 200, 0)
	tracker.Record("inst1", "/think", "model1", 300, 0)

	// Wait for async flush
	time.Sleep(200 * time.Millisecond)

	// Check records were flushed
	records, _ = GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

func TestTrackerFlushOnShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	tracker := NewTracker(db, 500, time.Hour) // Large timeout

	tracker.Record("inst1", "/think", "model1", 100, 0)

	// Shutdown should flush
	tracker.Shutdown()

	// Check record was flushed
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 1 {
		t.Errorf("expected 1 record after shutdown, got %d", len(records))
	}
}
```

Run: `go test ./internal/usage/... -v -run TestTracker`
Expected: FAIL with "undefined: NewTracker"

**Step 2: Write minimal implementation**

Create `internal/usage/tracker.go`:

```go
package usage

import (
	"database/sql"
	"sync"
	"time"
)

const (
	DefaultBufferSize = 500
	DefaultFlushTimeout = 3 * time.Second
)

// Tracker tracks usage records in memory and flushes to database.
type Tracker struct {
	db       *sql.DB
	buffer   []*Record
	mu       sync.Mutex
	bufferSize int
	flushTimeout time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewTracker creates a new usage tracker.
func NewTracker(db *sql.DB, bufferSize int, flushTimeout time.Duration) *Tracker {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	if flushTimeout <= 0 {
		flushTimeout = DefaultFlushTimeout
	}

	t := &Tracker{
		db:           db,
		buffer:       make([]*Record, 0, bufferSize),
		bufferSize:   bufferSize,
		flushTimeout: flushTimeout,
		done:         make(chan struct{}),
	}

	t.wg.Add(1)
	go t.flushLoop()

	return t
}

// Record adds a usage record.
func (t *Tracker) Record(instanceID, route, model string, tokens, fallbacks int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r := &Record{
		InstanceID: instanceID,
		Route:      route,
		Model:      model,
		Tokens:     tokens,
		Fallbacks:  fallbacks,
		Timestamp:  time.Now(),
	}

	t.buffer = append(t.buffer, r)

	if len(t.buffer) >= t.bufferSize {
		t.flush()
	}
}

// flush writes buffered records to database.
func (t *Tracker) flush() {
	if len(t.buffer) == 0 {
		return
	}

	// Copy buffer and clear
	records := make([]*Record, len(t.buffer))
	copy(records, t.buffer)
	t.buffer = t.buffer[:0]

	// Batch insert
	tx, err := t.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	for _, r := range records {
		if err := InsertRecord(tx, r); err != nil {
			return
		}
	}

	tx.Commit()
}

// flushLoop periodically flushes the buffer.
func (t *Tracker) flushLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			t.flush()
			t.mu.Unlock()
		case <-t.done:
			return
		}
	}
}

// Shutdown flushes remaining records and stops the tracker.
func (t *Tracker) Shutdown() {
	close(t.done)
	t.wg.Wait()

	t.mu.Lock()
	t.flush()
	t.mu.Unlock()
}
```

**Step 3: Update InsertRecord to support transactions**

Modify `InsertRecord` in `internal/usage/db.go` to accept both `*sql.DB` and `*sql.Tx`:

```go
// dbExecutor is an interface that both *sql.DB and *sql.Tx implement.
type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// InsertRecord inserts a usage record.
func InsertRecord(db dbExecutor, r *Record) error {
	query := `
	INSERT INTO usage_records (instance_id, route, model, tokens, fallbacks, timestamp)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, r.InstanceID, r.Route, r.Model, r.Tokens, r.Fallbacks, r.Timestamp)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/usage/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/usage/
git commit -m "feat(usage): add in-memory tracker with auto-flush"
```

---

## Task 4: Create Period Parser

**Files:**
- Create: `internal/usage/period.go`
- Test: `internal/usage/period_test.go`

**Step 1: Write the failing test**

Create `internal/usage/period_test.go`:

```go
package usage

import (
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	now := time.Date(2025, 2, 23, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		period      string
		wantStart   time.Time
		wantEnd     time.Time
		expectError bool
	}{
		{
			name:    "all-time",
			period:  "all-time",
			wantStart: time.Time{},
			wantEnd:   time.Now().AddDate(100, 0, 0), // Far future
		},
		{
			name:      "today",
			period:    "today",
			wantStart: time.Date(2025, 2, 23, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 2, 23, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "this-week",
			period:    "this-week",
			wantStart: time.Date(2025, 2, 17, 0, 0, 0, 0, time.UTC), // Monday
			wantEnd:   time.Date(2025, 2, 23, 23, 59, 59, 0, time.UTC), // Sunday
		},
		{
			name:      "this-month",
			period:    "this-month",
			wantStart: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 2, 28, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "this-year",
			period:    "this-year",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:        "invalid",
			period:      "invalid-period",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParsePeriod(tt.period, now)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// For all-time, just check start is zero
			if tt.period == "all-time" {
				if !start.IsZero() {
					t.Error("all-time start should be zero")
				}
				return
			}

			if !start.Equal(tt.wantStart) {
				t.Errorf("start = %v, want %v", start, tt.wantStart)
			}
			if !end.Equal(tt.wantEnd) {
				t.Errorf("end = %v, want %v", end, tt.wantEnd)
			}
		})
	}
}

func TestParseCustomRange(t *testing.T) {
	start, end, err := ParsePeriod("20250201-20250215", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantStart := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2025, 2, 15, 23, 59, 59, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}
```

Run: `go test ./internal/usage/... -v -run TestParsePeriod`
Expected: FAIL with "undefined: ParsePeriod"

**Step 2: Write minimal implementation**

Create `internal/usage/period.go`:

```go
package usage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParsePeriod converts a period string to start and end times.
func ParsePeriod(period string, now time.Time) (start, end time.Time, err error) {
	switch period {
	case "all-time", "":
		return time.Time{}, time.Now().AddDate(100, 0, 0), nil

	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "this-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day()+(7-weekday), 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "last-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1-7, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day()-weekday, 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "this-month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, -1, now.Location())
		return start, end, nil

	case "last-month":
		start = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, -1, now.Location())
		return start, end, nil

	case "this-quarter":
		quarter := (int(now.Month()) - 1) / 3
		start = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), time.Month(quarter*3+4), 1, 0, 0, 0, -1, now.Location())
		return start, end, nil

	case "last-quarter":
		quarter := (int(now.Month()) - 1) / 3
		start = time.Date(now.Year(), time.Month(quarter*3-2), 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, -1, now.Location())
		return start, end, nil

	case "this-year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "last-year":
		start = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year()-1, 12, 31, 23, 59, 59, 0, now.Location())
		return start, end, nil

	default:
		// Try custom range YYYYMMDD-YYYYMMDD
		return parseCustomRange(period)
	}
}

func parseCustomRange(period string) (start, end time.Time, err error) {
	parts := strings.Split(period, "-")
	if len(parts) != 3 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period format: %s (expected YYYYMMDD-YYYYMMDD)", period)
	}

	start, err = parseDate(parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
	}

	end, err = parseDate(parts[1]+parts[2])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
	}

	// Set end to end of day
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, end.Location())

	return start, end, nil
}

func parseDate(s string) (time.Time, error) {
	if len(s) != 8 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}

	year, err := strconv.Atoi(s[0:4])
	if err != nil {
		return time.Time{}, err
	}

	month, err := strconv.Atoi(s[4:6])
	if err != nil {
		return time.Time{}, err
	}

	day, err := strconv.Atoi(s[6:8])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./internal/usage/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/usage/
git commit -m "feat(usage): add period parser for time range filtering"
```

---

## Task 5: Create Stats Aggregator

**Files:**
- Create: `internal/usage/stats.go`
- Test: `internal/usage/stats_test.go`

**Step 1: Write the failing test**

Create `internal/usage/stats_test.go`:

```go
package usage

import (
	"testing"
	"time"
)

func TestAggregateSummary(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0, Timestamp: time.Now()},
	}

	summary := AggregateSummary(records)

	if summary.TotalRequests != 3 {
		t.Errorf("requests = %d, want 3", summary.TotalRequests)
	}
	if summary.TotalTokens != 600 {
		t.Errorf("tokens = %d, want 600", summary.TotalTokens)
	}
	if summary.TotalFallbacks != 1 {
		t.Errorf("fallbacks = %d, want 1", summary.TotalFallbacks)
	}
}

func TestAggregateByRoute(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 50, Fallbacks: 0},
	}

	byRoute := AggregateByRoute(records)

	// Should have 2 routes
	if len(byRoute) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(byRoute))
	}

	// Check /think
	think := byRoute["/think"]
	if think.Requests != 3 {
		t.Errorf("/think requests = %d, want 3", think.Requests)
	}
	if think.Tokens != 350 {
		t.Errorf("/think tokens = %d, want 350", think.Tokens)
	}
	if think.Fallbacks != 1 {
		t.Errorf("/think fallbacks = %d, want 1", think.Fallbacks)
	}

	// Check /ultrathink
	ultra := byRoute["/ultrathink"]
	if ultra.Requests != 1 {
		t.Errorf("/ultrathink requests = %d, want 1", ultra.Requests)
	}
	if ultra.Tokens != 300 {
		t.Errorf("/ultrathink tokens = %d, want 300", ultra.Tokens)
	}
}

func TestAggregateByModel(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0},
	}

	byModel := AggregateByModel(records)

	// Should have 2 models
	if len(byModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(byModel))
	}

	// Check m1
	m1 := byModel["m1"]
	if m1.Requests != 2 {
		t.Errorf("m1 requests = %d, want 2", m1.Requests)
	}
	if m1.Tokens != 400 {
		t.Errorf("m1 tokens = %d, want 400", m1.Tokens)
	}

	// Check m2
	m2 := byModel["m2"]
	if m2.Requests != 1 {
		t.Errorf("m2 requests = %d, want 1", m2.Requests)
	}
	if m2.Tokens != 200 {
		t.Errorf("m2 tokens = %d, want 200", m2.Tokens)
	}
}
```

Run: `go test ./internal/usage/... -v -run TestAggregate`
Expected: FAIL with "undefined: AggregateSummary" and other undefined errors

**Step 2: Write minimal implementation**

Create `internal/usage/stats.go`:

```go
package usage

// Summary represents aggregated usage summary.
type Summary struct {
	TotalRequests  int
	TotalTokens    int
	TotalFallbacks int
}

// RouteStats represents stats for a single route.
type RouteStats struct {
	Route      string
	Requests   int
	Tokens     int
	Fallbacks  int
}

// ModelStats represents stats for a single model.
type ModelStats struct {
	Model    string
	Requests int
	Tokens   int
}

// AggregateSummary computes overall summary from records.
func AggregateSummary(records []*Record) Summary {
	var s Summary
	for _, r := range records {
		s.TotalRequests++
		s.TotalTokens += r.Tokens
		s.TotalFallbacks += r.Fallbacks
	}
	return s
}

// AggregateByRoute groups records by route.
func AggregateByRoute(records []*Record) map[string]*RouteStats {
	result := make(map[string]*RouteStats)
	for _, r := range records {
		stats, ok := result[r.Route]
		if !ok {
			stats = &RouteStats{Route: r.Route}
			result[r.Route] = stats
		}
		stats.Requests++
		stats.Tokens += r.Tokens
		stats.Fallbacks += r.Fallbacks
	}
	return result
}

// AggregateByModel groups records by model.
func AggregateByModel(records []*Record) map[string]*ModelStats {
	result := make(map[string]*ModelStats)
	for _, r := range records {
		stats, ok := result[r.Model]
		if !ok {
			stats = &ModelStats{Model: r.Model}
			result[r.Model] = stats
		}
		stats.Requests++
		stats.Tokens += r.Tokens
	}
	return result
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./internal/usage/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/usage/
git commit -m "feat(usage): add stats aggregation functions"
```

---

## Task 6: Create Output Formatter

**Files:**
- Create: `internal/usage/formatter.go`
- Test: `internal/usage/formatter_test.go`

**Step 1: Write the failing test**

Create `internal/usage/formatter_test.go`:

```go
package usage

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		tokens  int
		want    string
	}{
		{20, "20"},
		{680, "680"},
		{1500, "1,500"},
		{1200000, "1.2M"},
		{3000000, "3.0M"},
		{200000000, "200.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatTokens(tt.tokens); got != tt.want {
				t.Errorf("FormatTokens(%d) = %s, want %s", tt.tokens, got, tt.want)
			}
		})
	}
}

func TestFormatUsage(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "gpt-4o", Tokens: 25000000, Fallbacks: 2, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/think", Model: "claude-sonnet-4", Tokens: 15000000, Fallbacks: 1, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "gemini-2.0-flash", Tokens: 5600000, Fallbacks: 0, Timestamp: time.Now()},
	}

	var buf bytes.Buffer
	FormatUsage(&buf, "inst1", "all-time", records)

	output := buf.String()

	// Check key elements are present
	checks := []string{
		"Usage Summary",
		"1,234", // Will be formatted
		"45.6M", // Will be close to this
		"12",
		"By Route",
		"By Model",
	}

	for _, check := range checks {
		if !contains(output, check) {
			t.Errorf("output missing expected text: %s\noutput:\n%s", check, output)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

Run: `go test ./internal/usage/... -v -run TestFormat`
Expected: FAIL with "undefined: FormatTokens" and "undefined: FormatUsage"

**Step 2: Write minimal implementation**

Create `internal/usage/formatter.go`:

```go
package usage

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// FormatTokens formats a token count for display.
func FormatTokens(tokens int) string {
	switch {
	case tokens < 1_000:
		return fmt.Sprintf("%d", tokens)
	case tokens < 1_000_000:
		return fmt.Sprintf("%d,%03d", tokens/1000, tokens%1000)
	case tokens < 10_000_000:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens < 100_000_000:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	default:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
}

// FormatUsage writes formatted usage statistics to the writer.
func FormatUsage(w io.Writer, instanceID, period string, records []*Record) {
	summary := AggregateSummary(records)
	byRoute := AggregateByRoute(records)
	byModel := AggregateByModel(records)

	// Header
	var title string
	if instanceID == "" {
		title = fmt.Sprintf("Usage Summary (%s, all instances)", period)
	} else {
		title = fmt.Sprintf("Usage Summary (%s, instance %s)", period, instanceID)
	}

	fmt.Fprintf(w, "%s\n", title)
	fmt.Fprintf(w, "  Requests: %s  |  Tokens: %s  |  Fallbacks: %d\n\n",
		formatNumber(summary.TotalRequests),
		FormatTokens(summary.TotalTokens),
		summary.TotalFallbacks)

	// By Route
	fmt.Fprintln(w, "By Route:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Route\tRequests\tTokens\tFallbacks")
	fmt.Fprintln(tw, "  ─────────────────────────────────────────────────")

	// Sort routes by name
	routes := make([]*RouteStats, 0, len(byRoute))
	for _, stats := range byRoute {
		routes = append(routes, stats)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Route < routes[j].Route
	})

	for _, stats := range routes {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\n",
			stats.Route,
			formatNumber(stats.Requests),
			FormatTokens(stats.Tokens),
			stats.Fallbacks)
	}
	tw.Flush()

	// By Model
	fmt.Fprintln(w, "\nBy Model:")
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Model\tRequests\tTokens")
	fmt.Fprintln(tw, "  ──────────────────────────────────────────")

	// Sort models by token count descending
	models := make([]*ModelStats, 0, len(byModel))
	for _, stats := range byModel {
		models = append(models, stats)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Tokens > models[j].Tokens
	})

	for _, stats := range models {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n",
			stats.Model,
			formatNumber(stats.Requests),
			FormatTokens(stats.Tokens))
	}
	tw.Flush()
}

// formatNumber adds thousands separator.
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{s[start:i]}, result...)
	}
	return strings.Join(result, ",")
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./internal/usage/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/usage/
git commit -m "feat(usage): add output formatter for CLI display"
```

---

## Task 7: Create CLI Usage Command

**Files:**
- Create: `internal/cli/usage.go`
- Modify: `internal/cli/root.go`

**Step 1: Write the usage command**

Create `internal/cli/usage.go`:

```go
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/usage"
	"github.com/spf13/cobra"
)

// NewUsageCommand creates the usage command.
func NewUsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage [instance-id] [period]",
		Short: "Show token usage statistics",
		Long:  "Displays token usage statistics per model, per route, and per instance.",
		Args:  cobra.MaximumNArgs(2),
		RunE:  runUsage,
	}

	return cmd
}

func runUsage(cmd *cobra.Command, args []string) error {
	// Parse arguments
	var instanceID, period string
	if len(args) > 0 {
		// Check if first arg is an instance ID or a period
		if isPeriod(args[0]) {
			period = args[0]
		} else {
			instanceID = args[0]
		}
	}
	if len(args) > 1 {
		period = args[1]
	}

	// Default period
	if period == "" {
		period = "all-time"
	}

	// Parse period
	now := time.Now()
	start, end, err := usage.ParsePeriod(period, now)
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	// Open database
	dbPath, err := usage.DBPath()
	if err != nil {
		return fmt.Errorf("failed to get db path: %w", err)
	}

	db, err := usage.InitDB(dbPath)
	if err != nil {
		// Database might not exist yet
		fmt.Fprintln(os.Stderr, "No usage data available yet")
		return nil
	}
	defer db.Close()

	// Query records
	records, err := usage.GetRecordsByPeriod(db, instanceID, start, end)
	if err != nil {
		return fmt.Errorf("failed to query usage: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No usage records found for the specified period")
		return nil
	}

	// Format and display
	usage.FormatUsage(os.Stdout, instanceID, period, records)
	return nil
}

// isPeriod checks if a string looks like a period specification.
func isPeriod(s string) bool {
	periods := []string{"all-time", "today", "this-week", "last-week",
		"this-month", "last-month", "this-quarter", "last-quarter",
		"this-year", "last-year"}
	for _, p := range periods {
		if s == p {
			return true
		}
	}
	// Check for custom date range format
	if len(s) == 17 && s[8] == '-' {
		return true
	}
	return false
}
```

**Step 2: Register the usage command in root.go**

Modify `internal/cli/root.go`:

```go
// In NewRootCommand function, add:
cmd.AddCommand(NewUsageCommand())
```

Full modified function:
```go
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ccrouter",
		Short:   "Claude Code Model Router",
		Version: Version,
	}

	cmd.AddCommand(NewCodeCommand())
	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStopCommand())
	cmd.AddCommand(NewRestartCommand())
	cmd.AddCommand(NewStatusCommand())
	cmd.AddCommand(NewCleanCommand())
	cmd.AddCommand(NewConfigCommand())
	cmd.AddCommand(NewLogsCommand())
	cmd.AddCommand(NewUsageCommand())

	return cmd
}
```

**Step 3: Build and test manually**

Run: `go build ./cmd/ccrouter`
Expected: Binary created successfully

Run: `./ccrouter usage`
Expected: Either "No usage data available yet" or usage summary if data exists

Run: `./ccrouter usage --help`
Expected: Usage help text displayed

**Step 4: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): add usage command for displaying statistics"
```

---

## Task 8: Integrate Tracker into Proxy Handler

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/server.go`
- Modify: `internal/cli/start.go`

**Step 1: Add tracker field to Handler**

Modify `internal/proxy/handler.go`:

Add tracker interface and field:
```go
// Add at top with other interfaces
type UsageTracker interface {
	Record(instanceID, route, model string, tokens, fallbacks int)
}

// In Handler struct, add:
type Handler struct {
	maxRequestSize      int64
	router              Router
	transformerRegistry TransformerRegistry
	providerClients     map[string]HTTPClient
	config              *config.Config
	usageTracker        UsageTracker  // Add this
	instanceID          string        // Add this
}
```

Update `NewHandler`:
```go
func NewHandler(maxRequestSize int64) *Handler {
	return &Handler{
		maxRequestSize:  maxRequestSize,
		providerClients: make(map[string]HTTPClient),
	}
}
```

Add setters:
```go
// Add after SetConfig
func (h *Handler) SetUsageTracker(tracker UsageTracker) {
	h.usageTracker = tracker
}

func (h *Handler) SetInstanceID(id string) {
	h.instanceID = id
}
```

**Step 2: Track usage in handleMessages**

Modify `internal/proxy/handler.go` `handleMessages` method:

Add tracking after successful response:
```go
// In handleMessages, after successful response:
var fallbackCount int
var successfulModel string

// Modify tryTarget to return model name and track fallbacks
for i, target := range targets {
	resp, err := h.tryTarget(r.Context(), req, target)
	if err != nil {
		fallbackCount = i
		continue
	}

	successfulModel = target.Model

	// Track usage
	if h.usageTracker != nil {
		h.usageTracker.Record(h.instanceID, routeName, successfulModel, h.estimateTokens(req), fallbackCount)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return
}
```

**Step 3: Add tracker setup in Server**

Modify `internal/proxy/server.go`:

Add tracker field and setter:
```go
type Server struct {
	config       *ServerConfig
	server       *http.Server
	handler      *Handler
	usageTracker UsageTracker  // Add this
	instanceID   string        // Add this
	mu           sync.Mutex
	running      bool
}

// Add after SetConfig
func (s *Server) SetUsageTracker(tracker UsageTracker) {
	s.usageTracker = tracker
	s.handler.SetUsageTracker(tracker)
}

func (s *Server) SetInstanceID(id string) {
	s.instanceID = id
	s.handler.SetInstanceID(id)
}

// Modify Stop to shutdown tracker
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	// Shutdown tracker if it exists
	if shutdowner, ok := s.usageTracker.(interface{ Shutdown() }); ok {
		shutdowner.Shutdown()
	}

	err := s.server.Shutdown(ctx)
	s.running = false
	return err
}
```

**Step 4: Wire up tracker in start command**

Modify `internal/cli/start.go`:

Add after server creation:
```go
// After proxy.NewServer, add:
import (
	"github.com/iimmutable/cc-modelrouter/internal/usage"
	// ... other imports
)

// In runStart function, after creating server:
// Initialize usage tracker
dbPath, err := usage.DBPath()
if err != nil {
	return fmt.Errorf("failed to get db path: %w", err)
}

usageDB, err := usage.InitDB(dbPath)
if err != nil {
	return fmt.Errorf("failed to init usage db: %w", err)
}

tracker := usage.NewTracker(usageDB, usage.DefaultBufferSize, usage.DefaultFlushTimeout)
server.SetUsageTracker(tracker)
server.SetInstanceID(instanceID)
```

**Step 5: Build and test**

Run: `go build ./cmd/ccrouter`
Expected: Build succeeds

Run: `./ccrouter start &`
Expected: Server starts with tracking enabled

Run: `./ccrouter usage`
Expected: Usage statistics displayed

**Step 6: Commit**

```bash
git add internal/proxy/ internal/cli/start.go
git commit -m "feat(proxy): integrate usage tracking into request handler"
```

---

## Task 9: Verify End-to-End

**Step 1: Manual testing**

Run these commands to verify:
```bash
# Start server
./ccrouter start &
SERVER_PID=$!

# Make some requests (use your actual API config)
curl -X POST http://localhost:8081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4","max_tokens":100,"messages":[{"role":"user","content":"hello"}]}'

# Check usage
./ccrouter usage
./ccrouter usage today

# Cleanup
kill $SERVER_PID
```

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat(usage): complete usage tracking feature implementation"
```

---

## Summary

This implementation creates a complete usage tracking system with:

1. **SQLite persistence** using pure Go (modernc.org/sqlite)
2. **In-memory buffer** with auto-flush (500 records or 3 seconds)
3. **Period filtering** for all-time, today, this-week, this-month, this-year, and custom ranges
4. **CLI command** `ccrouter usage [instance-id] [period]`
5. **Integration** with proxy handler for automatic tracking

The tracker records:
- Instance ID
- Route (think, thinkMore, ultrathink, etc.)
- Model used
- Token count (estimated)
- Fallback count
- Timestamp
