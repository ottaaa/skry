package tree

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
)

type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Status   git.Status
	Children []*Node
}

type row struct {
	node  *Node
	depth int
}

type Model struct {
	root     *Node
	expanded map[string]bool
	rows     []row
	cursor   int
	width    int
	height   int
	filter   string
	filterOn bool
	focused  bool
	flat     bool
}

func New() Model {
	return Model{expanded: map[string]bool{"": true}}
}

func (m *Model) SetFiles(paths []string, statuses map[string]git.Status) {
	root := &Node{IsDir: true}
	for _, p := range paths {
		insert(root, p, statuses[p])
	}
	sortNode(root)
	propagateStatus(root, statuses)
	m.root = root
	m.rebuildRows()
	if m.cursor >= len(m.rows) {
		m.cursor = 0
	}
}

func (m *Model) SetFocused(v bool) { m.focused = v }
func (m Model) Focused() bool      { return m.focused }
func (m Model) Filtering() bool    { return m.filterOn }
func (m Model) Flat() bool         { return m.flat }

func (m *Model) ToggleFlat() {
	m.flat = !m.flat
	m.cursor = 0
	m.rebuildRows()
}

func (m Model) SelectedPath() string {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	n := m.rows[m.cursor].node
	if n.IsDir {
		return ""
	}
	return n.Path
}

func (m Model) SelectedStatus() git.Status {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	return m.rows[m.cursor].node.Status
}

// CurrentMsg returns a CursorMovedMsg describing the currently-selected node,
// or the zero value when the tree is empty. Useful for triggering an initial
// preview after SetFiles.
func (m Model) CurrentMsg() events.CursorMovedMsg {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return events.CursorMovedMsg{}
	}
	n := m.rows[m.cursor].node
	return events.CursorMovedMsg{Path: n.Path, IsDir: n.IsDir}
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filterOn {
		switch km.String() {
		case "esc":
			m.filterOn = false
			m.filter = ""
			m.rebuildRows()
			return m, m.cursorCmd()
		case "enter":
			m.filterOn = false
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.rebuildRows()
				return m, m.cursorCmd()
			}
		default:
			if len(km.Runes) > 0 {
				m.filter += string(km.Runes)
				m.rebuildRows()
				return m, m.cursorCmd()
			}
		}
		return m, nil
	}
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			return m, m.cursorCmd()
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			return m, m.cursorCmd()
		}
	case "home", "g":
		m.cursor = 0
		return m, m.cursorCmd()
	case "end", "G":
		m.cursor = len(m.rows) - 1
		return m, m.cursorCmd()
	case "enter", " ", "l":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			n := m.rows[m.cursor].node
			if n.IsDir {
				m.expanded[n.Path] = !m.expanded[n.Path]
				m.rebuildRows()
				return m, m.cursorCmd()
			}
			return m, openCmd(n.Path)
		}
	case "h":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			n := m.rows[m.cursor].node
			if n.IsDir && m.expanded[n.Path] {
				m.expanded[n.Path] = false
				m.rebuildRows()
				return m, m.cursorCmd()
			}
		}
	case "/":
		m.filterOn = true
		m.filter = ""
	}
	return m, nil
}

// cursorCmd returns a Cmd emitting CursorMovedMsg for the currently highlighted
// node. The app uses this to drive right-pane previews as the user moves
// through the tree. Returns nil when the tree is empty.
func (m Model) cursorCmd() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	n := m.rows[m.cursor].node
	path, isDir := n.Path, n.IsDir
	return func() tea.Msg {
		return events.CursorMovedMsg{Path: path, IsDir: isDir}
	}
}

func openCmd(path string) tea.Cmd {
	return func() tea.Msg { return events.OpenFileMsg{Path: path} }
}

func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	if m.root == nil {
		return
	}
	if m.flat {
		m.rebuildFlatRows()
	} else {
		var walk func(n *Node, depth int)
		walk = func(n *Node, depth int) {
			for _, c := range n.Children {
				if m.filter != "" && !matches(c, m.filter) {
					continue
				}
				m.rows = append(m.rows, row{node: c, depth: depth})
				if c.IsDir && (m.filter != "" || m.expanded[c.Path]) {
					walk(c, depth+1)
				}
			}
		}
		walk(m.root, 0)
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) rebuildFlatRows() {
	q := strings.ToLower(m.filter)
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for _, c := range n.Children {
			if c.IsDir {
				walk(c)
				continue
			}
			if c.Status == git.StatusNone || c.Status == "" {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(c.Path), q) {
				continue
			}
			m.rows = append(m.rows, row{node: c, depth: 0})
		}
	}
	walk(m.root)
	sort.SliceStable(m.rows, func(i, j int) bool {
		return m.rows[i].node.Path < m.rows[j].node.Path
	})
}

