package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gmsakibursabbir/tinitui/internal/config"
	"github.com/gmsakibursabbir/tinitui/internal/pipeline"
	"github.com/gmsakibursabbir/tinitui/internal/version"
)

type SessionState int

const (
	StateSetup SessionState = iota
	StateBrowser
	StateQueue
	StateCompress
	StateHistory
	StateSettings
)

var (
	// Styles
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	titleStyle = lipgloss.NewStyle().MarginLeft(1).MarginRight(5).Padding(0, 1).Italic(true).Foreground(lipgloss.Color("#FFF7DB")).Background(lipgloss.Color("#5A56E0"))
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
)

type MainModel struct {
	state      SessionState
	config     *config.Config
	pipeline   *pipeline.Pipeline
	
	// Child Models
	setup       setupModel
	browser     browserModel
	queue       queueModel
	progress    progressModel
	history     historyModel
	settings    settingsModel
	
	showingHelp bool
	width  int
	height int
	
	quitting bool
}

func InitialModel(cfg *config.Config) MainModel {
	m := MainModel{
		config:   cfg,
		state:    StateBrowser, // Default to browser if configured
		setup:    newSetupModel(),
		browser:  newBrowserModel(),
		queue:    newQueueModel(),
		progress: newProgressModel(),
		history:  newHistoryModel(),
		settings: newSettingsModel(),
	}
	
	if !cfg.IsConfigured() {
		m.state = StateSetup
	}
	
	m.pipeline = pipeline.New(cfg, cfg.APIKey)
	
	return m
}

func (m MainModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		waitForPipeline(m.pipeline),
	)
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global Keys
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.state != StateSetup { // Allow q to quit except in input? Or always?
				// "Q quit (confirm if running)". 
				m.quitting = true
				return m, tea.Quit 
			}
		case "h":
			m.state = StateHistory
		case "r":
			// If we have jobs, start
			if len(m.pipeline.Jobs()) > 0 {
				m.state = StateCompress
				m.pipeline.Start() // Or ensure started
			}
		case "s":
			// Settings (Placeholder)
			m.state = StateSettings // Not fully impl
		case "a":
			m.state = StateBrowser
		case "w":
			// Toggle Mascot
			switch m.config.Mascot {
			case config.MascotOff:
				m.config.Mascot = config.MascotOn
			default:
				m.config.Mascot = config.MascotOff
			}
			m.config.Save()
		case "?":
			// Toggle Help?
			// Or simple modal state?
			// Let's just create a quick help state or overlay.
			// For minimal impact, just toggle a help variable in model.
			m.showingHelp = !m.showingHelp
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	
	var cmd tea.Cmd
	
	// Handle types
	switch msg.(type) {
	case *pipeline.Job:
		// Forward to progress model regardless of state, 
	}

	// Refactored State handling with type assertions
	switch m.state {
	case StateSetup:
		newModel, newCmd := m.updateSetup(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	case StateBrowser:
		newModel, newCmd := m.updateBrowser(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	case StateQueue:
		newModel, newCmd := m.updateQueue(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	case StateCompress:
		newModel, newCmd := m.updateProgress(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	case StateHistory:
		newModel, newCmd := m.updateHistory(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	case StateSettings:
		newModel, newCmd := m.updateSettings(msg)
		m = newModel.(MainModel)
		cmd = newCmd
	}
	
	// Handle pipeline updates globally if needed, or ensure waitForPipeline is re-dispatched
	if _, ok := msg.(*pipeline.Job); ok {
		// Re-dispatch wait
		return m, tea.Batch(cmd, waitForPipeline(m.pipeline))
	}
	
	return m, cmd
}

func (m MainModel) View() string {
	if m.quitting {
		return "Bye!\n"
	}
	
	// Layout
	topBar := m.renderTopBar()
	bottomBar := m.renderBottomBar()
	
	content := ""
	switch m.state {
	case StateSetup:
		// Setup takes full screen logic usually, but let's keep it simple
		content = m.viewSetup()
	case StateBrowser:
		content = m.viewBrowser()
	case StateQueue:
		content = m.viewQueue()
	case StateCompress:
		content = m.viewProgress()
	case StateHistory:
		content = m.viewHistory()
	case StateSettings:
		content = m.viewSettings()
	default:
		content = fmt.Sprintf("State: %v", m.state)
	}
	
	// Mascot
	mascot := ""
	if m.config.ShouldShowMascot(m.width) {
		mascot = m.renderMascot()
	}
	
	// Layout
	// We can place mascot on the right side of content or bottom?
	// Prompt says "Must not block UI".
	// Let's put it in bottom right corner of content area if space permits.
	// Or just append it to content.
	
	// Simple append for now
	if mascot != "" {
		// Use JoinHorizontal with content?
		// Content usually takes full width in docStyle.
		// Let's put mascot above bottom bar?
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, mascot)
	}

	if m.showingHelp {
		// Simple overlay
		helpText := docStyle.Render("Help:\n\nGlobal:\n A - Add Files\n R - Run\n S - Settings\n H - History\n W - Toggle Mascot\n ? - Toggle Help\n Q - Quit\n\nBrowser:\n Space - Select\n X - Toggle Recursive\n A - Add Selection\n\nQueue:\n Space - Select\n D - Delete\n C - Clear Completed\n\nCompress:\n P - Pause/Resume\n X - Cancel")
		// replace content? or overlay?
		// simple replacement for now
		content = helpText
	}

	return lipgloss.JoinVertical(lipgloss.Left, topBar, content, bottomBar)
}

func (m MainModel) renderMascot() string {
	isWorking := m.state == StateCompress
	art := getMascot(m.config.MascotType, isWorking)
	
	return lipgloss.NewStyle().
		MarginLeft(2).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("0")).
		Render(art)
}
func (m MainModel) renderTopBar() string {
	// App name + version | API status | Output mode | Tabs
	// Simple style
	status := "API: OK"
	if !m.config.IsConfigured() {
		status = "API: Missing"
	}
	
	mode := "Mode: " + m.config.OutputMode
	
	tabs := " [ Browser ] [ Queue ] [ Compress ] [ History ] "
	// Highlight current
	// Simple string replacement for highlight based on state
	switch m.state {
	case StateBrowser:
		tabs = strings.Replace(tabs, "[ Browser ]", "[ *Browser* ]", 1)
	case StateQueue:
		tabs = strings.Replace(tabs, "[ Queue ]", "[ *Queue* ]", 1)
	case StateCompress:
		tabs = strings.Replace(tabs, "[ Compress ]", "[ *Compress* ]", 1)
	case StateHistory:
		tabs = strings.Replace(tabs, "[ History ]", "[ *History* ]", 1)
	}
	
	return titleStyle.Render("TiniTUI "+version.Version) + " " + status + " | " + mode + " | " + tabs
}

func (m MainModel) renderBottomBar() string {
	return subtleStyle.Render("A: Add Files | R: Run | S: Settings | H: History | Q: Quit")
}

// ---------------- STUBS -----------------

// Methods moved to setup.go and browser.go

// Methods moved to setup.go and browser.go

// Update Start function in tui.go to use this
