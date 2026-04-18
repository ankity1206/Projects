// internal/progress/progress.go
// Terminal progress bars and spinners using Bubble Tea.

package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleBar      = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	stylePercent  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleMsg      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

const barWidth = 36

// Bar is a simple terminal progress bar.
type Bar struct {
	mu      sync.Mutex
	out     io.Writer
	label   string
	current int
	total   int
	msg     string
	done    bool
	start   time.Time
}

// NewBar creates a new progress bar.
func NewBar(label string) *Bar {
	return &Bar{
		out:   os.Stderr,
		label: label,
		start: time.Now(),
	}
}

// Update sets current progress (thread-safe).
func (b *Bar) Update(current, total int, msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = current
	b.total   = total
	if msg != "" {
		b.msg = msg
	}
	b.render()
}

// Done marks completion and prints a final line.
func (b *Bar) Done(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
	elapsed := time.Since(b.start)
	fmt.Fprintf(b.out, "\r%s\n",
		styleSuccess.Render(fmt.Sprintf("✓ %s — %s (%.0f ms)",
			b.label, msg, float64(elapsed.Milliseconds()))))
}

// Fail prints a failure line.
func (b *Bar) Fail(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fmt.Fprintf(b.out, "\r%s\n",
		styleError.Render(fmt.Sprintf("✗ %s — %v", b.label, err)))
}

func (b *Bar) render() {
	pct := 0.0
	if b.total > 0 {
		pct = float64(b.current) / float64(b.total)
	}
	filled := int(pct * barWidth)
	if filled > barWidth { filled = barWidth }

	bar := styleBar.Render(strings.Repeat("█", filled)) +
		styleDim.Render(strings.Repeat("░", barWidth-filled))

	fmt.Fprintf(b.out, "\r%s [%s] %s %s",
		styleMsg.Render(b.label),
		bar,
		stylePercent.Render(fmt.Sprintf("%3.0f%%", pct*100)),
		styleDim.Render(b.msg),
	)
}

// ── BridgeCallback returns a bridge.ProgressFunc that drives a Bar. ──

// MakeBridgeCb returns a progress callback compatible with bridge.ProgressFunc.
func MakeBridgeCb(bar *Bar) func(int, int, string) {
	return func(current, total int, msg string) {
		bar.Update(current, total, msg)
	}
}

// ── Human-readable helpers ────────────────────────────────────────────

// HumanBytes formats bytes as B/KB/MB/GB.
func HumanBytes(b int64) string {
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

// HumanMs formats milliseconds as "Xms" or "X.Xs".
func HumanMs(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.0f ms", ms)
	}
	return fmt.Sprintf("%.1f s", ms/1000)
}
