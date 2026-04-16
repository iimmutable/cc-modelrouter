package monitor

import (
	"sync"
)

// LogBuffer is a thread-safe ring buffer for console logs
type LogBuffer struct {
	lines    []LogLine
	capacity int
	head     int
	size     int
	mu       sync.RWMutex
}

// NewLogBuffer creates a new log buffer with the given capacity
func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		lines:    make([]LogLine, 0, capacity),
		capacity: capacity,
	}
}

// Append adds a new log line to the buffer
func (lb *LogBuffer) Append(line LogLine) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.size < lb.capacity {
		lb.lines = append(lb.lines, line)
		lb.size++
	} else {
		// Ring buffer: overwrite oldest
		lb.lines[lb.head] = line
		lb.head = (lb.head + 1) % lb.capacity
	}
}

// GetLines returns all log lines in order
func (lb *LogBuffer) GetLines() []LogLine {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if lb.size < lb.capacity {
		// Not full yet
		result := make([]LogLine, lb.size)
		copy(result, lb.lines)
		return result
	}

	// Ring buffer full: return in correct order
	result := make([]LogLine, lb.capacity)
	for i := 0; i < lb.capacity; i++ {
		result[i] = lb.lines[(lb.head+i)%lb.capacity]
	}
	return result
}

// GetFilteredLines returns log lines that match the given filters
func (lb *LogBuffer) GetFilteredLines(filters LogLevelSet) []LogLine {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var result []LogLine
	lines := lb.GetLinesUnsafe()

	for _, line := range lines {
		if LogLevelSet(line.Level)&filters != 0 {
			result = append(result, line)
		}
	}

	return result
}

// GetLinesUnsafe returns lines without locking (caller must hold lock)
func (lb *LogBuffer) GetLinesUnsafe() []LogLine {
	if lb.size < lb.capacity {
		result := make([]LogLine, lb.size)
		copy(result, lb.lines)
		return result
	}

	result := make([]LogLine, lb.capacity)
	for i := 0; i < lb.capacity; i++ {
		result[i] = lb.lines[(lb.head+i)%lb.capacity]
	}
	return result
}

// Clear resets the buffer
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.lines = make([]LogLine, 0, lb.capacity)
	lb.head = 0
	lb.size = 0
}

// Size returns the current number of lines in the buffer
func (lb *LogBuffer) Size() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.size
}

// Capacity returns the maximum capacity
func (lb *LogBuffer) Capacity() int {
	return lb.capacity
}