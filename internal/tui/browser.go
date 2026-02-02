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
	
	activePane int // 0 = MainList, 1 = PathInput (Preview is passive)
	
	currentEntries []fs.DirEntry // Cache
	selected       map[string]bool
	recursive      bool
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
	icon := getIcon(i.name, i.isDir)
	
	prefix := "[ ] "
	if i.selected {
		prefix = "[✔] "
	}
	
	// Highlight directory names?
	name := i.name
	if i.isDir {
		name = name + "/"
	}
	
	return fmt.Sprintf("%s%s %s", prefix, icon, name)
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

	m := browserModel{
		currentDir: cwd,
		mainList:   l,
		pathInput:  ti,
		activePane: 0,
		selected:   make(map[string]bool),
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
	
	// Sort: Directories first, then Files
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
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
	b.updatePreview()
}

func (b *browserModel) updatePreview() {
	i := b.mainList.SelectedItem()
	if i == nil {
		b.previewContent = "Empty"
		return
	}
	
	item := i.(browserItem)
	if item.name == ".." {
		b.previewContent = "⬆️  Parent Directory"
		return
	}
	
	// Simple Preview Info
	content := fmt.Sprintf("Name: %s\nType: %s\nSize: %s\nModified: %s\nPath: %s",
		item.name,
		map[bool]string{true: "Directory", false: "File"}[item.isDir],
		formatBytes(item.size),
		item.modTime.Format("2006-01-02 15:04:05"),
		item.path,
	)
	
	if !item.isDir {
		// Maybe add image dimensions if we read headers? (Too slow for now)
		// ext := filepath.Ext(item.name)
		// content += fmt.Sprintf("\nExtension: %s", ext)
	} else {
		// Count items in dir?
		entries, _ := os.ReadDir(item.path)
		content += fmt.Sprintf("\n\nContents: %d items", len(entries))
	}
	
	b.previewContent = content
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
			
		case "left", "h", "backspace":
			// Go Up
			parent := filepath.Dir(m.browser.currentDir)
			if parent != m.browser.currentDir {
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
	
	topBar := lipgloss.JoinHorizontal(lipgloss.Top,
		pathInputStyle.Render(m.browser.pathInput.View()),
		goBtnStyle.Render("➜ Go"),
	)

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
	previewText := m.browser.previewContent
	
	// Render
	browserView := lipgloss.JoinHorizontal(lipgloss.Top,
		listStyle.Render(m.browser.mainList.View()),
		previewStyle.Render(previewText),
	)
	
	statusText := fmt.Sprintf("Selected: %d | Recursive: %v (X) | [Space] Toggle | [Enter/Right] Enter | [Back/Left] Up", len(m.browser.selected), m.browser.recursive)
	status := subtleStyle.Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		browserView,
		lipgloss.NewStyle().Margin(0, 2).Render(status),
	)
}
