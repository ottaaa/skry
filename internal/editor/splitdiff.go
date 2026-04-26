package editor

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type DiffOp int

const (
	DiffEqual DiffOp = iota
	DiffChange
	DiffAdd
	DiffDel
)

type DiffRow struct {
	Op       DiffOp
	LeftNum  int // 1-based; 0 if absent
	RightNum int // 1-based; 0 if absent
	Left     string
	Right    string
}

// AlignLines produces side-by-side aligned rows from two versions of a file.
// Pairs adjacent delete/insert runs 1:1 as Change rows.
func AlignLines(left, right string) []DiffRow {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(left, right)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(a, b, false), lineArray)
	var rows []DiffRow
	leftNum, rightNum := 0, 0
	i := 0
	for i < len(diffs) {
		d := diffs[i]
		lines := splitKeep(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			for _, l := range lines {
				leftNum++
				rightNum++
				rows = append(rows, DiffRow{Op: DiffEqual, LeftNum: leftNum, RightNum: rightNum, Left: l, Right: l})
			}
			i++
		case diffmatchpatch.DiffDelete:
			var nextIns []string
			if i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
				nextIns = splitKeep(diffs[i+1].Text)
			}
			pairs := min(len(lines), len(nextIns))
			for k := range pairs {
				leftNum++
				rightNum++
				rows = append(rows, DiffRow{Op: DiffChange, LeftNum: leftNum, RightNum: rightNum, Left: lines[k], Right: nextIns[k]})
			}
			for k := pairs; k < len(lines); k++ {
				leftNum++
				rows = append(rows, DiffRow{Op: DiffDel, LeftNum: leftNum, Left: lines[k]})
			}
			for k := pairs; k < len(nextIns); k++ {
				rightNum++
				rows = append(rows, DiffRow{Op: DiffAdd, RightNum: rightNum, Right: nextIns[k]})
			}
			if nextIns != nil {
				i += 2
			} else {
				i++
			}
		case diffmatchpatch.DiffInsert:
			for _, l := range lines {
				rightNum++
				rows = append(rows, DiffRow{Op: DiffAdd, RightNum: rightNum, Right: l})
			}
			i++
		}
	}
	return rows
}

// splitKeep splits a block of text into lines but drops the trailing empty
// string that results from a final newline. An intentional empty line from
// consecutive newlines is kept.
func splitKeep(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// RenderSplit renders rows into a string of width `width`, paginated by top.
// The rightmost column is reserved for a scrollbar.
//
// Every emitted row is normalized to exactly `contentW = width-1` visible
// columns before the scrollbar character is appended, so the scrollbar
// stays in a fixed column regardless of whether the row is content or
// empty trailing space, and regardless of width parity (the per-side
// integer-divided width can drop a column on odd widths).
func RenderSplit(rows []DiffRow, top, visible, width int) string {
	if width < 22 {
		width = 22
	}
	contentW := width - 1
	numW := 4
	perSide := max((contentW-2*numW-3)/2, 4)
	bars := scrollbarChars(top, visible, len(rows))
	delBg := lipgloss.NewStyle().Background(lipgloss.Color("#3a1f26"))
	addBg := lipgloss.NewStyle().Background(lipgloss.Color("#1f3a2a"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
	var b strings.Builder
	for i := range visible {
		idx := top + i
		var content string
		if idx < len(rows) {
			r := rows[idx]
			leftNum, rightNum := fmtNum(r.LeftNum, numW), fmtNum(r.RightNum, numW)
			leftInner := fitANSI(r.Left, perSide)
			rightInner := fitANSI(r.Right, perSide)
			leftLine := leftNum + " " + leftInner
			rightLine := rightNum + " " + rightInner
			switch r.Op { //nolint:exhaustive // DiffEqual leaves both halves un-styled
			case DiffDel:
				leftLine = delBg.Render(leftLine)
			case DiffAdd:
				rightLine = addBg.Render(rightLine)
			case DiffChange:
				leftLine = delBg.Render(leftLine)
				rightLine = addBg.Render(rightLine)
			}
			content = leftLine + sepStyle.Render("│") + rightLine
		}
		// Normalize to contentW columns. fitANSI pads with spaces (or
		// truncates) using ansi.StringWidth so the existing background-
		// styling escape codes are preserved.
		content = fitANSI(content, contentW)
		b.WriteString(content)
		if i < len(bars) {
			b.WriteString(bars[i])
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func fmtNum(n, w int) string {
	if n == 0 {
		return strings.Repeat(" ", w)
	}
	s := intToString(n)
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
