package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	addProjectFlag  string
	addDurationFlag string
	addStartFlag    string
	addEndFlag      string
)

var addCmd = &cobra.Command{
	Use:   "add [task name]",
	Short: "Log a completed time entry retroactively",
	Long: `Log a completed time entry retroactively.
You must specify either --duration (-d) or both --start (-s) and optionally --end (-e).

Supported time formats:
  - YYYY-MM-DD HH:MM:SS (e.g., 2026-05-25 08:30:00)
  - YYYY-MM-DD HH:MM    (e.g., 2026-05-25 08:30)
  - HH:MM               (e.g., 08:30, assumes today)

If --end is omitted but --start is specified, it defaults to the current time.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]
		var startAt, stopAt time.Time

		if addDurationFlag != "" {
			if addStartFlag != "" || addEndFlag != "" {
				return fmt.Errorf("cannot specify both --duration and --start/--end")
			}
			duration, err := time.ParseDuration(addDurationFlag)
			if err != nil {
				return fmt.Errorf("invalid duration format %q: %w (e.g. 1h30m, 45m)", addDurationFlag, err)
			}
			if duration <= 0 {
				return fmt.Errorf("duration must be positive")
			}
			stopAt = time.Now()
			startAt = stopAt.Add(-duration)
		} else {
			if addStartFlag == "" {
				return fmt.Errorf("must specify either --duration or --start")
			}
			var err error
			startAt, err = parseTime(addStartFlag)
			if err != nil {
				return fmt.Errorf("invalid --start time: %w", err)
			}

			if addEndFlag != "" {
				stopAt, err = parseTime(addEndFlag)
				if err != nil {
					return fmt.Errorf("invalid --end time: %w", err)
				}
			} else {
				stopAt = time.Now()
			}

			if stopAt.Before(startAt) {
				return fmt.Errorf("end time %q cannot be before start time %q", stopAt.Format("2006-01-02 15:04:05"), startAt.Format("2006-01-02 15:04:05"))
			}
		}

		err := store.LogTimeEntry(taskName, addProjectFlag, startAt, stopAt)
		if err != nil {
			return err
		}

		projName := addProjectFlag
		if projName == "" {
			projName = "General"
		}
		duration := stopAt.Sub(startAt)
		fmt.Printf("Logged %s for task %q (project: %q) from %s to %s.\n",
			formatDuration(duration),
			taskName,
			projName,
			startAt.Local().Format("2006-01-02 15:04:05"),
			stopAt.Local().Format("2006-01-02 15:04:05"),
		)
		return nil
	},
}

func parseTime(s string) (time.Time, error) {
	now := time.Now()
	// Option 1: Full date and time
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	// Option 2: Date and time without seconds
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	// Option 3: Time only (assumes today)
	if t, err := time.ParseInLocation("15:04", s, time.Local); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time format (supported: 'YYYY-MM-DD HH:MM:SS', 'YYYY-MM-DD HH:MM', 'HH:MM')")
}

func init() {
	addCmd.Flags().StringVarP(&addProjectFlag, "project", "p", "", `project the task belongs to (default: "General")`)
	addCmd.Flags().StringVarP(&addDurationFlag, "duration", "d", "", "duration of the entry (e.g. 1h30m, 45m)")
	addCmd.Flags().StringVarP(&addStartFlag, "start", "s", "", "start time of the entry")
	addCmd.Flags().StringVarP(&addEndFlag, "end", "e", "", "end time of the entry (defaults to now)")
	rootCmd.AddCommand(addCmd)
}
