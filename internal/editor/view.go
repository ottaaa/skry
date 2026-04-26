package editor

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	// lineFirstRow[i] is the first visual row index of source line i in the
	// rendered (wrapped) viewport content. Used by GoToLine and jumpToMatch
	// to translate 1-based source line numbers into viewport y-offsets.
	lineFirstRow []int
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
	numW := max(digits(len(v.raw)), 3)
	// Reserve 1 column for the scrollbar on the right.
	maxLine := max(v.vp.Width-numW-2, 8)
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	contStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
	contGutter := contStyle.Render(strings.Repeat(" ", numW-1) + "↪")
	v.lineFirstRow = make([]int, len(v.raw))
	var b strings.Builder
	row := 0
	for i, ln := range v.lines {
		if i >= len(v.raw) {
			break
		}
		v.lineFirstRow[i] = row
		num := numStyle.Render(padNum(i+1, numW))
		rendered := ln
		if v.query != "" {
			rendered = highlightMatches(v.raw[i], v.query)
		}
		rendered = expandTabs(rendered)
		chunks := strings.Split(ansi.Hardwrap(rendered, maxLine, false), "\n")
		for j, c := range chunks {
			if j == 0 {
				b.WriteString(num + " " + c)
			} else {
				b.WriteString(contGutter + " " + c)
			}
			b.WriteByte('\n')
			row++
		}
	}
	out := strings.TrimSuffix(b.String(), "\n")
	v.vp.SetContent(out)
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
	v.vp.SetYOffset(v.visualRow(m.line) - v.vp.Height/2)
}

// GoToLine scrolls the viewport so the 1-based `line` is roughly in the
// middle. Out-of-range values are clamped. line == 0 is a no-op.
func (v *viewMode) GoToLine(line int) {
	if line <= 0 {
		return
	}
	target := max(v.visualRow(line-1)-v.vp.Height/2, 0)
	v.vp.SetYOffset(target)
}

// visualRow returns the viewport row that the (0-based) source line starts
// at, after wrapping. Falls back to the source index itself when the wrap
// table hasn't been built yet.
func (v *viewMode) visualRow(srcIdx int) int {
	if srcIdx < 0 {
		return 0
	}
	if srcIdx >= len(v.lineFirstRow) {
		return srcIdx
	}
	return v.lineFirstRow[srcIdx]
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
