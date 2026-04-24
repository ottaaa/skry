# peek — 実装ロードマップ

`requirements.md` / `design.md` をもとにした段階的実装計画。

## 基本方針

- **小さく積む**: 各マイルストーンは 1〜数日で終わる粒度
- **動くもの優先**: 見た目の磨きは後回し、まず機能で通しを作る
- **M1→M2→M3 で一度止まる**: 手触りを確認してから後続に進む（ユーザ要望）

## マイルストーン

### M1: プロジェクト骨格 【小】 ✅

- [x] `go mod init github.com/ottaaa/peek`
- [x] `cmd/peek/main.go` に Bubble Tea の hello world
- [x] 依存追加: bubbletea, lipgloss, bubbles
- [x] CLI 引数（起動時のリポジトリパス、`--version`）
- [x] `Ctrl+Q` / `q` で終了できることを確認
- [x] `make run` / `make build` を Makefile で用意

**完了条件**: `peek .` で空の Bubble Tea 画面が開き、`Ctrl+Q` で終了できる。

---

### M2: ファイルツリー + 変更マーカー 【中】 ✅

- [x] `internal/git/lsfiles.go`: `git ls-files --cached --others --exclude-standard -z`
- [x] `internal/git/status.go`: `git status --porcelain=v1 -z` をパース
- [x] `internal/tree/`: ツリーの Model（展開/折りたたみ、選択、親ディレクトリへの status 伝搬）
- [x] 変更マーカー（M/A/D/R/?）を色付きで表示
- [x] 矢印キー / hjkl で移動、`Enter` で展開、`Tab` で右ペインにフォーカス、`/` でフィルタ
- [x] 右ペインはまだプレースホルダで OK

**完了条件**: 実リポジトリで起動し、ツリーを矢印キーで操作できる。変更ありファイルに色付きマーカーが付く。

---

### M3: View モード（読み取り専用）+ 現ファイル内検索 【中】 ✅

- [x] `internal/editor/highlight.go`: chroma (`github-dark`, terminal256) でハイライト、500KB 超は無効化
- [x] `internal/editor/view.go`: viewport ベースの読み取りビュー + 行番号
- [x] ツリーでファイルを選択すると右ペインに内容表示
- [x] `Ctrl+F` で現ファイル内検索、`n` / `N` で次/前、マッチをハイライト

**完了条件**: `M1+M2+M3` の状態で、「リポジトリを開く → ツリーを歩く → ファイルを開いて読む → 検索する」が破綻なく動く。**ここでユーザに手触り確認。**

---

### M4: SplitDiff モード 【大】 ✅

- [x] `internal/git/diff.go`: `git show HEAD:<path>` で HEAD 側を取得（新規/未追跡は空文字）
- [x] go-diff の `DiffLinesToChars` で line-level アラインメント、delete/insert の隣接ペアは Change 行として 1:1 に寄せる
- [x] 2 カラム表示（行番号 + 追加/削除バックグラウンド色）
- [x] 選択ファイルに変更があれば自動で SplitDiff、なければ View
- [x] `Ctrl+D` で View ↔ SplitDiff 切替

**完了条件**: 変更のあるファイルを選択すると split diff が自動表示される。

---

### M5: 検索モーダル 3 種 【中】 ✅

- [x] `internal/search/file.go`: `Ctrl+P` ファイル名フジィ検索（sahilm/fuzzy）
- [x] `internal/search/content.go`: `Ctrl+G` 全文 grep（ripgrep 優先、フォールバック bufio 実装）
- [x] Recent Files（`Ctrl+E`）はセッション内の履歴 + `FileModal` の再利用
- [x] モーダル UI は `internal/modal` の共通枠を使用
- [x] `Esc` で閉じる、`Enter` で選択ジャンプ（grep は空クエリ確定で検索実行 → 結果選択）

**完了条件**: 3 種の検索がそれぞれ動き、結果からファイルを開ける。

---

### M6: Worktree 切替モーダル 【小】 ✅

- [x] `internal/git/worktree.go`: `git worktree list --porcelain` のパース（parseWorktreesOutput にテスト）
- [x] `internal/worktreeui/`: `Ctrl+W` モーダル、現在の worktree に `●` 印
- [x] 選択で `SwitchWorktreeMsg` 発行 → app が loadRepo を再実行しツリー再スキャン
- [x] ヘッダにブランチ（実質的な worktree 名）を表示

**完了条件**: 複数 worktree のあるリポジトリで、`Ctrl+W` で切替えられる。

---

### M7: Edit モード 【中】 ✅

- [x] `internal/editor/edit.go`: bubbles textarea ベースの編集
- [x] `i` / `e` で View → Edit へ、`Esc` で View に戻る
- [x] `Ctrl+S` で保存、ダーティフラグ（ヘッダに `[Edit *]` 表示）
- [x] 保存後 `SavedMsg` → `refreshStatus` でツリーの変更マーカー再計算

**完了条件**: 読み取り中のファイルを編集して保存できる。

---

### M8: 変更ファイルのみフラットリスト トグル 【小】 ✅

- [x] `t` で (B)ツリー ↔ (A)フラットリストを切替
- [x] フラットリストは変更のあるファイルを相対パス付きで表示（ソート済み）

**完了条件**: 変更ファイルだけを一覧する視点に切替えられる。

---

### M9: パスコピー + 仕上げ 【小】 ✅

- [x] `y` で現ファイルパスをクリップボードにコピー（editor 優先、次にツリー選択）
- [x] `?` でヘルプモーダル（全キー一覧）
- [x] フッタのショートカットヒント更新
- [x] Lip Gloss テーマは GitHub ダーク系で固定（追加調整は随時）

**完了条件**: 日常使用で「このキーがない」の違和感が出ない。

---

### M10: 配布整備（OSS 公開する場合のみ） 【小】

- [ ] README 充実（スクリーンショット、インストール方法、使い方）
- [ ] ライセンス（MIT 想定）
- [ ] GoReleaser で release workflow
- [ ] Homebrew tap

**完了条件**: `brew install ottaaa/peek/peek` で入れられる。

---

---

### M11: Git 履歴 / ブランチ切替 / Blame 【中〜大】 ✅（2026-04 追加）

当初は非スコープだったが、方針変更で追加。

- [x] `internal/git/log.go`: `git log --pretty=format:...` の直列ログ
- [x] `internal/git/branch.go`: `git branch --list` のパース、`git switch [--discard-changes]`
- [x] `internal/git/blame.go`: `git blame --porcelain` のパース（commit メタは commit 間で再利用）
- [x] `L` で Log モーダル → commit 選択 → ファイル一覧モーダル → 選択で editor を ModeCommitDiff に（parent 対 commit の SplitDiff）
- [x] `b` で Branch モーダル。dirty なら `y/N` で `--discard-changes` を確認してから `git switch`
- [x] `B` で View ↔ Blame トグル。Blame 行は `<#> <short> <author> <date>  <text>`

## 未スコープ（将来 backlog）

- キーマップ設定ファイル（`~/.config/peek/keymap.toml`）
- テーマ設定（ライト/ダーク等）
- word-level diff
- ファイル監視（fsnotify）による自動再読込
- commit 内の全ファイル横断の unified-diff 表示（現状は 1 ファイルずつ SplitDiff）
- `--graph` スタイルのコミットツリー（分岐可視化）
