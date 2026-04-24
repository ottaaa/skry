package editor

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const tabWidth = 4

// expandTabs replaces each tab with enough spaces to reach the next tabWidth
// boundary. Keeping this column-aware matters in SplitDiff where left and
// right columns must stay aligned.
func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			pad := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", pad))
			col += pad
			continue
		}
		b.WriteRune(r)
		// Rough display width — fine for MVP; wide glyph alignment is not
		// critical inside the diff columns.
		col++
	}
	return b.String()
}

// fitANSI expands tabs and truncates or right-pads the given (possibly
// ANSI-colored) string so its visible width is exactly w.
func fitANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = expandTabs(s)
	aw := ansi.StringWidth(s)
	if aw == w {
		return s
	}
	if aw > w {
		return ansi.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-aw)
}

// clipANSI truncates without padding; for single-line right-edge protection.
func clipANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = expandTabs(s)
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}
