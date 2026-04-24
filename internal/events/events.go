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

// LogCommitSelectedMsg is emitted when a commit is picked from the log modal.
type LogCommitSelectedMsg struct {
	Sha     string
	Short   string
	Subject string
}

// CommitFileSelectedMsg is emitted when a file is picked inside a commit.
type CommitFileSelectedMsg struct {
	Sha     string
	Short   string
	Subject string
	Path    string
}

// SwitchBranchMsg asks the app to run `git switch`. If Force is true, uses
// --discard-changes.
type SwitchBranchMsg struct {
	Name  string
	Force bool
}
