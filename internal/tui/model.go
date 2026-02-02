package tui

import (
	"fmt"

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
	// Styles
	docStyle = lipgloss.NewStyle().Margin(1, 2)
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
		case "esc":
			m.state = StateQueue
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
		case "d":
			if m.state != StateQueue {
				m.state = StateQueue
				// Sync just in case? Usually sync happens on 'a'.
				// But if user manually switches, we should ensure it's up to date.
				if m.pipeline != nil {
					m.queue.Sync(m.pipeline.Jobs())
				}
			}
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
	
	// Mascot Logic
	mascot := ""
	mascotWidth := 0
	if m.config.ShouldShowMascot(m.width) {
		mascot = m.renderMascot()
		mascotWidth = lipgloss.Width(mascot) + 2 // Margin
	}
	
	// Layout Content
	content := ""
	
	// Temporarily adjust width for content rendering if mascot is showing
	// We need to mutate m.width for the view generation, effectively passing a constrained width to children
	// A cleaner way is to pass width to view methods, but for now we simply substract and restore or handle inside views.
	// Since viewBrowser uses m.width directly, we have to hack specific width override or change viewBrowser to accept width.
	// Changing viewBrowser signature is big.
	// Let's modify m.width locally for the switch? No, m is by value receiver in View, so modifying it is safe for this call stack!
	effectiveWidth := m.width
	if mascot != "" {
		effectiveWidth = m.width - mascotWidth
	}
	// Update m width locally
	m.width = effectiveWidth 
	
	switch m.state {
	case StateSetup:
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
	
	if mascot != "" {
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, mascot)
	}

	if m.showingHelp {
		// Use a nice overlay
		helpPane := stylePane.
			BorderForeground(lipgloss.Color(ColorPink)).
			Width(60).
			Render(
			styleBold.Foreground(lipgloss.Color(ColorPink)).Render("Help & Keys") + "\n\n" +
			" Global:\n" + 
			"  [A] Add Files   [R] Run\n" +
			"  [S] Settings    [H] History\n" +
			"  [W] Mascot      [?] Close Help\n" +
			"  [Q] Quit\n\n" +
			" Browser:\n" +
			"  [Space] Select  [A] Batch Select\n" +
			"  [:] Command     [p] Preview\n" +
			"  [s] Sort        [S] Sort Dir\n\n" +
			" Queue:\n" +
			"  [d] Remove      [c] Clear",
		)
		
		// Center overlay
		content = lipgloss.Place(m.width + mascotWidth, m.height - 2,
			lipgloss.Center, lipgloss.Center,
			helpPane,
		)
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
	// Highlight current
	// Highlight current
	// Use lipgloss for better highlighting
	activeTabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true)
	inactiveTabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorComment))
	
	currentTabStr := func(name string, state SessionState) string {
		if m.state == state {
			return activeTabStyle.Render("[ " + name + " ]")
		}
		return inactiveTabStyle.Render("[ " + name + " ]")
	}
	
	tabs = fmt.Sprintf("%s %s %s %s",
		currentTabStr("Browser", StateBrowser),
		currentTabStr("Queue", StateQueue),
		currentTabStr("Compress", StateCompress),
		currentTabStr("History", StateHistory),
	)
	
	return lipgloss.JoinHorizontal(lipgloss.Center, 
		styleTitle.Render("TiniTUI "+version.Version),
		"  ",
		status,
		" | ",
		mode,
		" | ",
		tabs,
	)
}

func (m MainModel) renderBottomBar() string {
	return styleDim.Render("A: Add Files | R: Run | S: Settings | H: History | Q: Quit")
}

// ---------------- STUBS -----------------

// Methods moved to setup.go and browser.go

// Methods moved to setup.go and browser.go

// Update Start function in tui.go to use this
