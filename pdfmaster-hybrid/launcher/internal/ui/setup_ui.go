// launcher/internal/ui/setup_ui.go
//
// Bubble Tea TUI for the first-run setup experience.
// Shows a step-by-step progress display as the launcher
// extracts and configures everything.

package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/pdfmaster/launcher/internal/setup"
)

// ── Styles ────────────────────────────────────────────────────────────

var (
	styleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			PaddingLeft(2)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(2)

	styleStepDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	styleStepActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)

	styleStepPending = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleStepError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	styleBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63"))

	styleBarBg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("236"))

	styleMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(6)

	styleDone = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42")).
			PaddingLeft(2)

	styleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")).
			PaddingLeft(2)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 3).
			Width(58)
)

const barWidth = 40

// ── Messages ──────────────────────────────────────────────────────────

type progressMsg struct {
	stepIdx  int
	stepPct  int
	msg      string
}
type doneMsg    struct{ err error }
type tickMsg    time.Time

// ── Model ─────────────────────────────────────────────────────────────

type stepState int

const (
	stepPending stepState = iota
	stepActive
	stepDone
	stepFailed
)

type stepVM struct {
	title  string
	state  stepState
	pct    int
	msg    string
}

type model struct {
	plan       *setup.Plan
	installDir string
	steps      []stepVM
	activeIdx  int
	done       bool
	err        error
	spinner    int
	mu         sync.Mutex
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newModel(plan *setup.Plan, installDir string) model {
	steps := make([]stepVM, len(plan.Steps))
	for i, s := range plan.Steps {
		steps[i] = stepVM{title: s.Title, state: stepPending}
	}
	return model{plan: plan, installDir: installDir, steps: steps}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		runPlan(m.plan, m.installDir),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.spinner = (m.spinner + 1) % len(spinFrames)
		return m, tickCmd()

	case progressMsg:
		if msg.stepIdx < len(m.steps) {
			m.steps[msg.stepIdx].state  = stepActive
			m.steps[msg.stepIdx].pct    = msg.stepPct
			m.steps[msg.stepIdx].msg    = msg.msg
			m.activeIdx = msg.stepIdx
			// Mark previous steps done
			for i := 0; i < msg.stepIdx; i++ {
				if m.steps[i].state == stepActive {
					m.steps[i].state = stepDone
					m.steps[i].pct   = 100
				}
			}
		}
		return m, nil

	case doneMsg:
		m.done = true
		m.err  = msg.err
		// Mark all remaining steps
		for i := range m.steps {
			if msg.err == nil {
				if m.steps[i].state != stepDone {
					m.steps[i].state = stepDone
					m.steps[i].pct   = 100
				}
			} else {
				if m.steps[i].state == stepActive {
					m.steps[i].state = stepFailed
				}
			}
		}
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleBanner.Render("  PDFMaster  "))
	b.WriteString(styleSubtitle.Render("Setting up for first use…"))
	b.WriteString("\n\n")

	for i, step := range m.steps {
		icon := ""
		titleStyle := styleStepPending
		switch step.state {
		case stepDone:
			icon = styleStepDone.Render("  ✓ ")
			titleStyle = styleStepDone
		case stepActive:
			icon = styleStepActive.Render("  " + spinFrames[m.spinner] + " ")
			titleStyle = styleStepActive
		case stepFailed:
			icon = styleStepError.Render("  ✗ ")
			titleStyle = styleStepError
		default:
			icon = styleStepPending.Render("  ○ ")
		}

		b.WriteString(icon)
		b.WriteString(titleStyle.Render(step.title))

		// Show progress bar for active step
		if step.state == stepActive && i == m.activeIdx {
			b.WriteString("\n")
			filled := step.pct * barWidth / 100
			if filled > barWidth { filled = barWidth }
			bar := styleBar.Render(strings.Repeat("█", filled)) +
				styleBarBg.Render(strings.Repeat("░", barWidth-filled))
			b.WriteString(fmt.Sprintf("      [%s] %3d%%\n", bar, step.pct))
			if step.msg != "" {
				b.WriteString(styleMsg.Render(step.msg))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.done {
		if m.err == nil {
			box := styleBox.Render(
				styleDone.Render("✓  Setup complete!") + "\n\n" +
					styleSubtitle.Render("PDFMaster is ready.\n") +
					styleSubtitle.Render("Launching now…"),
			)
			b.WriteString(lipgloss.PlaceHorizontal(60, lipgloss.Left, box))
		} else {
			b.WriteString(styleError.Render(fmt.Sprintf(
				"  ✗  Setup failed: %v\n", m.err)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ── Commands ──────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func runPlan(plan *setup.Plan, installDir string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg, 64)
		go func() {
			err := setup.ExecutePlan(plan, installDir,
				func(stepIdx, stepTotal, pct int, msg string) {
					ch <- progressMsg{stepIdx: stepIdx, stepPct: pct, msg: msg}
				},
			)
			ch <- doneMsg{err: err}
		}()
		// Return the first message — subsequent ones handled via Subscribe
		return <-ch
	}
}

// RunSetupUI runs the full Bubble Tea setup screen.
// Returns nil on success, error if setup failed or was cancelled.
func RunSetupUI(plan *setup.Plan, installDir string) error {
	if !isTerminal() {
		return fmt.Errorf("no terminal — falling back to headless mode")
	}

	m := newModel(plan, installDir)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if fm, ok := finalModel.(model); ok {
		return fm.err
	}
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
