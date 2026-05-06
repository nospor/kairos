package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nospor/kairos/model"
)

// ReportFilter defines optional filtering for report and export commands.
type ReportFilter struct {
	ProjectName string
	From        *time.Time
	To          *time.Time
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

// StartTask begins tracking time for the named task.
// Only one task may be active at a time.
func (s *Store) StartTask(name string) error {
	active, err := s.GetActiveTask()
	if err != nil {
		return err
	}
	if active != nil {
		return fmt.Errorf("task %q is already being tracked; stop it first", active.TaskName)
	}

	var taskID int
	err = s.db.QueryRow("SELECT id FROM tasks WHERE name = ?", name).Scan(&taskID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("task %q not found; create it first with 'kairos create %q'", name, name)
	}
	if err != nil {
		return fmt.Errorf("could not find task: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO time_entries (task_id, start_at) VALUES (?, ?)",
		taskID, time.Now().Unix(),
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
		ORDER BY p.name, t.name
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
	rows, err := s.db.Query(`
		SELECT p.name, COALESCE(t.name, ''), COALESCE(SUM(
			CASE WHEN te.stop_at IS NOT NULL THEN
				te.stop_at - te.start_at
			ELSE
				CAST(strftime('%s', 'now') AS INTEGER) - te.start_at
			END
		), 0) as total_seconds
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		LEFT JOIN time_entries te ON te.task_id = t.id
		GROUP BY p.id, p.name, t.id, t.name
		ORDER BY p.name, t.name
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
		if err := rows.Scan(&projectName, &taskName, &totalSeconds); err != nil {
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
	query := `
		SELECT p.name, t.name, COALESCE(SUM(
			CASE WHEN te.stop_at IS NOT NULL THEN
				te.stop_at - te.start_at
			ELSE
				CAST(strftime('%s', 'now') AS INTEGER) - te.start_at
			END
		), 0) as total_seconds
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		LEFT JOIN time_entries te ON te.task_id = t.id
	`

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

	query += " GROUP BY p.name, t.name ORDER BY p.name, t.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not generate report: %w", err)
	}
	defer rows.Close()

	var report []model.ReportRow
	for rows.Next() {
		var r model.ReportRow
		var totalSeconds int64
		if err := rows.Scan(&r.ProjectName, &r.TaskName, &totalSeconds); err != nil {
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
