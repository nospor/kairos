package db

import (
	"database/sql"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/nospor/kairos/model"
)

// ReportFilter defines optional filtering for report and export commands.
type ReportFilter struct {
	ProjectName string
	From        *time.Time
	To          *time.Time
	GroupBy     string
}

// CreateProject creates a new project with the given name.
func (s *Store) CreateProject(name string) error {
	_, err := s.db.Exec("INSERT INTO projects (name) VALUES (?)", name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("project %q already exists", name)
		}
		return fmt.Errorf("could not create project: %w", err)
	}
	return nil
}

// DeleteProject deletes a project and all its tasks/time entries (via CASCADE).
// The default "General" project cannot be deleted.
func (s *Store) DeleteProject(name string) error {
	if name == "General" {
		return fmt.Errorf("cannot delete the default \"General\" project")
	}

	// Check for active time entries in this project
	var activeCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM time_entries te
		JOIN tasks t ON te.task_id = t.id
		JOIN projects p ON t.project_id = p.id
		WHERE p.name = ? AND te.stop_at IS NULL
	`, name).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("could not check active entries: %w", err)
	}
	if activeCount > 0 {
		return fmt.Errorf("project %q has an active time entry; stop it first", name)
	}

	result, err := s.db.Exec("DELETE FROM projects WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("could not delete project: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project %q not found", name)
	}
	return nil
}

// CreateTask creates a new task under the specified project.
// If projectName is empty, the task is assigned to "General".
func (s *Store) CreateTask(name, projectName string) error {
	if projectName == "" {
		projectName = "General"
	}

	var projectID int
	err := s.db.QueryRow("SELECT id FROM projects WHERE name = ?", projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("project %q not found", projectName)
	}
	if err != nil {
		return fmt.Errorf("could not find project: %w", err)
	}

	_, err = s.db.Exec("INSERT INTO tasks (name, project_id) VALUES (?, ?)", name, projectID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("task %q already exists in project %q", name, projectName)
		}
		return fmt.Errorf("could not create task: %w", err)
	}
	return nil
}

// DeleteTask deletes a task and all its time entries (via CASCADE).
// Returns an error if the task is currently being tracked.
func (s *Store) DeleteTask(name string) error {
	var activeCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM time_entries te
		JOIN tasks t ON te.task_id = t.id
		WHERE t.name = ? AND te.stop_at IS NULL
	`, name).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("could not check active tracking: %w", err)
	}
	if activeCount > 0 {
		return fmt.Errorf("task %q is currently being tracked; stop it first", name)
	}

	result, err := s.db.Exec("DELETE FROM tasks WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("could not delete task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %q not found", name)
	}
	return nil
}

// StartTask begins tracking time for the named task within the given project.
// If projectName is empty, "General" is used. Only one task may be active at a time.
func (s *Store) StartTask(name, projectName string) error {
	if projectName == "" {
		projectName = "General"
	}

	active, err := s.GetActiveTask()
	if err != nil {
		return err
	}
	if active != nil {
		return fmt.Errorf("task %q is already being tracked; stop it first", active.TaskName)
	}

	var taskID int
	err = s.db.QueryRow(`
		SELECT t.id FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.name = ? AND p.name = ?
	`, name, projectName).Scan(&taskID)
	if err == sql.ErrNoRows {
		// Give a more helpful hint if the task exists in a different project.
		var otherProject string
		_ = s.db.QueryRow(`
			SELECT p.name FROM tasks t
			JOIN projects p ON t.project_id = p.id
			WHERE t.name = ? LIMIT 1
		`, name).Scan(&otherProject)
		if otherProject != "" {
			return fmt.Errorf("task %q not found in project %q (it exists in project %q — use -p %q)", name, projectName, otherProject, otherProject)
		}
		return fmt.Errorf("task %q not found in project %q; create it first with 'kairos create %q -p %q'", name, projectName, name, projectName)
	}
	if err != nil {
		return fmt.Errorf("could not find task: %w", err)
	}

	now := time.Now().Unix()
	_, err = s.db.Exec(
		"INSERT INTO time_entries (task_id, start_at, last_heartbeat) VALUES (?, ?, ?)",
		taskID, now, now,
	)
	if err != nil {
		return fmt.Errorf("could not start tracking: %w", err)
	}
	return nil
}

