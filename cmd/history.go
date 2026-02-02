package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tinytui/tinytui/internal/history"
)

var csvOutput string

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View compression history",
	Run: func(cmd *cobra.Command, args []string) {
		hMgr, err := history.New()
		if err != nil {
			fmt.Printf("Error loading history: %v\n", err)
			return
		}

		if csvOutput != "" {
			if err := hMgr.ExportCSV(csvOutput); err != nil {
				fmt.Printf("Error exporting CSV: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Exported history to %s\n", csvOutput)
			return
		}

		// Dump table
		records := hMgr.All()
		for _, r := range records {
			fmt.Printf("%s\t%s\tSaved: %d bytes\n", r.Timestamp.Format("2006-01-02 15:04"), r.File, r.SavedBytes)
		}
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().StringVar(&csvOutput, "csv", "", "Export history to CSV file")
}
