package usage

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

const (
	DefaultBufferSize    = 500
	DefaultFlushTimeout = 3 * time.Second
)

// Tracker tracks usage records in memory and flushes to database.
type Tracker struct {
	db           *sql.DB
	buffer       []*Record
	mu           sync.Mutex
	bufferSize   int
	flushTimeout time.Duration
	done         chan struct{}
	wg           sync.WaitGroup
}

// NewTracker creates a new usage tracker.
func NewTracker(db *sql.DB, bufferSize int, flushTimeout time.Duration) *Tracker {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	if flushTimeout <= 0 {
		flushTimeout = DefaultFlushTimeout
	}

	t := &Tracker{
		db:           db,
		buffer:       make([]*Record, 0, bufferSize),
		bufferSize:   bufferSize,
		flushTimeout: flushTimeout,
		done:         make(chan struct{}),
	}

	t.wg.Add(1)
	go t.flushLoop()

	return t
}

// Record adds a usage record.
func (t *Tracker) Record(instanceID, route, model string, tokens, fallbacks int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r := &Record{
		InstanceID: instanceID,
		Route:      route,
		Model:      model,
		Tokens:     tokens,
		Fallbacks:  fallbacks,
		Timestamp:  time.Now(),
	}

	t.buffer = append(t.buffer, r)

	if len(t.buffer) >= t.bufferSize {
		t.flush()
	}
}

// flush writes buffered records to database.
// Must be called with mu held.
func (t *Tracker) flush() {
	if len(t.buffer) == 0 {
		return
	}

	// Copy buffer and clear
	records := make([]*Record, len(t.buffer))
	copy(records, t.buffer)
	t.buffer = t.buffer[:0]

	// Batch insert
	tx, err := t.db.Begin()
	if err != nil {
		log.Printf("usage tracker: failed to begin transaction: %v", err)
		// Restore records to buffer on error
		t.buffer = append(t.buffer, records...)
		return
	}
	defer tx.Rollback()

	for i, r := range records {
		if err := InsertRecord(tx, r); err != nil {
			log.Printf("usage tracker: failed to insert record %d: %v", i, err)
			// Restore unprocessed records to buffer
			t.buffer = append(t.buffer, records[i:]...)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("usage tracker: failed to commit transaction: %v", err)
		// Restore records to buffer on commit failure
		t.buffer = append(t.buffer, records...)
		return
	}
}

// flushLoop periodically flushes the buffer.
func (t *Tracker) flushLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			t.flush()
			t.mu.Unlock()
		case <-t.done:
			return
		}
	}
}

// Shutdown flushes remaining records and stops the tracker.
func (t *Tracker) Shutdown() {
	close(t.done)
	t.wg.Wait()

	t.mu.Lock()
	t.flush()
	t.mu.Unlock()
}
