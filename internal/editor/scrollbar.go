package editor

import "github.com/charmbracelet/lipgloss"

var (
	scrollTrack = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261")).Render("│")
	scrollThumb = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Render("█")
)

// scrollbarChars returns one-character cells for a vertical scrollbar of
// `visible` rows, given the current `top` and `total` row count. When the
// content fits entirely, every cell is the track character.
func scrollbarChars(top, visible, total int) []string {
	out := make([]string, visible)
	if visible <= 0 {
		return out
	}
	if total <= visible || total <= 0 {
		for i := range out {
			out[i] = scrollTrack
		}
		return out
	}
	barH := min(max(visible*visible/total, 1), visible)
	maxTop := max(total-visible, 1)
	barTop := min(max(top*(visible-barH)/maxTop, 0), visible-barH)
	for i := range visible {
		if i >= barTop && i < barTop+barH {
			out[i] = scrollThumb
		} else {
			out[i] = scrollTrack
		}
	}
	return out
}
