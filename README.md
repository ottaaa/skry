# skry

A lightweight TUI Git viewer / lightweight editor built with Go and
[Bubble Tea](https://github.com/charmbracelet/bubbletea). Designed for quickly
reviewing AI-generated code across multiple worktrees.

## What it does

- **File tree** on the left with `M/A/D/R/?` markers (status bubbles up to
  parent dirs). Fuzzy-filter with `/`, flat-list of changed files with `t`.
- **Right pane** adapts to the cursor:
  - File with changes → **SplitDiff** (HEAD vs working tree, aligned with
    `go-diff`)
  - Unchanged file → **View** (chroma syntax highlight)
  - Folder → listing of immediate children with status markers
  - Toggle View ↔ SplitDiff with `d`
- **Edit in place** (`i` / `e` to enter, `Ctrl+S` to save). Syntax highlighting
  stays on during editing. `Esc` returns to View; `Esc` again returns focus to
  the tree.
- **Search**
  - `p` — file name fuzzy search (`sahilm/fuzzy`)
  - `r` — recent files (session-local)
  - `F` — project grep (ripgrep when available, Go fallback otherwise).
    Incremental with 180 ms debounce.
  - `/` — find inside the current file, `n` / `N` navigate matches
- **Git**
  - `L` — commit history for the current branch. Pick a commit → file list →
    SplitDiff (parent vs commit) for any file
  - `B` — `git blame` the current file in the right pane
  - `b` — switch branches. Asks for confirmation (`--discard-changes`) when
    the working tree is dirty
  - `w` — switch between `git worktree`s
- **Misc**
  - `y` — copy current file path to clipboard
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
skry                 # open current repository
skry /path/to/repo   # open a specific repository
skry --version
```

`skry` needs a git repository — it reads via `git` CLI under the hood. Press
`?` once inside for the full keymap.

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
