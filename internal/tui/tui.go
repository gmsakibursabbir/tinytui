package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tinytui/tinytui/internal/config"
)

// Start initializes and runs the Bubble Tea program
func Start(cfg *config.Config) {
	p := tea.NewProgram(InitialModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
