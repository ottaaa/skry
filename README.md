# skry

[![build](https://github.com/ottaaa/skry/actions/workflows/ci.yml/badge.svg)](https://github.com/ottaaa/skry/actions/workflows/ci.yml)
<!--
Coverage / ratio / exec-time badges work after you create an `octocovs` repo
under your GitHub account (see docs/OPERATIONS.md §2-a). Once it exists,
uncomment these:

![Coverage](https://raw.githubusercontent.com/ottaaa/octocovs/main/badges/ottaaa/skry/coverage.svg)
![Code to Test Ratio](https://raw.githubusercontent.com/ottaaa/octocovs/main/badges/ottaaa/skry/ratio.svg)
![Test Execution Time](https://raw.githubusercontent.com/ottaaa/octocovs/main/badges/ottaaa/skry/time.svg)
-->


A lightweight TUI Git viewer / lightweight editor built with Go and
[Bubble Tea](https://github.com/charmbracelet/bubbletea). Designed for quickly
reviewing AI-generated code across multiple worktrees.

## What it does

- **File tree** on the left with `M/A/D/R/?` markers (status bubbles up to
  parent dirs). Fuzzy-filter with `/`, flat-list of changed files with `t`.
- **Right pane** adapts to the cursor:
  - File with changes → **SplitDiff** (HEAD vs working tree, aligned with
    `go-diff`). Opens scrolled to the first hunk; `n` / `N` jump to the
    next / previous hunk.
  - Unchanged file → **View** (chroma syntax highlight)
  - Folder → listing of immediate children with status markers
  - Toggle View ↔ SplitDiff with `d`
- **Edit in place** (`i` / `e` to enter, `Ctrl+S` to save now, `Ctrl+Z` /
  `Ctrl+Y` to undo / redo). Edits **autosave 1.5 s after the last keystroke**
  and are also flushed when you `Esc` or switch to another file. Syntax
  highlighting stays on during editing. `Esc` returns to View; `Esc` again
  returns focus to the tree.
- **Search**
  - `p` — file name fuzzy search (`sahilm/fuzzy`)
  - `r` — recent files (session-local)
  - `F` — project grep (ripgrep when available, Go fallback otherwise).
    Incremental with 180 ms debounce.
  - `/` — find inside the current file, `n` / `N` navigate matches
  - While `p` / `r` / `F` is open, the file under the cursor is previewed in
    a bottom pane. `PgUp` / `PgDn` (or `Alt+k` / `Alt+j`) scroll the preview;
    `Alt+↑` / `Alt+↓` shrink / grow the preview pane.
- **Git**
  - `L` — enter **Log mode**: a 3-pane view showing the branch graph
    (`git log --graph HEAD`, newest first) on the left, the focused commit's
    metadata + changed files in the middle, and the parent-vs-commit
    SplitDiff on the right. `Tab` / `←` / `→` move focus between panes;
    moving the cursor in the graph updates the file list, moving the cursor
    in the file list updates the diff. `Esc` / `q` returns to normal layout.
  - `B` — `git blame` the current file in the right pane
  - `b` — switch branches. Asks for confirmation (`--discard-changes`) when
    the working tree is dirty
  - `w` — switch between `git worktree`s
- **Auto-reload** — the working tree is watched with `fsnotify`. When an AI
  agent or another editor writes a file, the tree / statuses / right pane
  refresh automatically (debounced 250 ms, `.git` and common cache dirs like
  `node_modules` are ignored). Reload is suppressed while you are editing.
- **Misc**
  - `y` — copy current file path to clipboard
  - `I` — toggle showing `.gitignore`'d files (default off; heavy dirs like
    `node_modules` / `.venv` / `__pycache__` stay hidden either way)
  - `[` / `]` — resize the left pane (`<` / `>` / `Alt+h` / `Alt+l` also work)
  - `Tab` / `←` / `→` — move focus between tree and right pane
  - `?` — help with all key bindings

skry intentionally doesn't do destructive git operations (commit/push/pull,
rebase, stash, branch create/delete, etc.) — that's left to your AI agent or
regular git CLI.

## Install

Requires Go ≥ 1.22.

```sh
go install github.com/ottaaa/skry/cmd/skry@latest
```

Or from source:

```sh
git clone https://github.com/ottaaa/skry.git
cd skry
make build         # produces bin/skry
```

`ripgrep` is optional; if present, project grep uses it.

## Usage

```sh
skry                       # open current repository
skry /path/to/repo         # open a specific repository
skry /path/to/repo/sub/dir # scope the tree to a subdirectory of a repo
skry --version
git diff | skry -          # pipe content as a stdin source (see below)
```

`skry` needs a git repository — it reads via `git` CLI under the hood. Press
`?` once inside for the full keymap.

When you point skry at a subdirectory of a git repo (rather than the
toplevel), the tree is scoped to that subdirectory: file paths display
relative to it and only files beneath it are listed. Git operations
(branch / log / blame / status) still operate on the full repo. The
header shows the scope (e.g. `repo/services/devops`).

### stdin

`skry -` reads stdin into a temp directory, initializes a git repo, and
opens skry on the result. The content is saved as a single tracked file
named after the detected type (`stdin.diff`, `stdin.json`, `stdin.xml`,
or `stdin.txt` as a fallback). The temp directory is removed on exit.

This is handy for one-off views without going to disk:

```sh
git log -p HEAD~3..HEAD | skry -
curl -s https://example.com/foo.go | skry -
kubectl get -o yaml deploy/foo | skry -
```

Unified diffs currently render as plain text in View mode (with
syntax highlighting). A side-by-side renderer for diff streams is
tracked as a follow-up.

## Development

```sh
make run ARGS=.     # run against the current directory
make test
make vet
```

Tests live next to the code (`internal/.../foo_test.go`). Logic layers (git
parsing, diff alignment, search helpers) have unit tests; UI is verified by
eye.

## License

MIT — see [`LICENSE`](./LICENSE).
