package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection and provides data access methods.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at the given path.
// If dbPath is empty, the default location ~/.cache/kairos/kairos.db is used.
func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".cache", "kairos", "kairos.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("could not create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("could not enable foreign keys: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("could not run migrations: %w", err)
	}

	return store, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		UNIQUE(name, project_id)
	);

	CREATE TABLE IF NOT EXISTS time_entries (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id        INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		start_at       DATETIME NOT NULL,
		stop_at        DATETIME,
		last_heartbeat DATETIME
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// For existing databases, try to add the column if it doesn't exist
	_, _ = s.db.Exec("ALTER TABLE time_entries ADD COLUMN last_heartbeat DATETIME")

	// Ensure the default "General" project exists.
	_, err := s.db.Exec(`INSERT OR IGNORE INTO projects (name) VALUES ('General')`)
	return err
}
