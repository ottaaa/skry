# skry — 運用マニュアル

CI / リリース / lint / ローカル開発の手順書。各セクションは独立して読めるように
書いてあります。「何かが壊れた」「リリースしたい」「lint を fix したい」と
思ったときは、該当セクションだけ拾い読みでOK。

---

## 1. ローカル開発

### 1-a. 初回セットアップ

```bash
git clone https://github.com/ottaaa/skry.git
cd skry
go mod download
make build                # bin/skry を生成
```

`golangci-lint` をローカルで走らせたい場合 (任意):

```bash
brew install golangci-lint     # 推奨。バージョンは v2.7+
# または
make lint-install              # go install で最新版
```

### 1-b. 動作確認

`testdata/sample-repo/seed.sh` が小さな git 済みのフィクスチャを生成します。
`make dev` 一発でこれを再生成して TUI を起動します:

```bash
make dev                  # → testdata/sample-repo を再 seed して起動
make dev ARGS=~/code/foo  # 別の repo に向ける (seed はスキップ)
make dev-seed             # seed のみ
```

`testdata/sample-repo/` の中身 (`.git`, `README.md`, `src/`, `docs/`,
`notes.txt`) は全て `.gitignore` 済み。`seed.sh` だけが parent repo に
コミットされます。

### 1-c. ローカルチェックの一周

PR を出す前にこれだけ通っていればCI は基本的に緑です:

```bash
make fmt                  # gofmt
make vet                  # go vet
make lint                 # golangci-lint
make test-race            # -race + coverage.out
```

`make ci` は CI ランナーが叩くターゲット (`= test-race`) なので、CI と同じ
テスト条件をローカルで再現したいときに便利。

### 1-d. 手元バイナリを最新コードに揃える

skry は **2 箇所** にインストールされうる:

- `./bin/skry` — `make build` の出力。リポジトリ内で動かす用
- `~/go/bin/skry` (= `$(go env GOPATH)/bin/skry`) — `go install ./cmd/skry`
  の出力。`PATH` に通っているのでどこからでも `skry` で起動できる

コードを修正したあとは **両方** を更新しないと、ターミナルで `skry` を叩いた
ときに古い挙動が再現することがある (実際に line-jump 修正の検証時にハマった)。
最低限これだけ流す:

```bash
make build && go install ./cmd/skry
```

挙動確認は `./bin/skry --version` と `skry --version` の両方を見て、表示される
short SHA が `git rev-parse --short HEAD` と一致することを確認すると安全。

---

## 2. CI

`.github/workflows/` 配下に 3 本。push と PR で発火するのは `build` と `trivy`、
release は `tagpr` だけが発火源。

### 2-a. `build` (`.github/workflows/ci.yml`)

ジョブ: **`job-test`** (matrix: `ubuntu-latest` + `macos-latest`)

ステップ:

1. checkout
2. Go セットアップ (`go-version-file: go.mod`)
3. `go vet ./...`
4. **golangci-lint via reviewdog** (Linux のみ) — `fail_level: error`。
   PR には inline コメントで lint 結果が貼られます
5. **gostyle** (Linux のみ) — k1LoW 製の vettool。`.gostyle.yml` を読む
6. `make ci` (= `go test -race -covermode=atomic -coverprofile=coverage.out ./...`)
7. **octocov** (Linux のみ) — coverage / code-to-test ratio / 実行時間を集計し、
   PR のコメントに貼る + main ブランチでは badge 用 artifact を保存

> **README の badge を有効化したい場合**: GitHub の自分のアカウント直下に
> `octocovs` という空 repository を作ってください (例: `ottaaa/octocovs`)。
> octocov は `report.datastores: artifact://${GITHUB_REPOSITORY}` でこの repo に
> badges (`badges/ottaaa/skry/coverage.svg` 等) を push します。`README.md` に
> コメントアウト済みの badge URL があるので、`octocovs` repo ができたら
> コメントを外して反映。

#### 落ちたとき

| 症状 | 切り分け |
| --- | --- |
| `go vet` が落ちる | ローカルで `make vet`。出力にすべて理由が出ている |
| `golangci-lint` が落ちる | `make lint`。レポートは PR コメントにも貼られている |
| `gostyle` が落ちる | k1LoW/gostyle の出力を CI ログで読む。設定は `.gostyle.yml` で `mixedcaps`/`funcfmt` を off 済み |
| `make ci` が落ちる | `make test-race` をローカルで。`-race` で初めて出る race は CI でも reproducer になる |
| macOS だけ落ちる | fsnotify (kqueue) や `git` バージョン差異の可能性。watcher tests は OS-specific |

### 2-b. `trivy` (`.github/workflows/trivy.yml`)

依存ライブラリの **license スキャン**。`scanners: license`、`exit-code: 1`、
`severity: UNKNOWN,MEDIUM,HIGH`。

#### 落ちたとき

- 新しく追加した依存に viral / unknown ライセンスが含まれていないか確認
- 誤検知の場合は trivy 側で除外設定を追加 (今は持っていないので必要なら追加する)
- どうしても通らない依存は採用しない (CLAUDE.md「依存最小」の方針に従う)

### 2-c. Action の SHA ピン

全 action は `@<sha> # <tag>` の形式で SHA ピン済み。dependabot の更新を
受けるときは PR 内で SHA とタグの両方が変わっていることを確認。

---

## 3. Lint / フォーマット

### 3-a. 設定ファイル

- `.golangci.yml` — golangci-lint v2 設定
  - 有効: `errorlint`, `godot`, `gosec`, `misspell`, `revive`, `modernize`
  - **`funcorder` は意図的に無効**: 既存コードはメソッドを「責務順」で
    並べているため、機械的な exported→unexported 並び替えは churn のみ。
    将来コードベースが安定してから別 PR で入れる
  - 例外: テストファイル (`_test.go`) は `revive`/`godot`/`gosec`/`funcorder`
    を緩める。chroma の `lexers.Analyse` (英国スペル) は misspell から除外
