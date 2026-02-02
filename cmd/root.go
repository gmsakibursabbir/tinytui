package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tinytui/tinytui/internal/config"
	"github.com/tinytui/tinytui/internal/tui"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "tinytui",
	Short: "TinyTUI is a TUI for compressing images via TinyPNG",
	Long:  `A modern, beautiful Terminal User Interface for compressing images using the TinyPNG API.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action: Run TUI
		// Pass cfg to TUI
		tui.Start(cfg)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	// Flags for config might go here
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		// Log error but don't exit, might be first run
		// However, TUI Setup needs to handle "New Config"
		// If load fails because file doesn't exist, Load returns default config
		// If it fails for other reasons (perm denied?), we might be in trouble.
		// For now assume cfg is workable.
	}
}
