package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ottaaa/skry/internal/applog"
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
	"github.com/ottaaa/skry/internal/watcher"
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

	// Status bar state. setToast / toastExpiredMsg / renderToast live in
	// toast.go; we just hold the current toast and a sequence counter
	// here so that a fresh toast can supersede an in-flight expiry tick.
	toast    toast
	toastSeq int

	treeOuter int // current outer width of the tree pane; 0 = auto-init on first resize

	previewExtraH  int    // user delta on the modal-preview pane height (alt+up/alt+down)
	previewScroll  int    // line-offset added to the modal-preview top (pgup/pgdn)
	previewLastKey string // path+line key to detect cursor moves and reset scroll

	watcher *watcher.Watcher
	log     *applog.Logger
}

// fsChangedMsg is delivered when the file system watcher coalesced one or
// more events. The handler refreshes statuses/file list and reloads the
// editor's current file (if safe to do so).
type fsChangedMsg struct{}

// watcherStartedMsg delivers the watcher handle into the model so we can
// both close it later and start waiting for its events.
type watcherStartedMsg struct{ w *watcher.Watcher }

func startWatcher(root string, log *applog.Logger) tea.Cmd {
	return func() tea.Msg {
		var lg watcher.Logger
		if log != nil {
			lg = log
		}
		w, err := watcher.Start(root, lg)
		if err != nil {
			// Silently degrade: auto-reload is nice-to-have, not load-bearing.
			if log != nil {
				log.Log("app: watcher start failed", "root", root, "err", err.Error())
			}
			return watcherStartedMsg{w: nil}
		}
		return watcherStartedMsg{w: w}
	}
}

func waitForFS(w *watcher.Watcher) tea.Cmd {
	if w == nil {
		return nil
	}
	return func() tea.Msg {
		<-w.Events()
		return fsChangedMsg{}
	}
}

