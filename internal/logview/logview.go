// Package logview implements the persistent two-pane Log view: a left pane
// showing the ASCII commit graph + log of HEAD's ancestry (newest first),
// and a middle pane showing the focused commit's metadata and changed
// files. The editor pane (managed by the host app) renders the per-file
// SplitDiff.
package logview

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
)

type Focus int

const (
	FocusGraph Focus = iota
	FocusFiles
)

// Model holds the state of the two log panes. The editor pane is owned by
// the parent app — Model only emits events telling the parent which commit
// or file is currently focused.
type Model struct {
	rows    []git.GraphRow // graph + log rows (mixed commit and pure-graph rows)
	commits []int          // indices into rows that carry a commit (cursor lands here only)
	cursor  int            // index into commits
	top     int            // first visible row in rows[]

	// Files pane state (driven by the parent feeding SetFiles after a
	// commit is focused).
	files     []git.StatusEntry
	body      string // full commit message body (subject + body)
	fileCur   int
	fileTop   int
	loadingSh string // sha currently being loaded (UI hint while parent fetches)

	focus  Focus
	width  int // graph pane width
	width2 int // files pane width
	height int
}

// New builds a logview pre-populated with `rows`. The cursor lands on the
// first commit row (newest, since git log --graph emits newest first). If
// rows contains no commits the model is still safe to use — it will simply
// render empty.
func New(rows []git.GraphRow) Model {
	m := Model{}
	m.SetRows(rows)
	m.focus = FocusGraph
	return m
}

// SetRows replaces the graph rows in place — used when L flips the layout
// immediately and the actual `git log --graph` result arrives later. The
// cursor resets to the newest commit.
func (m *Model) SetRows(rows []git.GraphRow) {
	m.rows = rows
	m.commits = m.commits[:0]
	for i, r := range rows {
		if r.Commit != nil {
			m.commits = append(m.commits, i)
		}
	}
	m.cursor = 0
	m.top = 0
	m.files = nil
	m.body = ""
	m.fileCur = 0
	m.fileTop = 0
}

// NeighborShas returns up to n commit SHAs before and after the cursor
// (inclusive of the immediate next/prev), capped at boundary edges. Used
// to drive prefetch of likely-next-targets without duplicating the
// foreground request.
func (m Model) NeighborShas(n int) []string {
	if n <= 0 || len(m.commits) == 0 {
		return nil
	}
	out := make([]string, 0, 2*n)
	for d := 1; d <= n; d++ {
		if i := m.cursor + d; i < len(m.commits) {
			out = append(out, m.rows[m.commits[i]].Commit.Hash)
		}
		if i := m.cursor - d; i >= 0 {
			out = append(out, m.rows[m.commits[i]].Commit.Hash)
		}
	}
	return out
}

// SetSize tells the model how much horizontal/vertical space each pane has.
// w1 = graph pane inner width, w2 = files pane inner width, h = inner height.
func (m *Model) SetSize(w1, w2, h int) { m.width, m.width2, m.height = w1, w2, h }

// SetFocus moves keyboard focus between the two panes.
func (m *Model) SetFocus(f Focus) { m.focus = f }

// Focus reports which pane has keyboard focus.
func (m Model) Focus() Focus { return m.focus }

// CommitCount is the number of cursor-stoppable commit rows in the log.
func (m Model) CommitCount() int { return len(m.commits) }

// SelectedCommit returns the commit currently under the graph cursor, or
// nil if the log is empty.
func (m Model) SelectedCommit() *git.Commit {
	if len(m.commits) == 0 || m.cursor >= len(m.commits) {
		return nil
	}
	return m.rows[m.commits[m.cursor]].Commit
}

// SelectedFile returns the file under the files-pane cursor, or nil if the
// commit has no files (or none has been loaded yet).
func (m Model) SelectedFile() *git.StatusEntry {
	if len(m.files) == 0 || m.fileCur >= len(m.files) {
		return nil
	}
	e := m.files[m.fileCur]
	return &e
}

// SetFiles is called by the parent after fetching a commit's name-status
// list. body is the full commit message (subject + body). loadingSh is
// cleared once the matching sha arrives.
func (m *Model) SetFiles(sha string, files []git.StatusEntry, body string) {
	if sha != "" && m.loadingSh != "" && sha != m.loadingSh {
		// A newer focus event is in flight; ignore the stale payload.
		return
	}
	m.files = files
	m.body = body
	m.fileCur = 0
	m.fileTop = 0
	m.loadingSh = ""
}

