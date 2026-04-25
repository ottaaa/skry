package editor

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editMode is a minimal line-based editor with syntax highlighting. It keeps
// the whole buffer in memory as [][]rune and rebuilds a per-line chroma cache
// lazily when the view is drawn. Not production-grade — sufficient for MVP.
type editMode struct {
	path     string
	filename string
	perm     os.FileMode

	lines [][]rune
	row   int // 0-based
	col   int // 0-based rune offset into the row
	top   int // viewport top

	width  int
	height int

	saved string
	dirty bool

	cache      []string
	cacheValid bool

	undo []snapshot
	redo []snapshot
}

// snapshot captures enough buffer state to restore after an edit. Lines are
// deep-copied so later mutations can't corrupt the snapshot.
type snapshot struct {
	lines [][]rune
	row   int
	col   int
}

// undoCap bounds the history so large edit sessions don't grow unboundedly.
const undoCap = 200

func (e *editMode) takeSnapshot() snapshot {
	cp := make([][]rune, len(e.lines))
	for i, l := range e.lines {
		cp[i] = append([]rune(nil), l...)
	}
	return snapshot{lines: cp, row: e.row, col: e.col}
}

// pushUndo records current state before a mutating op and clears the redo
// stack (the classic "new edit branches history" behavior).
func (e *editMode) pushUndo() {
	e.undo = append(e.undo, e.takeSnapshot())
	if len(e.undo) > undoCap {
		e.undo = e.undo[len(e.undo)-undoCap:]
	}
	e.redo = nil
}

func (e *editMode) restore(s snapshot) {
	e.lines = s.lines
	e.row = s.row
	e.col = s.col
	e.invalidate()
}

func (e *editMode) Undo() bool {
	if len(e.undo) == 0 {
		return false
	}
	e.redo = append(e.redo, e.takeSnapshot())
	s := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.restore(s)
	// dirty flag: on undo back to the saved state, the buffer is clean again.
	e.dirty = e.Value() != e.saved
	return true
}

func (e *editMode) Redo() bool {
	if len(e.redo) == 0 {
		return false
	}
	e.undo = append(e.undo, e.takeSnapshot())
	s := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.restore(s)
	e.dirty = e.Value() != e.saved
	return true
}

func newEditMode() editMode { return editMode{lines: [][]rune{{}}} }

// Load pulls `content` into the buffer. relPath is used as the chroma hint for
// language detection.
func (e *editMode) Load(absPath, relPath, content string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	e.path = absPath
	e.filename = relPath
	e.perm = info.Mode().Perm()
	e.setContent(content)
	e.saved = content
	e.dirty = false
	e.row, e.col, e.top = 0, 0, 0
	e.cacheValid = false
	e.undo = nil
	e.redo = nil
	return nil
}

func (e *editMode) setContent(content string) {
	ls := strings.Split(content, "\n")
	e.lines = make([][]rune, len(ls))
	for i, l := range ls {
		e.lines[i] = []rune(l)
	}
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
}

func (e *editMode) Value() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (e *editMode) SetSize(w, h int) {
	e.width, e.height = w, h
	e.scrollIntoView()
}

func (e editMode) Dirty() bool { return e.dirty }

func (e *editMode) Save() error {
	content := e.Value()
	perm := e.perm
	if perm == 0 {
		perm = 0o644
	}
	if err := os.WriteFile(e.path, []byte(content), perm); err != nil {
		return err
	}
	e.saved = content
	e.dirty = false
	return nil
}

func (e *editMode) Update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	e.handleKey(km)
	e.scrollIntoView()
	return nil
}

func (e *editMode) handleKey(km tea.KeyMsg) {
	switch km.String() {
	case "up":
		if e.row > 0 {
			e.row--
			e.clampCol()
		}
	case "down":
		if e.row < len(e.lines)-1 {
			e.row++
			e.clampCol()
		}
	case "left":
		if e.col > 0 {
			e.col--
		} else if e.row > 0 {
			e.row--
			e.col = len(e.lines[e.row])
		}
	case "right":
		if e.col < len(e.lines[e.row]) {
			e.col++
		} else if e.row < len(e.lines)-1 {
			e.row++
			e.col = 0
		}
	case "home", "ctrl+a":
		e.col = 0
	case "end", "ctrl+e":
		e.col = len(e.lines[e.row])
	case "pgup":
		e.row -= maxInt(1, e.height-1)
		if e.row < 0 {
			e.row = 0
		}
		e.clampCol()
	case "pgdown":
		e.row += maxInt(1, e.height-1)
		if e.row >= len(e.lines) {
			e.row = len(e.lines) - 1
		}
		e.clampCol()
	case "backspace":
		e.backspace()
	case "delete":
		e.deleteForward()
	case "enter":
		e.splitLine()
	case "tab":
		e.insertRune('\t')
	case "ctrl+z":
		e.Undo()
	case "ctrl+y":
		e.Redo()
	default:
		if len(km.Runes) > 0 {
			for _, r := range km.Runes {
				if r < 0x20 {
					continue
				}
				e.insertRune(r)
			}
		}
	}
}

