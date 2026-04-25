package editor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ottaaa/skry/internal/git"
)

type Mode int

const (
	ModeEmpty Mode = iota
	ModeView
	ModeSplit
	ModeEdit
	ModeCommitDiff
	ModeBlame
	ModeFolder
	ModeBinary
)

type Model struct {
	repoRoot string // absolute
	path     string // relative to repoRoot
	absPath  string
	status   git.Status
	mode     Mode
	width    int
	height   int

	view     viewMode
	edit     editMode
	diffRows []DiffRow
	diffTop  int

	// ModeCommitDiff context
	commitSha     string
	commitSubject string
	commitShort   string

	// ModeBlame state
	blameLines []git.BlameLine
	blameTop   int

	// ModeFolder state
	folderEntries []folderEntry
	folderTop     int

	// ModeBinary state
	binarySize int64

	loadErr string
	focused bool
	message string
}

type SavedMsg struct{ Path string }

func New(repoRoot string) Model {
	return Model{
		repoRoot: repoRoot,
		mode:     ModeEmpty,
		view:     newViewMode(),
		edit:     newEditMode(),
	}
}

func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	bodyH := max(h-2, 1) // header line + status line inside the editor pane
	m.view.SetSize(w, bodyH)
	m.edit.SetSize(w, bodyH)
}

func (m *Model) SetFocused(v bool) { m.focused = v }
func (m Model) Focused() bool      { return m.focused }
func (m Model) Mode() Mode         { return m.mode }
func (m Model) Path() string       { return m.path }
func (m Model) Dirty() bool        { return m.mode == ModeEdit && m.edit.Dirty() }
func (m Model) Searching() bool    { return m.view.Searching() }

// GoToLine centers the viewport on the 1-based `line`. The interpretation
// depends on the current mode:
//   - ModeView: line is the line number in the file content
//   - ModeSplit / ModeCommitDiff: line is the right-hand (working tree /
//     commit) line number — the diff viewport scrolls to the row whose
//     RightNum matches
//
// line <= 0 is a no-op. Other modes (Blame, Edit, Folder, Binary) currently
// ignore the request.
func (m *Model) GoToLine(line int) {
	if line <= 0 {
		return
	}
	switch m.mode {
	case ModeView:
		m.view.GoToLine(line)
	case ModeSplit, ModeCommitDiff:
		idx := findRightLineRow(m.diffRows, line)
		if idx < 0 {
			return
		}
		viewportH := max(m.height-2, 1)
		maxTop := max(len(m.diffRows)-viewportH, 0)
		m.diffTop = min(max(idx-viewportH/2, 0), maxTop)
	}
}

// findRightLineRow returns the index of the first diff row whose RightNum
// equals `line` (1-based). Returns -1 for line <= 0 or when no row matches
// (the latter can happen if the requested line was deleted entirely).
func findRightLineRow(rows []DiffRow, line int) int {
	if line <= 0 {
		return -1
	}
	for i, r := range rows {
		if r.RightNum == line {
			return i
		}
	}
	return -1
}

// Open loads the given path (relative to the repo root) and picks a mode
// based on the supplied status.
func (m *Model) Open(path string, status git.Status) error {
	m.path = path
	m.status = status
	m.absPath = filepath.Join(m.repoRoot, path)
	m.message = ""
	m.loadErr = ""
	m.commitSha = ""

	raw, err := readWorkingBytes(m.absPath)
	if err != nil {
		m.mode = ModeEmpty
		m.loadErr = err.Error()
		return err
	}
	if isBinary(raw) {
		m.binarySize = int64(len(raw))
		m.mode = ModeBinary
		return nil
	}
	content := string(raw)
	m.view.Load(path, content)
	if isChanged(status) && !isNewFile(status) {
		// If HEAD side is binary (e.g. an image being replaced by text), skip
		// SplitDiff — AlignLines would produce garbage.
		headRaw, _ := git.HeadFileBytes(m.repoRoot, path)
		if isBinary(headRaw) {
			m.binarySize = int64(len(headRaw))
			m.mode = ModeBinary
			return nil
		}
		m.loadSplit(path, content)
		m.mode = ModeSplit
	} else {
		m.mode = ModeView
	}
	return nil
}