// markLoading is called when we've just emitted a LogCommitFocusedMsg, so
// SetFiles can ignore stale answers if the user moved past the commit
// before the parent's git call returned.
func (m *Model) markLoading(sha string) { m.loadingSh = sha }

// Update handles key input. Returns the (possibly mutated) model and any
// command(s) emitted as a result of cursor changes.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.focus { //nolint:exhaustive // two-valued enum
	case FocusGraph:
		return m.updateGraph(km)
	case FocusFiles:
		return m.updateFiles(km)
	}
	return m, nil
}

func (m Model) updateGraph(km tea.KeyMsg) (Model, tea.Cmd) {
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			return m, m.emitCommitFocus()
		}
	case "down", "j":
		if m.cursor < len(m.commits)-1 {
			m.cursor++
			return m, m.emitCommitFocus()
		}
	case "g":
		if m.cursor != 0 && len(m.commits) > 0 {
			m.cursor = 0
			return m, m.emitCommitFocus()
		}
	case "G":
		if last := len(m.commits) - 1; last >= 0 && m.cursor != last {
			m.cursor = last
			return m, m.emitCommitFocus()
		}
	}
	return m, nil
}

func (m Model) updateFiles(km tea.KeyMsg) (Model, tea.Cmd) {
	switch km.String() {
	case "up", "k":
		if m.fileCur > 0 {
			m.fileCur--
			return m, m.emitFileFocus()
		}
	case "down", "j":
		if m.fileCur < len(m.files)-1 {
			m.fileCur++
			return m, m.emitFileFocus()
		}
	case "g":
		if m.fileCur != 0 {
			m.fileCur = 0
			return m, m.emitFileFocus()
		}
	case "G":
		if last := len(m.files) - 1; last >= 0 && m.fileCur != last {
			m.fileCur = last
			return m, m.emitFileFocus()
		}
	}
	return m, nil
}

// EmitInitialFocus is what the host calls right after entering Log mode so
// the parent can prime the files pane with the cursor's commit (rather than
// waiting for a key press).
func (m *Model) EmitInitialFocus() tea.Cmd {
	if c := m.SelectedCommit(); c != nil {
		m.markLoading(c.Hash)
		sha, short, subj := c.Hash, c.Short, c.Subject
		return func() tea.Msg {
			return events.LogCommitFocusedMsg{Sha: sha, Short: short, Subject: subj}
		}
	}
	return nil
}

func (m *Model) emitCommitFocus() tea.Cmd {
	c := m.SelectedCommit()
	if c == nil {
		return nil
	}
	m.markLoading(c.Hash)
	sha, short, subj := c.Hash, c.Short, c.Subject
	return func() tea.Msg {
		return events.LogCommitFocusedMsg{Sha: sha, Short: short, Subject: subj}
	}
}

// EmitCurrentFileFocus re-emits the current file selection. Used when the
// parent wants to (re)render the editor's diff pane — for example after
// SetFiles arrives with a new file list.
func (m *Model) EmitCurrentFileFocus() tea.Cmd { return m.emitFileFocus() }

func (m *Model) emitFileFocus() tea.Cmd {
	c := m.SelectedCommit()
	f := m.SelectedFile()
	if c == nil || f == nil {
		return nil
	}
	sha, short, subj, path := c.Hash, c.Short, c.Subject, f.Path
	return func() tea.Msg {
		return events.LogFileFocusedMsg{Sha: sha, Short: short, Subject: subj, Path: path}
	}
}

