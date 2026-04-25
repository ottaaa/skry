package modal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Modal is a self-contained overlay that owns its own input.
type Modal interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Modal, tea.Cmd)
	View(width, height int) string
}

// Previewer is an optional capability: search-style modals that highlight a
// "current selection" can implement this so the host app renders a live
// preview of the file under the cursor in a dedicated pane. PreviewPath
// returns "" when nothing is selectable yet (empty result list, etc.) — the
// host treats that as "no preview". PreviewLine is 1-based; 0 means show
// the file from the top.
type Previewer interface {
	PreviewPath() string
	PreviewLine() int
}

var Frame = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7aa2f7")).
	Padding(0, 1)
