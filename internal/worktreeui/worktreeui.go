package worktreeui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
	"github.com/ottaaa/skry/internal/modal"
)

type Modal struct {
	worktrees []git.Worktree
	current   string
	cursor    int
}

func New(worktrees []git.Worktree, current string) modal.Modal {
	m := &Modal{worktrees: worktrees, current: current}
	for i, w := range worktrees {
		if w.Path == current {
			m.cursor = i
		}
	}
	return m
}

func (m *Modal) Init() tea.Cmd { return nil }

func (m *Modal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc":
		return m, func() tea.Msg { return events.CloseModalMsg{} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.worktrees)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < len(m.worktrees) {
			path := m.worktrees[m.cursor].Path
			return m, func() tea.Msg { return events.SwitchWorktreeMsg{Path: path} }
		}
	}
	return m, nil
}

func (m *Modal) View(width, height int) string {
	w := max(width-8, 50)
	h := min(max(len(m.worktrees)+3, 6), height-6)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render("Switch Worktree"))
	b.WriteByte('\n')
	for i, wt := range m.worktrees {
		marker := "  "
		if wt.Path == m.current {
			marker = "● "
		}
		branch := wt.Branch
		if branch == "" {
			if wt.Detached {
				branch = "(detached)"
			} else if wt.Bare {
				branch = "(bare)"
			}
		}
		line := marker + branch + "  " + wt.Path
		if lipgloss.Width(line) > w {
			line = line[:w-1] + "…"
		} else {
			line += strings.Repeat(" ", w-lipgloss.Width(line))
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(line)
		}
		b.WriteString(line)
		if i < len(m.worktrees)-1 {
			b.WriteByte('\n')
		}
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}
