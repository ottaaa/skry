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
	"github.com/ottaaa/skry/internal/logfetch"
	"github.com/ottaaa/skry/internal/logview"
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
	// FocusLogGraph / FocusLogFiles are only reachable while appMode == AppModeLog.
	FocusLogGraph
	FocusLogFiles
)

// AppMode is the top-level layout mode of the app. Most of the time we sit
// in AppModeNormal (the tree + editor split). Pressing `L` enters Log mode,
// which replaces the tree pane with the two-column logview (graph + files)
// and pins the editor to ModeCommitDiff. Esc / q returns to Normal.
type AppMode int

const (
	AppModeNormal AppMode = iota
	AppModeLog
)

const treeWidthMin = 24

type Model struct {
	repoRoot string
	width    int
	height   int
	focus    Focus
	appMode  AppMode

	tree     tree.Model
	editor   editor.Model
	modal    modal.Modal
	logView  logview.Model
	logFetch *logfetch.Fetcher
	// logSeq is bumped every time the user changes commit or file focus in
	// Log mode. Arrived messages tagged with an older Seq are silently
	// dropped — this is the cancellation invariant that lets prefetch +
	// foreground requests coexist without races.
	logSeq uint64

	scopeDir    string // relative subdir under repoRoot to scope the tree to ("" = whole repo)
	files       []string
	statuses    map[string]git.Status
	summary     summary
	branch      string
	recent      []string
	showIgnored bool // when true, gitignored files (filtered by skipDirs) are also listed

	// Status bar state. setToast / toastExpiredMsg / renderToast live in
	// toast.go; we just hold the current toast and a sequence counter
	// here so that a fresh toast can supersede an in-flight expiry tick.
	toast    toast
	toastSeq int

	treeOuter int // current outer width of the tree pane; 0 = auto-init on first resize

	previewExtraH  int    // user delta on the modal-preview pane height (alt+up/alt+down)
	previewScroll  int    // line-offset added to the modal-preview top (pgup/pgdn)
	previewLastKey string // path+line key to detect cursor moves and reset scroll

	watcher     *watcher.Watcher     // recursive: tree / status refresh
	fileWatcher *watcher.FileWatcher // per-file: faster editor reload
	log         *applog.Logger
}

// fsChangedMsg is delivered when the recursive file system watcher
// coalesced one or more events. The handler refreshes statuses/file list
// and reloads the editor's current file (if safe to do so).
type fsChangedMsg struct{}

// fileChangedMsg is delivered when the per-file watcher (scoped to the
// currently-open editor file) sees a write. Handler is intentionally
// narrow: it just calls editor.Reload(), no tree refresh, no git status
// shell-out. Keeps the focus-file feedback fast.
type fileChangedMsg struct{}

// watcherStartedMsg delivers both watcher handles into the model so we
// can close them later and start waiting for their events.
type watcherStartedMsg struct {
	w  *watcher.Watcher
	fw *watcher.FileWatcher
}

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
			return watcherStartedMsg{}
		}
		fw, err := watcher.StartFile(lg)
		if err != nil {
			if log != nil {
				log.Log("app: filewatcher start failed", "err", err.Error())
			}
			// Recursive watcher still works without the per-file fast path.
			return watcherStartedMsg{w: w}
		}
		return watcherStartedMsg{w: w, fw: fw}
	}
}

