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
	top     int
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
	m.top = 0
}

// PreviewPath implements modal.Previewer: returns the relative path of the
// currently-highlighted result, or "" when the result list is empty.
func (m *FileModal) PreviewPath() string {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return ""
	}
	return m.results[m.cursor].Str
}

// PreviewLine implements modal.Previewer: file-name search has no line, so
// the preview always starts at the top.
func (m *FileModal) PreviewLine() int { return 0 }

func (m *FileModal) View(width, height int) string {
	w := max(width-8, 40)
	h := max(height-6, 8)
	listH := max(h-3, 1)
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+listH {
		m.top = m.cursor - listH + 1
	}
	if m.top < 0 {
		m.top = 0
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render(m.title))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	end := min(m.top+listH, len(m.results))
	for i := m.top; i < end; i++ {
		line := m.results[i].Str
		if lipgloss.Width(line) > w {
			line = line[:max(0, w-1)] + "…"
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(padRight(line, w))
		}
		b.WriteString(line)
		if i < end-1 {
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
