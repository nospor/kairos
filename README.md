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

# Start tracking (must specify -p if task is not in General)
kairos start "Task 1" -p "Project A"

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
| `kairos edit "Task" "New" [-p "Proj"]`| Rename a task (defaults to "General" project) |
| `kairos delete "Task"`                | Delete a task and its time entries            |
| `kairos list`                         | List all tasks with their durations           |

### Projects

| Command                                     | Description                        |
| ------------------------------------------- | ---------------------------------- |
| `kairos create-project "Name"`              | Create a new project               |
| `kairos edit-project "Old Name" "New Name"` | Rename a project                   |
| `kairos delete-project "Name"`              | Delete a project and all its tasks |
| `kairos list-projects`                      | List all projects and their tasks  |

### Time Tracking

| Command                              | Description                                 |
| ------------------------------------ | ------------------------------------------- |
| `kairos start "Task" [-p "Project"]` | Start tracking a task (defaults to General) |
| `kairos stop`                        | Stop tracking the currently active task     |

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
| `--group-by "period"` | Group data by `day`, `week`, `month`, or `year` |

**Examples:**

```bash
# Report for a specific project
kairos report --project "Project A"

# Report for today
kairos report --today

# Report for a date range
kairos report --from "2026-01-01" --to "2026-01-31"

# Report grouped by month
kairos report --group-by month

# Export this week's data grouped by day
kairos export weekly.csv --week --group-by day
```

### Data Management

| Command        | Description                                |
| -------------- | ------------------------------------------ |
| `kairos reset` | Delete all data (with confirmation prompt) |

## Global Flags

These flags work with every command:

| Flag               | Description                                             |
| ------------------ | ------------------------------------------------------- |
| `--config <path>`  | Use a custom database file instead of the default one   |

```bash
# Use a separate DB for a work profile
kairos --config ~/work.db start "Standup" -p "Meetings"

# Use a separate DB for personal projects
kairos --config ~/personal.db report
```

## Default Project

Every task must belong to a project. If you omit `-p`, the **General** project is used:

```bash
# Create and start a task in General
kairos create "Quick task"
kairos start "Quick task"

# Create and start a task in a specific project
kairos create "Deep work" -p "Research"
kairos start "Deep work" -p "Research"
```

## Data Storage

All data is stored in a local SQLite database at:

```
~/.cache/kairos/kairos.db
```

The database is created automatically on first use.

## License

Kairos is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Thanks For Visiting
Hope you liked it. Wanna **[buy Me a coffee](https://www.buymeacoffee.com/nospor)**?

