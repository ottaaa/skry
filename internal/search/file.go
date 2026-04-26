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

type fileResultMsg struct {
	results []fuzzy.Match
}

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
	return &FileModal{title: title, input: ti, files: files}
}

func (m *FileModal) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.refineCmd(""))
}

func (m *FileModal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
	switch msg := msg.(type) {
	case fileResultMsg:
		m.results = msg.results
		if m.cursor >= len(m.results) {
			m.cursor = 0
		}
		m.top = 0
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return events.CloseModalMsg{} }
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.results) > 0 && m.cursor < len(m.results) {
				path := m.files[m.results[m.cursor].Index]
				return m, func() tea.Msg { return events.OpenFileMsg{Path: path} }
			}
			return m, nil
		}
	}
	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		return m, tea.Batch(cmd, m.refineCmd(m.input.Value()))
	}
	return m, cmd
}

// refineCmd runs fuzzy matching in a goroutine and returns the results as a
// fileResultMsg. This keeps the Bubble Tea update loop non-blocking even on
// large file lists.
func (m *FileModal) refineCmd(query string) tea.Cmd {
	files := m.files
	q := strings.TrimSpace(query)
	return func() tea.Msg {
		if q == "" {
			results := make([]fuzzy.Match, len(files))
			for i, f := range files {
				results[i] = fuzzy.Match{Str: f, Index: i}
			}
			return fileResultMsg{results: results}
		}
		return fileResultMsg{results: fuzzy.Find(q, files)}
	}
}

// PreviewPath implements modal.Previewer.
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
