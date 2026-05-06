# Kairos

A command-line time tracking application written in Go. Track time spent on tasks organised by projects, generate reports, and export data to CSV.

## Installation

### From source

```bash
go install github.com/nospor/kairos@latest
```

### Build locally

```bash
git clone https://github.com/nospor/kairos.git
cd kairos
go build -o kairos .

# or (builds slower, but binary is smaller)
go build -trimpath -ldflags="-s -w" -o kairos .

# then run
./kairos

# you may also want to copy the binary to your PATH (and run it from any place), e.g.:
sudo cp kairos /usr/local/bin/

```

## Quick Start

```bash
# Create a project and a task
kairos create-project "Project A"
kairos create "Task 1" -p "Project A"

# Start tracking
kairos start "Task 1"

# ... do some work ...

# Stop tracking
kairos stop
# Stopped tracking time for task "Task 1". Duration: 1h 30m.

# View your report
kairos report
```

## Commands

### Projects

| Command                        | Description                        |
| ------------------------------ | ---------------------------------- |
| `kairos create-project "Name"` | Create a new project               |
| `kairos delete-project "Name"` | Delete a project and all its tasks |
| `kairos list-projects`         | List all projects and their tasks  |

### Tasks

| Command                               | Description                                   |
| ------------------------------------- | --------------------------------------------- |
| `kairos create "Task" [-p "Project"]` | Create a task (defaults to "General" project) |
| `kairos delete "Task"`                | Delete a task and its time entries            |
| `kairos list`                         | List all tasks with their durations           |

### Time Tracking

| Command               | Description                             |
| --------------------- | --------------------------------------- |
| `kairos start "Task"` | Start tracking time for a task          |
| `kairos stop`         | Stop tracking the currently active task |

### Reporting

| Command                          | Description                      |
| -------------------------------- | -------------------------------- |
| `kairos report [flags]`          | Display a report of tracked time |
| `kairos export file.csv [flags]` | Export the report to CSV         |

#### Report & Export Filtering Flags

| Flag                  | Description                             |
| --------------------- | --------------------------------------- |
| `--project "Name"`    | Filter by project                       |
| `--today`             | Show only today's entries               |
| `--week`              | Show only this week's entries (Mon–Sun) |
| `--month`             | Show only this month's entries          |
| `--from "YYYY-MM-DD"` | Show entries starting from this date    |
| `--to "YYYY-MM-DD"`   | Show entries up to this date            |

**Examples:**

```bash
# Report for a specific project
kairos report --project "Project A"

# Report for today
kairos report --today

# Report for a date range
kairos report --from "2026-01-01" --to "2026-01-31"

# Export this week's data
kairos export weekly.csv --week
```

### Data Management

| Command        | Description                                |
| -------------- | ------------------------------------------ |
| `kairos reset` | Delete all data (with confirmation prompt) |

## Default Project

Every task must belong to a project. If you create a task without specifying a project, it is automatically assigned to the **General** project:

```bash
kairos create "Quick task"
# Task "Quick task" created under project "General".
```

## Data Storage

All data is stored in a local SQLite database at:

```
~/.cache/kairos/kairos.db
```

The database is created automatically on first use.

## License

MIT
