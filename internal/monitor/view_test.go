package monitor

import (
	"testing"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.00K"},
		{1500, "1.50K"},
		{9999, "10.00K"},
		{10000, "10.0K"},
		{100000, "100.0K"},
		{999999, "1000.0K"},
		{1000000, "1.00M"},
		{5000000, "5.00M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{50000, "50.0K"},
		{999999, "1000.0K"},
		{1000000, "1.00M"},
		{2500000, "2.50M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatTokens(tt.input)
			if got != tt.want {
				t.Errorf("formatTokens(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Run("short string", func(t *testing.T) {
		got := truncate("hello", 10)
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("exact length", func(t *testing.T) {
		got := truncate("hello", 5)
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("needs truncation", func(t *testing.T) {
		got := truncate("hello world", 8)
		if len(got) > 8 {
			t.Errorf("expected max 8 chars, got %d chars: %q", len(got), got)
		}
		if got != "hello..." {
			t.Errorf("expected 'hello...' for 8 char limit, got %q", got)
		}
	})
}

func TestMax(t *testing.T) {
	if max(1, 2) != 2 {
		t.Error("max(1,2) should be 2")
	}
	if max(5, 3) != 5 {
		t.Error("max(5,3) should be 5")
	}
	if max(0, 0) != 0 {
		t.Error("max(0,0) should be 0")
	}
}
