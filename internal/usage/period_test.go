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

func TestParsePeriod_LastPeriods(t *testing.T) {
	now := time.Date(2025, 2, 23, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		period    string
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "last-week",
			period:    "last-week",
			wantStart: time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC),  // Monday Feb 10
			wantEnd:   time.Date(2025, 2, 16, 23, 59, 59, 0, time.UTC), // Sunday Feb 16
		},
		{
			name:      "last-month",
			period:    "last-month",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "this-quarter", // Feb is in Q1
			period:    "this-quarter",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 3, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "last-quarter", // Q4 of previous year
			period:    "last-quarter",
			wantStart: time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "last-year",
			period:    "last-year",
			wantStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParsePeriod(tt.period, now)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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

func TestParsePeriod_EdgeCases(t *testing.T) {
	t.Run("empty string defaults to all-time", func(t *testing.T) {
		start, end, err := ParsePeriod("", time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !start.IsZero() {
			t.Error("empty string should return zero start time")
		}
		if end.Before(time.Now()) {
			t.Error("empty string should return far future end time")
		}
	})

	t.Run("invalid custom range", func(t *testing.T) {
		_, _, err := ParsePeriod("not-a-range", time.Now())
		if err == nil {
			t.Error("expected error for invalid range format")
		}
	})

	t.Run("custom range invalid date format", func(t *testing.T) {
		_, _, err := ParsePeriod("invalid-20250301", time.Now())
		if err == nil {
			t.Error("expected error for invalid date format")
		}
	})

	t.Run("custom range short format", func(t *testing.T) {
		_, _, err := ParsePeriod("2025-0201", time.Now())
		if err == nil {
			t.Error("expected error for short date format")
		}
	})
}
