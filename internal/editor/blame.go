package editor

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/git"
)

var (
	blameHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))
	blameAuthStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	blameDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	blameLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

// RenderBlame paints blame rows into a single column. The rightmost column is
// reserved for the scrollbar.
func RenderBlame(lines []git.BlameLine, top, visible, width int) string {
	if len(lines) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("(no blame)")
	}
	contentW := width - 1
	numW := digitsOf(len(lines))
	if numW < 3 {
		numW = 3
	}
	shortW := 8
	authorW := 12
	dateW := 10
	bars := scrollbarChars(top, visible, len(lines))
	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := top + i
		var line string
		if idx < len(lines) {
			bl := lines[idx]
			short := bl.Short
			if short == "" && len(bl.Hash) >= 8 {
				short = bl.Hash[:8]
			}
			prefix := blameLineStyle.Render(padLeft(strconv.Itoa(bl.Line), numW)) + " " +
				blameHashStyle.Render(padRightASCII(short, shortW)) + " " +
				blameAuthStyle.Render(padRightASCII(truncASCII(bl.Author, authorW), authorW)) + " " +
				blameDateStyle.Render(padRightASCII(bl.Date, dateW)) + "  "
			text := expandTabs(bl.Text)
			line = fitANSI(prefix+text, contentW)
		} else {
			line = fitANSI("", contentW)
		}
		b.WriteString(line)
		if i < len(bars) {
			b.WriteString(bars[i])
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func padRightASCII(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func truncASCII(s string, w int) string {
	if len(s) <= w {
		return s
	}
	return s[:w]
}

func digitsOf(n int) int {
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
