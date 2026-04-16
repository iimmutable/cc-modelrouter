package monitor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogTailer tails a log file and sends lines through a channel
type LogTailer struct {
	mu         sync.RWMutex
	LogPath    string
	Filters    LogLevelSet
	BufferSize int
}

// UpdateFilters safely updates log level filters
func (lt *LogTailer) UpdateFilters(filters LogLevelSet) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.Filters = filters
}

// shouldInclude checks if level passes filter
func (lt *LogTailer) shouldInclude(level LogLevel) bool {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	return LogLevelSet(level)&lt.Filters != 0
}

// Run tails the log file
func (lt *LogTailer) Run(ctx context.Context, logChan chan<- string, errChan chan<- error) {
	file, err := os.Open(lt.LogPath)
	if err != nil {
		select {
		case errChan <- fmt.Errorf("failed to open log: %w", err):
		case <-ctx.Done():
		}
		return
	}
	defer file.Close()

	// Track file info for rotation detection
	var initialStat os.FileInfo
	if stat, err := file.Stat(); err == nil {
		initialStat = stat
	}

	// Start reading from near the end (show some context)
	if initialStat != nil {
		seekPos := initialStat.Size() - 20*1024 // ~20KB back ≈ 100 lines
		if seekPos < 0 {
			seekPos = 0
		}
		file.Seek(seekPos, io.SeekStart)

		// Discard partial first line
		reader := bufio.NewReader(file)
		reader.ReadString('\n')
	}

	reader := bufio.NewReader(file)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check for rotation
			if currentStat, err := os.Stat(lt.LogPath); err == nil {
				if initialStat != nil && !os.SameFile(initialStat, currentStat) {
					// File rotated - reopen
					file.Close()

					newFile, err := os.Open(lt.LogPath)
					if err != nil {
						select {
						case errChan <- fmt.Errorf("log rotated, reopen failed: %w", err):
						case <-ctx.Done():
							return
						}
						return
					}

					file = newFile
					reader = bufio.NewReader(file)
					initialStat, _ = file.Stat()
				}
			} else if os.IsNotExist(err) {
				// Log file deleted - wait for recreation
				select {
				case errChan <- fmt.Errorf("log file deleted, waiting"):
				case <-ctx.Done():
					return
				}
				time.Sleep(1 * time.Second)
				continue
			}

			// Read available lines
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break // No more data
					}
					select {
					case errChan <- fmt.Errorf("log read error: %w", err):
					case <-ctx.Done():
						return
					}
					break
				}

				// Parse and filter
				parsedLine := parseLogLineRaw(line)
				if parsedLine != nil && lt.shouldInclude(parsedLine.Level) {
					select {
					case logChan <- line:
					case <-ctx.Done():
						return
					default:
						// Backpressure: drop line
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// parseLogLineRaw parses a log line and returns LogLine
func parseLogLineRaw(raw string) *LogLine {
	// Expected format: [TIMESTAMP] [LEVEL] message
	parts := strings.SplitN(raw, "]", 3)
	if len(parts) < 3 {
		return &LogLine{
			Timestamp: time.Now(),
			Level:     LogLevelInfo,
			Message:   strings.TrimSpace(raw),
			Raw:       raw,
		}
	}

	// Parse timestamp
	ts := strings.TrimPrefix(parts[0], "[")
	timestamp, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		timestamp = time.Now()
	}

	// Parse level
	levelStr := strings.TrimPrefix(strings.TrimSpace(parts[1]), "[")
	level := parseLogLevel(levelStr)

	// Message
	message := strings.TrimSpace(parts[2])

	return &LogLine{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Raw:       raw,
	}
}