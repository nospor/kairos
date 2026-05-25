package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var statusWatchFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the currently active task",
	Long: `Show details of the task currently being tracked, including project name, 
task name, start time, and elapsed duration. Use the --watch flag to see a live counter.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		active, err := store.GetActiveTask()
		if err != nil {
			return err
		}

		if active == nil {
			fmt.Println("No task is currently being tracked.")
			return nil
		}

		if statusWatchFlag {
			fmt.Printf("Project:      %s\n", active.ProjectName)
			fmt.Printf("Task:         %s\n", active.TaskName)
			fmt.Printf("Started at:   %s\n", active.StartedAt.Local().Format("2006-01-02 15:04:05"))

			// Handle graceful exit on Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			// Print initial elapsed time
			elapsed := time.Since(active.StartedAt)
			fmt.Printf("Elapsed:      %s", formatDurationWithSeconds(elapsed))

			for {
				select {
				case <-ticker.C:
					// Check if task is still active (e.g. stopped from another terminal)
					currentActive, err := store.GetActiveTask()
					if err != nil {
						fmt.Printf("\nError: %v\n", err)
						return nil
					}
					if currentActive == nil || currentActive.TaskName != active.TaskName || currentActive.ProjectName != active.ProjectName {
						fmt.Println("\nTracking stopped.")
						return nil
					}

					elapsed = time.Since(active.StartedAt)
					fmt.Printf("\rElapsed:      %s   ", formatDurationWithSeconds(elapsed))
				case <-sigChan:
					fmt.Println()
					return nil
				}
			}
		} else {
			elapsed := time.Since(active.StartedAt)
			fmt.Printf("Project:      %s\n", active.ProjectName)
			fmt.Printf("Task:         %s\n", active.TaskName)
			fmt.Printf("Started at:   %s\n", active.StartedAt.Local().Format("2006-01-02 15:04:05"))
			fmt.Printf("Elapsed:      %s\n", formatDurationWithSeconds(elapsed))
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusWatchFlag, "watch", "w", false, "continuously update the elapsed time in the terminal")
	rootCmd.AddCommand(statusCmd)
}

func formatDurationWithSeconds(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
