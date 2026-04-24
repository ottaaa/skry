package editor

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/peek/internal/git"
)

var (
	blameHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))
	blameAuthStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	blameDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	blameLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

// RenderBlame paints blame rows into a single column.
func RenderBlame(lines []git.BlameLine, top, visible, width int) string {
	if len(lines) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("(no blame)")
	}
	numW := digitsOf(len(lines))
	if numW < 3 {
		numW = 3
	}
	shortW := 8
	authorW := 12
	dateW := 10
	meta := numW + 1 + shortW + 1 + authorW + 1 + dateW + 2
	textW := width - meta
	if textW < 4 {
		textW = 4
	}
	var b strings.Builder
	end := top + visible
	if end > len(lines) {
		end = len(lines)
	}
	for i := top; i < end; i++ {
		bl := lines[i]
		short := bl.Short
		if short == "" && len(bl.Hash) >= 8 {
			short = bl.Hash[:8]
		}
		prefix := blameLineStyle.Render(padLeft(strconv.Itoa(bl.Line), numW)) + " " +
			blameHashStyle.Render(padRightASCII(short, shortW)) + " " +
			blameAuthStyle.Render(padRightASCII(truncASCII(bl.Author, authorW), authorW)) + " " +
			blameDateStyle.Render(padRightASCII(bl.Date, dateW)) + "  "
		text := expandTabs(bl.Text)
		line := prefix + text
		line = clipANSI(line, width)
		b.WriteString(line)
		if i < end-1 {
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