// StopActive stops the currently active time entry.
// Returns the task name and the elapsed duration.
func (s *Store) StopActive() (string, time.Duration, error) {
	active, err := s.GetActiveTask()
	if err != nil {
		return "", 0, err
	}
	if active == nil {
		return "", 0, fmt.Errorf("no task is currently being tracked")
	}

	now := time.Now().Unix()
	_, err = s.db.Exec("UPDATE time_entries SET stop_at = ? WHERE stop_at IS NULL", now)
	if err != nil {
		return "", 0, fmt.Errorf("could not stop tracking: %w", err)
	}

	duration := time.Duration(now-active.StartedAt.Unix()) * time.Second
	return active.TaskName, duration, nil
}

// GetActiveTask returns information about the currently running task, or nil.
func (s *Store) GetActiveTask() (*model.ActiveInfo, error) {
	var info model.ActiveInfo
	var startUnix int64
	err := s.db.QueryRow(`
		SELECT t.name, p.name, te.start_at
		FROM time_entries te
		JOIN tasks t ON te.task_id = t.id
		JOIN projects p ON t.project_id = p.id
		WHERE te.stop_at IS NULL
		LIMIT 1
	`).Scan(&info.TaskName, &info.ProjectName, &startUnix)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not check active task: %w", err)
	}
	info.StartedAt = time.Unix(startUnix, 0)
	return &info, nil
}

