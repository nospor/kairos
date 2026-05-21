package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestHeartbeatAndCleanup(t *testing.T) {
	// Create a new in-memory store
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create a task
	err = store.CreateTask("Test Task", "General")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Start tracking
	err = store.StartTask("Test Task", "General")
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	// Get active task
	active, err := store.GetActiveTask()
	if err != nil {
		t.Fatalf("failed to get active task: %v", err)
	}
	if active == nil {
		t.Fatal("expected active task, got nil")
	}

	// Update heartbeat
	err = store.UpdateActiveHeartbeat()
	if err != nil {
		t.Fatalf("failed to update heartbeat: %v", err)
	}

	// Check if heartbeat is updated in the DB
	var lastHeartbeat sql.NullInt64
	err = store.db.QueryRow("SELECT last_heartbeat FROM time_entries WHERE stop_at IS NULL").Scan(&lastHeartbeat)
	if err != nil {
		t.Fatalf("failed to query last_heartbeat: %v", err)
	}
	if !lastHeartbeat.Valid {
		t.Fatal("expected last_heartbeat to be valid")
	}

	// Test auto-stop with a threshold of 0 seconds (should instantly trigger cleanup)
	stopped, err := store.AutoStopStaleTasks(0 * time.Second)
	if err != nil {
		t.Fatalf("failed to run AutoStopStaleTasks: %v", err)
	}
	if stopped != 1 {
		t.Errorf("expected 1 stopped task, got %d", stopped)
	}

	// Verify task is no longer active
	active, err = store.GetActiveTask()
	if err != nil {
		t.Fatalf("failed to get active task: %v", err)
	}
	if active != nil {
		t.Errorf("expected no active task, got %v", active)
	}

	// Verify the stop_at time matches the last heartbeat
	var stopAt sql.NullInt64
	err = store.db.QueryRow("SELECT stop_at FROM time_entries LIMIT 1").Scan(&stopAt)
	if err != nil {
		t.Fatalf("failed to query stop_at: %v", err)
	}
	if !stopAt.Valid {
		t.Fatal("expected stop_at to be valid")
	}
	if stopAt.Int64 != lastHeartbeat.Int64 {
		t.Errorf("expected stop_at (%d) to equal last_heartbeat (%d)", stopAt.Int64, lastHeartbeat.Int64)
	}
}
