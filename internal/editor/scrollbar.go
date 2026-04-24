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
	barH := visible * visible / total
	if barH < 1 {
		barH = 1
	}
	if barH > visible {
		barH = visible
	}
	maxTop := total - visible
	if maxTop < 1 {
		maxTop = 1
	}
	barTop := top * (visible - barH) / maxTop
	if barTop < 0 {
		barTop = 0
	}
	if barTop > visible-barH {
		barTop = visible - barH
	}
	for i := 0; i < visible; i++ {
		if i >= barTop && i < barTop+barH {
			out[i] = scrollThumb
		} else {
			out[i] = scrollTrack
		}
	}
	return out
}