- `.gostyle.yml` — gostyle (k1LoW) の設定
  - `mixedcaps` / `funcfmt` は off (Go 慣習と細部が合わないため)
  - `errorstrings.exclude-test: true`

### 3-b. ルールを足したくなったら

1. `.golangci.yml` の `linters.enable` に追加
2. `make lint` を走らせて、現コードベースで何件落ちるか確認
3. **5件以下なら**: その場で fix
4. **多いなら**: 別 PR を立てて、`exclusions.rules` で path-rule にして
   段階的に直す。一気に直そうとしない (review が地獄になる)

### 3-c. False positive の抑制

優先順:

1. **コードを直す** — 95% はこれで足りる
2. `.golangci.yml` の `exclusions.rules` で path 単位に narrow exclusion を書く
   (例: chroma の `Analyse` は `internal/editor/highlight.go` のみ misspell を off)
3. 行末の `//nolint:lintname // 理由` は最終手段。**理由なしの nolint は禁止**

---

## 4. リリース手順

skry は **tagpr** が版管理 PR を自動生成し、それを merge すると **goreleaser** が
バイナリを配布する仕組みです。手で `git tag` を打つ必要はありません。

### 4-a. リリースの流れ

```
main に feat:/fix: コミット
        ↓
tagpr workflow が走る
        ↓
"Release for vX.Y.Z" PR が自動生成 / 既存ならコミットが追加される
        ↓
PR を merge
        ↓
tag (vX.Y.Z) が自動で push される
        ↓
goreleaser job が起動 → goreleaser が cross-compile → GitHub Releases に成果物 upload
```

### 4-b. 各コミットがどう version bump につながるか

- `feat:` → minor bump (`X.Y.Z` → `X.(Y+1).0`)
- `fix:` / その他 → patch bump
- PR のラベル `tagpr:major` を付けるとそのリリースは major bump、
  `tagpr:minor` なら minor 強制
- `docs:` / `chore:` / `test:` だけのコミットでは tagpr は新しい PR を作らない
  (`.goreleaser.yml` の changelog filter と整合)

### 4-c. version.go との関係

`version/version.go` の `const Version` を tagpr が書き換える。
**手で編集しない**。手で打った変更は次の tagpr PR で上書きされる。

`Revision` は ldflags で build 時に注入される短 SHA。`Makefile` の
`LDFLAGS` で `-X github.com/ottaaa/skry/version.Revision=$(REVISION)`。

### 4-d. リリース成果物

`.goreleaser.yml` で:

- darwin / linux / windows × amd64 / arm64 (windows は amd64 のみ)
- Linux のみ deb / rpm / apk (nfpms)
- `checksums.txt`
- アーカイブには `LICENSE` と `README.md` を同梱

### 4-e. リリースが落ちたとき

| ステップ | 切り分け |
| --- | --- |
| `tagpr` job が PR を作らない | 直近のコミットが `docs:`/`chore:` のみ。`feat:`/`fix:` を含むまで bump されない (期待動作) |
| tag は付いたが goreleaser が落ちた | `tagpr.yml` の `release` job のログを見る。`goreleaser --clean` が `dist/` を消すので、release ノートが draft で残っているはず |
| バイナリが上がらない | `release.use_existing_draft: true` なので、まずは `gh release view vX.Y.Z` で draft を確認。手で publish できる |

### 4-f. 手で fall-back するとき (最終手段)

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

---

## 5. ログとトラブルシューティング

### 5-a. skry のランタイムログ

watcher のエラーや editor の open 失敗、autosave の失敗などは、TUI を汚さない
ために OS 標準のキャッシュディレクトリに `charmbracelet/log` の logfmt 形式で
追記されます:

| OS | パス |
| --- | --- |
| macOS | `~/Library/Caches/skry/skry.log` |
| Linux | `~/.cache/skry/skry.log` (or `$XDG_CACHE_HOME/skry/skry.log`) |
| Windows | `%LOCALAPPDATA%\skry\Cache\skry.log` |

```bash
# 直近を見る (macOS)
tail -n 50 ~/Library/Caches/skry/skry.log

# 特定キーワードだけ
grep "watcher:" ~/Library/Caches/skry/skry.log

# 大きくなったら手動で削除して OK (append-only / rotation なし)
rm ~/Library/Caches/skry/skry.log
```

### 5-b. 自動再読込が効かない

1. `.git` / `node_modules` 等は意図的に除外している (`internal/watcher/watcher.go`
   の `skipDirs`)
2. macOS で大量のディレクトリを watch すると `kqueue` の上限に当たる場合がある。
   `ulimit -n` を確認。skry はサブツリーごとに add するので、1 リポジトリで
   数千ディレクトリある場合は `skipDirs` を増やす検討
3. Edit モード中は意図的に reload を抑止する。`Esc` で View に戻ってから再度確認

### 5-c. `make dev` のサンプルが壊れた

```bash
make dev-seed             # 強制再生成。冪等
```

`testdata/sample-repo/` に他の生成物が残っていても seed.sh が wipe する。

---

## 6. 「コードを変える前に」チェックリスト

- README.md と `internal/helpui/helpui.go` のキーバインド表を **両方** 同期する
- ロジック層に変更を入れたら同名 `_test.go` を更新 / 追加する
- `.git` / `node_modules` 等の skip 対象を変えたら `internal/watcher/watcher.go`
  と watcher の test を両方更新
- `version/version.go` を **手で書き換えない** (tagpr が壊れる)
- 新しい依存を増やすときは CLAUDE.md の「依存最小」方針に照らす
