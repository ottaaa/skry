package editor

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/git"
)

func renderFolder(entries []folderEntry, top, visible, width int) string {
	if len(entries) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("(empty)")
	}
	end := min(top+visible, len(entries))
	if top < 0 {
		top = 0
	}
	var b strings.Builder
	for i := top; i < end; i++ {
		e := entries[i]
		marker, col := statusGlyph(e.Status)
		icon := "  "
		if e.IsDir {
			icon = "▸ "
		}
		line := icon + lipgloss.NewStyle().Foreground(col).Render(marker) + " " + e.Name
		line = clipANSI(line, width)
		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func statusGlyph(s git.Status) (string, lipgloss.Color) {
	switch s { //nolint:exhaustive // StatusNone returns empty by intent (handled below)
	case git.StatusModified:
		return "M", lipgloss.Color("#e0af68")
	case git.StatusAdded:
		return "A", lipgloss.Color("#9ece6a")
	case git.StatusDeleted:
		return "D", lipgloss.Color("#f7768e")
	case git.StatusRenamed:
		return "R", lipgloss.Color("#bb9af7")
	case git.StatusUntracked:
		return "?", lipgloss.Color("#7dcfff")
	}
	return " ", lipgloss.Color("#c0caf5")
}
