# CLAUDE.md — peek プロジェクト

このリポジトリは **peek**（Go + Bubble Tea 製の軽量 TUI 差分ビューア）の開発用です。
新しいセッションで Claude Code を開いたときは、まず以下を読んでください。

## 必読ドキュメント（順番通り）

1. [`docs/requirements.md`](./docs/requirements.md) — ユーザ要件のヒアリング結果
2. [`docs/design.md`](./docs/design.md) — 技術設計（画面構成・キーバインド・データフロー）
3. [`docs/roadmap.md`](./docs/roadmap.md) — マイルストーン M1〜M10

## プロジェクトの核となる決定事項（議論済み・再検討不要）

- **言語**: Go（≥ 1.22）
- **TUI フレームワーク**: Bubble Tea + Lip Gloss + Bubbles
- **シンタックスハイライト**: chroma
- **diff**: `os/exec` で git CLI を呼び、`go-diff` で split 表示用にアラインメント
- **検索**: ファジー検索は `sahilm/fuzzy`、全文 grep は ripgrep を優先、無ければ Go フォールバック
- **キーバインド**: `⌘` は使わず **Ctrl ベース**（ターミナル制約のため）
- **設置ルート**: `~/ottaaa/peek`
- **配布**: 当面は自分専用。将来的に OSS 公開の可能性あり

## 開発方針

- **まず動かす**: M1→M2→M3 を小さく積む。完璧主義で止まらない
- **テスト**: ロジック層（git 操作、diff アラインメント、検索）はユニットテスト必須。UI 層は手動で確認
- **依存最小**: 新しい依存を入れる前に、標準ライブラリや既採用ライブラリで済むか検討
- **キーバインド変更時**: `docs/design.md` の表を必ず更新

## 現在の状態

- [x] 要件ヒアリング完了
- [x] 技術設計完了
- [x] リポジトリ初期化（git init 済み、main ブランチ）
- [x] M1: プロジェクト骨格（go mod、CLI、Bubble Tea hello world）
- [x] M2: ファイルツリー + 変更マーカー
- [x] M3: View モード + 現ファイル内検索
- [x] M4: SplitDiff モード
- [x] M5: 検索モーダル 3 種（Ctrl+P / Ctrl+E / Ctrl+G）
- [x] M6: Worktree 切替モーダル（Ctrl+W）
- [x] M7: Edit モード（i/e, Ctrl+S）
- [x] M8: 変更ファイルのみフラットリスト（t）
- [x] M9: パスコピー（y）+ ヘルプモーダル（?）+ 仕上げ
- [ ] M10: 配布整備（OSS 公開決定時）
- [x] M11: Git 履歴（L）/ ブランチ切替（b）/ Blame（B）

**注記**: `docs/design.md` のキーマップから `Ctrl+Shift+F` は `Ctrl+G` に差し替え済み（端末制約）。TUI の実動作確認はユーザの手元で実施してください（unit tests で git/diff/tree/search のロジックは検証済み）。

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

### git 操作の扱い（2026-04 更新）

当初 "git 操作は AI エージェントがやる" を方針としていたが、ユーザ要望により
**peek から以下の git 操作を実行可能にする**:

- コミット履歴の閲覧（ログ + commit 単位の SplitDiff）
- ブランチ切替（`git switch`。ワーキングツリーが dirty なら確認ダイアログで止める）
- blame

引き続き peek では扱わないもの:

- commit / push / pull / fetch / rebase / merge / cherry-pick 等の**履歴を書き換える**操作
- branch の作成 / 削除 / rename
- stash

要望が曖昧になったら、実装前にユーザに確認すること。