// fsReload is a lighter sibling of loadRepo: it refreshes the file list,
// statuses, and branch, but leaves the editor's currently-open file in place
// (loadRepo replaces the editor entirely, which would close the user's view).
func fsReload(root string) tea.Cmd {
	return func() tea.Msg {
		files, err := git.ListFiles(root)
		if err != nil {
			return fsReloadedMsg{err: err}
		}
		entries, err := git.Statuses(root)
		if err != nil {
			return fsReloadedMsg{err: err}
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
		branch, _ := git.CurrentBranch(root)
		return fsReloadedMsg{files: files, statuses: statuses, summary: sum, branch: branch}
	}
}

type fsReloadedMsg struct {
	files    []string
	statuses map[string]git.Status
	summary  summary
	branch   string
	err      error
}

type summary struct{ M, A, D, Other int }

func New(repoRoot string) Model {
	// Best-effort open: a write failure must never crash the TUI. The log is
	// diagnostic only (watcher errors, etc.); when Open fails we just run
	// without one.
	lg, _ := applog.Setup()
	m := Model{
		repoRoot: repoRoot,
		focus:    FocusTree,
		tree:     tree.New(),
		editor:   editor.New(repoRoot),
		statuses: map[string]git.Status{},
		log:      lg,
	}
	m.tree.SetFocused(true)
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadRepo(m.repoRoot), startWatcher(m.repoRoot, m.log))
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
			return m, m.setToast(msg.err.Error(), ToastError)
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

	case watcherStartedMsg:
		if msg.w == nil {
			return m, nil
		}
		m.watcher = msg.w
		return m, waitForFS(m.watcher)

	case fsChangedMsg:
		// Coalesce any further events into the next refresh by issuing a
		// single reload Cmd, then arm for the next watcher event.
		return m, tea.Batch(fsReload(m.repoRoot), waitForFS(m.watcher))

	case fsReloadedMsg:
		if msg.err != nil {
			// A partial read (e.g. git running in parallel) shouldn't be
			// noisy. Swallow to avoid overwriting the user's status line.
			return m, nil
		}
		m.files = msg.files
		m.statuses = msg.statuses
		m.summary = msg.summary
		if msg.branch != "" {
			m.branch = msg.branch
		}
		m.tree.SetFiles(msg.files, msg.statuses)
		m.editor.Reload()
		return m, nil

	case events.OpenFileMsg:
		return m.handleOpenFile(msg)

	case events.CursorMovedMsg:
		m.previewAt(msg)
		return m, nil

	case events.CloseModalMsg:
		m.modal = nil
		m.previewScroll = 0
		m.previewLastKey = ""
		return m, nil

	case events.SwitchWorktreeMsg:
		m.modal = nil
		if m.watcher != nil {
			_ = m.watcher.Close()
			m.watcher = nil
		}
		return m, tea.Batch(loadRepo(msg.Path), startWatcher(msg.Path, m.log))

	case toastExpiredMsg:
		// Stale ticks (a newer toast bumped seq) are silently ignored —
		// the live toast has its own pending expiry.
		if msg.seq == m.toast.seq {
			m.toast = toast{}
		}
		return m, nil

	case editor.SavedMsg:
		return m, refreshStatus(m.repoRoot)

	case editor.AutosaveTickMsg:
		// Forward to the editor; it owns the version-vs-current check and
		// only triggers a write when the tick is still authoritative.
		newEd, cmd := m.editor.Update(msg)
		m.editor = newEd
		return m, cmd

	case editor.AutoSavedMsg:
		// Q3 graduates from a quiet "m.message" to a kind-styled 3-second
		// toast. Failures stay visible long enough to read and are also
		// recorded in the persistent logfile for post-mortem.
		if msg.Err != nil {
			if m.log != nil {
				m.log.Log("editor: autosave failed", "path", msg.Path, "err", msg.Err.Error())
			}
			return m, m.setToast("auto-save failed: "+msg.Err.Error(), ToastError)
		}
		return m, tea.Batch(
			refreshStatus(m.repoRoot),
			m.setToast("Auto-saved "+msg.At.Format("15:04:05"), ToastSuccess),
		)

	case logLoadedMsg:
		if msg.err != nil {
			return m, m.setToast("log: "+msg.err.Error(), ToastError)
		}
		if len(msg.commits) == 0 {
			return m, m.setToast("no commits yet", ToastInfo)
		}
		m.modal = logui.NewLogModal(msg.commits)
		return m, m.modal.Init()

	case branchesLoadedMsg:
		if msg.err != nil {
			return m, m.setToast("branches: "+msg.err.Error(), ToastError)
		}
		if len(msg.branches) == 0 {
			return m, m.setToast("no branches yet", ToastInfo)
		}
		m.modal = branchui.New(msg.branches, msg.dirty)
		return m, m.modal.Init()

	case events.LogCommitSelectedMsg:
		files, err := git.CommitFiles(m.repoRoot, msg.Sha)
		if err != nil {
			m.modal = nil
			return m, m.setToast("commit files: "+err.Error(), ToastError)
		}
		if len(files) == 0 {
			m.modal = nil
			return m, m.setToast("commit has no file changes", ToastInfo)
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
			return m, m.setToast("switch: "+msg.err.Error(), ToastError)
		}
		return m, tea.Batch(
			loadRepo(m.repoRoot),
			m.setToast("switched to "+msg.name, ToastSuccess),
		)

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
		return m, m.setToast("open: "+err.Error(), ToastError)
	}
	if msg.Line > 0 {
		m.editor.GoToLine(msg.Line)
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
		if _, ok := m.modal.(modal.Previewer); ok {
			switch s {
			case "alt+up":
				m.previewExtraH = clampPreviewExtra(m.previewExtraH-2, m.height)
				return m, nil
			case "alt+down":
				m.previewExtraH = clampPreviewExtra(m.previewExtraH+2, m.height)
				return m, nil
			case "pgup", "alt+k":
				m.previewScroll -= m.previewPageStep()
				return m, nil
			case "pgdown", "alt+j":
				m.previewScroll += m.previewPageStep()
				return m, nil
			}
		}
		newModal, cmd := m.modal.Update(km)
		m.modal = newModal
		m.syncPreviewState()
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
				return m, m.setToast("worktree: "+err.Error(), ToastError)
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
				return m, m.setToast("nothing to copy", ToastInfo)
			}
			if err := clipboard.Copy(path); err != nil {
				return m, m.setToast("copy failed: "+err.Error(), ToastError)
			}
			return m, m.setToast("copied: "+path, ToastSuccess)
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
	if err := m.editor.Open(msg.Path, m.statuses[msg.Path]); err != nil && m.log != nil {
		m.log.Log("editor: open failed", "path", msg.Path, "err", err.Error())
	}
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
	bodyH := max(m.height-headerH-footerH, 3)
	if m.treeOuter == 0 {
		m.treeOuter = m.width / 4
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
	if m.modal == nil {
		return screen
	}
	return m.renderModalOverlay()
}

