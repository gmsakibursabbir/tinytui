package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gmsakibursabbir/tinitui/internal/scanner"
)

type browserModel struct {
	currentDir string
	dims       struct { width, height int }
	
	dirs       list.Model
	files      list.Model
	activePane int // 0 = Dirs, 1 = Files
	
	currentEntries []fs.DirEntry // Cache to avoid re-reading disk on selection
	selected       map[string]bool
	recursive      bool
	err            error
}

// Item types for lists
type dirItem struct {
	name string
	path string
	selected bool
}
func (d dirItem) Title() string { 
	if d.name == ".." { return d.name } // Don't select parent
	prefix := "[ ] "
	if d.selected { prefix = "[x] " }
	return prefix + d.name 
}
func (d dirItem) Description() string { return "" }
func (d dirItem) FilterValue() string { return d.name }

type fileItem struct {
	name string
	path string
	size int64
	selected bool
}
func (f fileItem) Title() string {
	prefix := "[ ] "
	if f.selected { prefix = "[x] " }
	return prefix + f.name
}
func (f fileItem) Description() string { return fmt.Sprintf("%d bytes", f.size) }
func (f fileItem) FilterValue() string { return f.name }

func newBrowserModel() browserModel {
	cwd, _ := os.Getwd()
	
	// Init Lists
	dList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	dList.Title = "Directories"
	dList.SetShowHelp(false)
	
	fList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	fList.Title = "Files"
	fList.SetShowHelp(false)

	m := browserModel{
		currentDir: cwd,
		dirs:       dList,
		files:      fList,
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
	b.currentEntries = entries
	b.updateListItems()
}

// updateListItems regenerates list.Items based on b.currentEntries and b.selected
func (b *browserModel) updateListItems() {
	var dirs []list.Item
	var files []list.Item

	// Add ".." if not root
	if filepath.Dir(b.currentDir) != b.currentDir {
		dirs = append(dirs, dirItem{name: "..", path: filepath.Dir(b.currentDir)})
	}

	for _, e := range b.currentEntries {
		path := filepath.Join(b.currentDir, e.Name())
		isSelected := b.selected[path]

		if e.IsDir() {
			if strings.HasPrefix(e.Name(), ".") { continue }
			dirs = append(dirs, dirItem{
				name: e.Name() + "/", 
				path: path,
				selected: isSelected,
			})
		} else {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
				info, _ := e.Info()
				files = append(files, fileItem{
					name: e.Name(), 
					path: path, 
					size: info.Size(),
					selected: isSelected,
				})
			}
		}
	}
	
	// Preserve cursor positions if possible? 
	// SetItems resets items but keeps index if valid.
	b.dirs.SetItems(dirs)
	b.files.SetItems(files)
}

