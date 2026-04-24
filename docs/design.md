# skry — 技術設計

`requirements.md` を満たすための設計。**実装の一次ソース**。

## 1. 技術スタック

| レイヤ | 採用 | 理由 |
|---|---|---|
| 言語 | **Go ≥ 1.22** | 単一バイナリ、クロスコンパイル、ユーザ指定 |
| TUI | **Bubble Tea** (`github.com/charmbracelet/bubbletea`) | ユーザ指定。Elm アーキテクチャで状態管理が明快 |
| スタイリング | **Lip Gloss** (`github.com/charmbracelet/lipgloss`) | 宣言的スタイル。リッチな見た目の要 |
| 既製部品 | **Bubbles** (`github.com/charmbracelet/bubbles`) | viewport / textinput / textarea / list |
| ハイライト | **chroma** (`github.com/alecthomas/chroma/v2`) | ANSI 出力対応、豊富な言語カバレッジ |
| diff | **go-diff** (`github.com/sergi/go-diff`) | split 表示用の line-level アラインメント |
| fuzzy | **fuzzy** (`github.com/sahilm/fuzzy`) | ファイル名・Recent Files 用 |
| git | **`os/exec` で git CLI** | ホスト設定流用、実装が簡素 |
| grep | **ripgrep shell out** → 無ければ Go 実装にフォールバック | 速度最優先 |
| CLI | **Cobra** (`github.com/spf13/cobra`) もしくは標準 `flag` | 軽量でよければ `flag`。サブコマンドを増やすなら Cobra |

## 2. 画面レイアウト

```
┌─ <project-name>  wt: <current-worktree>  branch: <ref>  [M:3 A:1 D:0] ──────────┐
│┌─── Project ─────────┐┌── src/components/Button.tsx  [M]  [View|SplitDiff] ───┐│
││ ▼ src                ││  ┌── HEAD ─────────────┐┌── Working ─────────────┐   ││
││   ▼ components       ││  │ 41  const x = 1     ││ 41  const x = 2        │   ││
││    ● Button.tsx [M]  ││  │ 42  return x        ││ 42  console.log(x)     │   ││
││      Icon.tsx        ││  │                     ││ 43  return x           │   ││
││   app.tsx [A]        ││  │                     ││                        │   ││
││ ▼ test               ││  └─────────────────────┘└────────────────────────┘   ││
│└──────────────────────┘└───────────────────────────────────────────────────────┘│
│[F1 Help] [Ctrl+1 Proj] [Ctrl+E Recent] [Ctrl+P File] [Ctrl+Shift+F Grep] ...     │
└──────────────────────────────────────────────────────────────────────────────────┘
```

- ヘッダ: リポジトリ名 / 現 worktree 名 / ブランチ / 変更件数サマリ
- 左ペイン: ファイルツリー（幅は可変、`Ctrl+1` でフォーカス）
- 右ペイン: ファイル/差分ビュー（幅は可変、`Ctrl+2` でフォーカス）
- フッタ: よく使うショートカットのヒント

### 変更マーカー

| 記号 | 意味 |
|---|---|
| `● M` | Modified |
| `+ A` | Added |
| `× D` | Deleted |
| `→ R` | Renamed |
| `? U` | Untracked |

色は Lip Gloss のテーマで一元管理（GitHub ダーク系配色ベース）。

## 3. キーバインド

ターミナル制約のため **Ctrl ベース**で実装。IntelliJ ユーザは `⌘` を `Ctrl` に読み替え。

skry のグローバルショートカットは **編集中を除き Ctrl を使わない**。`Ctrl+Q` / `Ctrl+C`（終了）と、Edit モード内の `Ctrl+S` / `Ctrl+A` / `Ctrl+E` のみ Ctrl を使う。macOS の `Ctrl+←/→` が Space 切替に取られるため、ペイン幅変更は `[` / `]` を採用。

| 機能 | skry | 備考 |
|---|---|---|
| 終了 | `Ctrl+Q` / `Ctrl+C` / `q` | `q` は文字入力中は無効 |
| ペイン切替 | `Tab` | |
| ファイル名フジィ検索 | `p` | IntelliJ `⌘⇧O` / VSCode `⌘P` 相当 |
| Recent Files | `r` | IntelliJ `⌘E` 相当 |
| プロジェクト全文検索 | `F`（Shift+F） | IntelliJ `⌘⇧F` 相当 |
| Worktree 切替モーダル | `w` | |
| View ↔ SplitDiff 切替 | `d` | 差分ありのファイルのみ |
| 現在のファイル/diff 内検索 | `/` | `n`/`N` で次/前 |
| ツリー内クイックフィルタ | `/`（ツリー focus 時） | |
| View → Edit | `i` / `e` | |
| Edit → View | `Esc` | 未保存は破棄 |
| View/Split → ツリー focus | `Esc` | |
| Edit モード保存 | `Ctrl+S` | |
| Edit モード行頭/行末 | `Ctrl+A` / `Ctrl+E` / `Home` / `End` | |
| モーダル/検索を閉じる | `Esc` | |
| 左ペイン幅 縮/拡 | `[` / `]`（代替 `<`/`>`, `Alt+h`/`Alt+l`） | 4 桁刻み、下限 24、上限 70% |
| 変更ファイルのみフラットリスト | `t` | |
| パスコピー | `y` | editor 優先、次に tree 選択 |
| ヘルプ | `?` | 全バインディング一覧 |
| ← / → でペイン移動 | `←` / `→` | |
| コミット履歴 | `L` | 直列 log（最大 200）→ commit 選択 → ファイル一覧 → commit の SplitDiff |
| ブランチ切替 | `b` | dirty なら `y/N` で --discard-changes 確認 |
| Blame | `B` | 右ペインを Blame 表示にトグル |

