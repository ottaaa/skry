// Package logfetch is the I/O + caching layer for Log mode. The host
// (internal/app) calls Meta / Diff to obtain a tea.Cmd that resolves to
// either MetaArrivedMsg or DiffArrivedMsg. Cache hits return immediately;
// misses spawn a single git invocation per (sha) or (sha,path), and
// concurrent requests for the same key share that invocation.
//
// Cancellation is cooperative: callers attach a monotonically increasing
// Seq to each request; on arrival the host checks Seq against its current
// value and silently drops stale ones. The fetcher itself never cancels
// in-flight git processes — they're cheap enough that running them to
// completion (and warming the cache) is preferable.
package logfetch

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ottaaa/skry/internal/editor"
	"github.com/ottaaa/skry/internal/git"
)

// MetaArrivedMsg carries the (files, body) result for a single commit.
// Seq mirrors the value the host attached to the request — old values
// are dropped by the host without action.
type MetaArrivedMsg struct {
	Sha   string
	Seq   uint64
	Files []git.StatusEntry
	Body  string
	Err   error
}

// DiffArrivedMsg carries the aligned diff rows for a single (sha, path).
type DiffArrivedMsg struct {
	Sha    string
	Path   string
	Seq    uint64
	Rows   []editor.DiffRow
	Binary bool
	Err    error
}

type metaEntry struct {
	files []git.StatusEntry
	body  string
}

type diffEntry struct {
	rows   []editor.DiffRow
	binary bool
}

// Fetcher caches commit metadata and per-file diff rows for Log mode.
// Safe for concurrent use; intended as a single instance per app.
type Fetcher struct {
	repoRoot string

	meta  *lru[string, metaEntry] // key: sha
	diffs *lru[diffKey, diffEntry]

	metaSF *singleflight[metaEntry]
	diffSF *singleflight[diffEntry]

	prefetchCh chan func()
	prefetchN  int

	// Hooks for tests — production wiring goes through git.* directly.
	showMeta func(repo, sha string) (string, []git.StatusEntry, error)
	showFile func(repo, sha, path string) ([]byte, error)
	parentOf func(repo, sha string) (string, error)
}

type diffKey struct{ sha, path string }

// New constructs a Fetcher rooted at repoRoot. Capacities are deliberately
// chosen to be safe under typical use:
//   - meta: 500 commits (~tens of KB)
//   - diffs: 200 entries, total ≤ 32 MB (a single >2 MB row set is rejected)
//   - prefetch pool: 2 workers
func New(repoRoot string) *Fetcher {
	return &Fetcher{
		repoRoot:   repoRoot,
		meta:       newLRU[string, metaEntry](500, 0, nil),
		diffs:      newLRU[diffKey, diffEntry](200, 32<<20, diffEntrySize),
		metaSF:     newSingleflight[metaEntry](),
		diffSF:     newSingleflight[diffEntry](),
		prefetchCh: make(chan func(), 64),
		prefetchN:  2,
		showMeta:   git.ShowMetaCombined,
		showFile:   git.FileAtBytes,
		parentOf:   git.ParentOf,
	}
}

// Start launches the prefetch worker pool. Call once after New. The
// caller is responsible for invoking Stop to terminate workers (typically
// when the host is being torn down).
func (f *Fetcher) Start() {
	for range f.prefetchN {
		go f.prefetchLoop()
	}
}

// Stop closes the prefetch channel so workers return. Safe to call once.
func (f *Fetcher) Stop() {
	close(f.prefetchCh)
}

func (f *Fetcher) prefetchLoop() {
	for fn := range f.prefetchCh {
		fn()
	}
}

// Reset drops all cached state. Call after a worktree switch — repoRoot
// changes invalidate every cached blob and diff.
func (f *Fetcher) Reset(newRoot string) {
	f.repoRoot = newRoot
	f.meta.Reset()
	f.diffs.Reset()
}

// Meta returns a tea.Cmd that resolves to MetaArrivedMsg for sha. Cache
// hits resolve immediately (synchronous closure, returned as a tea.Cmd
// for uniform handling). Misses spawn a goroutine via tea.Cmd's natural
// scheduling (Bubble Tea executes Cmds on its own runner).
func (f *Fetcher) Meta(sha string, seq uint64) tea.Cmd {
	if v, ok := f.meta.Get(sha); ok {
		return func() tea.Msg {
			return MetaArrivedMsg{Sha: sha, Seq: seq, Files: v.files, Body: v.body}
		}
	}
	return func() tea.Msg {
		v, err := f.fetchMeta(sha)
		if err != nil {
			return MetaArrivedMsg{Sha: sha, Seq: seq, Err: err}
		}
		return MetaArrivedMsg{Sha: sha, Seq: seq, Files: v.files, Body: v.body}
	}
}

