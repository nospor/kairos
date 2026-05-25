package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var startProjectFlag string
var startNotifyFlag int

var startCmd = &cobra.Command{
	Use:   "start [task name]",
	Short: "Start tracking time for a task",
	Long: `Start tracking time for a task. Use -p to specify the project it belongs to.
Without -p, the task is looked up in the default "General" project.
If no task name is provided, an interactive prompt will list all available tasks to choose from.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if startNotifyFlag < 0 {
			return fmt.Errorf("notification interval must be a non-negative number of minutes")
		}

		var taskName, projectName string

		if len(args) == 0 {
			tasks, err := store.ListTasks()
			if err != nil {
				return err
			}

			var filtered []struct {
				TaskName    string
				ProjectName string
			}
			for _, t := range tasks {
				if startProjectFlag == "" || strings.EqualFold(t.ProjectName, startProjectFlag) {
					filtered = append(filtered, struct {
						TaskName    string
						ProjectName string
					}{
						TaskName:    t.TaskName,
						ProjectName: t.ProjectName,
					})
				}
			}

			if len(filtered) == 0 {
				if startProjectFlag != "" {
					return fmt.Errorf("no tasks found in project %q", startProjectFlag)
				}
				return fmt.Errorf("no tasks found; create a task first with 'kairos create \"Task Name\"'")
			}

			fmt.Println("Choose a task to start tracking:")
			for i, t := range filtered {
				fmt.Printf("  [%d] %s (project: %s)\n", i+1, t.TaskName, t.ProjectName)
			}
			fmt.Printf("Enter selection (1-%d): ", len(filtered))

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			input = strings.TrimSpace(input)
			var choice int
			_, err = fmt.Sscanf(input, "%d", &choice)
			if err != nil || choice < 1 || choice > len(filtered) {
				return fmt.Errorf("invalid selection")
			}

			selected := filtered[choice-1]
			taskName = selected.TaskName
			projectName = selected.ProjectName
		} else {
			taskName = args[0]
			projectName = startProjectFlag
		}

		if err := store.StartTask(taskName, projectName); err != nil {
			return err
		}

		if projectName == "" {
			projectName = "General"
		}
		fmt.Printf("Started tracking time for task %q (project: %q).\n", taskName, projectName)

		// Spawn the daemon in the background to track heartbeat and notifications
		startDaemon(startNotifyFlag)

		return nil
	},
}

func startDaemon(notifyMinutes int) {
	args := []string{}
	if configFlag != "" {
		args = append(args, "--config", configFlag)
	}
	args = append(args, "daemon")
	if notifyMinutes > 0 {
		args = append(args, "--notify", fmt.Sprintf("%d", notifyMinutes))
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	configureSysProcAttr(cmd)

	_ = cmd.Start()
}

func init() {
	startCmd.Flags().StringVarP(&startProjectFlag, "project", "p", "", `project the task belongs to (default: "General")`)
	startCmd.Flags().IntVar(&startNotifyFlag, "notify", 0, "send a desktop notification every N minutes to remind you of the active task")
	rootCmd.AddCommand(startCmd)
}
