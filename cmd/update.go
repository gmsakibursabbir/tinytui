package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tinytui/tinitui/internal/updater"
	"github.com/tinytui/tinitui/internal/version"
)

var autoYes bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update TiniTUI to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")

		latest, release, err := updater.GetLatestVersion()
		if err != nil {
			fmt.Printf("Failed to check for updates: %v\n", err)
			os.Exit(1)
		}

		if !updater.IsNewer(version.Version, latest) {
			fmt.Printf("TinyTUI is already up to date (%s)\n", version.Version)
			return
		}

		fmt.Printf("\nUpdate available!\n Current: %s\n Latest:  %s\n", version.Version, latest)

		if !autoYes {
			fmt.Print("\nProceed with update? [Y/n]: ")
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text != "" && strings.ToLower(text) != "y" {
				fmt.Println("Update cancelled.")
				return
			}
		}

		fmt.Printf("Updating to %s...\n", latest)
		if err := updater.Update(release); err != nil {
			fmt.Printf("\n❌ Update failed: %v\n", err)
			fmt.Println("\nFix options:")
			fmt.Println("1) Run: sudo tinytui update")
			fmt.Println("2) Or reinstall using:")
			fmt.Println("   curl -fsSL https://tinytui.dev/install.sh | sh")
			os.Exit(1)
		}

		fmt.Printf("\n✔ Updated to %s\nRestart TinyTUI to apply the update.\n", latest)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Skip confirmation prompt")
}
