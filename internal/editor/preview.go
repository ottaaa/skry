package editor

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// previewMaxBytes caps the file size we will syntax-highlight for the preview
// pane. Larger files fall back to a plain "binary or too large" placeholder
// so opening the search modal over a multi-MB log file stays responsive.
const previewMaxBytes = 1 << 20 // 1 MiB

// PreviewFile renders a small read-only view of `absPath` into a rectangle
// of (width, height). When line > 0 the viewport is positioned so that the
// 1-based line is visible roughly in the middle. line == 0 starts at the
// top. `scroll` is added to the computed top line index and clamped to the
// content (negative scrolls up, positive scrolls down).
//
// This helper is intentionally side-effect-free — no caching, no
// goroutines. It is called once per render frame from the host app while a
// search modal is open, so it must be cheap; that is what previewMaxBytes
// guards.
func PreviewFile(absPath, relPath string, line, scroll, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	hint := lipgloss.NewStyle().Faint(true)

	if absPath == "" {
		return hint.Render("(no selection)")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return hint.Render(fmt.Sprintf("(cannot read: %s)", relPath))
	}
	if info.IsDir() {
		return hint.Render(fmt.Sprintf("%s/  (directory)", relPath))
	}
	if info.Size() > previewMaxBytes {
		return hint.Render(fmt.Sprintf("%s  (%s — too large to preview)", relPath, humanSize(info.Size())))
	}
	raw, err := readWorkingBytes(absPath)
	if err != nil {
		return hint.Render(fmt.Sprintf("(read error: %v)", err))
	}
	if isBinary(raw) {
		return hint.Render(fmt.Sprintf("%s  (binary, %s)", relPath, humanSize(info.Size())))
	}

	highlighted := Highlight(string(raw), relPath)
	total := len(highlighted)
	numW := max(digits(total), 3)
	contentW := max(width-numW-1, 8)

	top := previewTop(line, height, total) + scroll
	if top < 0 {
		top = 0
	}
	if maxTop := total - height; top > maxTop {
		top = max(maxTop, 0)
	}
	hl := -1
	if line > 0 && line <= total {
		hl = line - 1
	}

	var b strings.Builder
	for i := range height {
		idx := top + i
		if idx >= total {
			b.WriteString(hint.Render("~"))
			if i < height-1 {
				b.WriteByte('\n')
			}
			continue
		}
		num := numStyle.Render(padNum(idx+1, numW))
		body := clipANSI(highlighted[idx], contentW)
		row := num + " " + body
		if idx == hl {
			row = lipgloss.NewStyle().Background(lipgloss.Color("#2d3050")).Render(padRightAnsi(row, width))
		}
		b.WriteString(row)
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// previewTop picks the top line index so that `line` (1-based) appears near
// the middle of a `height`-row viewport. Falls back to 0 when line is
// unspecified or the file fits entirely in view.
func previewTop(line, height, total int) int {
	if line <= 0 || total <= height {
		return 0
	}
	target := line - 1 - height/2
	if target < 0 {
		return 0
	}
	if target > total-height {
		return total - height
	}
	return target
}

// padRightAnsi pads s to width w taking ANSI escape sequences into account
// (lipgloss-rendered strings include them around colored runs).
func padRightAnsi(s string, w int) string {
	pad := w - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}
