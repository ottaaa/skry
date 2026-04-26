package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToastDuration is how long a toast stays visible before fading. Picked to
// match glow's statusMessageTimeout (3s) — long enough to be readable,
// short enough to not feel sticky.
const ToastDuration = 3 * time.Second

// ToastKind drives the toast's styling in the status bar.
type ToastKind int

const (
	// ToastInfo is the default neutral kind for "no commits yet" / "nothing
	// to copy" style messages — informational, not a result.
	ToastInfo ToastKind = iota
	// ToastSuccess marks a positive outcome ("auto-saved 12:34:56",
	// "copied: /foo"). Rendered in green.
	ToastSuccess
	// ToastError marks a failed action ("save failed: ...", "switch: ...").
	// Rendered with a red background so it's hard to miss in a passing
	// glance.
	ToastError
)

// toast is the ephemeral status displayed in the footer. seq lets a fresh
// toast invalidate any earlier expiry tick still in flight: when the
// toastExpiredMsg arrives we only clear if seq still matches, so a newer
// toast posted within the 3s window survives until its own timer fires.
type toast struct {
	text string
	kind ToastKind
	seq  int
}

// toastExpiredMsg is delivered ToastDuration after a toast was set. The
// handler is in app.go's Update and uses seq as a fenced cancel token —
// see toast.seq comment.
type toastExpiredMsg struct{ seq int }

// setToast installs a new toast and returns the Cmd that will eventually
// expire it. Callers should combine the returned Cmd with any other Cmds
// they were planning to return (typically via tea.Batch). Passing an empty
// text clears any active toast immediately and returns nil.
func (m *Model) setToast(text string, kind ToastKind) tea.Cmd {
	if text == "" {
		m.toast = toast{}
		return nil
	}
	m.toastSeq++
	m.toast = toast{text: text, kind: kind, seq: m.toastSeq}
	seq := m.toastSeq
	return tea.Tick(ToastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{seq: seq}
	})
}

// renderToast formats the active toast for the footer. Returns "" when no
// toast is active so the caller falls back to the regular hint text.
func (m Model) renderToast() string {
	if m.toast.text == "" {
		return ""
	}
	switch m.toast.kind {
	case ToastSuccess:
		return toastSuccessStyle.Render(" " + m.toast.text + " ")
	case ToastError:
		return toastErrorStyle.Render(" " + m.toast.text + " ")
	default:
		return toastInfoStyle.Render(m.toast.text)
	}
}

// Foreground/background pairs picked to read against the existing footer
// background. Success uses the same dark-green pair glow uses; error
// borrows the project's existing red accent.
var (
	toastSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89F0CB")).
				Background(lipgloss.Color("#1C8760")).
				Bold(true)

	toastErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD7D7")).
			Background(lipgloss.Color("#A23F4D")).
			Bold(true)

	toastInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7aa2f7")).
			Bold(true)
)
