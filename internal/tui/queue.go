package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gmsakibursabbir/tinitui/internal/pipeline"
)

type queueModel struct {
	table table.Model
}

func newQueueModel() queueModel {
	columns := []table.Column{
		{Title: "File", Width: 30},
		{Title: "Status", Width: 12},
		{Title: "Size", Width: 12},
		{Title: "After", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorComment)).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color(ColorCyan))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color(ColorCyan)).
		Background(lipgloss.Color(ColorSelectionBg)).
		Bold(true)
	t.SetStyles(s)

	return queueModel{table: t}
}

// Sync updates the table rows from the provided jobs
func (m *queueModel) Sync(jobs []*pipeline.Job) {
	rows := make([]table.Row, len(jobs))
	for i, j := range jobs {
		after := "-"
		status := string(j.Status)

		if j.Status == pipeline.StatusProcessing {
			status = "⏳ Compressing"
		} else if j.Status == pipeline.StatusDone {
			if j.CompressedSize > 0 {
				after = formatBytes(j.CompressedSize)
				// Calculate savings %
				saved := 100.0 - (float64(j.CompressedSize)/float64(j.OriginalSize))*100.0
				status = fmt.Sprintf("✔ %.1f%% Saved", saved)
			} else {
				status = "✔ Done"
			}
		} else if j.Status == pipeline.StatusFailed {
			status = "❌ Failed"
		}

		rows[i] = table.Row{
			filepath.Base(j.FilePath),
			status,
			formatBytes(j.OriginalSize),
			after,
		}
	}
	m.table.SetRows(rows)
}

func (m MainModel) updateQueue(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Run compression
			m.state = StateCompress
			m.pipeline.Start() 
			return m, nil 
		case "d":
			if len(m.queue.table.Rows()) > 0 {
				idx := m.queue.table.Cursor()
				jobs := m.pipeline.Jobs() 
				if idx >= 0 && idx < len(jobs) {
					job := jobs[idx]
					m.pipeline.RemoveJob(job.FilePath)
				}
			}
		case "c":
			m.pipeline.ClearCompleted()
		}
	
	}
	
	// Sync table with pipeline jobs
	m.queue.Sync(m.pipeline.Jobs())

	m.queue.table, cmd = m.queue.table.Update(msg)
	return m, cmd
}

func (m MainModel) viewQueue() string {
	jobs := m.pipeline.Jobs()
	total := len(jobs)
	processed := 0
	savedBytes := int64(0)
	
	for _, j := range jobs {
		if j.Status == pipeline.StatusDone {
			processed++
			if j.CompressedSize > 0 {
				savedBytes += (j.OriginalSize - j.CompressedSize)
			}
		}
	}
	
	// Stats Line
	stats := fmt.Sprintf(" Processed: %d/%d ", processed, total)
	if savedBytes > 0 {
		stats += fmt.Sprintf("| Saved: %s ", formatBytes(savedBytes))
	}
	statsView := styleStatusMode.Copy().Background(lipgloss.Color(ColorGreen)).Render(stats)

	// Ensure table dimensions
	m.queue.table.SetWidth(m.width - 4)
	m.queue.table.SetHeight(m.height - 6)
	
	// Create a view
	tView := stylePaneActive.Width(m.width - 4).Height(m.height - 6).Render(m.queue.table.View())
	
	return lipgloss.JoinVertical(lipgloss.Left, 
		lipgloss.JoinHorizontal(lipgloss.Center, styleHeaderPath.Render("Queue"), statsView),
		tView,
		styleDim.Render(" [R] Run | [D] Delete | [C] Clear Completed"),
	)
}
