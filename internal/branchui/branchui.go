package branchui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/peek/internal/events"
	"github.com/ottaaa/peek/internal/git"
	"github.com/ottaaa/peek/internal/modal"
)

type Modal struct {
	branches []git.Branch
	cursor   int

	dirty         bool
	pendingBranch string // when dirty and user hit enter, we await y/n confirmation
}

func New(branches []git.Branch, dirty bool) modal.Modal {
	m := &Modal{branches: branches, dirty: dirty}
	for i, b := range branches {
		if b.Current {
			m.cursor = i
			break
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

	if m.pendingBranch != "" {
		switch km.String() {
		case "y", "Y":
			name := m.pendingBranch
			m.pendingBranch = ""
			return m, func() tea.Msg { return events.SwitchBranchMsg{Name: name, Force: true} }
		case "n", "N", "esc":
			m.pendingBranch = ""
			return m, nil
		}
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
		if m.cursor < len(m.branches)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor >= len(m.branches) {
			return m, nil
		}
		target := m.branches[m.cursor]
		if target.Current {
			return m, nil
		}
		if m.dirty {
			m.pendingBranch = target.Name
			return m, nil
		}
		return m, func() tea.Msg { return events.SwitchBranchMsg{Name: target.Name, Force: false} }
	}
	return m, nil
}

func (m *Modal) View(width, height int) string {
	w := width - 8
	if w < 50 {
		w = 50
	}
	extra := 3
	if m.pendingBranch != "" {
		extra = 5
	}
	h := len(m.branches) + extra
	if h < 8 {
		h = 8
	}
	if h > height-4 {
		h = height - 4
	}
	var b strings.Builder
	title := "Switch Branch"
	if m.dirty {
		title += "  (working tree dirty)"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render(title))
	b.WriteByte('\n')
	for i, br := range m.branches {
		marker := "  "
		if br.Current {
			marker = "● "
		}
		line := marker + br.Name
		if ansi.StringWidth(line) > w {
			line = ansi.Truncate(line, w, "…")
		} else {
			line = line + strings.Repeat(" ", w-ansi.StringWidth(line))
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(line)
		}
		b.WriteString(line)
		if i < len(m.branches)-1 {
			b.WriteByte('\n')
		}
	}
	if m.pendingBranch != "" {
		b.WriteString("\n")
		warn := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(
			"working tree is dirty; switch to \"" + m.pendingBranch + "\" with --discard-changes? y/N",
		)
		b.WriteString(warn)
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}