func (m MainModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.browser.currentDir == "" {
		m.browser = newBrowserModel()
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "right", "l": // Support Arrow Right and 'l' (vim)
			if m.browser.activePane == 0 {
				m.browser.activePane = 1
			} else if msg.String() == "tab" {
				m.browser.activePane = 0
			}
		case "left", "h": // Support Arrow Left and 'h' (vim)
			if m.browser.activePane == 1 {
				m.browser.activePane = 0
			}
		case "enter":
			if m.browser.activePane == 0 {
				i := m.browser.dirs.SelectedItem()
				if i != nil {
					d := i.(dirItem)
					if d.name == ".." {
						m.browser.currentDir = d.path
						m.browser.scanDirectory()
						m.browser.dirs.ResetSelected()
					} else {
						m.browser.currentDir = d.path
						m.browser.scanDirectory()
						m.browser.dirs.ResetSelected()
					}
				}
			} else {
			    // Toggle selection on Enter for files
				i := m.browser.files.SelectedItem()
				if i != nil {
					f := i.(fileItem)
					if m.browser.selected[f.path] {
						delete(m.browser.selected, f.path)
					} else {
						m.browser.selected[f.path] = true
					}
					m.browser.updateListItems()
				}
			}

			if m.browser.activePane == 0 {
				// Prevent recursive toggle on Enter? No, 'x' does that.
				// m.browser.recursive = !m.browser.recursive 
			}
		case "x":
			if m.browser.activePane == 0 {
				m.browser.recursive = !m.browser.recursive
			} else {
			    // Allow 'x' to mark as well? No, stick to Space/Enter.
			}
		case "backspace":
			parent := filepath.Dir(m.browser.currentDir)
			if parent != m.browser.currentDir {
				m.browser.currentDir = parent
				m.browser.scanDirectory()
			}
		case " ":
			// Toggle Selection
			var path string
			if m.browser.activePane == 1 {
				i := m.browser.files.SelectedItem()
				if i != nil { path = i.(fileItem).path }
			} else {
				i := m.browser.dirs.SelectedItem()
				if i != nil { 
					d := i.(dirItem)
					if d.name != ".." { path = d.path }
				}
			}

			if path != "" {
				if m.browser.selected[path] {
					delete(m.browser.selected, path)
				} else {
					m.browser.selected[path] = true
				}
				m.browser.updateListItems() // Re-render to show [x]
			}

		case "ctrl+a":
			if m.browser.activePane == 1 {
				// Select all visible files
				for _, it := range m.browser.files.Items() {
					f := it.(fileItem)
					m.browser.selected[f.path] = true
				}
				m.browser.updateListItems()
			}

		case "a":
			// Done -> Add to queue
			var paths []string
			
			// If nothing selected, try to add currently highlighted item
			if len(m.browser.selected) == 0 {
				if m.browser.activePane == 1 {
					i := m.browser.files.SelectedItem()
					if i != nil { paths = append(paths, i.(fileItem).path) }
				} else {
					i := m.browser.dirs.SelectedItem()
					if i != nil { 
						d := i.(dirItem)
						if d.name != ".." { paths = append(paths, d.path) }
					}
				}
			} else {
				for p := range m.browser.selected {
					paths = append(paths, p)
				}
			}
			
			if len(paths) > 0 {
				res, _ := scanner.Scan(paths, m.browser.recursive)
				if len(res.Images) > 0 {
					m.pipeline.AddFiles(res.Images)
					m.state = StateQueue
					m.browser.selected = make(map[string]bool)
					m.browser.updateListItems()
				}
			}
		}
	case tea.WindowSizeMsg:
		m.browser.dims.width = m.width
		m.browser.dims.height = m.height
		halfWidth := m.width / 2
		m.browser.dirs.SetWidth(halfWidth - 2)
		m.browser.files.SetWidth(halfWidth - 2)
		m.browser.dirs.SetHeight(m.height - 4)
		m.browser.files.SetHeight(m.height - 4)
	}

	if m.browser.activePane == 0 {
		m.browser.dirs, cmd = m.browser.dirs.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.browser.files, cmd = m.browser.files.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m MainModel) viewBrowser() string {
    // Compact layout: margin 0 or 1
	leftStyle := docStyle.Copy().Width(m.width/2 - 2).Margin(0, 1)
	rightStyle := docStyle.Copy().Width(m.width/2 - 2).Margin(0, 1)
	
	activeBorder := lipgloss.Color("62") // Purple
	inactiveBorder := lipgloss.Color("240") // Grey

	if m.browser.activePane == 0 {
		leftStyle = leftStyle.Border(lipgloss.RoundedBorder()).BorderForeground(activeBorder)
		rightStyle = rightStyle.Border(lipgloss.RoundedBorder()).BorderForeground(inactiveBorder)
	} else {
		leftStyle = leftStyle.Border(lipgloss.RoundedBorder()).BorderForeground(inactiveBorder)
		rightStyle = rightStyle.Border(lipgloss.RoundedBorder()).BorderForeground(activeBorder)
	}

	// Status line
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(0, 2)
	status := statusStyle.Render(fmt.Sprintf("Selected: %d | Recursive: %v (X) | [Space/Ent] Toggle | [A] Add | [Esc] Dashboard", len(m.browser.selected), m.browser.recursive))
	
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			leftStyle.Render(m.browser.dirs.View()),
			rightStyle.Render(m.browser.files.View()),
		),
		status,
	)
}
