package monitor

import (
	"testing"
	"time"
)

func TestNewLogBuffer(t *testing.T) {
	buf := NewLogBuffer(100)
	if buf.Capacity() != 100 {
		t.Errorf("expected capacity 100, got %d", buf.Capacity())
	}
	if buf.Size() != 0 {
		t.Errorf("expected size 0, got %d", buf.Size())
	}
}

func TestLogBuffer_AppendAndGetLines(t *testing.T) {
	buf := NewLogBuffer(5)
	lines := []LogLine{
		{Timestamp: now(), Level: LogLevelInfo, Message: "line 1", Raw: "[INFO] line 1"},
		{Timestamp: now(), Level: LogLevelInfo, Message: "line 2", Raw: "[INFO] line 2"},
		{Timestamp: now(), Level: LogLevelInfo, Message: "line 3", Raw: "[INFO] line 3"},
	}

	for _, l := range lines {
		buf.Append(l)
	}

	if buf.Size() != 3 {
		t.Errorf("expected size 3, got %d", buf.Size())
	}

	result := buf.GetLines()
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
	for i, l := range result {
		if l.Message != lines[i].Message {
			t.Errorf("line[%d]: expected %q, got %q", i, lines[i].Message, l.Message)
		}
	}
}

func TestLogBuffer_RingBufferOverflow(t *testing.T) {
	buf := NewLogBuffer(3)

	for i := 0; i < 5; i++ {
		buf.Append(LogLine{
			Timestamp: now(),
			Level:     LogLevelInfo,
			Message:   "line 5",
			Raw:       "line 5",
		})
	}

	if buf.Size() != 3 {
		t.Errorf("expected size 3 (capacity), got %d", buf.Size())
	}

	result := buf.GetLines()
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
	// Oldest 2 should have been overwritten; only last 3 remain
	for _, l := range result {
		if l.Message != "line 5" {
			t.Errorf("expected all lines to be 'line 5' (last appended), got %q", l.Message)
		}
	}
}

func TestLogBuffer_RingBufferOrder(t *testing.T) {
	buf := NewLogBuffer(3)

	// Append 5 lines with different messages
	messages := []string{"a", "b", "c", "d", "e"}
	for _, msg := range messages {
		buf.Append(LogLine{Timestamp: now(), Level: LogLevelInfo, Message: msg, Raw: msg})
	}

	result := buf.GetLines()
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}

	// Should have c, d, e (oldest a, b overwritten)
	expected := []string{"c", "d", "e"}
	for i, l := range result {
		if l.Message != expected[i] {
			t.Errorf("line[%d]: expected %q, got %q", i, expected[i], l.Message)
		}
	}
}

func TestLogBuffer_GetFilteredLines(t *testing.T) {
	buf := NewLogBuffer(10)
	buf.Append(LogLine{Timestamp: now(), Level: LogLevelInfo, Message: "info msg", Raw: "[INFO] info msg"})
	buf.Append(LogLine{Timestamp: now(), Level: LogLevelError, Message: "error msg", Raw: "[ERROR] error msg"})
	buf.Append(LogLine{Timestamp: now(), Level: LogLevelWarn, Message: "warn msg", Raw: "[WARN] warn msg"})

	// Filter for ERROR only
	result := buf.GetFilteredLines(LogLevelSet(LogLevelError))
	if len(result) != 1 {
		t.Fatalf("expected 1 line with ERROR filter, got %d", len(result))
	}
	if result[0].Message != "error msg" {
		t.Errorf("expected 'error msg', got %q", result[0].Message)
	}

	// Filter for ERROR | WARN
	result2 := buf.GetFilteredLines(LogLevelSet(LogLevelError | LogLevelWarn))
	if len(result2) != 2 {
		t.Fatalf("expected 2 lines with ERROR|WARN filter, got %d", len(result2))
	}

	// Filter for INFO only
	result3 := buf.GetFilteredLines(LogLevelSet(LogLevelInfo))
	if len(result3) != 1 {
		t.Fatalf("expected 1 line with INFO filter, got %d", len(result3))
	}

	// Filter for none
	result4 := buf.GetFilteredLines(LogLevelSet(0))
	if len(result4) != 0 {
		t.Errorf("expected 0 lines with empty filter, got %d", len(result4))
	}
}

func TestLogBuffer_Clear(t *testing.T) {
	buf := NewLogBuffer(10)
	buf.Append(LogLine{Timestamp: now(), Level: LogLevelInfo, Message: "test", Raw: "test"})
	buf.Append(LogLine{Timestamp: now(), Level: LogLevelInfo, Message: "test2", Raw: "test2"})

	if buf.Size() != 2 {
		t.Fatalf("expected size 2 before clear, got %d", buf.Size())
	}

	buf.Clear()

	if buf.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", buf.Size())
	}

	result := buf.GetLines()
	if len(result) != 0 {
		t.Errorf("expected 0 lines after clear, got %d", len(result))
	}
}

func TestLogBuffer_ConcurrentAccess(t *testing.T) {
	buf := NewLogBuffer(1000)
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < 1000; i++ {
			buf.Append(LogLine{Timestamp: now(), Level: LogLevelInfo, Message: "concurrent", Raw: "concurrent"})
		}
		close(done)
	}()

	// Reader goroutine
	for {
		select {
		case <-done:
			return
		default:
			_ = buf.GetLines()
			_ = buf.GetFilteredLines(LogLevelSet(LogLevelInfo))
			_ = buf.Size()
		}
	}
}

// helper
func now() time.Time { return time.Now() }
