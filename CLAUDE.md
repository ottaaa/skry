# CLAUDE.md — skry プロジェクト

このリポジトリは **skry**（Go + Bubble Tea 製の軽量 TUI 差分ビューア）の開発用です。
新しいセッションで Claude Code を開いたときは、まず以下を読んでください。

> 旧名は `peek` でした。名前衝突（`phw/peek` 等）回避のため `skry` にリネーム。
> モジュールパスも `github.com/ottaaa/skry`。ローカルのディレクトリ名
> (`~/ottaaa/peek`) はユーザが任意のタイミングで `mv` してください。

## 必読ドキュメント（順番通り）

1. [`docs/requirements.md`](./docs/requirements.md) — ユーザ要件のヒアリング結果
2. [`docs/design.md`](./docs/design.md) — 技術設計（画面構成・キーバインド・データフロー）
3. [`docs/roadmap.md`](./docs/roadmap.md) — マイルストーン M1〜M11

## プロジェクトの核となる決定事項（議論済み・再検討不要）

- **言語**: Go（≥ 1.22）
- **TUI フレームワーク**: Bubble Tea + Lip Gloss + Bubbles
- **シンタックスハイライト**: chroma
- **diff**: `os/exec` で git CLI を呼び、`go-diff` で split 表示用にアラインメント
- **検索**: ファジー検索は `sahilm/fuzzy`、全文 grep は ripgrep を優先、無ければ Go フォールバック
- **キーバインド**: 編集中以外は **Ctrl を使わない**（単キー / Vim スタイル）。詳細は `docs/design.md`
- **配布**: 自分用（macOS）。`make build` で `bin/skry`。Homebrew tap / GoReleaser は現時点では用意しない

## 開発方針

- **まず動かす**: マイルストーンを小さく積む。完璧主義で止まらない
- **テスト**: ロジック層（git 操作、diff アラインメント、検索）はユニットテスト必須。UI 層は手動で確認
- **依存最小**: 新しい依存を入れる前に、標準ライブラリや既採用ライブラリで済むか検討
- **キーバインド変更時**: `docs/design.md` の表を必ず更新
- **CI**: `.github/workflows/ci.yml` で push / PR 時に `go vet` + `go test -race` を実行

## 現在の状態

- [x] 要件ヒアリング完了
- [x] 技術設計完了
- [x] リポジトリ初期化（git init 済み、main ブランチ）
- [x] M1: プロジェクト骨格（go mod、CLI、Bubble Tea hello world）
- [x] M2: ファイルツリー + 変更マーカー
- [x] M3: View モード + 現ファイル内検索
- [x] M4: SplitDiff モード
- [x] M5: 検索モーダル 3 種（p / r / F）
- [x] M6: Worktree 切替モーダル（w）
- [x] M7: Edit モード（i/e, Ctrl+S）+ syntax highlight
- [x] M8: 変更ファイルのみフラットリスト（t）
- [x] M9: パスコピー（y）+ ヘルプモーダル（?）
- [x] M10: リネーム + LICENSE + CI（Homebrew / GoReleaser は保留）
- [x] M11: Git 履歴（L）/ ブランチ切替（b）/ Blame（B）

## コミット規約

- Conventional Commits（`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`）
- 1 コミット 1 論理変更
- AI エージェントによるコミットは `Co-Authored-By: Claude` を付けてよい

## 重要な制約・非対象（脱線しないこと）

以下はユーザが **明示的に不要と言った**機能。実装しないこと。

- diff へのコメント機能 / AI プロンプトとしてコピーする機能（difit にあった機能）
- 閲覧済みファイルのマーキング、再起動後への永続化
- worktree の追加・削除・間の差分比較（一覧表示と切替のみ）
- Code Jump / LSP 機能
- 複数 worktree を同時に一画面で表示する機能

### git 操作の扱い

**skry から実行可能**:

- コミット履歴の閲覧（ログ + commit 単位の SplitDiff）
- ブランチ切替（`git switch`。ワーキングツリーが dirty なら確認ダイアログで止める）
- blame

**skry では扱わない**（引き続き AI エージェント側の責務）:

- commit / push / pull / fetch / rebase / merge / cherry-pick 等の**履歴を書き換える**操作
- branch の作成 / 削除 / rename
- stash

要望が曖昧になったら、実装前にユーザに確認すること。
