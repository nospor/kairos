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

// New opens (or creates) the SQLite database at ~/.cache/kairos/kairos.db,
// runs migrations, and returns a ready-to-use Store.
func New() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	dbDir := filepath.Join(homeDir, ".cache", "kairos")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create database directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "kairos.db")
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
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id  INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		start_at DATETIME NOT NULL,
		stop_at  DATETIME
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Ensure the default "General" project exists.
	_, err := s.db.Exec(`INSERT OR IGNORE INTO projects (name) VALUES ('General')`)
	return err
}
