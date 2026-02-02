package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gmsakibursabbir/tinitui/internal/scanner"
)

type browserModel struct {
	currentDir string
	dims       struct { width, height int }
	
	mainList   list.Model
	pathInput  textinput.Model
	commandInput textinput.Model
	
	activePane int // 0 = MainList, 1 = PathInput (Preview is passive), 2 = CommandPalette
	
	currentEntries []fs.DirEntry // Cache
	selected       map[string]bool
	recursive      bool
	
	// Power User Features
	sortMode    int  // 0=Name, 1=Size, 2=ModTime
	sortAsc     bool
	showPreview bool
	
	history      []string
	historyIndex int
	bookmarks    map[string]string
	
	err            error
	previewContent string // Cached preview string for currently selected item
}

// Unified Item Type
type browserItem struct {
	name     string
	path     string
	isDir    bool
	size     int64
	modTime  time.Time
	selected bool
}

func (i browserItem) Title() string {
	var sb strings.Builder
	
	// Selection Checkbox
	if i.selected {
		sb.WriteString(" [✔] ") // Strong check
	} else {
		sb.WriteString(" [ ] ")
	}
	
	// Icon
	sb.WriteString(getIcon(i.name, i.isDir) + " ")
	
	// Name
	if i.isDir {
		sb.WriteString(i.name + "/")
	} else {
		sb.WriteString(i.name)
	}
	
	return sb.String()
}

func (i browserItem) Description() string {
	// Optional: Show modification time or size in description line?
	// For compactness, maybe valid.
	if i.isDir {
		return "Directory"
	}
	return formatBytes(i.size)
}

func (i browserItem) FilterValue() string { return i.name }

func newBrowserModel() browserModel {
	cwd, _ := os.Getwd()
	
	// Init List
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Current Directory"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	
	ti := textinput.New()
	ti.Placeholder = "/path/to/dir"
	ti.CharLimit = 156
	ti.Width = 50
	ti.SetValue(cwd)

	ci := textinput.New()
	ci.Placeholder = "Command > (copy, move, delete...)"
	ci.CharLimit = 100
	ci.Width = 50
	ci.Prompt = ":"

	m := browserModel{
		currentDir:   cwd,
		mainList:     l,
		pathInput:    ti,
		commandInput: ci,
		activePane:   0,
		selected:     make(map[string]bool),
		sortMode:     0,    // Name
		sortAsc:      true, // Ascending
		showPreview:  true,
		history:      []string{cwd},
		historyIndex: 0,
		bookmarks:    make(map[string]string),
	}
	m.scanDirectory()
	return m
}

// scanDirectory reads disk and updates currentEntries
func (b *browserModel) scanDirectory() {
	entries, err := os.ReadDir(b.currentDir)
	if err != nil {
		b.err = err
		return
	}
	
	// Sort
	sort.Slice(entries, func(i, j int) bool {
		// Always Directories First
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		
		// Then Sort By Mode
		
		switch b.sortMode {
		case 1: // Size
			iInfo, _ := entries[i].Info()
			jInfo, _ := entries[j].Info()
			if iInfo != nil && jInfo != nil {
				return iInfo.Size() < jInfo.Size() == b.sortAsc
			}
		case 2: // Date
			iInfo, _ := entries[i].Info()
			jInfo, _ := entries[j].Info()
			if iInfo != nil && jInfo != nil {
				return iInfo.ModTime().Before(jInfo.ModTime()) == b.sortAsc
			}
		default: // Name (0)
			// String comparison for Name
			less := entries[i].Name() < entries[j].Name()
			if !b.sortAsc {
				return !less
			}
			return less
		}
		return entries[i].Name() < entries[j].Name() // Fallback
	})
	
	b.currentEntries = entries
	b.updateListItems()
}

// updateListItems regenerates list.Items based on b.currentEntries and b.selected
func (b *browserModel) updateListItems() {
	var items []list.Item

	// Add ".." if not root
	if filepath.Dir(b.currentDir) != b.currentDir {
		items = append(items, browserItem{
			name: "..", 
			path: filepath.Dir(b.currentDir), 
			isDir: true,
		})
	}

	for _, e := range b.currentEntries {
		if strings.HasPrefix(e.Name(), ".") { continue } // Skip hidden for now
		
		path := filepath.Join(b.currentDir, e.Name())
		info, _ := e.Info()
		
		size := int64(0)
		modTime := time.Now()
		if info != nil {
			size = info.Size()
			modTime = info.ModTime()
		}
		
		items = append(items, browserItem{
			name:     e.Name(),
			path:     path,
			isDir:    e.IsDir(),
			size:     size,
			modTime:  modTime,
			selected: b.selected[path],
		})
	}
	
	b.mainList.SetItems(items)
	b.mainList.Title = fmt.Sprintf("📂 %s", b.currentDir)
	b.updatePreview()
}

