package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browserModel struct {
	currentDir string
	dirs       list.Model
	files      list.Model
	activePane int // 0 = Dirs, 1 = Files
	selected   map[string]bool
	err        error
}

// Item types for lists
type dirItem struct {
	name string
	path string
}
func (d dirItem) Title() string       { return d.name } // Folder icon?
func (d dirItem) Description() string { return "" }
func (d dirItem) FilterValue() string { return d.name }

type fileItem struct {
	name string
	path string
	size int64
}
func (f fileItem) Title() string       { return f.name }
func (f fileItem) Description() string { return fmt.Sprintf("%d bytes", f.size) } // Format nice later
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
	m.refresh()
	return m
}

func (b *browserModel) refresh() {
	entries, err := os.ReadDir(b.currentDir)
	if err != nil {
		b.err = err
		return
	}

	var dirs []list.Item
	var files []list.Item

	// Add ".." if not root
	if filepath.Dir(b.currentDir) != b.currentDir {
		dirs = append(dirs, dirItem{name: "..", path: filepath.Dir(b.currentDir)})
	}

	for _, e := range entries {
		if e.IsDir() {
			// Skip hidden?
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dirs = append(dirs, dirItem{name: e.Name() + "/", path: filepath.Join(b.currentDir, e.Name())})
		} else {
			// Check support
			// We handle extension check manually or use scanner helper
			// Scanner implementation used internal map, maybe expose it?
			// Or just duplicate logic: png/jpg/webp
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
				info, _ := e.Info()
				files = append(files, fileItem{name: e.Name(), path: filepath.Join(b.currentDir, e.Name()), size: info.Size()})
			}
		}
	}
	
	b.dirs.SetItems(dirs)
	b.files.SetItems(files)
}

func (m MainModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Ensure initialized (simple check, or do in Init)
	if m.browser.currentDir == "" {
		m.browser = newBrowserModel()
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.browser.activePane = (m.browser.activePane + 1) % 2
		case "enter":
			if m.browser.activePane == 0 {
				// Change directory
				i := m.browser.dirs.SelectedItem()
				if i != nil {
					d := i.(dirItem)
					m.browser.currentDir = d.path
					m.browser.refresh()
					// Reset cursor?
					m.browser.dirs.ResetSelected()
				}
			} else {
				// On file: toggle select? Or "Done"?
				// Prompt: "Space select/unselect". "Done -> add to queue".
				// Enter on file might just select? Or open?
				// Maybe Enter finishes selection? "Done -> add to queue".
				// Let's make Enter = Done if in File pane? Or explicit key?
				// "Done -> add to queue". Maybe a separate button or Key?
				// "A add files" (Global).
				// If in browser, maybe 'd' or Enter finishes?
				// Let's use 'a' to add selected and go to queue?
				// Or Enter toggles.
			}
		case "backspace":
			// Go up
			parent := filepath.Dir(m.browser.currentDir)
			if parent != m.browser.currentDir {
				m.browser.currentDir = parent
				m.browser.refresh()
			}
		case " ":
			if m.browser.activePane == 1 {
				// Toggle
				i := m.browser.files.SelectedItem()
				if i != nil {
					f := i.(fileItem)
					if m.browser.selected[f.path] {
						delete(m.browser.selected, f.path)
					} else {
						m.browser.selected[f.path] = true
					}
				}
			}
		case "ctrl+a":
			// Select all in current view
			if m.browser.activePane == 1 {
				for _, it := range m.browser.files.Items() {
					f := it.(fileItem)
					m.browser.selected[f.path] = true
				}
			}
		case "a":
			// Done -> Add to queue
			// Scan selected
			var paths []string
			for p := range m.browser.selected {
				paths = append(paths, p)
			}
			if len(paths) > 0 {
				m.pipeline.AddFiles(paths)
				m.state = StateQueue
				// Clear selection?
				m.browser.selected = make(map[string]bool)
			}
		}
	case tea.WindowSizeMsg:
		// Resize lists
		halfWidth := m.width / 2
		m.browser.dirs.SetWidth(halfWidth - 2)
		m.browser.files.SetWidth(halfWidth - 2)
		m.browser.dirs.SetHeight(m.height - 4)
		m.browser.files.SetHeight(m.height - 4)
	}

	// Update active list
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
	// Simple split view
	// Highlight active pane border
	
	leftStyle := docStyle.Copy().Width(m.width/2 - 4)
	rightStyle := docStyle.Copy().Width(m.width/2 - 4)
	
	if m.browser.activePane == 0 {
		leftStyle = leftStyle.Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("62"))
		rightStyle = rightStyle.Border(lipgloss.NormalBorder())
	} else {
		leftStyle = leftStyle.Border(lipgloss.NormalBorder())
		rightStyle = rightStyle.Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("62"))
	}
	
	// Custom delegate to show selection for files
	// For now just standard list
	
	// Hack to show selection status in title or description?
	// bubbles/list doesn't support dynamic item update easily without SetItems again.
	// But we render the list.
	// We might need a custom item delegate to render the [x].
	
	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(m.browser.dirs.View()),
		rightStyle.Render(m.browser.files.View()),
	)
}
