package cli

import (
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single line no newline",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "single line with trailing newline",
			input: "hello\n",
			want:  []string{"hello"},
		},
		{
			name:  "two lines",
			input: "line1\nline2",
			want:  []string{"line1", "line2"},
		},
		{
			name:  "two lines with trailing newline",
			input: "line1\nline2\n",
			want:  []string{"line1", "line2"},
		},
		{
			name:  "three lines",
			input: "a\nb\nc",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "newline only",
			input: "\n",
			want:  []string{""},
		},
		{
			name:  "multiple consecutive newlines",
			input: "a\n\nb",
			want:  []string{"a", "", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitLines(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no whitespace",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "leading spaces",
			input: "  hello",
			want:  "hello",
		},
		{
			name:  "trailing spaces",
			input: "hello  ",
			want:  "hello",
		},
		{
			name:  "leading and trailing spaces",
			input: "  hello  ",
			want:  "hello",
		},
		{
			name:  "tabs",
			input: "\thello\t",
			want:  "hello",
		},
		{
			name:  "carriage returns",
			input: "\rhello\r",
			want:  "hello",
		},
		{
			name:  "mixed whitespace",
			input: " \t\r hello \t\r ",
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   \t  ",
			want:  "",
		},
		{
			name:  "internal spaces preserved",
			input: "  hello world  ",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimSpace(tt.input)
			if got != tt.want {
				t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewConfigCommand(t *testing.T) {
	cmd := NewConfigCommand()

	if cmd.Use != "config" {
		t.Errorf("expected Use %q, got %q", "config", cmd.Use)
	}

	// Verify --shell-export flag exists
	f := cmd.Flags().Lookup("shell-export")
	if f == nil {
		t.Fatal("expected --shell-export flag to exist")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --shell-export default %q, got %q", "false", f.DefValue)
	}
}