func (b *browserModel) updatePreview() {
	i := b.mainList.SelectedItem()
	if i == nil {
		b.previewContent = "No selection"
		return
	}
	
	item := i.(browserItem)
	// Calculate available space in preview pane
	w := (b.dims.width / 2) - 6
	h := b.dims.height - 8
	
	b.previewContent = generatePreview(item.path, w, h)
}

func (m MainModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.browser.currentDir == "" {
		m.browser = newBrowserModel()
	}

	// Handle Input Focus specifically
	if m.browser.activePane == 1 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Handle Go / Navigate
				path := m.browser.pathInput.Value()
				info, err := os.Stat(path)
				if err == nil && info.IsDir() {
					m.browser.currentDir = path
					m.browser.scanDirectory()
					m.browser.mainList.ResetSelected()
					m.browser.activePane = 0 // Switch focus to list
					m.browser.pathInput.Blur()
				}
				return m, nil
				
			case "tab":
				// Cycle focus
				m.browser.activePane = 0
				m.browser.pathInput.Blur()
				return m, nil
			
			case "esc":
				m.browser.activePane = 0
				m.browser.pathInput.Blur()
				return m, nil
			}
		}
		
		// If focused, pass all other messages to input and return
		m.browser.pathInput, cmd = m.browser.pathInput.Update(msg)
		return m, cmd
	}

	// Command Palette Handling
	if m.browser.activePane == 2 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Execute Command
				val := m.browser.commandInput.Value()
				// Simple parser
				parts := strings.Fields(val)
				if len(parts) > 0 {
					cmdStr := parts[0]
					// args := parts[1:]
					
					switch cmdStr {
					case "copy", "cp":
						// Copy logic (mock)
						// In real power user update: implement clipboard
					case "delete", "rm":
						// Delete selected
						for p := range m.browser.selected {
							os.RemoveAll(p) // Dangerous but requested "Power User"
						}
						m.browser.selected = make(map[string]bool)
						m.browser.scanDirectory()
					case "mkdir":
						if len(parts) > 1 {
							os.MkdirAll(filepath.Join(m.browser.currentDir, parts[1]), 0755)
							m.browser.scanDirectory()
						}
					case "touch":
						if len(parts) > 1 {
							f, _ := os.Create(filepath.Join(m.browser.currentDir, parts[1]))
							f.Close()
							m.browser.scanDirectory()
						}
					}
				}
				
				m.browser.commandInput.SetValue("")
				m.browser.activePane = 0
				m.browser.commandInput.Blur()
				return m, nil
				
			case "esc":
				m.browser.activePane = 0
				m.browser.commandInput.Blur()
				return m, nil
			}
		}
		m.browser.commandInput, cmd = m.browser.commandInput.Update(msg)
		return m, cmd
	}

	// Main List Handling
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.browser.activePane = 1
			m.browser.pathInput.Focus()
			return m, nil
			
		case "enter":
			// List Item Enter
			i := m.browser.mainList.SelectedItem()
			if i != nil {
				item := i.(browserItem)
				if item.isDir {
					// Push History
					if m.browser.historyIndex+1 < len(m.browser.history) {
						m.browser.history = m.browser.history[:m.browser.historyIndex+1]
					}
					m.browser.history = append(m.browser.history, item.path)
					m.browser.historyIndex = len(m.browser.history) - 1

					m.browser.currentDir = item.path
					m.browser.scanDirectory()
					m.browser.mainList.ResetSelected()
					m.browser.pathInput.SetValue(m.browser.currentDir)
				} else {
					if m.browser.selected[item.path] {
						delete(m.browser.selected, item.path)
					} else {
						m.browser.selected[item.path] = true
					}
					m.browser.updateListItems()
				}
			}
			
		case "right", "l":
			// If Dir, enter it like Yazi
			if i := m.browser.mainList.SelectedItem(); i != nil {
				item := i.(browserItem)
				if item.isDir {
					m.browser.currentDir = item.path
					m.browser.scanDirectory()
					m.browser.mainList.ResetSelected()
					m.browser.pathInput.SetValue(m.browser.currentDir)
				}
			}
			
			// Go Up
			parent := filepath.Dir(m.browser.currentDir)
			if parent != m.browser.currentDir {
				// Push History
				if m.browser.historyIndex+1 < len(m.browser.history) {
					m.browser.history = m.browser.history[:m.browser.historyIndex+1]
				}
				m.browser.history = append(m.browser.history, parent)
				m.browser.historyIndex = len(m.browser.history) - 1
				
				m.browser.currentDir = parent
				m.browser.scanDirectory()
				m.browser.mainList.ResetSelected() // Ideally find "previous" dir
				m.browser.pathInput.SetValue(m.browser.currentDir)
			}

		case " ":
			// Toggle
			if i := m.browser.mainList.SelectedItem(); i != nil {
				item := i.(browserItem)
				path := item.path
				if item.name != ".." {
					if m.browser.selected[path] {
						delete(m.browser.selected, path)
					} else {
						m.browser.selected[path] = true
					}
					m.browser.updateListItems()
				}
			}
			
		case "x":
			m.browser.recursive = !m.browser.recursive
			
		case "ctrl+a":
			// Select All Files in current view
			for _, it := range m.browser.mainList.Items() {
				item := it.(browserItem)
				if !item.isDir && item.name != ".." {
					m.browser.selected[item.path] = true
				}
			}
			m.browser.updateListItems()

		case "I":
			// Invert Selection
			for _, it := range m.browser.mainList.Items() {
				item := it.(browserItem)
				if !item.isDir && item.name != ".." {
					if m.browser.selected[item.path] {
						delete(m.browser.selected, item.path)
					} else {
						m.browser.selected[item.path] = true
					}
				}
			}
			m.browser.updateListItems()

		case "s":
			// Cycle Sort Mode
			m.browser.sortMode = (m.browser.sortMode + 1) % 3
			m.browser.scanDirectory()
			
		case "S":
			// Toggle Sort Asc/Desc
			m.browser.sortAsc = !m.browser.sortAsc
			m.browser.scanDirectory()

		case "p":
			// Toggle Preview
			m.browser.showPreview = !m.browser.showPreview
			// Force window resize event logic to recalculate widths
			m.browser.dims.width = m.width // trigger update next cycle or manually set widths here
			listWidth := m.width - 2
			if m.browser.showPreview {
				listWidth = (m.width / 2) - 2
			}
			m.browser.mainList.SetWidth(listWidth)

		case "i":
			// Focus Input
			m.browser.activePane = 1
			m.browser.pathInput.Focus()
			return m, nil

		case "m":
			// Mark/Bookmark
			if m.browser.currentDir != "" {
				m.browser.bookmarks[filepath.Base(m.browser.currentDir)] = m.browser.currentDir
			}

		// case "'":
		// 	// Jump to bookmark (Simple Implementation: just go to first bookmark for now or cycle?
		// 	// Real picker needs overlay. For MVP, let's skip complex UI or just cycle.)
		// 	// Let's implement cycle for now.
		// 	for _, path := range m.browser.bookmarks {
		// 		m.browser.currentDir = path
		// 		m.browser.scanDirectory()
		// 		break 
		// 	}

		case "alt+left":
			// Back History
			if m.browser.historyIndex > 0 {
				m.browser.historyIndex--
				if m.browser.historyIndex < len(m.browser.history) {
					m.browser.currentDir = m.browser.history[m.browser.historyIndex]
					m.browser.scanDirectory()
				}
			}

		case "alt+right":
			// Forward History
			if m.browser.historyIndex < len(m.browser.history)-1 {
				m.browser.historyIndex++
				m.browser.currentDir = m.browser.history[m.browser.historyIndex]
				m.browser.scanDirectory()
			}
		
		case "d":
			// Switch to Dashboard
			m.state = StateQueue
			return m, nil

		case "a":
			var paths []string
			// If nothing selected, add current item if file
			if len(m.browser.selected) == 0 {
				i := m.browser.mainList.SelectedItem()
				if i != nil {
					item := i.(browserItem)
					if !item.isDir {
						paths = append(paths, item.path)
					}
				}
			} else {
				for p := range m.browser.selected {
					paths = append(paths, p)
				}
			}
			
			if len(paths) > 0 {
				scannerJobs, _ := scanner.Scan(paths, m.browser.recursive)
				if len(scannerJobs.Images) > 0 {
					m.pipeline.AddFiles(scannerJobs.Images)
					m.state = StateQueue
					m.queue.Sync(m.pipeline.Jobs())
					m.browser.selected = make(map[string]bool)
					m.browser.updateListItems()
				}
			}
		
		case ":":
			m.browser.activePane = 2
			m.browser.commandInput.Focus()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.browser.dims.width = m.width
		m.browser.dims.height = m.height
		// Adjust list width
		listWidth := (m.width / 2) - 2
		m.browser.mainList.SetWidth(listWidth)
		m.browser.mainList.SetHeight(m.height - 6) // - input - status
	}
	
	// Pass updates to list
	m.browser.mainList, cmd = m.browser.mainList.Update(msg)
	cmds = append(cmds, cmd)
	// Update preview after selection change
	m.browser.updatePreview()
	
	return m, tea.Batch(cmds...)
}

