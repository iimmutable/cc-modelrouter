package cli

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "seconds only - 1s",
			duration: 1 * time.Second,
			want:     "1s",
		},
		{
			name:     "seconds only - 30s",
			duration: 30 * time.Second,
			want:     "30s",
		},
		{
			name:     "seconds only - 59s",
			duration: 59 * time.Second,
			want:     "59s",
		},
		{
			name:     "minute boundary - 60s",
			duration: 60 * time.Second,
			want:     "1m0s",
		},
		{
			name:     "minutes and seconds - 1m30s",
			duration: 90 * time.Second,
			want:     "1m30s",
		},
		{
			name:     "minutes and seconds - 5m45s",
			duration: 345 * time.Second,
			want:     "5m45s",
		},
		{
			name:     "hour boundary - 60m",
			duration: 60 * time.Minute,
			want:     "1h0m",
		},
		{
			name:     "hours and minutes - 1h30m",
			duration: 90 * time.Minute,
			want:     "1h30m",
		},
		{
			name:     "hours and minutes - 23h59m",
			duration: 23*time.Hour + 59*time.Minute,
			want:     "23h59m",
		},
		{
			name:     "day boundary - 24h",
			duration: 24 * time.Hour,
			want:     "1d0h",
		},
		{
			name:     "days and hours - 1d12h",
			duration: 36 * time.Hour,
			want:     "1d12h",
		},
		{
			name:     "days and hours - 7d2h",
			duration: 170 * time.Hour,
			want:     "7d2h",
		},
		{
			name:     "large duration - 30d5h",
			duration: 30*24*time.Hour + 5*time.Hour,
			want:     "30d5h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatDuration_EdgeCases(t *testing.T) {
	// Test that formatDuration handles truncated seconds correctly
	// due to Round(time.Second)
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "milliseconds truncated to seconds",
			duration: 1500 * time.Millisecond, // 1.5s -> rounds to 2s
			want:     "2s",
		},
		{
			name:     "milliseconds below 1s",
			duration: 500 * time.Millisecond, // 0.5s -> rounds to 1s
			want:     "1s",
		},
		{
			name:     "mixed seconds and milliseconds",
			duration: 90*time.Second + 500*time.Millisecond, // 90.5s -> rounds to 91s
			want:     "1m31s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rounded := tt.duration.Round(time.Second)
			got := formatDuration(rounded)
			if got != tt.want {
				t.Errorf("formatDuration(%v.Round(1s)) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}