func (e *editMode) clampCol() {
	if e.col > len(e.lines[e.row]) {
		e.col = len(e.lines[e.row])
	}
	if e.col < 0 {
		e.col = 0
	}
}

func (e *editMode) insertRune(r rune) {
	e.pushUndo()
	line := e.lines[e.row]
	out := make([]rune, len(line)+1)
	copy(out, line[:e.col])
	out[e.col] = r
	copy(out[e.col+1:], line[e.col:])
	e.lines[e.row] = out
	e.col++
	e.invalidate()
	e.dirty = true
}

func (e *editMode) backspace() {
	if e.col == 0 && e.row == 0 {
		return
	}
	e.pushUndo()
	if e.col > 0 {
		line := e.lines[e.row]
		e.lines[e.row] = append(line[:e.col-1], line[e.col:]...)
		e.col--
	} else {
		prev := e.lines[e.row-1]
		newCol := len(prev)
		merged := append(prev, e.lines[e.row]...)
		e.lines[e.row-1] = merged
		e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
		e.row--
		e.col = newCol
	}
	e.invalidate()
	e.dirty = true
}

func (e *editMode) deleteForward() {
	line := e.lines[e.row]
	if e.col >= len(line) && e.row >= len(e.lines)-1 {
		return
	}
	e.pushUndo()
	if e.col < len(line) {
		e.lines[e.row] = append(line[:e.col], line[e.col+1:]...)
	} else {
		merged := append(line, e.lines[e.row+1]...)
		e.lines[e.row] = merged
		e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
	}
	e.invalidate()
	e.dirty = true
}

func (e *editMode) splitLine() {
	e.pushUndo()
	line := e.lines[e.row]
	head := append([]rune{}, line[:e.col]...)
	tail := append([]rune{}, line[e.col:]...)
	e.lines[e.row] = head
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.row+1])
	newLines[e.row+1] = tail
	copy(newLines[e.row+2:], e.lines[e.row+1:])
	e.lines = newLines
	e.row++
	e.col = 0
	e.invalidate()
	e.dirty = true
}

func (e *editMode) invalidate() { e.cacheValid = false }

func (e *editMode) rebuildCache() {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	e.cache = Highlight(strings.Join(parts, "\n"), e.filename)
	e.cacheValid = true
}

func (e *editMode) scrollIntoView() {
	if e.height <= 0 {
		return
	}
	if e.row < e.top {
		e.top = e.row
	}
	if e.row >= e.top+e.height {
		e.top = e.row - e.height + 1
	}
	if e.top < 0 {
		e.top = 0
	}
}

func (e *editMode) View() string {
	if !e.cacheValid {
		e.rebuildCache()
	}
	h := max(e.height, 1)
	numW := max(digits(len(e.lines)), 3)
	maxLine := max(e.width-numW-1, 8)
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	var b strings.Builder
	for i := range h {
		idx := e.top + i
		if idx >= len(e.lines) {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render("~"))
			if i < h-1 {
				b.WriteByte('\n')
			}
			continue
		}
		num := numStyle.Render(padNum(idx+1, numW))
		var line string
		if idx == e.row {
			line = renderCursorLine(e.lines[idx], e.col, maxLine)
		} else if idx < len(e.cache) {
			line = clipANSI(e.cache[idx], maxLine)
		} else {
			line = clipANSI(string(e.lines[idx]), maxLine)
		}
		b.WriteString(num + " " + line)
		if i < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderCursorLine renders the cursor row without syntax highlighting so we
// can reliably overlay an inverse-video cursor. The rendered line is kept to
// at most maxW columns by shifting the visible window when the cursor is past
// the right edge.
func renderCursorLine(runes []rune, col, maxW int) string {
	if col < 0 {
		col = 0
	}
	if col > len(runes) {
		col = len(runes)
	}
	expandedBefore := expandTabs(string(runes[:col]))
	atRune := rune(' ')
	var after string
	if col < len(runes) {
		atRune = runes[col]
		after = string(runes[col+1:])
	}
	if atRune == '\t' {
		atRune = ' '
	}
	expandedAfter := expandTabs(after)
	beforeRunes := []rune(expandedBefore)
	leftShift := 0
	if len(beforeRunes) >= maxW {
		leftShift = len(beforeRunes) - maxW + 1
	}
	beforeVisible := string(beforeRunes[leftShift:])
	line := beforeVisible + lipgloss.NewStyle().Reverse(true).Render(string(atRune)) + expandedAfter
	return clipANSI(line, maxW)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
