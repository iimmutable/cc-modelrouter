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
	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 100, 0)

	// Should not be flushed yet (buffer not full)
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 0 {
		t.Error("records flushed before buffer full")
	}

	// Fill buffer
	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 200, 0)
	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 300, 0)

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

	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 100, 0)

	// Shutdown should flush
	tracker.Shutdown()

	// Check record was flushed
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 1 {
		t.Errorf("expected 1 record after shutdown, got %d", len(records))
	}
}

func TestTrackerFlushOnTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	tracker := NewTracker(db, 500, 50*time.Millisecond) // Short timeout

	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 100, 0)

	// Should not be flushed immediately
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 0 {
		t.Error("records flushed before timeout")
	}

	// Wait for timeout flush
	time.Sleep(150 * time.Millisecond)

	// Check record was flushed
	records, _ = GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 1 {
		t.Errorf("expected 1 record after timeout, got %d", len(records))
	}
}

func TestTrackerConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	tracker := NewTracker(db, 100, 100*time.Millisecond)

	// Record concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			tracker.Record("inst1", "/think", "model1", "default", "openrouter", n*100, 0)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Wait for flush
	time.Sleep(150 * time.Millisecond)

	// Check all records were flushed
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 10 {
		t.Errorf("expected 10 records, got %d", len(records))
	}
}

func TestTrackerDefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	tracker := NewTracker(db, 0, 0) // Invalid values, should use defaults

	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 100, 0)

	// Shutdown should flush
	tracker.Shutdown()

	// Check record was flushed
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 1 {
		t.Errorf("expected 1 record with defaults, got %d", len(records))
	}
}

func TestTrackerDoubleShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	tracker := NewTracker(db, 100, time.Hour)

	tracker.Record("inst1", "/think", "model1", "default", "openrouter", 100, 0)

	// First shutdown
	tracker.Shutdown()

	// Check record was flushed
	records, _ := GetRecordsByPeriod(db, "", time.Time{}, time.Now())
	if len(records) != 1 {
		t.Errorf("expected 1 record after first shutdown, got %d", len(records))
	}

	// Second shutdown - panic is acceptable for closing closed channel
	// This test documents the expected behavior
	defer func() {
		if r := recover(); r != nil {
			// Panic is expected when calling Shutdown twice
			// This is acceptable behavior for misuse of the API
		}
	}()

	tracker.Shutdown()
}
