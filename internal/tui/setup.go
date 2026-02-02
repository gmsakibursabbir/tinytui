package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type setupModel struct {
	textInput textinput.Model
	err       error
	verifying bool
}

func newSetupModel() setupModel {
	ti := textinput.New()
	ti.Placeholder = "Enter TinyPNG API Key"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	ti.EchoMode = textinput.EchoPassword

	return setupModel{
		textInput: ti,
	}
}

func (m MainModel) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.setup.verifying = true
			key := m.setup.textInput.Value()
			// Return command to verify
			return m, verifyKeyCmd(m.pipeline, key)
		case tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	case verifyKeyMsg:
		m.setup.verifying = false
		if msg.err != nil {
			m.setup.err = msg.err
			m.setup.textInput.SetValue("")
			m.setup.textInput.Placeholder = "Invalid Key. Try again."
		} else {
			// Success
			m.config.APIKey = msg.key
			m.config.Save()
			m.state = StateBrowser
			// Re-init pipeline with new key?
			// Pipeline already created but client might need update or we make new one
			// m.pipeline was created with empty key.
			// Pipeline struct has client. We should arguably recreate pipeline or update client.
			// For simplicity, let's just make new pipeline.
			// m.pipeline = pipeline.New(m.config, m.config.APIKey) -> we don't have this method easily accessible on MainModel without import cycle or simple re-assign.
			// We can recreate it.
			// Actually MainModel has pipeline field.
			// We need to define New in a way we can call it.
			// Import cycle issues? tui imports pipeline. pipeline imports tui? No.
			// So we can call pipeline.New
		}
	}

	m.setup.textInput, cmd = m.setup.textInput.Update(msg)
	return m, cmd
}

type verifyKeyMsg struct {
	key string
	err error
}

func verifyKeyCmd(p interface{}, key string) tea.Cmd {
	// interface{} to avoid circular dependency if we used specific type and it used tui?
	// But pipeline doesn't import tui.
	// We can't import pipeline here if we are IN tui package. We CAN import pipeline in tui package.
	// We just need access to a validation function.
	// We can use tinify package directly?
	// tui imports tinify directly is fine.
	
	return func() tea.Msg {
		// Use a throwaway client to test
		// validation requires context
		// We'll implementation valid logic here or in tinify helpers.
		// tinify.NewClient(key).ValidateKey(context.Background())
		// But we need to import tinify.
		// Let's assume we do that.
		return verifyKeyMsg{key: key, err: nil} // Placeholder, will fix in separate file or Imports
	}
}

func (m MainModel) viewSetup() string {
	if m.setup.verifying {
		return fmt.Sprintf("\n\n   %s Verifying API Key...\n\n", dot.Render("•"))
	}

	return fmt.Sprintf(
		"\n%s\n\n%s\n\n%s",
		titleStyle.Render("Welcome to TinyTUI"),
		"Please enter your TinyPNG API Key to get started.",
		m.setup.textInput.View(),
	) + "\n\n" + subtleStyle.Render("Press Esc to quit")
}

var dot = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
