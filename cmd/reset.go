package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all tracked tasks and their durations",
	Long:  `Delete all projects, tasks, and time entries. The default "General" project will be re-created.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Are you sure you want to reset all data? This cannot be undone. [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer != "y" && answer != "yes" {
			fmt.Println("Reset cancelled.")
			return nil
		}

		if err := store.ResetAll(); err != nil {
			return err
		}
		fmt.Println("All data has been reset.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
