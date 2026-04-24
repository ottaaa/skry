package search

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/modal"
)

type FileModal struct {
	title   string
	input   textinput.Model
	files   []string
	results []fuzzy.Match
	cursor  int
}

func NewFileModal(files []string) modal.Modal {
	return newFileModal("Go to File", files)
}

func NewRecentModal(files []string) modal.Modal {
	return newFileModal("Recent Files", files)
}

func newFileModal(title string, files []string) *FileModal {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Focus()
	m := &FileModal{title: title, input: ti, files: files}
	m.refine()
	return m
}

func (m *FileModal) Init() tea.Cmd { return textinput.Blink }

func (m *FileModal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch km.String() {
	case "esc":
		return m, func() tea.Msg { return events.CloseModalMsg{} }
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.results)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.results) > 0 && m.cursor < len(m.results) {
			path := m.files[m.results[m.cursor].Index]
			return m, func() tea.Msg { return events.OpenFileMsg{Path: path} }
		}
		// No match: empty query with empty file list
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refine()
		return m, cmd
	}
	return m, nil
}

func (m *FileModal) refine() {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		m.results = m.results[:0]
		for i, f := range m.files {
			m.results = append(m.results, fuzzy.Match{Str: f, Index: i})
		}
	} else {
		m.results = fuzzy.Find(q, m.files)
	}
	if m.cursor >= len(m.results) {
		m.cursor = 0
	}
}

func (m *FileModal) View(width, height int) string {
	w := width - 8
	if w < 40 {
		w = 40
	}
	h := height - 6
	if h < 8 {
		h = 8
	}
	listH := h - 3
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render(m.title))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	for i := 0; i < listH && i < len(m.results); i++ {
		line := m.results[i].Str
		if lipgloss.Width(line) > w {
			line = line[:max(0, w-1)] + "…"
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(padRight(line, w))
		}
		b.WriteString(line)
		if i < listH-1 && i < len(m.results)-1 {
			b.WriteByte('\n')
		}
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}

func padRight(s string, w int) string {
	diff := w - lipgloss.Width(s)
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
