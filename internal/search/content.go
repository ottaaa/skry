package search

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/modal"
)

type GrepHit struct {
	Path string
	Line int
	Text string
}

type GrepModal struct {
	root    string
	files   []string // fallback list when ripgrep is absent
	input   textinput.Model
	hits    []GrepHit
	cursor  int
	err     string
	running bool

	version int // debounce token — latest keystroke wins
}

const grepDebounce = 180 * time.Millisecond

type grepResultMsg struct {
	version int
	hits    []GrepHit
	err     error
}

type grepDebounceMsg struct{ version int }

func NewGrepModal(root string, files []string) modal.Modal {
	ti := textinput.New()
	ti.Placeholder = "grep (incremental)…"
	ti.Focus()
	return &GrepModal{root: root, files: files, input: ti}
}

func (m *GrepModal) Init() tea.Cmd { return textinput.Blink }

func (m *GrepModal) Update(msg tea.Msg) (modal.Modal, tea.Cmd) {
	switch msg := msg.(type) {
	case grepDebounceMsg:
		if msg.version != m.version {
			return m, nil
		}
		q := strings.TrimSpace(m.input.Value())
		if q == "" {
			m.hits = nil
			m.err = ""
			m.running = false
			return m, nil
		}
		m.running = true
		root := m.root
		files := m.files
		version := m.version
		return m, func() tea.Msg {
			hits, err := runGrep(root, q, files)
			return grepResultMsg{version: version, hits: hits, err: err}
		}
	case grepResultMsg:
		if msg.version != m.version {
			// Stale result from a query that has been superseded.
			return m, nil
		}
		m.running = false
		m.hits = msg.hits
		m.cursor = 0
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
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
			if m.cursor < len(m.hits)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.hits) > 0 && m.cursor < len(m.hits) {
				h := m.hits[m.cursor]
				return m, func() tea.Msg { return events.OpenFileMsg{Path: h.Path, Line: h.Line} }
			}
			return m, nil
		}
	}
	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.version++
		v := m.version
		tick := tea.Tick(grepDebounce, func(time.Time) tea.Msg { return grepDebounceMsg{version: v} })
		return m, tea.Batch(cmd, tick)
	}
	return m, cmd
}

func (m *GrepModal) View(width, height int) string {
	w := width - 8
	if w < 40 {
		w = 40
	}
	h := height - 6
	if h < 10 {
		h = 10
	}
	listH := h - 4
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7")).Render("Find in Project"))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')
	switch {
	case m.err != "":
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(m.err))
	case m.running:
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("searching…"))
	default:
		b.WriteString(lipgloss.NewStyle().Faint(true).Render(legendForHits(len(m.hits))))
	}
	b.WriteByte('\n')
	for i := 0; i < listH && i < len(m.hits); i++ {
		h := m.hits[i]
		line := h.Path + ":" + strconv.Itoa(h.Line) + "  " + strings.TrimSpace(h.Text)
		if lipgloss.Width(line) > w {
			line = line[:max(0, w-1)] + "…"
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).Foreground(lipgloss.Color("#1a1b26")).Render(padRight(line, w))
		}
		b.WriteString(line)
		if i < listH-1 && i < len(m.hits)-1 {
			b.WriteByte('\n')
		}
	}
	return modal.Frame.Width(w).Height(h).Render(b.String())
}

func legendForHits(n int) string {
	if n == 0 {
		return "(no hits — keep typing)"
	}
	return strconv.Itoa(n) + " hit(s)"
}

func runGrep(root, query string, files []string) ([]GrepHit, error) {
	if _, err := exec.LookPath("rg"); err == nil {
		return runRg(root, query)
	}
	return runFallback(root, query, files)
}

func runRg(root, query string) ([]GrepHit, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "rg", "--line-number", "--no-heading", "--color=never", "--smart-case", "--max-count=200", "--", query)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil // no matches
		}
		return nil, err
	}
	var hits []GrepHit
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		hit, ok := parseGrepLine(line)
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

func parseGrepLine(line string) (GrepHit, bool) {
	p1 := strings.Index(line, ":")
	if p1 < 0 {
		return GrepHit{}, false
	}
	rest := line[p1+1:]
	p2 := strings.Index(rest, ":")
	if p2 < 0 {
		return GrepHit{}, false
	}
	lineNum, err := strconv.Atoi(rest[:p2])
	if err != nil {
		return GrepHit{}, false
	}
	return GrepHit{Path: line[:p1], Line: lineNum, Text: rest[p2+1:]}, true
}

func runFallback(root, query string, files []string) ([]GrepHit, error) {
	var hits []GrepHit
	q := strings.ToLower(query)
	for _, rel := range files {
		full := filepath.Join(root, rel)
		f, err := os.Open(full)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		n := 0
		for sc.Scan() {
			n++
			if strings.Contains(strings.ToLower(sc.Text()), q) {
				hits = append(hits, GrepHit{Path: rel, Line: n, Text: sc.Text()})
				if len(hits) > 500 {
					_ = f.Close()
					return hits, nil
				}
			}
		}
		_ = f.Close()
	}
	return hits, nil
}
