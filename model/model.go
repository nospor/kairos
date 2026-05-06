package model

import "time"

// Project represents a project that groups tasks together.
type Project struct {
	ID   int
	Name string
}

// Task represents a trackable unit of work within a project.
type Task struct {
	ID        int
	Name      string
	ProjectID int
}

// TimeEntry represents a recorded time interval for a task.
type TimeEntry struct {
	ID      int
	TaskID  int
	StartAt time.Time
	StopAt  *time.Time
}

// ReportRow holds aggregated duration data for a single task in a report.
type ReportRow struct {
	ProjectName string
	TaskName    string
	Duration    time.Duration
	Date        string
}

// TaskInfo holds task details along with its project name and total duration.
type TaskInfo struct {
	TaskName      string
	ProjectName   string
	TotalDuration time.Duration
}

// ProjectInfo holds a project and its associated tasks.
type ProjectInfo struct {
	ProjectName string
	Tasks       []TaskInfo
}

// ActiveInfo holds information about the currently active (running) task.
type ActiveInfo struct {
	TaskName    string
	ProjectName string
	StartedAt   time.Time
}
