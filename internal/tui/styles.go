package tui

import "github.com/charmbracelet/lipgloss"

// Theme Colors (Dracula-inspired / Modern Dark)
const (
	ColorBackground    = "#282A36" // Dark Gray
	ColorForeground    = "#F8F8F2" // Off White
	ColorSelectionBg   = "#44475A" // Selection Highlight
	ColorComment       = "#6272A4" // Muted Blue/Purple
	ColorCyan          = "#8BE9FD" // Primary Accent
	ColorGreen         = "#50FA7B" // Success / Active
	ColorOrange        = "#FFB86C" // Warning
	ColorPink          = "#FF79C6" // Command / Error
	ColorRed           = "#FF5555" // Error
	ColorYellow        = "#F1FA8C" // Highlight
)

var (
	// Base Text Styles
	styleNormal = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForeground))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorComment))
	styleBold   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorForeground))

	// Pane Styles
	stylePane = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorComment)).
		Padding(0, 1)

	stylePaneActive = stylePane.Copy().
		BorderForeground(lipgloss.Color(ColorCyan))

	// List Styles
	styleItemNormal = lipgloss.NewStyle().
		PaddingLeft(2). // Indent
		Foreground(lipgloss.Color(ColorForeground))

	styleItemSelected = lipgloss.NewStyle().
		PaddingLeft(2).
		Background(lipgloss.Color(ColorSelectionBg)).
		Foreground(lipgloss.Color(ColorCyan)).
		Bold(true)

	// Status Bar Styles
	styleStatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBackground)).
		Background(lipgloss.Color(ColorComment)).
		Padding(0, 1)
	
	styleStatusMode = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBackground)).
		Background(lipgloss.Color(ColorCyan)).
		Bold(true).
		Padding(0, 1)

	// Header Styles
	styleHeaderPath = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorCyan)).
		Background(lipgloss.Color(ColorSelectionBg)).
		Bold(true).
		Padding(0, 1)

	styleTitle = lipgloss.NewStyle().
		MarginLeft(1).
		MarginRight(5).
		Padding(0, 1).
		Bold(true).
		Foreground(lipgloss.Color(ColorForeground)).
		Background(lipgloss.Color(ColorComment))
)