// Helper needed if not exported, but formatBytes is in progress.go which is same package tui.
// It is unexported there `func formatBytes`. I should check if I can use it.
// It is in the same package `tui`. If I declared it in `progress.go` as `func formatBytes`, it is accessible here.

func (m MainModel) viewBrowser() string {
	// Yazi Layout: [ List (50%) ] [ Preview (50%) ]
	
	listWidth := (m.width / 2) - 2
	previewWidth := (m.width / 2) - 4 // minus borders
	
	// Adjust list width
	m.browser.mainList.SetWidth(listWidth)
	m.browser.mainList.SetHeight(m.height - 6) // - input - status
	
	// Styles
	activeBorder := highlightColor
	inactiveBorder := lipgloss.Color("240")
	
	listStyle := docStyle.Copy().
		Margin(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inactiveBorder)

	if m.browser.activePane == 0 {
		listStyle = listStyle.BorderForeground(activeBorder)
	}
	
	pathInputStyle := docStyle.Copy().Margin(0, 0, 0, 1). // Reduced margin to fit button
	    Border(lipgloss.RoundedBorder()).
	    BorderForeground(inactiveBorder)
	
	goBtnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("240")).
		Padding(0, 2).
		Margin(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inactiveBorder)

	if m.browser.activePane == 1 {
		pathInputStyle = pathInputStyle.BorderForeground(activeBorder)
		goBtnStyle = goBtnStyle.BorderForeground(activeBorder).Background(highlightColor).Foreground(lipgloss.Color("#000000")).Bold(true)
	}
	
	// Top Bar Logic: Path Input OR Command Palette
	var topBar string
	if m.browser.activePane == 2 {
		// Command Palette Mode
		cmdStyle := docStyle.Copy().
			Margin(0, 0, 0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF79C6")). // Pink/Purple for command
			Width(m.width - 4)
		
		topBar = cmdStyle.Render(m.browser.commandInput.View())
	} else {
		// Normal Path Input
		topBar = lipgloss.JoinHorizontal(lipgloss.Top,
			pathInputStyle.Render(m.browser.pathInput.View()),
			goBtnStyle.Render("➜ Go"),
		)
	}

	// Preview Pane
	previewStyle := docStyle.Copy().
		Width(previewWidth).
		Height(m.height - 6).
		Margin(0, 1).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inactiveBorder)
	
	// Generate Preview Content
	// If current item is image, maybe use some ASCII art placeholder or detail info?
	var previewText string
	i := m.browser.mainList.SelectedItem()
	if i != nil {
		item := i.(browserItem)
		// Big Header
		header := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Render(item.name)
			
		previewText = fmt.Sprintf("%s\n%s\n%s", header, strings.Repeat("─", previewWidth-4), m.browser.previewContent)
	} else {
		previewText = m.browser.previewContent
	}
	
	// Render
	var browserView string
	if m.browser.showPreview {
		browserView = lipgloss.JoinHorizontal(lipgloss.Top,
			listStyle.Render(m.browser.mainList.View()),
			previewStyle.Render(previewText),
		)
	} else {
		// Full Width List
		m.browser.mainList.SetWidth(m.width - 4)
		browserView = listStyle.Width(m.width - 4).Render(m.browser.mainList.View())
	}
	
	// Status Bar
	sortModeStr := "Name"
	if m.browser.sortMode == 1 { sortModeStr = "Size" }
	if m.browser.sortMode == 2 { sortModeStr = "Date" }
	sortDir := "ASC"
	if !m.browser.sortAsc { sortDir = "DESC" }
	
	statusText := fmt.Sprintf("Sel: %d | Sort: %s (%s) | [:] Cmd | [p] Preview | [m] Mark", 
		len(m.browser.selected), sortModeStr, sortDir)
	status := subtleStyle.Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		browserView,
		lipgloss.NewStyle().Margin(0, 2).Render(status),
	)
}
