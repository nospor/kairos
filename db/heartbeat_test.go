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

func TestLogTimeEntry(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Log with non-existent project (should fail)
	now := time.Now()
	err = store.LogTimeEntry("New Task", "NonExistent", now.Add(-1*time.Hour), now)
	if err == nil {
		t.Error("expected error logging time for non-existent project, got nil")
	}

	// 2. Log with default project (should succeed and create task)
	start := now.Add(-2 * time.Hour)
	end := now.Add(-1 * time.Hour)
	err = store.LogTimeEntry("Manual Task", "General", start, end)
	if err != nil {
		t.Fatalf("failed to log time entry: %v", err)
	}

	// Verify task was created
	var taskCount int
	err = store.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE name = 'Manual Task'").Scan(&taskCount)
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}
	if taskCount != 1 {
		t.Errorf("expected 1 task created, got %d", taskCount)
	}

	// Verify time entry was created
	var startUnix, stopUnix int64
	err = store.db.QueryRow("SELECT start_at, stop_at FROM time_entries WHERE task_id = (SELECT id FROM tasks WHERE name = 'Manual Task')").Scan(&startUnix, &stopUnix)
	if err != nil {
		t.Fatalf("failed to query time entry: %v", err)
	}
	if startUnix != start.Unix() || stopUnix != end.Unix() {
		t.Errorf("expected start %d and stop %d, got start %d and stop %d", start.Unix(), end.Unix(), startUnix, stopUnix)
	}
}

func TestDeleteTimeEntry(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	err = store.LogTimeEntry("Delete Me Task", "General", now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("failed to log time entry: %v", err)
	}

	// Get history to retrieve the ID
	history, err := store.GetHistory(0)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	entryID := history[0].ID

	// Delete the entry
	err = store.DeleteTimeEntry(entryID)
	if err != nil {
		t.Fatalf("failed to delete time entry: %v", err)
	}

	// Verify history is empty
	history, err = store.GetHistory(0)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 history entries after deletion, got %d", len(history))
	}

	// Try deleting a non-existent ID (should fail)
	err = store.DeleteTimeEntry(9999)
	if err == nil {
		t.Error("expected error deleting non-existent time entry, got nil")
	}
}