// isNewFile reports whether the status represents a file that exists only in
// the working tree (no HEAD version), so SplitDiff has no useful base side.
func isNewFile(s git.Status) bool {
	return s == git.StatusAdded || s == git.StatusUntracked
}

// OpenCommitDiff renders the parent-vs-commit diff for a single file. If the
// commit has no parent (root commit), base content is treated as empty.
func (m *Model) OpenCommitDiff(sha, short, subject, path string) {
	m.path = path
	m.absPath = filepath.Join(m.repoRoot, path)
	m.commitSha = sha
	m.commitShort = short
	m.commitSubject = subject
	m.status = ""
	m.message = ""
	m.loadErr = ""
	parent, _ := git.ParentOf(m.repoRoot, sha)
	newRaw, _ := git.FileAtBytes(m.repoRoot, sha, path)
	var baseRaw []byte
	if parent != "" {
		baseRaw, _ = git.FileAtBytes(m.repoRoot, parent, path)
	}
	if isBinary(newRaw) || isBinary(baseRaw) {
		m.binarySize = int64(len(newRaw))
		m.mode = ModeBinary
		return
	}
	m.diffRows = AlignLines(string(baseRaw), string(newRaw))
	m.diffTop = 0
	m.mode = ModeCommitDiff
}

type folderEntry struct {
	Name   string
	IsDir  bool
	Status git.Status
}

// OpenFolderPreview switches the editor to a folder listing view. Used by the
// tree to preview the node currently under the cursor.
func (m *Model) OpenFolderPreview(path string, entries []string, statuses map[string]git.Status) {
	m.path = path
	m.status = ""
	m.commitSha = ""
	m.loadErr = ""
	m.message = ""
	prefix := ""
	if path != "" {
		prefix = path + "/"
	}
	list := make([]folderEntry, 0, len(entries))
	for _, e := range entries {
		isDir := strings.HasSuffix(e, "/")
		name := strings.TrimSuffix(e, "/")
		child := prefix + name
		var st git.Status
		if isDir {
			st = statuses[child+"/"]
			if st == "" {
				// compute from children: any status under this dir bubbles up
				for p, s := range statuses {
					if strings.HasPrefix(p, child+"/") {
						st = s
						break
					}
				}
			}
		} else {
			st = statuses[child]
		}
		list = append(list, folderEntry{Name: e, IsDir: isDir, Status: st})
	}
	m.folderEntries = list
	m.folderTop = 0
	m.mode = ModeFolder
}

// ToggleBlame swaps the current ModeView (or ModeSplit) with ModeBlame. Errors
// are surfaced as the editor's message.
func (m *Model) ToggleBlame() {
	if m.mode == ModeBlame {
		m.mode = ModeView
		return
	}
	if m.path == "" {
		m.message = "no file to blame"
		return
	}
	lines, err := git.Blame(m.repoRoot, "", m.path)
	if err != nil {
		m.message = "blame: " + err.Error()
		return
	}
	if len(lines) == 0 {
		m.message = "no blame available (untracked or not yet committed)"
		return
	}
	m.blameLines = lines
	m.blameTop = 0
	m.mode = ModeBlame
}

