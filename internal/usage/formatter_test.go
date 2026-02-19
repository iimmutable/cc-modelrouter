package usage

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		tokens int
		want   string
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

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1000000, "1,000,000"},
		{1234567890, "1,234,567,890"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatNumber(tt.n); got != tt.want {
				t.Errorf("formatNumber(%d) = %s, want %s", tt.n, got, tt.want)
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
		"Requests",
		"Tokens",
		"Fallbacks",
		"By Route",
		"By Model",
	}

	for _, check := range checks {
		if !contains(output, check) {
			t.Errorf("output missing expected text: %s\noutput:\n%s", check, output)
		}
	}

	// Verify specific values are present in output
	expectedValues := []string{
		"3",       // Total requests
		"45.6M",   // Total tokens (25M + 15M + 5.6M)
		"3",       // Total fallbacks
		"/think",  // Route name
		"2",       // /think requests count
		"40.0M",   // /think tokens (25M + 15M)
		"3",       // /think fallbacks
		"/ultrathink", // Route name
		"1",       // /ultrathink requests
		"5.6M",    // /ultrathink tokens
		"0",       // /ultrathink fallbacks
	}

	for _, val := range expectedValues {
		if !contains(output, val) {
			t.Errorf("output missing expected value: %s\noutput:\n%s", val, output)
		}
	}

	// Verify models are sorted by tokens (descending)
	// Find gpt-4o position (should be first with 25M)
	gptPos := findSubstring(output, "gpt-4o")
	sonnetPos := findSubstring(output, "claude-sonnet-4")
	geminiPos := findSubstring(output, "gemini-2.0-flash")

	if gptPos == -1 || sonnetPos == -1 || geminiPos == -1 {
		t.Fatalf("could not find all models in output")
	}

	// Models should be sorted by tokens descending: gpt-4o (25M) > claude-sonnet-4 (15M) > gemini-2.0-flash (5.6M)
	// But sorting is done in the "By Model" section, so verify they appear in the correct order
	// in the "By Model" section specifically
	byModelIdx := findSubstring(output, "By Model:")
	if byModelIdx == -1 {
		t.Fatal("could not find 'By Model:' section")
	}
	byModelSection := output[byModelIdx:]

	gptPosModel := findSubstring(byModelSection, "gpt-4o")
	sonnetPosModel := findSubstring(byModelSection, "claude-sonnet-4")
	geminiPosModel := findSubstring(byModelSection, "gemini-2.0-flash")

	if gptPosModel == -1 || sonnetPosModel == -1 || geminiPosModel == -1 {
		t.Fatalf("could not find all models in 'By Model' section")
	}

	if gptPosModel > sonnetPosModel || sonnetPosModel > geminiPosModel {
		t.Errorf("models not sorted correctly by tokens (descending) in 'By Model' section.\ngpt-4o pos: %d\nclaude-sonnet-4 pos: %d\ngemini-2.0-flash pos: %d\noutput:\n%s",
			gptPosModel, sonnetPosModel, geminiPosModel, byModelSection)
	}
}

func contains(s, substr string) bool {
	return findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
