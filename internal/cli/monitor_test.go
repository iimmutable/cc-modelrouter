package cli

import (
	"testing"
	"time"
)

func TestNewMonitorCommand(t *testing.T) {
	cmd := NewMonitorCommand()

	if cmd.Use != "monitor" {
		t.Errorf("expected Use %q, got %q", "monitor", cmd.Use)
	}

	// Verify --refresh flag exists
	f := cmd.Flags().Lookup("refresh")
	if f == nil {
		t.Fatal("expected --refresh flag to exist")
	}

	// Verify default value is 500ms
	got, err := cmd.Flags().GetDuration("refresh")
	if err != nil {
		t.Fatalf("failed to get --refresh value: %v", err)
	}
	if got != 500*time.Millisecond {
		t.Errorf("expected --refresh default 500ms, got %v", got)
	}
}

func TestRunMonitorExists(t *testing.T) {
	// Verify that runMonitor is a valid function (not nil).
	// We cannot fully test the TUI launch, but we can verify
	// the function exists and returns a non-nil closure.
	var zero time.Duration
	fn := runMonitor(&zero)
	if fn == nil {
		t.Fatal("runMonitor should return a non-nil function")
	}
}
