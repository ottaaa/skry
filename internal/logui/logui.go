package logui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
	"github.com/ottaaa/skry/internal/modal"
)

// LogModal shows the straight-line history of the current branch.
type LogModal struct {
	commits []git.Commit
	cursor  int
	top     int
}

func NewLogModal(commits []git.Commit) modal.Modal {
	return &LogModal{commits: commits}
}

func (m *LogModal) Init() tea.Cmd { return nil }

func (m *LogModal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
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
		if m.cursor < len(m.commits)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = len(m.commits) - 1
	case "enter":
		if m.cursor < len(m.commits) {
			c := m.commits[m.cursor]
			return m, func() tea.Msg {
				return events.LogCommitSelectedMsg{Sha: c.Hash, Short: c.Short, Subject: c.Subject}
			}
		}
	}
	return m, nil
}

func (m *LogModal) View(width, height int) string {
	w := width - 8
	if w < 60 {
		w = 60
	}
	h := height - 4
	if h < 10 {
		h = 10
	}
	listH := h - 2
	// keep cursor visible
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+listH {
		m.top = m.cursor - listH + 1
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).
		Render("Commit History — " + strconv.Itoa(len(m.commits)) + " commits"))
	b.WriteByte('\n')
	for i := 0; i < listH; i++ {
		idx := m.top + i
		if idx >= len(m.commits) {
			break
		}
		c := m.commits[idx]
		line := fmtCommitRow(c, w)
		if idx == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(line)
		}
		b.WriteString(line)
		if i < listH-1 {
			b.WriteByte('\n')
		}
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}

func fmtCommitRow(c git.Commit, w int) string {
	short := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff")).Render(c.Short)
	date := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(c.Date)
	author := c.Author
	if len(author) > 14 {
		author = author[:14]
	}
	author = padRight(author, 14)
	authorStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Render(author)
	line := short + "  " + date + "  " + authorStyled + "  " + c.Subject
	if ansi.StringWidth(line) > w {
		line = ansi.Truncate(line, w, "")
	} else {
		line = line + strings.Repeat(" ", w-ansi.StringWidth(line))
	}
	return line
}

// CommitFilesModal lists files changed in a commit and lets the user pick one
// to inspect as a SplitDiff against the commit's parent.
type CommitFilesModal struct {
	sha     string
	short   string
	subject string
	files   []string
	cursor  int
	top     int
}

func NewCommitFilesModal(sha, short, subject string, files []string) modal.Modal {
	return &CommitFilesModal{sha: sha, short: short, subject: subject, files: files}
}

func (m *CommitFilesModal) Init() tea.Cmd { return nil }

func (m *CommitFilesModal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
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
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < len(m.files) {
			p := m.files[m.cursor]
			return m, func() tea.Msg {
				return events.CommitFileSelectedMsg{Sha: m.sha, Short: m.short, Subject: m.subject, Path: p}
			}
		}
	}
	return m, nil
}

func (m *CommitFilesModal) View(width, height int) string {
	w := width - 8
	if w < 60 {
		w = 60
	}
	h := height - 4
	if h < 8 {
		h = 8
	}
	listH := h - 2
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+listH {
		m.top = m.cursor - listH + 1
	}
	var b strings.Builder
	head := m.short + " " + m.subject
	if ansi.StringWidth(head) > w {
		head = ansi.Truncate(head, w, "…")
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render("Files in " + head))
	b.WriteByte('\n')
	for i := 0; i < listH; i++ {
		idx := m.top + i
		if idx >= len(m.files) {
			break
		}
		line := m.files[idx]
		if ansi.StringWidth(line) > w {
			line = ansi.Truncate(line, w, "…")
		} else {
			line = line + strings.Repeat(" ", w-ansi.StringWidth(line))
		}
		if idx == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(line)
		}
		b.WriteString(line)
		if i < listH-1 {
			b.WriteByte('\n')
		}
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