// ListTasks returns all tasks with their project name and total tracked duration.
func (s *Store) ListTasks() ([]model.TaskInfo, error) {
	rows, err := s.db.Query(`
		SELECT t.name, p.name, COALESCE(SUM(
			CASE WHEN te.stop_at IS NOT NULL THEN
				te.stop_at - te.start_at
			ELSE
				CAST(strftime('%s', 'now') AS INTEGER) - te.start_at
			END
		), 0) as total_seconds
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		LEFT JOIN time_entries te ON te.task_id = t.id
		GROUP BY t.id, t.name, p.name
		ORDER BY MAX(te.start_at) DESC NULLS LAST, t.name
	`)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.TaskInfo
	for rows.Next() {
		var t model.TaskInfo
		var totalSeconds int64
		if err := rows.Scan(&t.TaskName, &t.ProjectName, &totalSeconds); err != nil {
			return nil, fmt.Errorf("could not scan task: %w", err)
		}
		t.TotalDuration = time.Duration(totalSeconds) * time.Second
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListProjects returns all projects with their tasks and durations.
func (s *Store) ListProjects() ([]model.ProjectInfo, error) {
	// We need the max start_at per project for ordering, computed in a subquery.
	rows, err := s.db.Query(`
		SELECT p.name, COALESCE(t.name, ''), COALESCE(SUM(
			CASE WHEN te.stop_at IS NOT NULL THEN
				te.stop_at - te.start_at
			ELSE
				CAST(strftime('%s', 'now') AS INTEGER) - te.start_at
			END
		), 0) as total_seconds,
		(SELECT MAX(te2.start_at) FROM time_entries te2
		 JOIN tasks t2 ON te2.task_id = t2.id
		 WHERE t2.project_id = p.id) as last_used
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		LEFT JOIN time_entries te ON te.task_id = t.id
		GROUP BY p.id, p.name, t.id, t.name
		ORDER BY last_used DESC NULLS LAST, p.name, MAX(te.start_at) DESC NULLS LAST, t.name
	`)
	if err != nil {
		return nil, fmt.Errorf("could not list projects: %w", err)
	}
	defer rows.Close()

	projectMap := make(map[string]*model.ProjectInfo)
	var projectOrder []string

	for rows.Next() {
		var projectName, taskName string
		var totalSeconds int64
		var lastUsed interface{} // consumed for ordering only
		if err := rows.Scan(&projectName, &taskName, &totalSeconds, &lastUsed); err != nil {
			return nil, fmt.Errorf("could not scan row: %w", err)
		}

		pi, exists := projectMap[projectName]
		if !exists {
			pi = &model.ProjectInfo{ProjectName: projectName}
			projectMap[projectName] = pi
			projectOrder = append(projectOrder, projectName)
		}

		if taskName != "" {
			pi.Tasks = append(pi.Tasks, model.TaskInfo{
				TaskName:      taskName,
				ProjectName:   projectName,
				TotalDuration: time.Duration(totalSeconds) * time.Second,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []model.ProjectInfo
	for _, name := range projectOrder {
		result = append(result, *projectMap[name])
	}
	return result, nil
}

// GetReport returns aggregated report rows, optionally filtered by project and/or date range.
func (s *Store) GetReport(filter ReportFilter) ([]model.ReportRow, error) {
	var dateExpr string
	switch filter.GroupBy {
	case "day":
		dateExpr = "strftime('%Y-%m-%d', datetime(te.start_at, 'unixepoch', 'localtime'))"
	case "week":
		dateExpr = "strftime('%Y', date(datetime(te.start_at, 'unixepoch', 'localtime'), '-3 days', 'weekday 4')) || '-' || printf('%02d', (strftime('%j', date(datetime(te.start_at, 'unixepoch', 'localtime'), '-3 days', 'weekday 4')) - 1) / 7 + 1)"
	case "month":
		dateExpr = "strftime('%Y-%m', datetime(te.start_at, 'unixepoch', 'localtime'))"
	case "year":
		dateExpr = "strftime('%Y', datetime(te.start_at, 'unixepoch', 'localtime'))"
	default:
		dateExpr = "NULL"
	}

	query := fmt.Sprintf(`
		SELECT p.name, t.name, COALESCE(SUM(
			CASE WHEN te.stop_at IS NOT NULL THEN
				te.stop_at - te.start_at
			ELSE
				CAST(strftime('%%s', 'now') AS INTEGER) - te.start_at
			END
		), 0) as total_seconds, COALESCE(%s, '') as date
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		LEFT JOIN time_entries te ON te.task_id = t.id
	`, dateExpr)

	var conditions []string
	var args []interface{}

	if filter.ProjectName != "" {
		conditions = append(conditions, "p.name = ?")
		args = append(args, filter.ProjectName)
	}
	if filter.From != nil {
		conditions = append(conditions, "te.start_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if filter.To != nil {
		conditions = append(conditions, "(te.start_at <= ?)")
		args = append(args, filter.To.Unix())
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	groupByStr := "p.name, t.name"
	if filter.GroupBy != "" {
		groupByStr += ", date"
	}

	orderByStr := "p.name, t.name"
	if filter.GroupBy != "" {
		orderByStr += ", date"
	}

	query += " GROUP BY " + groupByStr + " ORDER BY " + orderByStr

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not generate report: %w", err)
	}
	defer rows.Close()

	var report []model.ReportRow
	for rows.Next() {
		var r model.ReportRow
		var totalSeconds int64
		if err := rows.Scan(&r.ProjectName, &r.TaskName, &totalSeconds, &r.Date); err != nil {
			return nil, fmt.Errorf("could not scan row: %w", err)
		}
		r.Duration = time.Duration(totalSeconds) * time.Second
		report = append(report, r)
	}
	return report, rows.Err()
}

// ResetAll deletes all data and re-creates the default "General" project.
func (s *Store) ResetAll() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, table := range []string{"time_entries", "tasks", "projects"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("could not clear %s: %w", table, err)
		}
	}

	// Re-create the General project
	if _, err := tx.Exec("INSERT INTO projects (name) VALUES ('General')"); err != nil {
		return fmt.Errorf("could not re-create General project: %w", err)
	}

	return tx.Commit()
}

// RenameProject renames an existing project. The "General" project cannot be renamed.
func (s *Store) RenameProject(oldName, newName string) error {
	if oldName == "General" {
		return fmt.Errorf("cannot rename the default \"General\" project")
	}
	if newName == "" {
		return fmt.Errorf("new project name cannot be empty")
	}

	result, err := s.db.Exec("UPDATE projects SET name = ? WHERE name = ?", newName, oldName)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("project %q already exists", newName)
		}
		return fmt.Errorf("could not rename project: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project %q not found", oldName)
	}
	return nil
}

// RenameTask renames an existing task within the specified project.
// If projectName is empty, the task is looked up in the "General" project.
func (s *Store) RenameTask(oldName, newName, projectName string) error {
	if projectName == "" {
		projectName = "General"
	}
	if newName == "" {
		return fmt.Errorf("new task name cannot be empty")
	}

	result, err := s.db.Exec(`
		UPDATE tasks SET name = ?
		WHERE name = ?
		AND project_id = (SELECT id FROM projects WHERE name = ?)
	`, newName, oldName, projectName)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("task %q already exists in project %q", newName, projectName)
		}
		return fmt.Errorf("could not rename task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Distinguish between "project not found" and "task not found"
		var projectExists int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE name = ?", projectName).Scan(&projectExists)
		if projectExists == 0 {
			return fmt.Errorf("project %q not found", projectName)
		}
		return fmt.Errorf("task %q not found in project %q", oldName, projectName)
	}
	return nil
}

// UpdateActiveHeartbeat updates the last_heartbeat timestamp of the currently active task.
func (s *Store) UpdateActiveHeartbeat() error {
	_, err := s.db.Exec("UPDATE time_entries SET last_heartbeat = ? WHERE stop_at IS NULL", time.Now().Unix())
	if err != nil {
		return fmt.Errorf("could not update heartbeat: %w", err)
	}
	return nil
}

// AutoStopStaleTasks checks for any active time entry whose last_heartbeat is stale (older than threshold).
// It stops them at the last_heartbeat time (or start_at if last_heartbeat is NULL).
func (s *Store) AutoStopStaleTasks(threshold time.Duration) (int, error) {
	rows, err := s.db.Query("SELECT id, start_at, last_heartbeat FROM time_entries WHERE stop_at IS NULL")
	if err != nil {
		return 0, fmt.Errorf("could not query active entries for cleanup: %w", err)
	}
	defer rows.Close()

	type staleEntry struct {
		id            int
		startAt       int64
		lastHeartbeat *int64
	}

	var stale []staleEntry
	now := time.Now().Unix()

	for rows.Next() {
		var id int
		var startAt int64
		var lastHeartbeat sql.NullInt64
		if err := rows.Scan(&id, &startAt, &lastHeartbeat); err != nil {
			return 0, fmt.Errorf("could not scan active entry: %w", err)
		}

		isStale := false
		if lastHeartbeat.Valid {
			if now-lastHeartbeat.Int64 >= int64(threshold.Seconds()) {
				isStale = true
			}
		} else {
			if now-startAt >= int64(threshold.Seconds()) {
				isStale = true
			}
		}

		if isStale {
			var lh *int64
			if lastHeartbeat.Valid {
				val := lastHeartbeat.Int64
				lh = &val
			}
			stale = append(stale, staleEntry{
				id:            id,
				startAt:       startAt,
				lastHeartbeat: lh,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	stoppedCount := 0
	for _, entry := range stale {
		var stopTime int64
		if entry.lastHeartbeat != nil && *entry.lastHeartbeat >= entry.startAt {
			stopTime = *entry.lastHeartbeat
		} else {
			stopTime = entry.startAt
		}

		_, err := s.db.Exec("UPDATE time_entries SET stop_at = ? WHERE id = ?", stopTime, entry.id)
		if err != nil {
			return stoppedCount, fmt.Errorf("could not stop stale entry %d: %w", entry.id, err)
		}
		stoppedCount++
	}

	return stoppedCount, nil
}

// RunDaemon runs a loop that updates the active task's heartbeat periodically.
// If it detects a system sleep or shutdown (i.e. ticker interval exceeded), it stops the active task.
// If notifyMinutes > 0, it also sends desktop notifications every notifyMinutes.
func (s *Store) RunDaemon(notifyMinutes int) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	lastNotifiedMinutes := 0

	for {
		select {
		case <-ticker.C:
			active, err := s.GetActiveTask()
			if err != nil {
				return err
			}
			if active == nil {
				return nil
			}

			var lastHeartbeat sql.NullInt64
			err = s.db.QueryRow("SELECT last_heartbeat FROM time_entries WHERE stop_at IS NULL").Scan(&lastHeartbeat)
			if err != nil {
				return err
			}

			if lastHeartbeat.Valid {
				timeSinceLastHeartbeat := time.Now().Unix() - lastHeartbeat.Int64
				if timeSinceLastHeartbeat > 120 { // 2 minutes
					_, err = s.db.Exec("UPDATE time_entries SET stop_at = ? WHERE stop_at IS NULL", lastHeartbeat.Int64)
					return err
				}
			}

			err = s.UpdateActiveHeartbeat()
			if err != nil {
				return err
			}

			if notifyMinutes > 0 {
				elapsed := time.Since(active.StartedAt)
				elapsedMinutes := int(elapsed.Minutes())
				if elapsedMinutes > 0 && elapsedMinutes%notifyMinutes == 0 && elapsedMinutes != lastNotifiedMinutes {
					lastNotifiedMinutes = elapsedMinutes
					title := "Kairos Time Tracker"
					message := fmt.Sprintf("Still tracking task %q in project %q\nElapsed: %s", active.TaskName, active.ProjectName, formatDuration(elapsed))
					sendNotification(title, message)
				}
			}
		}
	}
}

func sendNotification(title, message string) {
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("notify-send", title, message).Run()
	case "darwin":
		escapedMsg := strings.ReplaceAll(message, `"`, `\"`)
		escapedTitle := strings.ReplaceAll(title, `"`, `\"`)
		script := fmt.Sprintf("display notification %q with title %q", escapedMsg, escapedTitle)
		_ = exec.Command("osascript", "-e", script).Run()
	case "windows":
		escapedMsg := strings.ReplaceAll(message, `"`, "`\"")
		escapedTitle := strings.ReplaceAll(title, `"`, "`\"")
		psCmd := fmt.Sprintf(`$wshell = New-Object -ComObject Wscript.Shell; $wshell.Popup("%s", 0, "%s", 64)`, escapedMsg, escapedTitle)
		_ = exec.Command("powershell", "-Command", psCmd).Run()
	}
}

func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// GetHistoryFiltered returns individual time entries in reverse chronological order, filtered by minimum duration, and optionally limited.
func (s *Store) GetHistoryFiltered(limit int, minDuration time.Duration) ([]model.HistoryEntry, error) {
	query := `
		SELECT te.id, p.name, t.name, te.start_at, te.stop_at
		FROM time_entries te
		JOIN tasks t ON te.task_id = t.id
		JOIN projects p ON t.project_id = p.id
	`
	var args []interface{}
	if minDuration > 0 {
		minSec := int64(minDuration.Seconds())
		nowSec := time.Now().Unix()
		query += ` WHERE (
			(te.stop_at IS NOT NULL AND (te.stop_at - te.start_at) >= ?) OR
			(te.stop_at IS NULL AND (? - te.start_at) >= ?)
		)`
		args = append(args, minSec, nowSec, minSec)
	}

	query += " ORDER BY te.start_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query history: %w", err)
	}
	defer rows.Close()

	var history []model.HistoryEntry
	for rows.Next() {
		var h model.HistoryEntry
		var startUnix int64
		var stopUnix sql.NullInt64

		if err := rows.Scan(&h.ID, &h.ProjectName, &h.TaskName, &startUnix, &stopUnix); err != nil {
			return nil, fmt.Errorf("could not scan history row: %w", err)
		}

		h.StartAt = time.Unix(startUnix, 0)
		if stopUnix.Valid {
			stopTime := time.Unix(stopUnix.Int64, 0)
			h.StopAt = &stopTime
			h.Duration = stopTime.Sub(h.StartAt)
		} else {
			h.Duration = time.Since(h.StartAt)
		}

		history = append(history, h)
	}
	return history, rows.Err()
}

// GetHistory returns individual time entries in reverse chronological order, optionally limited.
func (s *Store) GetHistory(limit int) ([]model.HistoryEntry, error) {
	return s.GetHistoryFiltered(limit, 0)
}

// LogTimeEntry manually inserts a time entry.
// It resolves the task (creating it if it doesn't exist under the given project).
func (s *Store) LogTimeEntry(taskName, projectName string, startAt, stopAt time.Time) error {
	if projectName == "" {
		projectName = "General"
	}

	// Verify project exists
	var projectID int
	err := s.db.QueryRow("SELECT id FROM projects WHERE name = ?", projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("project %q not found", projectName)
	}
	if err != nil {
		return fmt.Errorf("could not find project: %w", err)
	}

	// Find or create task
	var taskID int
	err = s.db.QueryRow("SELECT id FROM tasks WHERE name = ? AND project_id = ?", taskName, projectID).Scan(&taskID)
	if err == sql.ErrNoRows {
		// Create task
		res, err := s.db.Exec("INSERT INTO tasks (name, project_id) VALUES (?, ?)", taskName, projectID)
		if err != nil {
			return fmt.Errorf("could not create task %q: %w", taskName, err)
		}
		lastID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("could not get created task ID: %w", err)
		}
		taskID = int(lastID)
	} else if err != nil {
		return fmt.Errorf("could not lookup task: %w", err)
	}

	// Insert time entry
	_, err = s.db.Exec(
		"INSERT INTO time_entries (task_id, start_at, stop_at, last_heartbeat) VALUES (?, ?, ?, ?)",
		taskID, startAt.Unix(), stopAt.Unix(), stopAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("could not log time entry: %w", err)
	}
	return nil
}

// DeleteTimeEntry deletes a specific time entry by its ID.
func (s *Store) DeleteTimeEntry(id int) error {
	result, err := s.db.Exec("DELETE FROM time_entries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("could not delete time entry: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("time entry with ID %d not found", id)
	}
	return nil
}

// GetTimeEntry returns a single time entry by ID.
func (s *Store) GetTimeEntry(id int) (*model.TimeEntry, error) {
	var entry model.TimeEntry
	var startUnix int64
	var stopUnix sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, task_id, start_at, stop_at
		FROM time_entries
		WHERE id = ?
	`, id).Scan(&entry.ID, &entry.TaskID, &startUnix, &stopUnix)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("time entry with ID %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("could not query time entry: %w", err)
	}
	entry.StartAt = time.Unix(startUnix, 0)
	if stopUnix.Valid {
		stopTime := time.Unix(stopUnix.Int64, 0)
		entry.StopAt = &stopTime
	}
	return &entry, nil
}

// UpdateTimeEntry updates a time entry's start and stop times.
func (s *Store) UpdateTimeEntry(id int, startAt time.Time, stopAt *time.Time) error {
	var stopUnix interface{}
	var lastHeartbeat interface{}
	if stopAt != nil {
		stopUnix = stopAt.Unix()
		lastHeartbeat = stopAt.Unix()
	} else {
		stopUnix = nil
		lastHeartbeat = startAt.Unix()
	}

	result, err := s.db.Exec(`
		UPDATE time_entries
		SET start_at = ?, stop_at = ?, last_heartbeat = ?
		WHERE id = ?
	`, startAt.Unix(), stopUnix, lastHeartbeat, id)
	if err != nil {
		return fmt.Errorf("could not update time entry: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("time entry with ID %d not found", id)
	}
	return nil
}
