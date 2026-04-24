package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/skry/internal/branchui"
	"github.com/ottaaa/skry/internal/clipboard"
	"github.com/ottaaa/skry/internal/editor"
	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
	"github.com/ottaaa/skry/internal/helpui"
	"github.com/ottaaa/skry/internal/logui"
	"github.com/ottaaa/skry/internal/modal"
	"github.com/ottaaa/skry/internal/search"
	"github.com/ottaaa/skry/internal/tree"
	"github.com/ottaaa/skry/internal/worktreeui"
)

type Focus int

const (
	FocusTree Focus = iota
	FocusEditor
)

const treeWidthMin = 24

type Model struct {
	repoRoot string
	width    int
	height   int
	focus    Focus

	tree   tree.Model
	editor editor.Model
	modal  modal.Modal

	files    []string
	statuses map[string]git.Status
	summary  summary
	branch   string
	recent   []string
	message  string

	treeOuter int // current outer width of the tree pane; 0 = auto-init on first resize
}

type summary struct{ M, A, D, Other int }

func New(repoRoot string) Model {
	m := Model{
		repoRoot: repoRoot,
		focus:    FocusTree,
		tree:     tree.New(),
		editor:   editor.New(repoRoot),
		statuses: map[string]git.Status{},
	}
	m.tree.SetFocused(true)
	return m
}

func (m Model) Init() tea.Cmd {
	return loadRepo(m.repoRoot)
}

type repoLoadedMsg struct {
	root     string
	files    []string
	statuses map[string]git.Status
	summary  summary
	branch   string
	err      error
}

func loadRepo(root string) tea.Cmd {
	return func() tea.Msg {
		top, err := git.TopLevel(root)
		if err != nil {
			return repoLoadedMsg{root: root, err: err}
		}
		files, err := git.ListFiles(top)
		if err != nil {
			return repoLoadedMsg{root: top, err: err}
		}
		entries, err := git.Statuses(top)
		if err != nil {
			return repoLoadedMsg{root: top, err: err}
		}
		statuses := map[string]git.Status{}
		var sum summary
		for _, e := range entries {
			statuses[e.Path] = e.Status
			switch e.Status {
			case git.StatusModified:
				sum.M++
			case git.StatusAdded:
				sum.A++
			case git.StatusDeleted:
				sum.D++
			default:
				sum.Other++
			}
		}
		// Merge untracked into files if ls-files missed them (it shouldn't with --others).
		seen := make(map[string]bool, len(files))
		for _, f := range files {
			seen[f] = true
		}
		for _, e := range entries {
			if !seen[e.Path] && e.Status != git.StatusDeleted {
				files = append(files, e.Path)
				seen[e.Path] = true
			}
		}
		branch, _ := git.CurrentBranch(top)
		return repoLoadedMsg{root: top, files: files, statuses: statuses, summary: sum, branch: branch}
	}
}

func refreshStatus(root string) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.Statuses(root)
		if err != nil {
			return repoLoadedMsg{root: root, err: err}
		}
		statuses := map[string]git.Status{}
		var sum summary
		for _, e := range entries {
			statuses[e.Path] = e.Status
			switch e.Status {
			case git.StatusModified:
				sum.M++
			case git.StatusAdded:
				sum.A++
			case git.StatusDeleted:
				sum.D++
			default:
				sum.Other++
			}
		}
		return statusRefreshedMsg{statuses: statuses, summary: sum}
	}
}

type statusRefreshedMsg struct {
	statuses map[string]git.Status
	summary  summary
}

type logLoadedMsg struct {
	commits []git.Commit
	err     error
}

func loadLog(root string) tea.Cmd {
	return func() tea.Msg {
		commits, err := git.Log(root, 200)
		return logLoadedMsg{commits: commits, err: err}
	}
}

type branchesLoadedMsg struct {
	branches []git.Branch
	dirty    bool
	err      error
}

func loadBranches(root string) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.Branches(root)
		if err != nil {
			return branchesLoadedMsg{err: err}
		}
		dirty, err := git.WorkingDirty(root)
		if err != nil {
			return branchesLoadedMsg{err: err}
		}
		return branchesLoadedMsg{branches: branches, dirty: dirty}
	}
}