// View renders the two side-by-side panes joined horizontally. The host is
// expected to wrap this in its own pane border so Log mode looks visually
// consistent with the rest of the app.
func (m Model) View() string {
	left := m.renderGraph()
	right := m.renderFiles()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// LeftView / RightView return the two panes individually so the host can
// border each one (and highlight the focused pane separately).
func (m Model) LeftView() string  { return m.renderGraph() }
func (m Model) RightView() string { return m.renderFiles() }

func (m Model) renderGraph() string {
	w := max(m.width, 10)
	h := max(m.height, 1)
	if len(m.rows) == 0 {
		// Layout already switched but `git log --graph` hasn't returned yet.
		hint := lipgloss.NewStyle().Faint(true).Render("loading commit graph…")
		var lines []string
		// Center the hint vertically.
		pad := h / 2
		for range pad {
			lines = append(lines, strings.Repeat(" ", w))
		}
		lines = append(lines, padOrTruncate(hint, w))
		for len(lines) < h {
			lines = append(lines, strings.Repeat(" ", w))
		}
		return strings.Join(lines[:h], "\n")
	}
	// Map cursor (commits[]) to row index, then keep that row visible.
	cursorRow := 0
	if len(m.commits) > 0 {
		cursorRow = m.commits[m.cursor]
	}
	top := m.top
	if cursorRow < top {
		top = cursorRow
	}
	if cursorRow >= top+h {
		top = cursorRow - h + 1
	}
	if top < 0 {
		top = 0
	}
	var b strings.Builder
	for i := range h {
		idx := top + i
		if idx >= len(m.rows) {
			b.WriteString(strings.Repeat(" ", w))
		} else {
			line := renderGraphRow(m.rows[idx], w, idx == cursorRow && m.focus == FocusGraph)
			b.WriteString(line)
		}
		if i < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderGraphRow(r git.GraphRow, w int, selected bool) string {
	graphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	if r.Commit == nil {
		line := graphStyle.Render(r.Graph)
		return padOrTruncate(line, w)
	}
	short := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff")).Render(r.Commit.Short)
	date := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(r.Commit.Date)
	subj := r.Commit.Subject
	line := graphStyle.Render(r.Graph) + short + " " + date + " " + subj
	line = padOrTruncate(line, w)
	if selected {
		line = lipgloss.NewStyle().Background(lipgloss.Color("#3b4261")).Render(line)
	}
	return line
}

func (m Model) renderFiles() string {
	w := max(m.width2, 10)
	h := max(m.height, 1)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	var lines []string

	c := m.SelectedCommit()
	if c != nil {
		// Header: short + date/author, subject, body, separator.
		head := headStyle.Render(c.Short) + " " + metaStyle.Render(c.Date+" "+c.Author)
		lines = append(lines, padOrTruncate(head, w))
		lines = append(lines, padOrTruncate(c.Subject, w))
		body := strings.TrimSpace(strings.TrimPrefix(m.body, c.Subject))
		if body != "" {
			for line := range strings.SplitSeq(body, "\n") {
				lines = append(lines, padOrTruncate(metaStyle.Render(line), w))
			}
		}
		lines = append(lines, padOrTruncate(metaStyle.Render(strings.Repeat("─", w)), w))
	}

	// Cap the header to half the pane so the file list always has space.
	headerCap := h / 2
	if headerCap < 2 {
		headerCap = 2
	}
	if len(lines) > headerCap {
		lines = lines[:headerCap]
	}

	// Files list takes the remaining height.
	listH := h - len(lines)
	if listH < 1 {
		listH = 1
	}
	if m.fileCur < m.fileTop {
		m.fileTop = m.fileCur
	}
	if m.fileCur >= m.fileTop+listH {
		m.fileTop = m.fileCur - listH + 1
	}
	if m.fileTop < 0 {
		m.fileTop = 0
	}

	for i := range listH {
		idx := m.fileTop + i
		var line string
		if idx >= len(m.files) {
			line = strings.Repeat(" ", w)
		} else {
			line = formatFileRow(m.files[idx], w)
			if idx == m.fileCur && m.focus == FocusFiles {
				line = lipgloss.NewStyle().Background(lipgloss.Color("#3b4261")).Render(line)
			}
		}
		lines = append(lines, line)
	}

	// Final pad to exactly h lines.
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func formatFileRow(e git.StatusEntry, w int) string {
	mark := string(e.Status)
	if mark == "" {
		mark = " "
	}
	color := "#c0caf5"
	switch e.Status {
	case git.StatusModified:
		color = "#e0af68"
	case git.StatusAdded:
		color = "#9ece6a"
	case git.StatusDeleted:
		color = "#f7768e"
	case git.StatusRenamed:
		color = "#bb9af7"
	}
	markStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(mark)
	line := markStyled + " " + e.Path
	return padOrTruncate(line, w)
}

func padOrTruncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