func (f *Fetcher) fetchMeta(sha string) (metaEntry, error) {
	return f.metaSF.Do(sha, func() (metaEntry, error) {
		// Re-check the cache under singleflight: a concurrent caller may
		// have populated it while we were queueing.
		if v, ok := f.meta.Get(sha); ok {
			return v, nil
		}
		body, files, err := f.showMeta(f.repoRoot, sha)
		if err != nil {
			return metaEntry{}, err
		}
		v := metaEntry{files: files, body: body}
		f.meta.Set(sha, v)
		return v, nil
	})
}

// Diff returns a tea.Cmd that resolves to DiffArrivedMsg for (sha, path).
// The diff is parent-vs-sha, i.e. the same shape the editor's
// ModeCommitDiff renders.
func (f *Fetcher) Diff(sha, path string, seq uint64) tea.Cmd {
	key := diffKey{sha: sha, path: path}
	if v, ok := f.diffs.Get(key); ok {
		return func() tea.Msg {
			return DiffArrivedMsg{Sha: sha, Path: path, Seq: seq, Rows: v.rows, Binary: v.binary}
		}
	}
	return func() tea.Msg {
		v, err := f.fetchDiff(sha, path)
		if err != nil {
			return DiffArrivedMsg{Sha: sha, Path: path, Seq: seq, Err: err}
		}
		return DiffArrivedMsg{Sha: sha, Path: path, Seq: seq, Rows: v.rows, Binary: v.binary}
	}
}

func (f *Fetcher) fetchDiff(sha, path string) (diffEntry, error) {
	key := diffKey{sha: sha, path: path}
	return f.diffSF.Do(sha+"\x00"+path, func() (diffEntry, error) {
		if v, ok := f.diffs.Get(key); ok {
			return v, nil
		}
		parent, _ := f.parentOf(f.repoRoot, sha)
		newRaw, err := f.showFile(f.repoRoot, sha, path)
		if err != nil {
			return diffEntry{}, err
		}
		var baseRaw []byte
		if parent != "" {
			baseRaw, _ = f.showFile(f.repoRoot, parent, path)
		}
		if isBinaryBytes(newRaw) || isBinaryBytes(baseRaw) {
			v := diffEntry{binary: true}
			f.diffs.Set(key, v)
			return v, nil
		}
		rows := editor.AlignLines(string(baseRaw), string(newRaw))
		v := diffEntry{rows: rows}
		f.diffs.Set(key, v)
		return v, nil
	})
}

// Prefetch schedules background fetches for the given shas. Foreground
// requests on the same sha will share the in-flight call via singleflight,
// so prefetch never starves the user even when the queue is busy. The
// channel is non-blocking with a small buffer; if it overflows we silently
// drop the warm-up — a missed prefetch only costs one foreground delay.
func (f *Fetcher) Prefetch(shas []string) {
	for _, sha := range shas {
		if sha == "" {
			continue
		}
		if _, ok := f.meta.Get(sha); ok {
			continue
		}
		s := sha
		select {
		case f.prefetchCh <- func() { _, _ = f.fetchMeta(s) }:
		default:
			return // queue full; skip remaining
		}
	}
}

// PrefetchDiff is the diff-cache analogue of Prefetch. We use it to warm
// the editor pane for the focused commit's first file as soon as metadata
// arrives, so the user doesn't see a flicker when stepping into the files
// pane.
func (f *Fetcher) PrefetchDiff(sha, path string) {
	if sha == "" || path == "" {
		return
	}
	if _, ok := f.diffs.Get(diffKey{sha: sha, path: path}); ok {
		return
	}
	select {
	case f.prefetchCh <- func() { _, _ = f.fetchDiff(sha, path) }:
	default:
	}
}

func diffEntrySize(v diffEntry) int {
	// Best-effort accounting: each row carries two strings. Chars in a
	// diffmatchpatch run are de-duped via DiffLinesToChars, but we hold the
	// expanded strings here. Estimating 256 B per row is a reasonable
	// upper bound for typical source code without overshooting.
	if v.binary {
		return 64
	}
	return 256 * len(v.rows)
}

// isBinaryBytes mirrors editor.isBinary (which is unexported). Kept here
// to avoid a dependency cycle: editor → logfetch would be wrong.
func isBinaryBytes(b []byte) bool {
	n := len(b)
	if n > 8000 {
		n = 8000
	}
	for i := range n {
		if b[i] == 0 {
			return true
		}
	}
	return false
}
