package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDBInit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Check table exists
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='usage_records'")
	row.Scan(&count)
	if count != 1 {
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

func TestDBPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	path, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath failed: %v", err)
	}

	expected := filepath.Join(home, ".cc-modelrouter", "usage.db")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}

	// Verify it ends with the correct components
	if !strings.Contains(path, ".cc-modelrouter") {
		t.Error("path should contain .cc-modelrouter directory")
	}
	if !strings.HasSuffix(path, "usage.db") {
		t.Error("path should end with usage.db")
	}
}

func TestGetRecordsByPeriodWithInstanceFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	now := time.Now()

	// Insert records for different instances
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0, Timestamp: now},
		{InstanceID: "inst2", Route: "/think", Model: "m1", Tokens: 200, Fallbacks: 0, Timestamp: now},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 150, Fallbacks: 0, Timestamp: now},
	}

	for _, r := range records {
		if err := InsertRecord(db, r); err != nil {
			t.Fatalf("InsertRecord failed: %v", err)
		}
	}

	// Query records for inst1 only
	startTime := now.Add(-1 * time.Hour)
	results, err := GetRecordsByPeriod(db, "inst1", startTime, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("GetRecordsByPeriod failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 records for inst1, got %d", len(results))
	}

	// Verify all results are for inst1
	for _, r := range results {
		if r.InstanceID != "inst1" {
			t.Errorf("expected instance_id inst1, got %s", r.InstanceID)
		}
	}
}
