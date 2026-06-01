package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var historyLimitFlag int
var historyLongerThanFlag string

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show chronological history of individual time entries",
	Long: `Display a list of individual time entries, starting with the most recent.
Use the --limit (-n) flag to restrict the number of entries shown.
Use the --longer-than (-d) flag to only show entries longer than a specific duration (e.g. 15m, 1h30m).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var minDuration time.Duration
		var err error
		if historyLongerThanFlag != "" {
			minDuration, err = time.ParseDuration(historyLongerThanFlag)
			if err != nil {
				return fmt.Errorf("invalid duration format %q: %w (e.g. 1h30m, 45m)", historyLongerThanFlag, err)
			}
		}

		history, err := store.GetHistoryFiltered(historyLimitFlag, minDuration)
		if err != nil {
			return err
		}

		if len(history) == 0 {
			fmt.Println("No time entries found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tProject\tTask\tStart\tEnd\tDuration")
		fmt.Fprintln(w, strings.Repeat("-", 4)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 19)+"\t"+strings.Repeat("-", 19)+"\t"+strings.Repeat("-", 10))

		for _, h := range history {
			startStr := h.StartAt.Local().Format("2006-01-02 15:04:05")
			endStr := "(active)"
			if h.StopAt != nil {
				endStr = h.StopAt.Local().Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", h.ID, h.ProjectName, h.TaskName, startStr, endStr, formatDuration(h.Duration))
		}
		w.Flush()
		return nil
	},
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimitFlag, "limit", "n", 0, "limit the number of history entries displayed")
	historyCmd.Flags().StringVarP(&historyLongerThanFlag, "longer-than", "d", "", "only show entries longer than a specific duration (e.g. 15m, 1h30m)")
	rootCmd.AddCommand(historyCmd)
}
