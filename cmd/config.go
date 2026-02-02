package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tinytui/tinytui/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var setKeyCmd = &cobra.Command{
	Use:   "set-key <key>",
	Short: "Set TinyPNG API Key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		// Validate?
		// User requirement 3 says check on first launch or setup.
		// "Automatically test the key"
		// We'll trust the input for CLI set-key for now, or just save it.
		// "Prompt user to paste ... Automatically test the key ... Save config on success"
		// This likely refers to TUI setup.
		// For CLI `config set-key`, testing it is good practice.
		
		// Load existing or default
		if cfg == nil {
			cfg = config.DefaultConfig()
		}
		cfg.APIKey = key
		if err := cfg.Save(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("API Key saved.")
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setKeyCmd)
}