func readWorking(absPath string) (string, error) {
	b, err := readWorkingBytes(absPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readWorkingBytes(absPath string) ([]byte, error) {
	b, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

// isBinary uses the same heuristic git does: a NUL byte in the first 8000
// bytes means the file is treated as binary.
func isBinary(b []byte) bool {
	n := min(len(b), 8000)
	return bytes.IndexByte(b[:n], 0) >= 0
}

func (m *Model) loadSplit(relPath, working string) {
	head, _ := git.HeadFile(m.repoRoot, relPath)
	m.diffRows = AlignLines(head, working)
	m.diffTop = 0
}

func isChanged(s git.Status) bool {
	switch s {
	case git.StatusModified, git.StatusAdded, git.StatusDeleted, git.StatusRenamed, git.StatusUntracked:
		return true
	}
	return false
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(km tea.KeyMsg) (Model, tea.Cmd) {
	if m.mode == ModeEdit {
		switch km.String() {
		case "esc":
			m.mode = ModeView
			content, _ := readWorking(m.absPath)
			m.view.Load(m.path, content)
			m.message = ""
			return m, nil
		case "ctrl+s":
			if err := m.edit.Save(); err != nil {
				m.message = "save failed: " + err.Error()
				return m, nil
			}
			m.message = "saved"
			return m, func() tea.Msg { return SavedMsg{Path: m.path} }
		}
		cmd := m.edit.Update(km)
		return m, cmd
	}

	if m.view.Searching() {
		switch km.String() {
		case "esc":
			m.view.CancelSearch()
		case "enter":
			m.view.ConfirmSearch()
		case "backspace":
			if q := m.view.Query(); len(q) > 0 {
				m.view.UpdateQuery(q[:len(q)-1])
			}
		default:
			if len(km.Runes) > 0 {
				m.view.UpdateQuery(m.view.Query() + string(km.Runes))
			}
		}
		return m, nil
	}

	switch km.String() {
	case "d":
		if m.mode == ModeSplit {
			m.mode = ModeView
		} else if m.mode == ModeView && isChanged(m.status) {
			m.loadSplit(m.path, readWorkingOrEmpty(m.absPath))
			m.mode = ModeSplit
		}
	case "B":
		// Blame only works for working-tree files, not commit-diff or binary.
		if m.mode != ModeCommitDiff && m.mode != ModeBinary {
			m.ToggleBlame()
		}
	case "/":
		if m.mode == ModeView {
			m.view.StartSearch()
		}
	case "n":
		if m.mode == ModeView {
			m.view.NextMatch()
		}
	case "N":
		if m.mode == ModeView {
			m.view.PrevMatch()
		}
	case "i", "e":
		if (m.mode == ModeView || m.mode == ModeSplit || m.mode == ModeBlame) && m.path != "" && m.commitSha == "" {
			content, err := os.ReadFile(m.absPath)
			if err == nil {
				if err := m.edit.Load(m.absPath, m.path, string(content)); err == nil {
					m.mode = ModeEdit
					m.message = ""
				}
			}
		}
	case "up", "k":
		m.scrollUp(1)
	case "down", "j":
		m.scrollDown(1)
	case "pgup":
		m.scrollUp(m.height - 2)
	case "pgdown", " ":
		m.scrollDown(m.height - 2)
	case "g":
		m.scrollTop()
	case "G":
		m.scrollBottom()
	}
	return m, nil
}

func readWorkingOrEmpty(absPath string) string {
	s, _ := readWorking(absPath)
	return s
}

func (m *Model) scrollUp(n int) {
	switch m.mode {
	case ModeView:
		for range n {
			m.view.ScrollUp()
		}
	case ModeSplit, ModeCommitDiff:
		m.diffTop -= n
		if m.diffTop < 0 {
			m.diffTop = 0
		}
	case ModeBlame:
		m.blameTop -= n
		if m.blameTop < 0 {
			m.blameTop = 0
		}
	}
}

func (m *Model) scrollDown(n int) {
	switch m.mode {
	case ModeView:
		for range n {
			m.view.ScrollDown()
		}
	case ModeSplit, ModeCommitDiff:
		max := len(m.diffRows) - (m.height - 2)
		if max < 0 {
			max = 0
		}
		m.diffTop += n
		if m.diffTop > max {
			m.diffTop = max
		}
	case ModeBlame:
		max := len(m.blameLines) - (m.height - 2)
		if max < 0 {
			max = 0
		}
		m.blameTop += n
		if m.blameTop > max {
			m.blameTop = max
		}
	}
}

func (m *Model) scrollTop() {
	switch m.mode {
	case ModeView:
		m.view.Top()
	case ModeSplit, ModeCommitDiff:
		m.diffTop = 0
	case ModeBlame:
		m.blameTop = 0
	}
}

func (m *Model) scrollBottom() {
	switch m.mode {
	case ModeView:
		m.view.Bottom()
	case ModeSplit, ModeCommitDiff:
		max := len(m.diffRows) - (m.height - 2)
		if max < 0 {
			max = 0
		}
		m.diffTop = max
	case ModeBlame:
		max := len(m.blameLines) - (m.height - 2)
		if max < 0 {
			max = 0
		}
		m.blameTop = max
	}
}

func (m Model) View() string {
	header := m.renderHeader()
	body := ""
	bodyH := max(m.height-2, 1)
	switch m.mode {
	case ModeEmpty:
		body = lipgloss.NewStyle().Faint(true).Render("Select a file to view.")
		if m.loadErr != "" {
			body = "load error: " + m.loadErr
		}
	case ModeView:
		body = m.view.Render()
	case ModeSplit, ModeCommitDiff:
		body = RenderSplit(m.diffRows, m.diffTop, bodyH, m.width)
	case ModeEdit:
		body = m.edit.View()
	case ModeBlame:
		body = RenderBlame(m.blameLines, m.blameTop, bodyH, m.width)
	case ModeFolder:
		body = renderFolder(m.folderEntries, m.folderTop, bodyH, m.width)
	case ModeBinary:
		body = renderBinary(m.binarySize, m.width, bodyH)
	}
	status := m.renderStatus()
	return header + "\n" + body + "\n" + status
}

func renderBinary(size int64, w, h int) string {
	msg := fmt.Sprintf("Binary file — not shown (%s)", humanSize(size))
	style := lipgloss.NewStyle().Faint(true)
	pad := max(h/2, 0)
	return strings.Repeat("\n", pad) + style.Render(msg)
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (m Model) renderHeader() string {
	if m.path == "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("(no file)")
	}
	modeTag := ""
	switch m.mode {
	case ModeView:
		if isChanged(m.status) {
			modeTag = "[*View|SplitDiff]"
		} else {
			modeTag = "[View]"
		}
	case ModeSplit:
		modeTag = "[View|*SplitDiff]"
	case ModeEdit:
		if m.edit.Dirty() {
			modeTag = "[Edit *]"
		} else {
			modeTag = "[Edit]"
		}
	case ModeBlame:
		modeTag = "[Blame]"
	case ModeCommitDiff:
		modeTag = "[Commit " + m.commitShort + "]"
	case ModeFolder:
		modeTag = "[Folder]"
	case ModeBinary:
		modeTag = "[Binary]"
	}
	statusTag := ""
	if m.status != "" {
		statusTag = " [" + string(m.status) + "]"
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5")).Render(m.path)
	tag := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Render(modeTag)
	return fmt.Sprintf("%s%s  %s", title, statusTag, tag)
}

func (m Model) renderStatus() string {
	var bits []string
	if m.view.Searching() {
		bits = append(bits, "/"+m.view.Query()+"_")
	} else if n, total := m.view.MatchInfo(); total > 0 && m.mode == ModeView {
		bits = append(bits, fmt.Sprintf("%d/%d", n, total))
	}
	if m.message != "" {
		bits = append(bits, m.message)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render(strings.Join(bits, "  "))
}

// Reload re-reads the working file (after an external change or a save).
// Reload re-reads the working file after an external change. It is a no-op
// when the user is actively editing (so we don't clobber their buffer) and
// when the current mode is showing a commit-pinned view (commit diff, blame,
// binary placeholder, folder listing) where the working tree is irrelevant.
func (m *Model) Reload() {
	if m.path == "" {
		return
	}
	if m.mode == ModeEdit || m.mode == ModeCommitDiff || m.mode == ModeFolder {
		return
	}
	raw, err := readWorkingBytes(m.absPath)
	if err != nil {
		// File went away, or became unreadable. Leave the last-good view
		// rather than flashing an error for what's often a transient rename.
		return
	}
	if isBinary(raw) {
		m.binarySize = int64(len(raw))
		m.mode = ModeBinary
		return
	}
	// If we were in Binary mode and the file is now text, drop back to View.
	if m.mode == ModeBinary {
		m.mode = ModeView
	}
	content := string(raw)
	m.view.Load(m.path, content)
	if m.mode == ModeSplit {
		m.loadSplit(m.path, content)
	}
	if m.mode == ModeBlame {
		// Blame needs a fresh run; fall back to View if it errors.
		lines, err := git.Blame(m.repoRoot, "", m.path)
		if err == nil && len(lines) > 0 {
			m.blameLines = lines
		} else {
			m.mode = ModeView
		}
	}
}

