
## [1.2.6] - 2026-06-16

### Features

- *(tui)* Add 4th tab for report commands and analytics

### Bug Fixes

- *(tui)* Disable tab-key pane switching on projects tab

    Disable pane switching with Tab/Shift+Tab on the projects tab to avoid
    breaking the main tab navigation. Users can still navigate panes using
    the arrow keys (left/right/h/l).

## [1.2.5] - 2026-06-16

### Bug Fixes

- *(tui)* Prevent view scrolling/jumping on recent history tab

### Miscellaneous Tasks

- Update CHANGELOG.md for v1.2.4 [skip ci]
- Update CHANGELOG.md for v1.2.5 [skip ci]

## [1.2.4] - 2026-06-01

### Bug Fixes

- Fixing ci

### Miscellaneous Tasks

- Update CHANGELOG.md for v1.2.2 [skip ci]

## [1.2.3] - 2026-06-01

### Bug Fixes

- File formats

## [1.2.2] - 2026-06-01

### Features

- *(tui)* Add interactive dashboard command and shortcut alias
- Add update command and TUI modal to modify task time entries
- *(history)* Add duration filtering to history command

### Miscellaneous Tasks

- Update CHANGELOG.md for v1.2.1 [skip ci]

## [1.2.1] - 2026-05-25

### Styling

- Format start.go and queries.go with gofmt

### Miscellaneous Tasks

- Update CHANGELOG.md for v1.1.1 [skip ci]
- Include commit descriptions in changelog and fix release pipeline conflict
- Clean workspace before rebasing in release workflow
- Update CHANGELOG.md for v1.2.1 [skip ci]
- Migrate deprecated goreleaser format fields

## [1.2.0] - 2026-05-25

### Features

- Add --notify flag to start command for periodic notifications

    Adds a `--notify N` flag to the start command to send cross-platform
    desktop
    reminders every N minutes. Supports notify-send on Linux, osascript on
    macOS,
    and PowerShell on Windows.
- Add status command with live watch mode

    Adds a `status` command to show the currently active task. Includes a
    `-w`/`--watch`
    flag to continuously display a live-updating counter of elapsed time in
    the
    foreground, exit gracefully on Ctrl+C, and automatically detect if
    tracking is
    stopped from another process.
- Add history command to show individual time entries

    Adds a `history` command to display a chronological list of individual
    tracking
    sessions. Supports a `-n`/`--limit` flag to limit the number of entries
    shown.
- Add manual logging command for retroactive entries

    Adds an `add` command to manually log time entries. Supports parsing
    time durations
    (e.g., `-d 45m`) and specific start/end timestamps. Automatically
    creates the task
    under the specified project if it does not exist.
- Add interactive task selection to start command

    Allows running `start` with no arguments to trigger a menu that prompts
    the user to select an existing task from a numbered list.
    Supports filtering the list by project if the `-p`/`--project` flag is
    provided.

## [1.1.1] - 2026-05-21

### Bug Fixes

- Prevent tracking tasks after system shutdown ([#1](https://github.com/nospor/kairos/issues/1))
- Make background process spawning platform independent

### Miscellaneous Tasks

- Setup testing, release pipelines, and changelog automation
- Update CHANGELOG.md for v1.1.1 [skip ci]
- Ignore GoReleaser artifacts and release notes
- Reorder release workflow steps to run GoReleaser first
- Update CHANGELOG.md for v1.1.1 [skip ci]

## [1.1] - 2026-05-06

### Features

- Report group by

## [1.0] - 2026-05-06

### Features

- Ordering projects and tasks
- Edit projects/tasks and custom path for db
- Starting task may get project name too now

### Bug Fixes

- Fixing export csv
- Fixing report style
