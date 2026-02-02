package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gmsakibursabbir/tinitui/internal/pipeline"
)

type progressModel struct {
	spinner  spinner.Model
	progress progress.Model
	active   bool
	done     bool
	// Cache stats
	total    int
	completed int
}

func newProgressModel() progressModel {
	// Custom Gradient: Pink to Purple (Waifu aesthetic)
	p := progress.New(
		progress.WithGradient("#FF7CCB", "#8888FF"),
		progress.WithoutPercentage(),
	)
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("213")) // Pinkish
	
	return progressModel{
		spinner:  s,
		progress: p,
	} 
}

// Tick command to drive updates if not driven by pipeline events solely?
// Pipeline sends events.
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m MainModel) updateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "x":
			// Cancel
			m.pipeline.Stop() // Or cancel current
			m.state = StateQueue // Go back
		case "p":
			isPaused := m.pipeline.TogglePause()
			// Maybe show paused indicator?
			// View will handle it if we check pipeline state?
			// Pipeline struct has isPaused. We need to expose it or job status update?
			// Pipeline pause logic updates workers.
			// Let's rely on View showing "Paused" if we can access it.
			// For now just toggle.
			_ = isPaused
		}
	case tickMsg:
		// Periodic update if needed, but we rely on pipeline updates
		// Actually spinner needs tick
		if m.progress.active && !m.progress.done {
			cmds = append(cmds, tickCmd())
		}
	case spinner.TickMsg:
		if m.progress.active {
			m.progress.spinner, cmd = m.progress.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	
	case *pipeline.Job:
		if m.progress.active {
			// Update stats
			// We need access to all jobs to calc total/completed?
			// Or pipeline provides stats?
			// Pipeline.Jobs() returns all.
			jobs := m.pipeline.Jobs()
			total := len(jobs)
			completed := 0
			for _, j := range jobs {
				if j.Status == pipeline.StatusDone || j.Status == pipeline.StatusFailed {
					completed++
				}
			}
			m.progress.total = total
			m.progress.completed = completed
			
			if completed == total && total > 0 {
				m.progress.done = true
				m.state = StateHistory // Auto switch to history? Or just show done.
				// Let's stay in Compress but show Done.
			}
		}
	}
	
	return m, tea.Batch(cmds...)
}

func waitForPipeline(p *pipeline.Pipeline) tea.Cmd {
	return func() tea.Msg {
		job := <-p.Updates()
		if job == nil {
			return nil // Channel closed
		}
		return job
	}
}

// Ensure MainModel generic update handles *pipeline.Job msg
// We need to add that logic to MainModel.Update or here.
// But MainModel.Update delegates to sub-update functions.
// Let's add handling in MainModel.Update for *pipeline.Job and pass it down.

// Helper to format bytes
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m MainModel) viewProgress() string {
	// Calculate stats from pipeline
	jobs := m.pipeline.Jobs()
	total := len(jobs)
	if total == 0 {
		return docStyle.Render("No items in queue.")
	}

	completed := 0
	var totalOrig, totalComp, totalSaved int64

	
	// Process jobs for stats and recent log
	// Iterate in reverse for recent log? Or just separate loop.
	for _, j := range jobs {
		if j.Status == pipeline.StatusDone {
			completed++
			totalOrig += j.OriginalSize
			totalComp += j.CompressedSize
			totalSaved += j.SavedBytes
		} else if j.Status == pipeline.StatusFailed {
			completed++
		}
	}

	// Completion View
	if m.progress.done {
		percentSaved := 0.0
		if totalOrig > 0 {
			percentSaved = (float64(totalSaved) / float64(totalOrig)) * 100
		}
		
		summary := fmt.Sprintf(
			"COMPLETED!\n\n"+
			"Total Files:      %d\n"+
			"Original Size:    %s\n"+
			"Compressed Size:  %s\n"+
			"Total Savings:    %s (%.1f%%)\n\n"+
			"Press [Esc] to return to Dashboard",
			total,
			formatBytes(totalOrig),
			formatBytes(totalComp),
			formatBytes(totalSaved),
			percentSaved,
		)
		
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Align(lipgloss.Center).
			Render(summary)
	}

	// Active Processing View
	pad := "\n"
	
	// Collect recent logs (last 5 done)
	count := 0
	activeFile := "Waiting..."
	
	// Find active
	for _, j := range jobs {
		if j.Status == pipeline.StatusProcessing {
			activeFile = "Processing: " + filepath.Base(j.FilePath)
			break
		}
	}
	
	// Recent finished
	logBuilder := strings.Builder{}
	logBuilder.WriteString("Recent Activity:\n")
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if j.Status == pipeline.StatusDone {
			if count < 5 {
				logBuilder.WriteString(fmt.Sprintf("[✓] %s: -%s (-%.0f%%)\n", 
					filepath.Base(j.FilePath), 
					formatBytes(j.SavedBytes), 
					j.SavedPercent))
				count++
			}
		} else if j.Status == pipeline.StatusFailed {
			if count < 5 {
				logBuilder.WriteString(fmt.Sprintf("[X] %s: Failed\n", filepath.Base(j.FilePath)))
				count++
			}
		}
	}

	prog := m.progress.progress.ViewAs(float64(completed) / float64(total))
	
	return docStyle.Render(
		"Compressing Assets..." + "\n\n" +
		m.progress.spinner.View() + " " + activeFile + "\n\n" +
		prog + "\n\n" +
		fmt.Sprintf("%d / %d processed", completed, total) + "\n" +
		pad + logBuilder.String() + "\n" +
		"(Press 'x' to cancel)",
	)
}
