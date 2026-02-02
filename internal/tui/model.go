package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tinytui/tinytui/internal/config"
	"github.com/tinytui/tinytui/internal/pipeline"
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
		// Init checks?
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
		case "?":
			// Help
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	
	// Route based on state
	switch m.state {
	case StateSetup:
		return m.updateSetup(msg)
	case StateBrowser:
		return m.updateBrowser(msg)
	case StateQueue:
		return m.updateQueue(msg)
	case StateCompress:
		return m.updateProgress(msg)
	case StateHistory:
		return m.updateHistory(msg)
	}
	
	return m, nil
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

	return lipgloss.JoinVertical(lipgloss.Left, topBar, content, bottomBar)
}

func (m MainModel) renderMascot() string {
	// Simple ASCII Panda Face
	// State based?
	face := "(^. .^)"
	if m.state == StateCompress {
		face = "(O . O)" // working
	}
	// Happy if success? we need to track last success status in main model?
	// For now simple.
	
	return lipgloss.NewStyle().
		MarginLeft(2).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("0")).
		Render(face + " \n/| |\\ \n U U")
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
	
	return titleStyle.Render("TinyTUI v1.0") + " " + status + " | " + mode + " | " + tabs
}

func (m MainModel) renderBottomBar() string {
	return subtleStyle.Render("A: Add Files | R: Run | S: Settings | H: History | Q: Quit")
}

// ---------------- STUBS -----------------

// Methods moved to setup.go and browser.go

// Methods moved to setup.go and browser.go

// Update Start function in tui.go to use this
