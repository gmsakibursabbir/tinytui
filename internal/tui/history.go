package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tinytui/tinytui/internal/history"
)

type historyModel struct {
	table table.Model
	mgr   *history.Manager
}

func newHistoryModel() historyModel {
	columns := []table.Column{
		{Title: "Time", Width: 20},
		{Title: "File", Width: 30},
		{Title: "Before", Width: 10},
		{Title: "After", Width: 10},
		{Title: "Saved", Width: 10},
	}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(10))
	
	// reuse styles...
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).Bold(false)
	t.SetStyles(s)
	
	mgr, _ := history.New()
	m := historyModel{
		table: t,
		mgr:   mgr,
	}
	m.refresh()
	return m
}

func (h *historyModel) refresh() {
	if h.mgr == nil { return }
	h.mgr.Load()
	recs := h.mgr.All()
	var rows []table.Row
	for i := len(recs)-1; i >= 0; i-- { // Reverse order
		r := recs[i]
		rows = append(rows, table.Row{
			r.Timestamp.Format("2006-01-02 15:04"),
			filepath.Base(r.File),
			fmt.Sprintf("%d", r.BeforeSize),
			fmt.Sprintf("%d", r.AfterSize),
			fmt.Sprintf("%d", r.SavedBytes),
		})
	}
	h.table.SetRows(rows)
}

func (m MainModel) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = StateBrowser
			return m, nil
		}
	}
	m.history.table, cmd = m.history.table.Update(msg)
	return m, cmd
}

func (m MainModel) viewHistory() string {
	return docStyle.Render(
		"History\n" + m.history.table.View() + "\n(Esc to go back)",
	)
}
