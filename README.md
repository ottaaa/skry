# peek

軽量な Git 差分ビューア / 読み取り専用エディタ（TUI）。
**AI エージェントが書いたコードを worktree を跨ぎつつ素早く確認する**ための自作ツール。

> Status: **design phase** — コード未着手。詳細は [`docs/`](./docs/) 参照。

## 想定する使い方

```bash
peek                    # カレントリポジトリを開く
peek /path/to/repo      # 指定リポジトリを開く
```

起動すると左にファイルツリー、右に split diff / コードビュー。
`Ctrl+W` で worktree 切替、`Ctrl+P` でファイル名検索、`Ctrl+Shift+F` で全文 grep。

## 技術スタック

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles)
- [chroma](https://github.com/alecthomas/chroma) — シンタックスハイライト
- [go-diff](https://github.com/sergi/go-diff) — split diff アラインメント
- git CLI を `os/exec` で呼び出し

## ドキュメント

- [`docs/requirements.md`](./docs/requirements.md) — 要件（ヒアリング結果）
- [`docs/design.md`](./docs/design.md) — 技術設計
- [`docs/roadmap.md`](./docs/roadmap.md) — マイルストーン
- [`CLAUDE.md`](./CLAUDE.md) — Claude Code 向けプロジェクト指示

## License

TBD（自分専用で始めるが、将来 MIT で公開する可能性あり）