func matches(n *Node, q string) bool {
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(n.Path), q) {
		return true
	}
	for _, c := range n.Children {
		if matches(c, q) {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	if len(m.rows) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("(no files)")
	}
	h := m.height
	if h <= 0 {
		h = len(m.rows)
	}
	if m.filterOn || m.filter != "" {
		h--
	}
	start := 0
	if m.cursor >= h {
		start = m.cursor - h + 1
	}
	end := min(start+h, len(m.rows))
	var b strings.Builder
	for i := start; i < end; i++ {
		r := m.rows[i]
		line := renderRow(r, i == m.cursor, m.width, m.flat)
		b.WriteString(line)
		if i < end-1 || m.filterOn || m.filter != "" {
			b.WriteByte('\n')
		}
	}
	if m.filterOn || m.filter != "" {
		prompt := "/" + m.filter
		if m.filterOn {
			prompt += "_"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff")).Render(prompt))
	}
	return b.String()
}

func renderRow(r row, selected bool, width int, flat bool) string {
	indent := strings.Repeat("  ", r.depth)
	var icon string
	if r.node.IsDir {
		icon = "▸ "
	} else if flat {
		icon = ""
	} else {
		icon = "  "
	}
	marker := " "
	col := lipgloss.Color("#c0caf5")
	switch r.node.Status {
	case git.StatusModified:
		marker, col = "M", lipgloss.Color("#e0af68")
	case git.StatusAdded:
		marker, col = "A", lipgloss.Color("#9ece6a")
	case git.StatusDeleted:
		marker, col = "D", lipgloss.Color("#f7768e")
	case git.StatusRenamed:
		marker, col = "R", lipgloss.Color("#bb9af7")
	case git.StatusUntracked:
		marker, col = "?", lipgloss.Color("#7dcfff")
	}
	name := r.node.Name
	if flat {
		name = r.node.Path
	}
	markerPart := lipgloss.NewStyle().Foreground(col).Render(marker)
	line := indent + icon + markerPart + " " + name
	if width > 0 {
		if ansi.StringWidth(line) > width {
			line = ansi.Truncate(line, width, "")
		} else {
			line = line + strings.Repeat(" ", width-ansi.StringWidth(line))
		}
	}
	if selected {
		line = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(lipgloss.Color("#7aa2f7")).
			Render(line)
	}
	return line
}

func insert(root *Node, path string, status git.Status) {
	segs := strings.Split(path, "/")
	cur := root
	for i, s := range segs {
		isLeaf := i == len(segs)-1
		child := find(cur, s)
		if child == nil {
			joined := strings.Join(segs[:i+1], "/")
			child = &Node{Name: s, Path: joined, IsDir: !isLeaf}
			cur.Children = append(cur.Children, child)
		}
		if isLeaf {
			child.Status = status
		}
		cur = child
	}
}

func find(parent *Node, name string) *Node {
	for _, c := range parent.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func sortNode(n *Node) {
	sort.SliceStable(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return a.Name < b.Name
	})
	for _, c := range n.Children {
		if c.IsDir {
			sortNode(c)
		}
	}
}

// propagateStatus bubbles child status up to parent dirs so folders show a
// marker when any descendant is changed. Priority: D > R > A > M > ? > none.
func propagateStatus(n *Node, statuses map[string]git.Status) git.Status {
	if !n.IsDir {
		return n.Status
	}
	var best git.Status
	for _, c := range n.Children {
		s := propagateStatus(c, statuses)
		best = higher(best, s)
	}
	n.Status = best
	return best
}

func higher(a, b git.Status) git.Status {
	rank := func(s git.Status) int {
		switch s {
		case git.StatusDeleted:
			return 5
		case git.StatusRenamed:
			return 4
		case git.StatusAdded:
			return 3
		case git.StatusModified:
			return 2
		case git.StatusUntracked:
			return 1
		}
		return 0
	}
	if rank(a) >= rank(b) {
		return a
	}
	return b
}