// waitForFile blocks on the per-file watcher and emits a fileChangedMsg.
func waitForFile(fw *watcher.FileWatcher) tea.Cmd {
	if fw == nil {
		return nil
	}
	return func() tea.Msg {
		<-fw.Events()
		return fileChangedMsg{}
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

// appendIgnored augments files with gitignored entries (excluding paths under
// skipDirs like node_modules/, .venv/, etc.). The seen map is updated in place
// so subsequent merges don't double-add. Returns the (possibly extended) slice.
func appendIgnored(root string, files []string, seen map[string]bool) []string {
	ignored, err := git.ListIgnoredFiles(root)
	if err != nil {
		return files
	}
	for _, p := range ignored {
		if seen[p] {
			continue
		}
		if watcher.ShouldSkip(p) {
			continue
		}
		files = append(files, p)
		seen[p] = true
	}
	return files
}

// scopeFilter keeps only paths under scopeDir and strips the scopeDir prefix
// from each. When scopeDir is "", paths are returned unchanged. The returned
// slice is the input slice's storage (filtered in place).
func scopeFilter(scopeDir string, paths []string) []string {
	if scopeDir == "" {
		return paths
	}
	prefix := scopeDir + "/"
	out := paths[:0]
	for _, p := range paths {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		out = append(out, strings.TrimPrefix(p, prefix))
	}
	return out
}

// scopeFilterStatuses keeps only entries whose path lives under scopeDir, with
// the scopeDir prefix stripped. Used to filter the statuses map and entries
// slice in lockstep with scopeFilter.
func scopeFilterStatuses(scopeDir string, entries []git.StatusEntry) []git.StatusEntry {
	if scopeDir == "" {
		return entries
	}
	prefix := scopeDir + "/"
	out := entries[:0]
	for _, e := range entries {
		if !strings.HasPrefix(e.Path, prefix) {
			continue
		}
		e.Path = strings.TrimPrefix(e.Path, prefix)
		out = append(out, e)
	}
	return out
}

// expandScope converts a tree-relative path (from m.files / m.statuses /
// tree events) back to a path relative to the git toplevel, suitable for
// editor.Open and git operations.
func expandScope(scopeDir, path string) string {
	if scopeDir == "" || path == "" {
		return path
	}
	return scopeDir + "/" + path
}

// fsReload is a lighter sibling of loadRepo: it refreshes the file list,
// statuses, and branch, but leaves the editor's currently-open file in place
// (loadRepo replaces the editor entirely, which would close the user's view).
func fsReload(root, scopeDir string, showIgnored bool) tea.Cmd {
	return func() tea.Msg {
		files, err := git.ListFiles(root)
		if err != nil {
			return fsReloadedMsg{err: err}
		}
		entries, err := git.Statuses(root)
		if err != nil {
			return fsReloadedMsg{err: err}
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
		if showIgnored {
			files = appendIgnored(root, files, seen)
		}
		// Scope reduction: drop everything outside scopeDir and strip the prefix
		// so the tree starts inside the scope. Status summary still uses the
		// scoped subset so the header counts reflect what the user can see.
		files = scopeFilter(scopeDir, files)
		entries = scopeFilterStatuses(scopeDir, entries)
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

// New constructs the root app model. repoRoot is the git toplevel; scopeDir
// is an optional relative subdir under repoRoot — when non-empty the tree
// only shows files beneath it, but git operations still run against the
// real toplevel (paths are full from toplevel internally).
func New(repoRoot, scopeDir string) Model {
	// Best-effort open: a write failure must never crash the TUI. The log is
	// diagnostic only (watcher errors, etc.); when Open fails we just run
	// without one.
	lg, _ := applog.Setup()
	m := Model{
		repoRoot: repoRoot,
		scopeDir: scopeDir,
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
	return tea.Batch(loadRepo(m.repoRoot, m.scopeDir, m.showIgnored), startWatcher(m.repoRoot, m.log))
}

type repoLoadedMsg struct {
	root     string
	files    []string
	statuses map[string]git.Status
	summary  summary
	branch   string
	err      error
}

func loadRepo(root, scopeDir string, showIgnored bool) tea.Cmd {
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
		if showIgnored {
			files = appendIgnored(top, files, seen)
		}
		files = scopeFilter(scopeDir, files)
		entries = scopeFilterStatuses(scopeDir, entries)
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
	rows []git.GraphRow
	err  error
}

func loadLog(root string) tea.Cmd {
	return func() tea.Msg {
		rows, err := git.LogGraph(root, 500)
		return logLoadedMsg{rows: rows, err: err}
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
		// (Re-)create the Log-mode fetcher rooted at the new repo. A worktree
		// switch lands here too, so a stale cache from the previous toplevel
		// is dropped automatically.
		if m.logFetch != nil {
			m.logFetch.Reset(msg.root)
		} else {
			m.logFetch = logfetch.New(msg.root)
			m.logFetch.Start()
		}
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
		m.fileWatcher = msg.fw
		// If the editor is already showing a file (e.g. preview-on-load
		// fired before the watcher was ready), point the per-file watcher
		// at it now.
		if m.fileWatcher != nil {
			if p := m.editor.Path(); p != "" {
				m.fileWatcher.Watch(filepath.Join(m.repoRoot, p))
			}
		}
		return m, tea.Batch(waitForFS(m.watcher), waitForFile(m.fileWatcher))

	case fsChangedMsg:
		// Coalesce any further events into the next refresh by issuing a
		// single reload Cmd, then arm for the next watcher event.
		return m, tea.Batch(fsReload(m.repoRoot, m.scopeDir, m.showIgnored), waitForFS(m.watcher))

	case fileChangedMsg:
		// Per-file fast path: just reload the editor's current buffer.
		// No git status shell-out, no tree refresh — those will arrive
		// shortly after via fsChangedMsg if the change is also visible to
		// the recursive watcher.
		m.editor.Reload()
		return m, waitForFile(m.fileWatcher)

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
		if m.fileWatcher != nil {
			_ = m.fileWatcher.Close()
			m.fileWatcher = nil
		}
		// A worktree switch lands in a different toplevel; the previous scope
		// (if any) was relative to the old toplevel and no longer applies.
		m.scopeDir = ""
		return m, tea.Batch(loadRepo(msg.Path, m.scopeDir, m.showIgnored), startWatcher(msg.Path, m.log))

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
			// If the user already entered Log mode (we did so optimistically
			// on `L` press), pop back out so they don't sit on a blank pane.
			if m.appMode == AppModeLog {
				m = m.exitLogMode()
			}
			return m, m.setToast("log: "+msg.err.Error(), ToastError)
		}
		hasCommit := false
		for _, r := range msg.rows {
			if r.Commit != nil {
				hasCommit = true
				break
			}
		}
		if !hasCommit {
			if m.appMode == AppModeLog {
				m = m.exitLogMode()
			}
			return m, m.setToast("no commits yet", ToastInfo)
		}
		m.logView.SetRows(msg.rows)
		return m, m.logView.EmitInitialFocus()

	case branchesLoadedMsg:
		if msg.err != nil {
			return m, m.setToast("branches: "+msg.err.Error(), ToastError)
		}
		if len(msg.branches) == 0 {
			return m, m.setToast("no branches yet", ToastInfo)
		}
		m.modal = branchui.New(msg.branches, msg.dirty)
		return m, m.modal.Init()

	case events.LogCommitFocusedMsg:
		// Bump seq, fire async fetch through the cached fetcher, and
		// schedule prefetch of neighboring commits. Foreground & prefetch
		// share singleflight, so this never duplicates git work.
		m.logSeq++
		seq := m.logSeq
		cmd := m.logFetch.Meta(msg.Sha, seq)
		m.logFetch.Prefetch(m.logView.NeighborShas(2))
		return m, cmd

	case logfetch.MetaArrivedMsg:
		if msg.Seq < m.logSeq {
			return m, nil // stale; user already moved on
		}
		if msg.Err != nil {
			return m, m.setToast("commit files: "+msg.Err.Error(), ToastError)
		}
		m.logView.SetFiles(msg.Sha, msg.Files, msg.Body)
		// Warm the diff cache for the focused commit's first file so the
		// editor pane lights up immediately when SetFiles emits the
		// follow-up file-focus event.
		if len(msg.Files) > 0 {
			m.logFetch.PrefetchDiff(msg.Sha, msg.Files[0].Path)
		}
		return m, m.logView.EmitCurrentFileFocus()

	case events.LogFileFocusedMsg:
		m.logSeq++
		seq := m.logSeq
		return m, m.logFetch.Diff(msg.Sha, msg.Path, seq)

	case logfetch.DiffArrivedMsg:
		if msg.Seq < m.logSeq {
			return m, nil
		}
		if msg.Err != nil {
			return m, m.setToast("commit diff: "+msg.Err.Error(), ToastError)
		}
		c := m.logView.SelectedCommit()
		short, subject := "", ""
		if c != nil {
			short, subject = c.Short, c.Subject
		}
		m.editor.SetCommitDiff(msg.Sha, short, subject, msg.Path, msg.Rows, msg.Binary)
		return m, nil

	case events.LogExitMsg:
		return m.exitLogMode(), nil

	case events.SwitchBranchMsg:
		m.modal = nil
		return m, switchBranch(m.repoRoot, msg.Name, msg.Force)

	case branchSwitchedMsg:
		if msg.err != nil {
			return m, m.setToast("switch: "+msg.err.Error(), ToastError)
		}
		return m, tea.Batch(
			loadRepo(m.repoRoot, m.scopeDir, m.showIgnored),
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
	// msg.Path is relative to scopeDir (the visible tree). The editor and
	// file watcher both want a path from the git toplevel, so re-add the
	// prefix here at the boundary.
	status := m.statuses[msg.Path]
	fullPath := expandScope(m.scopeDir, msg.Path)
	if err := m.editor.Open(fullPath, status); err != nil {
		return m, m.setToast("open: "+err.Error(), ToastError)
	}
	if msg.Line > 0 {
		m.editor.GoToLine(msg.Line)
	}
	m.focus = FocusEditor
	m.tree.SetFocused(false)
	m.editor.SetFocused(true)
	m.pushRecent(msg.Path)
	m.refocusFileWatcher(fullPath)
	return m, nil
}

// refocusFileWatcher points the per-file watcher at the absolute path of
// `relPath`. nil-safe: if the watcher failed to start we just don't get
// the fast path. Empty path detaches.
func (m *Model) refocusFileWatcher(relPath string) {
	if m.fileWatcher == nil {
		return
	}
	if relPath == "" {
		m.fileWatcher.Watch("")
		return
	}
	m.fileWatcher.Watch(filepath.Join(m.repoRoot, relPath))
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

	// Log-mode keys run *before* the q-quits-app check so `q` exits Log
	// mode (returning to Normal) rather than quitting the whole app.
	if m.appMode == AppModeLog && !m.absorbingText() {
		switch s {
		case "tab":
			m.cycleLogFocus(+1)
			return m, nil
		case "shift+tab":
			m.cycleLogFocus(-1)
			return m, nil
		case "left":
			m.setLogFocus(prevLogFocus(m.focus))
			return m, nil
		case "right":
			m.setLogFocus(nextLogFocus(m.focus))
			return m, nil
		case "esc", "q":
			return m.exitLogMode(), nil
		}
	}

	// q quits only when we aren't absorbing text input. (Log mode handled
	// above; here we are guaranteed to be in AppModeNormal.)
	if s == "q" && !m.absorbingText() {
		return m, tea.Quit
	}

	// Global navigation / modal openers only when we aren't in text-entry contexts.
	if !m.absorbingText() && m.appMode == AppModeNormal {
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
			// Switch layout immediately, then load the graph in the
			// background. The user sees the new pane structure with a
			// "loading…" placeholder while git log --graph runs.
			m.appMode = AppModeLog
			m.logView = logview.New(nil)
			m.logView.SetFocus(logview.FocusGraph)
			m.focus = FocusLogGraph
			m.tree.SetFocused(false)
			m.editor.SetFocused(false)
			m.applySizes()
			return m, loadLog(m.repoRoot)
		case "b":
			return m, loadBranches(m.repoRoot)
		case "I":
			m.showIgnored = !m.showIgnored
			tag := "off"
			if m.showIgnored {
				tag = "on"
			}
			return m, tea.Batch(
				fsReload(m.repoRoot, m.scopeDir, m.showIgnored),
				m.setToast("show ignored: "+tag, ToastInfo),
			)
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
	case FocusLogGraph, FocusLogFiles:
		newLV, cmd := m.logView.Update(km)
		m.logView = newLV
		return m, cmd
	}
	return m, nil
}

// cycleLogFocus walks Graph → Files → Editor → Graph (or in reverse).
func (m *Model) cycleLogFocus(dir int) {
	order := []Focus{FocusLogGraph, FocusLogFiles, FocusEditor}
	cur := 0
	for i, f := range order {
		if f == m.focus {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(order)) % len(order)
	m.setLogFocus(order[cur])
}

func nextLogFocus(f Focus) Focus {
	switch f {
	case FocusLogGraph:
		return FocusLogFiles
	case FocusLogFiles:
		return FocusEditor
	case FocusEditor:
		return FocusEditor // already rightmost
	}
	return FocusLogGraph
}

func prevLogFocus(f Focus) Focus {
	switch f {
	case FocusEditor:
		return FocusLogFiles
	case FocusLogFiles:
		return FocusLogGraph
	case FocusLogGraph:
		return FocusLogGraph // already leftmost
	}
	return FocusLogGraph
}

func (m *Model) setLogFocus(f Focus) {
	m.focus = f
	m.tree.SetFocused(false)
	switch f { //nolint:exhaustive // Tree focus is unreachable in Log mode
	case FocusLogGraph:
		m.logView.SetFocus(logview.FocusGraph)
		m.editor.SetFocused(false)
	case FocusLogFiles:
		m.logView.SetFocus(logview.FocusFiles)
		m.editor.SetFocused(false)
	case FocusEditor:
		m.editor.SetFocused(true)
	}
}

// exitLogMode restores the Normal layout. It does NOT reset the editor's
// open file — the user might want to keep looking at the last commit's
// diff. The next file open from the tree will replace it.
func (m Model) exitLogMode() Model {
	m.appMode = AppModeNormal
	m.focus = FocusTree
	m.tree.SetFocused(true)
	m.editor.SetFocused(false)
	m.applySizes()
	return m
}

func (m *Model) previewAt(msg events.CursorMovedMsg) {
	if msg.Path == "" {
		return
	}
	// msg.Path is scope-relative; the editor and file watcher want the path
	// from the git toplevel. The folder-preview entries come from m.files
	// (also scope-relative) and m.statuses keyed by scope-relative paths, so
	// keep the dirPath scope-relative when computing folderEntries.
	if msg.IsDir {
		entries := m.folderEntries(msg.Path)
		m.editor.OpenFolderPreview(expandScope(m.scopeDir, msg.Path), entries, m.statuses)
		m.refocusFileWatcher("")
		return
	}
	fullPath := expandScope(m.scopeDir, msg.Path)
	if err := m.editor.Open(fullPath, m.statuses[msg.Path]); err != nil && m.log != nil {
		m.log.Log("editor: open failed", "path", msg.Path, "err", err.Error())
	}
	m.refocusFileWatcher(fullPath)
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
// currently-highlighted leaf. Returned path is relative to the git toplevel
// (i.e. scopeDir is re-applied when sourced from the scope-relative tree).
func (m Model) currentPath() string {
	if p := m.editor.Path(); p != "" {
		return p
	}
	return expandScope(m.scopeDir, m.tree.SelectedPath())
}

func (m Model) currentTreeOuter() int {
	if m.treeOuter == 0 {
		return m.width / 6
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
	innerH := bodyH - 2
	if innerH < 1 {
		innerH = 1
	}

	if m.appMode == AppModeLog {
		// 3-pane layout: graph + files + editor. Graph and files share the
		// space the tree pane normally has, plus a chunk so the file list
		// has breathing room.
		graphOuter := clampTreeOuter(m.width*3/10, m.width)
		filesOuter := clampTreeOuter(m.width*3/10, m.width)
		editorOuter := m.width - graphOuter - filesOuter
		if editorOuter < 30 {
			// Squeeze the two left panes proportionally if the terminal is narrow.
			editorOuter = 30
			rest := m.width - editorOuter
			if rest < treeWidthMin*2 {
				rest = treeWidthMin * 2
			}
			graphOuter = rest / 2
			filesOuter = rest - graphOuter
		}
		graphInner := max(graphOuter-2, 4)
		filesInner := max(filesOuter-2, 4)
		editorInner := max(editorOuter-2, 4)
		m.logView.SetSize(graphInner, filesInner, innerH)
		m.editor.SetSize(editorInner, innerH)
		return
	}

	if m.treeOuter == 0 {
		m.treeOuter = m.width / 6
	}
	m.treeOuter = clampTreeOuter(m.treeOuter, m.width)
	editorOuter := m.width - m.treeOuter
	treeInner := m.treeOuter - 2
	editorInner := editorOuter - 2
	if treeInner < 4 {
		treeInner = 4
	}
	if editorInner < 4 {
		editorInner = 4
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
	if m.scopeDir != "" {
		repo += "/" + m.scopeDir
	}
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
	hint := "q quit  Tab/←/→ pane  p file  r recent  F grep  L log  b branch  w worktree  B blame  t flat  I ignored  d diff  / find  y copy  i edit  ? help"
	if m.appMode == AppModeLog {
		hint = "Esc/q exit log  Tab/←/→ pane  k/j cursor  g/G top/bottom  ? help"
	}
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
	bodyH := m.height - 2

	if m.appMode == AppModeLog {
		graphOuter := clampTreeOuter(m.width*3/10, m.width)
		filesOuter := clampTreeOuter(m.width*3/10, m.width)
		editorOuter := m.width - graphOuter - filesOuter
		if editorOuter < 30 {
			editorOuter = 30
			rest := m.width - editorOuter
			if rest < treeWidthMin*2 {
				rest = treeWidthMin * 2
			}
			graphOuter = rest / 2
			filesOuter = rest - graphOuter
		}
		graphStyle := PaneStyle
		filesStyle := PaneStyle
		editorStyle := PaneStyle
		switch m.focus { //nolint:exhaustive
		case FocusLogGraph:
			graphStyle = ActivePane
		case FocusLogFiles:
			filesStyle = ActivePane
		case FocusEditor:
			editorStyle = ActivePane
		}
		left := graphStyle.Width(graphOuter - 2).Height(bodyH - 2).Render(m.logView.LeftView())
		mid := filesStyle.Width(filesOuter - 2).Height(bodyH - 2).Render(m.logView.RightView())
		right := editorStyle.Width(editorOuter - 2).Height(bodyH - 2).Render(m.editor.View())
		return lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)
	}

	treeOuterW := m.treeOuter
	if treeOuterW == 0 {
		treeOuterW = m.width / 6
	}
	treeOuterW = clampTreeOuter(treeOuterW, m.width)
	editorOuterW := m.width - treeOuterW

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