type branchSwitchedMsg struct {
	name string
	err  error
}

func switchBranch(root, name string, force bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if force {
			err = git.SwitchForce(root, name)
		} else {
			err = git.Switch(root, name)
		}
		return branchSwitchedMsg{name: name, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applySizes()
		return m, nil

	case repoLoadedMsg:
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.repoRoot = msg.root
		m.files = msg.files
		m.statuses = msg.statuses
		m.summary = msg.summary
		m.branch = msg.branch
		m.editor = editor.New(msg.root)
		m.editor.SetFocused(m.focus == FocusEditor)
		m.applySizes()
		m.tree.SetFiles(msg.files, msg.statuses)
		m.previewAt(m.tree.CurrentMsg())
		return m, nil

	case statusRefreshedMsg:
		m.statuses = msg.statuses
		m.summary = msg.summary
		m.tree.SetFiles(m.files, msg.statuses)
		m.editor.Reload()
		return m, nil

	case events.OpenFileMsg:
		return m.handleOpenFile(msg)

	case events.CursorMovedMsg:
		m.previewAt(msg)
		return m, nil

	case events.CloseModalMsg:
		m.modal = nil
		return m, nil

	case events.SwitchWorktreeMsg:
		m.modal = nil
		return m, loadRepo(msg.Path)

	case editor.SavedMsg:
		return m, refreshStatus(m.repoRoot)

	case logLoadedMsg:
		if msg.err != nil {
			m.message = "log: " + msg.err.Error()
			return m, nil
		}
		if len(msg.commits) == 0 {
			m.message = "no commits yet"
			return m, nil
		}
		m.modal = logui.NewLogModal(msg.commits)
		return m, m.modal.Init()

	case branchesLoadedMsg:
		if msg.err != nil {
			m.message = "branches: " + msg.err.Error()
			return m, nil
		}
		if len(msg.branches) == 0 {
			m.message = "no branches yet"
			return m, nil
		}
		m.modal = branchui.New(msg.branches, msg.dirty)
		return m, m.modal.Init()

	case events.LogCommitSelectedMsg:
		files, err := git.CommitFiles(m.repoRoot, msg.Sha)
		if err != nil {
			m.message = "commit files: " + err.Error()
			m.modal = nil
			return m, nil
		}
		if len(files) == 0 {
			m.message = "commit has no file changes"
			m.modal = nil
			return m, nil
		}
		m.modal = logui.NewCommitFilesModal(msg.Sha, msg.Short, msg.Subject, files)
		return m, m.modal.Init()

	case events.CommitFileSelectedMsg:
		m.modal = nil
		m.editor.OpenCommitDiff(msg.Sha, msg.Short, msg.Subject, msg.Path)
		m.focus = FocusEditor
		m.tree.SetFocused(false)
		m.editor.SetFocused(true)
		return m, nil

	case events.SwitchBranchMsg:
		m.modal = nil
		return m, switchBranch(m.repoRoot, msg.Name, msg.Force)

	case branchSwitchedMsg:
		if msg.err != nil {
			m.message = "switch: " + msg.err.Error()
			return m, nil
		}
		m.message = "switched to " + msg.name
		return m, loadRepo(m.repoRoot)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.modal != nil {
		newModal, cmd := m.modal.Update(msg)
		m.modal = newModal
		return m, cmd
	}
	return m, nil
}

func (m Model) handleOpenFile(msg events.OpenFileMsg) (tea.Model, tea.Cmd) {
	m.modal = nil
	status := m.statuses[msg.Path]
	if err := m.editor.Open(msg.Path, status); err != nil {
		m.message = "open: " + err.Error()
		return m, nil
	}
	m.focus = FocusEditor
	m.tree.SetFocused(false)
	m.editor.SetFocused(true)
	m.pushRecent(msg.Path)
	return m, nil
}

func (m *Model) pushRecent(path string) {
	for i, p := range m.recent {
		if p == path {
			m.recent = append(m.recent[:i], m.recent[i+1:]...)
			break
		}
	}
	m.recent = append([]string{path}, m.recent...)
	if len(m.recent) > 50 {
		m.recent = m.recent[:50]
	}
}

func (m Model) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := km.String()

	// Always-on globals run before modal routing so Ctrl+Q/Ctrl+C still quit
	// with any modal open.
	switch s {
	case "ctrl+q", "ctrl+c":
		return m, tea.Quit
	}

	if m.modal != nil {
		newModal, cmd := m.modal.Update(km)
		m.modal = newModal
		return m, cmd
	}

	// q quits only when we aren't absorbing text input.
	if s == "q" && !m.absorbingText() {
		return m, tea.Quit
	}

	// Global navigation / modal openers only when we aren't in text-entry contexts.
	if !m.absorbingText() {
		switch s {
		case "tab":
			if m.focus == FocusTree {
				m.focus = FocusEditor
				m.tree.SetFocused(false)
				m.editor.SetFocused(true)
			} else {
				m.focus = FocusTree
				m.tree.SetFocused(true)
				m.editor.SetFocused(false)
			}
			return m, nil
		case "left":
			m.focus = FocusTree
			m.tree.SetFocused(true)
			m.editor.SetFocused(false)
			return m, nil
		case "right":
			m.focus = FocusEditor
			m.tree.SetFocused(false)
			m.editor.SetFocused(true)
			return m, nil
		case "p":
			m.modal = search.NewFileModal(m.files)
			return m, m.modal.Init()
		case "r":
			m.modal = search.NewRecentModal(m.recent)
			return m, m.modal.Init()
		case "F":
			m.modal = search.NewGrepModal(m.repoRoot, m.files)
			return m, m.modal.Init()
		case "w":
			wts, err := git.Worktrees(m.repoRoot)
			if err != nil {
				m.message = "worktree: " + err.Error()
				return m, nil
			}
			m.modal = worktreeui.New(wts, m.repoRoot)
			return m, m.modal.Init()
		case "[", "<", "alt+h":
			m.treeOuter = clampTreeOuter(m.currentTreeOuter()-4, m.width)
			m.applySizes()
			return m, nil
		case "]", ">", "alt+l":
			m.treeOuter = clampTreeOuter(m.currentTreeOuter()+4, m.width)
			m.applySizes()
			return m, nil
		case "t":
			m.tree.ToggleFlat()
			return m, nil
		case "y":
			path := m.currentPath()
			if path == "" {
				m.message = "nothing to copy"
				return m, nil
			}
			if err := clipboard.Copy(path); err != nil {
				m.message = "copy failed: " + err.Error()
			} else {
				m.message = "copied: " + path
			}
			return m, nil
		case "?":
			m.modal = helpui.New()
			return m, m.modal.Init()
		case "L":
			return m, loadLog(m.repoRoot)
		case "b":
			return m, loadBranches(m.repoRoot)
		}
	}

	// Esc in View/SplitDiff returns focus to the file tree. In Edit mode and
	// in-file search, editor consumes Esc first (exit edit / cancel search),
	// so we only intercept when we aren't absorbing text.
	if s == "esc" && !m.absorbingText() && m.focus == FocusEditor {
		m.focus = FocusTree
		m.tree.SetFocused(true)
		m.editor.SetFocused(false)
		return m, nil
	}

	// Route to focused pane.
	switch m.focus {
	case FocusTree:
		newTree, cmd := m.tree.Update(km)
		m.tree = newTree
		return m, cmd
	case FocusEditor:
		newEd, cmd := m.editor.Update(km)
		m.editor = newEd
		return m, cmd
	}
	return m, nil
}

