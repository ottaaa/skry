# CLAUDE.md

このファイルは Claude Code (claude.ai/code) がこのリポジトリで作業するときのガイドです。
README.md は機能・キーバインドの一次ソースなので、セッション開始時にまず読んでください。

## What is skry

`skry` は Go + Bubble Tea で作られた軽量 TUI の git ビューア / 簡易エディタです。
複数の worktree にまたがって AI 生成コードを素早くレビューする用途を想定しています。
モジュールパスは `github.com/ottaaa/skry`。

## Build & Run

Go ≥ 1.25 が必要です。`golangci-lint` v2+ を入れておくとローカルで lint を回せます
（任意。`brew install golangci-lint` か `make lint-install`）。

```bash
# ビルド (revision を ldflags で埋め込む)
make build                                # → bin/skry

# 一時的に手元の repo を覗く (ARGS にパス)
make run ARGS=.

# サンプル repo を再生成して TUI を起動 (testdata/sample-repo を再シード)
make dev

# 別の repo に向けて起動 (シードはスキップ)
make dev ARGS=~/code/myrepo

# サンプル repo の再シードのみ (.git ごと作り直す)
make dev-seed

# テスト
make test                                 # unit tests
make test-race                            # -race + coverage.out 生成
make ci                                   # CI runner が呼ぶターゲット (= test-race)
make cover                                # HTML カバレッジレポート

# 単独テストの実行 (-run でフィルタ)
go test ./internal/editor/ -run TestUndoInsertRune

# Lint
make lint                                 # golangci-lint run ./...
make fmt                                  # go fmt ./...
make vet                                  # go vet ./...

# 後片付け
make clean                                # bin/ coverage.* dist/
```

## CLI Flags

| Flag        | Default | Description                                   |
| ----------- | ------- | --------------------------------------------- |
| `--version` | `false` | バージョンを表示して終了                      |
| (位置引数)  | `.`     | 対象リポジトリのパス。省略時はカレントディレクトリ |

## Architecture

**Go + Bubble Tea (Elm-architecture) の単一バイナリ**。フロントエンドや埋め込み静的
資産は持ちません。

### コマンド & 起動

- `cmd/skry/main.go` — CLI エントリ。flag 解析と `tea.NewProgram(..., WithAltScreen())`
- `version/version.go` — `Version` (tagpr が書き換える) と `Revision` (ldflags 注入)

### 中核 (Bubble Tea Model)

- `internal/app/app.go` — トップレベル Model。tree とエディタの focus、modal、
  watcher 経由の自動再読込、git 操作の dispatch を持つ
- `internal/app/theme.go` — lipgloss スタイルの色定義
- `internal/events/events.go` — pane 間で飛ぶ tea.Msg 型 (`OpenFileMsg`,
  `CursorMovedMsg`, `SwitchWorktreeMsg`, `CloseModalMsg`, ...)

### ペイン

- `internal/tree/tree.go` — 左ペインのファイルツリー。M/A/D/R/? マーカー、
  fuzzy filter (`/`)、フラット変更ファイル一覧 (`t`)
- `internal/editor/editor.go` — 右ペインの司令塔。モード遷移
  (`ModeView` / `ModeSplit` / `ModeCommitDiff` / `ModeBlame` / `ModeEdit` /
  `ModeFolder` / `ModeBinary`)、git からの読み込み、binary 判定
- `internal/editor/view.go` — シンタックスハイライト付きの View モード
- `internal/editor/splitdiff.go` — `go-diff` で left/right 行を整列する
  SplitDiff レンダラ
- `internal/editor/blame.go` — `git blame --porcelain` のフォーマッタ
- `internal/editor/edit.go` — Edit モード (`i`/`e`)。`[][]rune` バッファ +
  snapshot ベースの undo/redo (`undoCap = 200`)
- `internal/editor/folder.go` — フォルダにカーソルが乗ったときの listing
- `internal/editor/highlight.go` — chroma ラッパ (lexer match → analyse → fallback)
- `internal/editor/render.go` / `scrollbar.go` — ANSI clip / scrollbar グリフ

### モーダル

- `internal/modal/modal.go` — Modal インターフェース (`Init/Update/View`)
- `internal/branchui/branchui.go` — ブランチ切替 (`b`)。dirty 時は確認ダイアログ
- `internal/worktreeui/worktreeui.go` — worktree 切替 (`w`)
- `internal/logui/logui.go` — `git log` 一覧 (`L`) と commit 内 file picker
- `internal/helpui/helpui.go` — キーバインド一覧 (`?`)
- `internal/search/file.go` — ファイル名 fuzzy 検索 (`p`) / recent (`r`)
- `internal/search/content.go` — プロジェクト grep (`F`)。ripgrep 優先、無ければ Go fallback

### Git / 周辺ユーティリティ

- `internal/git/gitcmd.go` — `git` CLI を `os/exec` で叩く共通ラッパ
- `internal/git/lsfiles.go` / `status.go` / `diff.go` / `log.go` / `blame.go` /
  `branch.go` / `worktree.go` — 各 git サブコマンドの parser
