package usage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Record represents a usage record.
type Record struct {
	ID         int64
	InstanceID string
	Route      string
	Model      string
	Tokens     int
	Fallbacks  int
	Timestamp  time.Time
}

// DBPath returns the path to the usage database.
func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-modelrouter", "usage.db"), nil
}

// InitDB initializes the usage database.
func InitDB(path string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	// Set SQLite pragmas for better performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// SQLite works best with single connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Create table
	query := `
	CREATE TABLE IF NOT EXISTS usage_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		instance_id TEXT NOT NULL,
		route TEXT NOT NULL,
		model TEXT NOT NULL,
		tokens INTEGER NOT NULL,
		fallbacks INTEGER NOT NULL DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_instance_route ON usage_records(instance_id, route);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON usage_records(timestamp);
	`

	if _, err := db.Exec(query); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// dbExecutor is an interface that both *sql.DB and *sql.Tx implement.
type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// InsertRecord inserts a usage record.
func InsertRecord(db dbExecutor, r *Record) error {
	query := `
	INSERT INTO usage_records (instance_id, route, model, tokens, fallbacks, timestamp)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, r.InstanceID, r.Route, r.Model, r.Tokens, r.Fallbacks, r.Timestamp)
	return err
}

// GetRecordsByPeriod retrieves records within a time range, optionally filtered by instance.
func GetRecordsByPeriod(db *sql.DB, instanceID string, start, end time.Time) ([]*Record, error) {
	query := `
	SELECT id, instance_id, route, model, tokens, fallbacks, timestamp
	FROM usage_records
	WHERE timestamp >= ? AND timestamp <= ?
	`
	args := []any{start, end}

	if instanceID != "" {
		query += " AND instance_id = ?"
		args = append(args, instanceID)
	}

	query += " ORDER BY timestamp DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*Record
	for rows.Next() {
		var r Record
		err := rows.Scan(&r.ID, &r.InstanceID, &r.Route, &r.Model, &r.Tokens, &r.Fallbacks, &r.Timestamp)
		if err != nil {
			return nil, err
		}
		records = append(records, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