func (m *Model) previewAt(msg events.CursorMovedMsg) {
	if msg.Path == "" {
		return
	}
	if msg.IsDir {
		entries := m.folderEntries(msg.Path)
		m.editor.OpenFolderPreview(msg.Path, entries, m.statuses)
		return
	}
	m.editor.Open(msg.Path, m.statuses[msg.Path])
}

// folderEntries returns the immediate children of dirPath in the current file
// list. Directories are suffixed with "/".
func (m Model) folderEntries(dirPath string) []string {
	prefix := dirPath + "/"
	seen := map[string]bool{}
	var res []string
	for _, f := range m.files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := f[len(prefix):]
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			name := rest[:idx] + "/"
			if !seen[name] {
				seen[name] = true
				res = append(res, name)
			}
		} else {
			res = append(res, rest)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		di := strings.HasSuffix(res[i], "/")
		dj := strings.HasSuffix(res[j], "/")
		if di != dj {
			return di
		}
		return res[i] < res[j]
	})
	return res
}

// currentPath prefers the editor's open file; falls back to the tree's
// currently-highlighted leaf.
func (m Model) currentPath() string {
	if p := m.editor.Path(); p != "" {
		return p
	}
	return m.tree.SelectedPath()
}

func (m Model) currentTreeOuter() int {
	if m.treeOuter == 0 {
		return m.width / 3
	}
	return m.treeOuter
}