// renderModalOverlay draws the active modal centered on screen. Search-style
// modals that implement modal.Previewer also get a live file preview pane
// at the bottom of the terminal, so the user can see what they are about to
// open without leaving the modal.
func (m Model) renderModalOverlay() string {
	pv, ok := m.modal.(modal.Previewer)
	if !ok || pv.PreviewPath() == "" {
		overlay := m.modal.View(m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "))
	}

	previewH := m.previewPaneHeight()
	modalH := m.height - previewH
	if modalH < 8 {
		// Terminal too small for a useful split; fall back to plain modal.
		overlay := m.modal.View(m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "))
	}

	overlay := m.modal.View(m.width, modalH)
	modalArea := lipgloss.Place(m.width, modalH, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "))

	relPath := pv.PreviewPath()
	abs := filepath.Join(m.repoRoot, relPath)
	preview := editor.PreviewFile(abs, relPath, pv.PreviewLine(), m.previewScroll, m.width, previewH-2)
	previewBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("#3b4261")).
		Width(m.width).
		Height(previewH - 1).
		Render(preview)
	return lipgloss.JoinVertical(lipgloss.Left, modalArea, previewBox)
}

// previewPaneHeight returns the current preview pane height, factoring in the
// user's resize delta. Baseline is roughly half the screen so the preview is
// genuinely useful; floor 10 / ceiling height-10 keeps the modal usable.
func (m Model) previewPaneHeight() int {
	base := m.height / 2
	return clampPreviewHeight(base+m.previewExtraH, m.height)
}

func clampPreviewHeight(v, totalH int) int {
	low := 10
	high := max(totalH-10, low+1)
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clampPreviewExtra(extra, totalH int) int {
	base := totalH / 2
	low := 10 - base
	high := (totalH - 10) - base
	if low > high {
		return 0
	}
	if extra < low {
		return low
	}
	if extra > high {
		return high
	}
	return extra
}

// previewPageStep is the line delta for one PgUp/PgDn — about half the
// preview pane so the user can keep context as they scroll.
func (m Model) previewPageStep() int {
	return max(m.previewPaneHeight()/2-1, 1)
}

// syncPreviewState resets the preview scroll when the modal's selection
// changes (cursor move or new search results), so each new file always
// starts from the centered/top position.
func (m *Model) syncPreviewState() {
	pv, ok := m.modal.(modal.Previewer)
	if !ok {
		m.previewLastKey = ""
		m.previewScroll = 0
		return
	}
	key := pv.PreviewPath() + ":" + fmt.Sprintf("%d", pv.PreviewLine())
	if key != m.previewLastKey {
		m.previewLastKey = key
		m.previewScroll = 0
	}
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
	hint := "q quit  Tab/←/→ pane  p file  r recent  F grep  L log  b branch  w worktree  B blame  t flat  d diff  / find  y copy  i edit  ? help"
	// While a toast is active it owns the left segment and the hint is
	// pushed to the right (or trimmed first when space is tight). This
	// matches the convention from glow's pager footer.
	toastSeg := m.renderToast()
	line := hint
	if toastSeg != "" {
		line = toastSeg + "  " + hint
	}
	if ansi.StringWidth(line) > m.width-2 {
		line = ansi.Truncate(line, m.width-2, "…")
	}
	return FooterStyle.Width(m.width).Render(line)
}

func (m Model) renderBody() string {
	treeOuterW := m.treeOuter
	if treeOuterW == 0 {
		treeOuterW = m.width / 4
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
