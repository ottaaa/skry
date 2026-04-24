package editor

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type viewMode struct {
	vp       viewport.Model
	lines    []string
	raw      []string // unhighlighted, for search
	filename string

	searching bool
	query     string
	matches   []matchHit
	matchIdx  int
}

type matchHit struct {
	line int // 0-based
	col  int // 0-based rune offset
}

func newViewMode() viewMode {
	return viewMode{vp: viewport.New(0, 0)}
}

func (v *viewMode) Load(filename, content string) {
	v.filename = filename
	v.raw = strings.Split(content, "\n")
	v.lines = Highlight(content, filename)
	v.searching = false
	v.query = ""
	v.matches = nil
	v.matchIdx = 0
	v.rebuild()
	v.vp.GotoTop()
}

func (v *viewMode) SetSize(w, h int) {
	v.vp.Width, v.vp.Height = w, h
	v.rebuild()
}

func (v *viewMode) rebuild() {
	numW := digits(len(v.raw))
	if numW < 3 {
		numW = 3
	}
	// Reserve 1 column for the scrollbar on the right.
	maxLine := v.vp.Width - numW - 2
	if maxLine < 8 {
		maxLine = 8
	}
	var b strings.Builder
	for i, ln := range v.lines {
		if i >= len(v.raw) {
			break
		}
		num := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(padNum(i+1, numW))
		rendered := ln
		if v.query != "" {
			rendered = highlightMatches(v.raw[i], v.query)
		}
		rendered = clipANSI(rendered, maxLine)
		b.WriteString(num + " " + rendered)
		if i < len(v.lines)-1 {
			b.WriteByte('\n')
		}
	}
	v.vp.SetContent(b.String())
}

func highlightMatches(raw, q string) string {
	if q == "" {
		return raw
	}
	lowerRaw := strings.ToLower(raw)
	lowerQ := strings.ToLower(q)
	var b strings.Builder
	i := 0
	style := lipgloss.NewStyle().Background(lipgloss.Color("#3d3a1f"))
	for i < len(raw) {
		idx := strings.Index(lowerRaw[i:], lowerQ)
		if idx < 0 {
			b.WriteString(raw[i:])
			break
		}
		b.WriteString(raw[i : i+idx])
		b.WriteString(style.Render(raw[i+idx : i+idx+len(q)]))
		i += idx + len(q)
	}
	return b.String()
}

func (v *viewMode) StartSearch() {
	v.searching = true
	v.query = ""
	v.matches = nil
	v.matchIdx = 0
}

func (v *viewMode) CancelSearch() {
	v.searching = false
	v.query = ""
	v.matches = nil
	v.rebuild()
}

func (v *viewMode) ConfirmSearch() {
	v.searching = false
}

func (v *viewMode) UpdateQuery(q string) {
	v.query = q
	v.computeMatches()
	if len(v.matches) > 0 {
		v.matchIdx = 0
		v.jumpToMatch(0)
	}
	v.rebuild()
}

func (v *viewMode) computeMatches() {
	v.matches = nil
	if v.query == "" {
		return
	}
	q := strings.ToLower(v.query)
	for i, ln := range v.raw {
		l := strings.ToLower(ln)
		col := 0
		for {
			idx := strings.Index(l[col:], q)
			if idx < 0 {
				break
			}
			v.matches = append(v.matches, matchHit{line: i, col: col + idx})
			col += idx + len(q)
			if col > len(l) {
				break
			}
		}
	}
}

func (v *viewMode) NextMatch() {
	if len(v.matches) == 0 {
		return
	}
	v.matchIdx = (v.matchIdx + 1) % len(v.matches)
	v.jumpToMatch(v.matchIdx)
}

func (v *viewMode) PrevMatch() {
	if len(v.matches) == 0 {
		return
	}
	v.matchIdx = (v.matchIdx - 1 + len(v.matches)) % len(v.matches)
	v.jumpToMatch(v.matchIdx)
}

func (v *viewMode) jumpToMatch(idx int) {
	if idx < 0 || idx >= len(v.matches) {
		return
	}
	m := v.matches[idx]
	v.vp.SetYOffset(m.line - v.vp.Height/2)
}

func (v viewMode) Searching() bool { return v.searching }
func (v viewMode) Query() string   { return v.query }
func (v viewMode) MatchInfo() (int, int) {
	if len(v.matches) == 0 {
		return 0, 0
	}
	return v.matchIdx + 1, len(v.matches)
}

func (v *viewMode) ScrollDown() { v.vp.ScrollDown(1) }
func (v *viewMode) ScrollUp()   { v.vp.ScrollUp(1) }
func (v *viewMode) PageDown()   { v.vp.HalfPageDown() }
func (v *viewMode) PageUp()     { v.vp.HalfPageUp() }
func (v *viewMode) Top()        { v.vp.GotoTop() }
func (v *viewMode) Bottom()     { v.vp.GotoBottom() }

func (v viewMode) Render() string {
	out := v.vp.View()
	lines := strings.Split(out, "\n")
	bars := scrollbarChars(v.vp.YOffset, v.vp.Height, v.vp.TotalLineCount())
	contentW := v.vp.Width - 1
	for i := range lines {
		lines[i] = fitANSI(lines[i], contentW)
		if i < len(bars) {
			lines[i] += bars[i]
		}
	}
	return strings.Join(lines, "\n")
}

func padNum(n, w int) string {
	s := intToString(n)
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func digits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}