**設定ファイル**: MVP 後に `~/.config/skry/keymap.toml` で上書き可能にする。

## 4. モジュール構成

```
skry/
├── cmd/skry/main.go              # エントリポイント
├── internal/
│   ├── app/                      # Bubble Tea Model / Update / View
│   │   ├── app.go                # トップレベルの Model
│   │   ├── keymap.go
│   │   └── theme.go
│   ├── git/                      # git CLI ラッパ
│   │   ├── status.go             # git status --porcelain
│   │   ├── diff.go               # git diff
│   │   ├── worktree.go           # git worktree list
│   │   └── lsfiles.go            # git ls-files（.gitignore 尊重）
│   ├── tree/                     # ファイルツリーの Model + View
│   ├── editor/                   # View / Edit / SplitDiff の各モード
│   │   ├── view.go
│   │   ├── edit.go
│   │   ├── splitdiff.go
│   │   └── highlight.go          # chroma ラッパ
│   ├── search/                   # 検索モーダル群
│   │   ├── file.go               # Ctrl+P
│   │   ├── content.go            # Ctrl+Shift+F
│   │   ├── recent.go             # Ctrl+E
│   │   └── infile.go             # Ctrl+F
│   ├── worktreeui/               # Ctrl+W モーダル
│   └── clipboard/                # パスコピー
├── docs/
├── go.mod
└── go.sum
```

## 5. データフロー

```
CLI args ─▶ bootstrap ─▶ Bubble Tea Program
                              │
                              ├─ on init: git scan (ls-files + status)
                              │    └─▶ FileTree / ChangedFiles
                              │
                              ├─ state: { worktree, selection, mode, modal, focus }
                              │
                              └─ subviews:
                                   ├─ TreeView
                                   ├─ EditorView / SplitDiffView / EditView
                                   └─ Modals: File / Grep / Recent / Worktree / InFile
```

### 状態遷移の主要イベント

- **ツリー選択変更** → 右ペインに再描画（モードは選択ファイルに応じて自動決定）
- **`Ctrl+D`** → View ↔ SplitDiff トグル
- **`Ctrl+W` → worktree 選択** → アプリ内ルートを切替え、ツリー再スキャン、右ペインリセット
- **`Ctrl+S`（Edit モード）** → ファイル書き込み、ツリーの変更マーカーを再計算

## 6. 実装上の注意

### 6.1 SplitDiff の行アラインメント

`go-diff` で line-level diff を取り、以下のルールで 2 カラム化:

- 削除行 → 左に表示、右は空白
- 追加行 → 右に表示、左は空白
- 変更行（削除→追加が連続）→ 左右同じ行位置に揃える
- 変更なし行 → 両側に表示

MVP では完璧な word-level diff は追わない（hunk 単位のハイライトで十分）。

### 6.2 シンタックスハイライト

- chroma は一度 ANSI 文字列化してから viewport に流す
- 初回ロード時にフル描画。以降はキャッシュ
- 5000 行超のファイルはハイライトを遅延 or 無効化（パフォーマンス保護）

### 6.3 全文 grep

- `rg` が `$PATH` にあれば `rg --json -n <query>` を子プロセスで起動
- ストリーミングで受け取り、モーダル内で順次表示
- ripgrep が無い環境では `filepath.Walk` + 正規表現の簡易実装にフォールバック

### 6.4 worktree 切替

- `git worktree list --porcelain` を parse し `{path, branch, is_bare}` のリストを作成
- アプリ内の「現 worktree」は文字列（絶対パス）で保持。以降の `git` 呼び出しは `git -C <path>` で実行
- カレントプロセスの cwd は変更しない（サブコマンド側で制御）

### 6.5 Edit モード

- `textarea` ベース、MVP ではプレーンテキスト編集のみ
- 保存は `os.WriteFile`（元のパーミッションを維持）
- ダーティフラグで「未保存」を表示

## 7. テスト戦略

- **ユニットテスト必須**: `internal/git/`, `internal/search/`, SplitDiff アラインメント
- **スナップショットテスト**: Lip Gloss の View 出力を固定幅で比較
- **UI の手動確認**: 各マイルストーン完了時に実リポジトリで触る
- **テスト実行**: ファイル単位以上で実行（個別テスト関数単位では実行しない）

## 8. リスクと対策

| リスク | 対策 |
|---|---|
| `⌘` キーが終端に届かず IntelliJ 再現度が下がる | Ctrl ベースに寄せ、キーマップを設定で上書き可能に |
| 大規模リポジトリで初回スキャンが遅い | `git ls-files` で一括取得、バックグラウンドで status 更新 |
| chroma のハイライトが重い | 大ファイルは遅延ロード、再ハイライトはキャッシュ |
| worktree 切替時の状態リセットで「何を見ていたか」忘れる | Recent Files（`Ctrl+E`）で緩和 |
| TUI の見た目がユーザ基準で「リッチ」に達しない | Lip Gloss のテーマを早期に固める（M2 で決める） |
