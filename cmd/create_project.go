package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var createProjectCmd = &cobra.Command{
	Use:   "create-project [project name]",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.CreateProject(name); err != nil {
			return err
		}
		fmt.Printf("Project %q created.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createProjectCmd)
}
