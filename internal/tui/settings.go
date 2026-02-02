package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tinytui/tinitui/internal/config"
)

type settingsModel struct {
	cursor  int
	inputs  []textinput.Model // For API Key
	editing bool              // Are we editing a text input?
}

func newSettingsModel() settingsModel {
	ti := textinput.New()
	ti.Placeholder = "API Key"
	ti.CharLimit = 64
	ti.EchoMode = textinput.EchoPassword
	ti.Width = 30

	return settingsModel{
		cursor: 0,
		inputs: []textinput.Model{ti},
	}
}

// Ensure inputs are synced with config when entering settings
func (m MainModel) syncSettingsInputs() {
	if m.settings.inputs[0].Value() == "" {
		m.settings.inputs[0].SetValue(m.config.APIKey)
	}
}

func (m MainModel) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Options:
	// 0. API Key (TextInput)
	// 1. Concurrency (1-4)
	// 2. Mascot Type (panda, waifu1, waifu2)
	// 3. Output Mode (replace, directory)
	// 4. Preserve Metadata (bool)
	// 5. Back

	// If editing text input
	if m.settings.editing {
		var cmd tea.Cmd
		if m.settings.cursor == 0 {
			m.settings.inputs[0], cmd = m.settings.inputs[0].Update(msg)
		}
		
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEsc {
				m.settings.editing = false
				m.config.APIKey = m.settings.inputs[0].Value()
				m.config.Save()
				return m, nil
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.settings.cursor--
			if m.settings.cursor < 0 {
				m.settings.cursor = 5
			}
		case "down", "j":
			m.settings.cursor++
			if m.settings.cursor > 5 {
				m.settings.cursor = 0
			}
		case "enter", " ":
			switch m.settings.cursor {
			case 0: // API Key
				m.settings.editing = true
				m.settings.inputs[0].Focus()
				return m, nil
			case 1: // Concurrency
				m.config.Concurrency++
				if m.config.Concurrency > 4 {
					m.config.Concurrency = 1
				}
				// Update pipeline? 
				// m.pipeline.Configure(m.config.Concurrency) // Main model should do this or we do it here
				if m.pipeline != nil {
					m.pipeline.Configure(m.config.Concurrency)
				}
			case 2: // Mascot Type
				types := []string{"panda", "waifu1", "waifu2"}
				idx := 0
				for i, t := range types {
					if t == m.config.MascotType {
						idx = i
						break
					}
				}
				idx = (idx + 1) % len(types)
				m.config.MascotType = types[idx]
				// Also ensure Mascot is On if we switch type?
				m.config.Mascot = config.MascotOn
			case 3: // Output Mode
				if m.config.OutputMode == "replace" {
					m.config.OutputMode = "directory"
				} else {
					m.config.OutputMode = "replace"
				}
			case 4: // Metadata
				m.config.Metadata = !m.config.Metadata
			case 5: // Back
				m.state = StateBrowser
			}
			m.config.Save()
		}
	}
	return m, nil
}

func (m MainModel) viewSettings() string {
	s := strings.Builder{}
	s.WriteString("Settings\n\n")

	// Helper
	renderItem := func(i int, name, val string) {
		cursor := "  "
		if m.settings.cursor == i {
			cursor = "> "
		}
		
		style := lipgloss.NewStyle()
		if m.settings.cursor == i {
			style = style.Foreground(lipgloss.Color("205")).Bold(true)
		}
		
		line := fmt.Sprintf("%s%s: %s", cursor, name, val)
		s.WriteString(style.Render(line) + "\n")
	}

	// 0 API Key
	apiKeyDisplay := "(Set)"
	if m.config.APIKey == "" {
		apiKeyDisplay = "(Missing)"
	}
	if m.settings.editing && m.settings.cursor == 0 {
		apiKeyDisplay = m.settings.inputs[0].View()
	} else if len(m.config.APIKey) > 4 {
		apiKeyDisplay = "..." + m.config.APIKey[len(m.config.APIKey)-4:]
	}
	renderItem(0, "API Key", apiKeyDisplay)

	// 1 Concurrency
	renderItem(1, "Concurrency", fmt.Sprintf("%d Workers", m.config.Concurrency))

	// 2 Mascot Type
	renderItem(2, "Mascot Type", m.config.MascotType)

	// 3 Output Mode
	renderItem(3, "Output Mode", m.config.OutputMode)

	// 4 Metadata
	metaVal := "OFF"
	if m.config.Metadata { metaVal = "ON" }
	renderItem(4, "Preserve Metadata", metaVal)

	// 5 Back
	renderItem(5, "Back", "")

	help := "(Space/Enter to change)"
	if m.settings.editing {
		help = "(Enter to save, Esc to cancel)"
	}
	
	mascotPreview := ""
	if m.config.ShouldShowMascot(m.width) {
		mascotPreview = getMascot(m.config.MascotType, false)
	}

	content := docStyle.Render(s.String() + "\n" + help)
	
	if mascotPreview != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, content, lipgloss.NewStyle().MarginLeft(4).Render("Preview:\n"+mascotPreview))
	}

	return content
}
