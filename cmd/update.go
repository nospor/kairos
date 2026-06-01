package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	updateDurationFlag string
	updateStartFlag    string
	updateEndFlag      string
)

var updateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update a time entry's time",
	Long: `Update a time entry's duration, start time, or end time.
You must specify at least one of --duration (-d), --start (-s), or --end (-e).

Supported time formats:
  - YYYY-MM-DD HH:MM:SS (e.g., 2026-05-25 08:30:00)
  - YYYY-MM-DD HH:MM    (e.g., 2026-05-25 08:30)
  - HH:MM               (e.g., 08:30, assumes today)

If you update only the duration of a completed entry, its end time will be adjusted.
If you update only the duration of an active entry, its start time will be adjusted.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid time entry ID %q: %w", args[0], err)
		}

		if updateDurationFlag == "" && updateStartFlag == "" && updateEndFlag == "" {
			return fmt.Errorf("must specify at least one of --duration, --start, or --end")
		}

		entry, err := store.GetTimeEntry(id)
		if err != nil {
			return err
		}

		startAt := entry.StartAt
		var stopAt *time.Time
		if entry.StopAt != nil {
			t := *entry.StopAt
			stopAt = &t
		}

		var duration time.Duration
		if updateDurationFlag != "" {
			duration, err = time.ParseDuration(updateDurationFlag)
			if err != nil {
				return fmt.Errorf("invalid duration format %q: %w (e.g. 1h30m, 45m)", updateDurationFlag, err)
			}
			if duration <= 0 {
				return fmt.Errorf("duration must be positive")
			}
		}

		var parsedStart, parsedEnd time.Time
		var hasStart, hasEnd bool

		if updateStartFlag != "" {
			parsedStart, err = parseTime(updateStartFlag)
			if err != nil {
				return fmt.Errorf("invalid --start time: %w", err)
			}
			hasStart = true
		}

		if updateEndFlag != "" {
			parsedEnd, err = parseTime(updateEndFlag)
			if err != nil {
				return fmt.Errorf("invalid --end time: %w", err)
			}
			hasEnd = true
		}

		if updateDurationFlag != "" {
			if hasStart && hasEnd {
				return fmt.Errorf("cannot specify all three: --duration, --start, and --end")
			}
			if hasStart {
				startAt = parsedStart
				endT := startAt.Add(duration)
				stopAt = &endT
			} else if hasEnd {
				endT := parsedEnd
				stopAt = &endT
				startAt = endT.Add(-duration)
			} else {
				if stopAt != nil {
					endT := startAt.Add(duration)
					stopAt = &endT
				} else {
					startAt = time.Now().Add(-duration)
				}
			}
		} else {
			if hasStart {
				startAt = parsedStart
			}
			if hasEnd {
				endT := parsedEnd
				stopAt = &endT
			}
		}

		if stopAt != nil && stopAt.Before(startAt) {
			return fmt.Errorf("end time %q cannot be before start time %q", stopAt.Format("2006-01-02 15:04:05"), startAt.Format("2006-01-02 15:04:05"))
		}

		if err := store.UpdateTimeEntry(id, startAt, stopAt); err != nil {
			return err
		}

		var dur time.Duration
		if stopAt != nil {
			dur = stopAt.Sub(startAt)
		} else {
			dur = time.Since(startAt)
		}

		endStr := "(active)"
		if stopAt != nil {
			endStr = stopAt.Local().Format("2006-01-02 15:04:05")
		}

		fmt.Printf("Updated time entry %d:\n  Start:    %s\n  End:      %s\n  Duration: %s\n",
			id,
			startAt.Local().Format("2006-01-02 15:04:05"),
			endStr,
			formatDuration(dur),
		)
		return nil
	},
}

func init() {
	updateCmd.Flags().StringVarP(&updateDurationFlag, "duration", "d", "", "duration of the entry (e.g. 1h30m, 45m)")
	updateCmd.Flags().StringVarP(&updateStartFlag, "start", "s", "", "start time of the entry")
	updateCmd.Flags().StringVarP(&updateEndFlag, "end", "e", "", "end time of the entry")
	rootCmd.AddCommand(updateCmd)
}