func (m Model) absorbingText() bool {
	if m.focus == FocusEditor {
		if m.editor.Mode() == editor.ModeEdit {
			return true
		}
		if m.editor.Searching() {
			return true
		}
	}
	if m.focus == FocusTree && m.tree.Filtering() {
		return true
	}
	return false
}

func (m *Model) applySizes() {
	if m.width == 0 || m.height == 0 {
		return
	}
	headerH := 1
	footerH := 1
	bodyH := m.height - headerH - footerH
	if bodyH < 3 {
		bodyH = 3
	}
	if m.treeOuter == 0 {
		m.treeOuter = m.width / 3
	}
	m.treeOuter = clampTreeOuter(m.treeOuter, m.width)
	editorOuter := m.width - m.treeOuter
	treeInner := m.treeOuter - 2
	editorInner := editorOuter - 2
	innerH := bodyH - 2
	if treeInner < 4 {
		treeInner = 4
	}
	if editorInner < 4 {
		editorInner = 4
	}
	if innerH < 1 {
		innerH = 1
	}
	m.tree.SetSize(treeInner, innerH)
	m.editor.SetSize(editorInner, innerH)
}

func clampTreeOuter(v, totalW int) int {
	min := treeWidthMin
	max := totalW * 7 / 10
	if max < min+8 {
		max = min + 8
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()
	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	if m.modal != nil {
		overlay := m.modal.View(m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
	}
	return screen
}

func (m Model) renderHeader() string {
	repo := filepath.Base(m.repoRoot)
	summary := fmt.Sprintf("[M:%d A:%d D:%d]", m.summary.M, m.summary.A, m.summary.D)
	branch := m.branch
	if branch == "" {
		branch = "(detached)"
	}
	parts := []string{
		TitleStyle.Render(repo),
		HintStyle.Render("branch: " + branch),
		HintStyle.Render(summary),
	}
	line := strings.Join(parts, "  ")
	if ansi.StringWidth(line) > m.width-2 {
		line = ansi.Truncate(line, m.width-2, "…")
	}
	return HeaderStyle.Width(m.width).Render(line)
}

func (m Model) renderFooter() string {
	msg := m.message
	hint := "q quit  Tab/←/→ pane  p file  r recent  F grep  L log  b branch  w worktree  B blame  t flat  d diff  / find  y copy  i edit  ? help"
	if msg != "" {
		hint = msg + "  |  " + hint
	}
	if ansi.StringWidth(hint) > m.width-2 {
		hint = ansi.Truncate(hint, m.width-2, "…")
	}
	return FooterStyle.Width(m.width).Render(hint)
}

func (m Model) renderBody() string {
	treeOuterW := m.treeOuter
	if treeOuterW == 0 {
		treeOuterW = m.width / 3
	}
	treeOuterW = clampTreeOuter(treeOuterW, m.width)
	editorOuterW := m.width - treeOuterW
	bodyH := m.height - 2

	treeStyle := PaneStyle
	editorStyle := PaneStyle
	if m.focus == FocusTree {
		treeStyle = ActivePane
	} else {
		editorStyle = ActivePane
	}

	left := treeStyle.Width(treeOuterW - 2).Height(bodyH - 2).Render(m.tree.View())
	right := editorStyle.Width(editorOuterW - 2).Height(bodyH - 2).Render(m.editor.View())
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