- `internal/clipboard/clipboard.go` — `atotto/clipboard` ラッパ (`y` で path コピー)
- `internal/watcher/watcher.go` — fsnotify 再帰 watcher。debounce 250 ms、
  `.git` / `node_modules` / `.next` / `dist` / `build` / `target` は除外。
  新しいサブディレクトリは Create 検知で自動 add
- `internal/xdg/xdg.go` — `$XDG_STATE_HOME` 解決 (`~/.local/state` フォールバック)
- `internal/logfile/logfile.go` — `$XDG_STATE_HOME/skry/log/skry.log` への
  ローテート JSON-lines logger (10 MiB / 3 backup / 7 day retention)

### Fixtures

- `testdata/sample-repo/seed.sh` — `make dev` が呼ぶ。空ディレクトリに git init して
  modified / staged / untracked が混ざった状態を作る

## Key Design Patterns

- **Bubble Tea pure-update**: `Model.Update(msg)` は副作用を `tea.Cmd` に押し出して
  Model を返り値で返す。直接 stdin を読まない / goroutine から Model を触らない
- **Modal は inline state**: `app.Model.modal modal.Modal` がアクティブなら
  そちらに `Update`/`View` を委譲。close は `events.CloseModalMsg` を発火
- **Editor のモード切替**: `Open()` 内でファイル種別 (binary / 新規 / 変更あり / 通常)
  を判定して mode を選ぶ。`Reload()` は **edit モード中は no-op**
  (ユーザの未保存編集を fsnotify で潰さないため)
- **Watcher debounce**: `time.NewTimer` + nil-channel select trick。バーストを
  単一の `fsChangedMsg` に潰す。`out chan` への送信は non-blocking で coalesce
- **Snapshot ベースの undo/redo**: `editMode.takeSnapshot()` で `[][]rune` を
  deep-copy。`pushUndo` は redo を都度クリア (古典的な history-branching)
- **Binary heuristic**: 先頭 8000 byte の NUL byte で判定 (git の慣習に合わせる)。
  HEAD 側 / working 側のどちらかが binary なら `ModeBinary` にフォールバックして
  diff alignment を回避
- **Logger は narrow interface**: `watcher.Logger` は `Log(msg string, kv ...any)`
  だけを要求。テストでは nil を渡す。`internal/logfile.Logger` がそれを満たす
- **Ctrl はエディタ専用**: 通常モードでは単キー / Vim スタイルのみ。Ctrl+S /
  Ctrl+Z / Ctrl+Y は Edit モードに入って初めて使える (項目 README + helpui で同期)

## Testing Conventions

- ロジック層 (git parsing, diff alignment, search, watcher, logfile, undo) は
  必ず `_test.go` を併設
- UI 層 (lipgloss でレンダリングしている部分) は基本的に手動確認 (`make dev` で
  サンプル repo を起動)
- `t.TempDir()` を使ってテンポラリリポジトリを作る。実 `git` を呼ぶテストは
  `git` バイナリが PATH にあること前提

## CI / Release

- **CI** (`.github/workflows/ci.yml`)
  - `ubuntu-latest` + `macos-latest` の matrix
  - `go vet`、`golangci-lint` (reviewdog 経由で PR コメント)、`gostyle`、
    `make ci` (`go test -race -coverprofile=coverage.out`)、`octocov` (coverage 集計)
  - GitHub Actions は全て SHA ピン
- **License scan** (`.github/workflows/trivy.yml`) — Trivy で UNKNOWN / MEDIUM /
  HIGH の license violation を検出すると fail
- **Release** (`.github/workflows/tagpr.yml`)
  - `tagpr` が conventional commits から次バージョンを判定し、リリース PR を
    自動生成。マージするとタグが切られて `goreleaser` が動く
  - `goreleaser` (`.goreleaser.yml`) が darwin/linux/windows × amd64/arm64 の
    バイナリと、Linux 用の deb / rpm / apk を成果物にする
  - `version/version.go` の `Version` 定数は tagpr が書き換える。手で触らない

詳しい運用手順は `docs/OPERATIONS.md` を参照。

## コミット規約

- Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
- 1 コミット 1 論理変更
- AI エージェントによるコミットは `Co-Authored-By: Claude` を付けてよい
- キーバインド変更時は **README.md と `internal/helpui/helpui.go` の両方**を更新

## Out of Scope (脱線しないこと)

ユーザが **明示的に不要と言った**機能。提案・実装しないこと。

- diff へのコメント機能 / AI プロンプトとしてコピーする機能 (difit 由来)
- 閲覧済みファイルのマーキング、再起動後への永続化
- worktree の追加・削除・間の差分比較 (一覧表示と切替のみ)
- Code Jump / LSP 機能
- 複数 worktree を同時に一画面で表示する機能

### Git 操作の扱い

**skry から実行可能**:

- コミット履歴の閲覧 (ログ + commit 単位の SplitDiff)
- ブランチ切替 (`git switch`。dirty なら確認ダイアログで止める)
- blame

**skry では扱わない** (AI エージェント側の責務):

- commit / push / pull / fetch / rebase / merge / cherry-pick (履歴書き換え)
- branch の作成 / 削除 / rename
- stash

要望が曖昧になったら、実装前にユーザに確認すること。
