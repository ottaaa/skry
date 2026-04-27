package helpui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/modal"
)

type Modal struct{}

func New() modal.Modal { return &Modal{} }

func (m *Modal) Init() tea.Cmd { return nil }

func (m *Modal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, func() tea.Msg { return events.CloseModalMsg{} }
	}
	return m, nil
}

const body = `skry key bindings

Global
  q / Ctrl+Q / Ctrl+C   quit
  Tab / ← / →           switch pane
  p                     file search palette
  r                     recent files
  F                     project grep
  L                     commit history (log)
  b                     branch switch
  w                     worktree switch
  t                     toggle flat changed list
  I                     toggle showing .gitignore'd files
  y                     copy current file path
  [ ]                   resize left pane
  ?                     this help

Tree
  ↑ / ↓ or k / j        navigate
  enter / space / l     open or expand
  h                     collapse
  g / G                 first / last
  /                     quick filter

Editor (View / SplitDiff / CommitDiff)
  ↑ / ↓ or k / j        scroll
  pgdn / space          page down
  pgup                  page up
  g / G                 top / bottom
  d                     toggle View ↔ SplitDiff (working tree)
  B                     toggle Blame
  /                     find in file (View)
  n / N                 next / prev match (View) or next / prev hunk (Diff)
  i / e                 enter Edit mode
  Esc                   back to tree

Edit mode (autosaves 1.5s after each edit)
  Esc                   commit & back to View
  Ctrl+S                save now
  Ctrl+Z / Ctrl+Y       undo / redo
  Ctrl+A / Ctrl+E       home / end
  Home / End            home / end
  PgUp / PgDn           page

Search modals (p / r / F) preview pane
  PgUp / PgDn           scroll preview
  Alt+j / Alt+k         scroll preview down / up
  Alt+↑ / Alt+↓         shrink / grow preview pane`

func (m *Modal) View(width, height int) string {
	w := max(width-8, 60)
	h := height - 4
	lines := strings.Split(body, "\n")
	if h < len(lines)+2 {
		h = len(lines) + 2
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render(lines[0])
	rest := strings.Join(lines[1:], "\n")
	return modal.Frame.Width(w).Height(h).Render(heading + "\n" + rest + "\n\n" + lipgloss.NewStyle().Faint(true).Render("press any key to close"))
}
