package events

// Cross-package Bubble Tea messages. Keeping them in a leaf package avoids
// import cycles between app, modals, tree, and editor.

type OpenFileMsg struct {
	Path string // path relative to the repo root
	Line int    // 1-based line to jump to, 0 if unspecified
}

type CloseModalMsg struct{}

type SwitchWorktreeMsg struct {
	Path string // absolute path of the chosen worktree
}

type RefreshMsg struct{}

type StatusRefreshedMsg struct{}

// CursorMovedMsg fires from the tree whenever the highlighted node changes.
// The app uses it to drive a preview of the right pane.
type CursorMovedMsg struct {
	Path  string
	IsDir bool
}

// LogCommitFocusedMsg fires from the LogView's Graph pane whenever the
// highlighted commit changes. The app responds by loading that commit's
// file list into the Files pane.
type LogCommitFocusedMsg struct {
	Sha     string
	Short   string
	Subject string
}

// LogFileFocusedMsg fires from the LogView's Files pane whenever the
// highlighted file changes. The app responds by rendering that commit/file
// pair as a SplitDiff in the editor pane.
type LogFileFocusedMsg struct {
	Sha     string
	Short   string
	Subject string
	Path    string
}

// LogExitMsg asks the app to leave Log mode and restore the normal layout.
type LogExitMsg struct{}

// SwitchBranchMsg asks the app to run `git switch`. If Force is true, uses
// --discard-changes.
type SwitchBranchMsg struct {
	Name  string
	Force bool
}
