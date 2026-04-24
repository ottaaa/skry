// Package watcher observes file system events beneath a repository root and
// emits debounced "something changed" notifications. It is used to drive
// auto-reload of the tree, statuses, and currently open file when external
// tools (AI agents, editors, git CLI) modify the working tree.
package watcher

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Default directories skipped when walking the repo. `.git` is always skipped
// by Start since it produces high-frequency noise (packed-objects writes,
// index.lock churn). Heavy caches are skipped to avoid exhausting per-process
// file watch limits on macOS.
var skipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	".next":        {},
	"dist":         {},
	"build":        {},
	"target":       {},
}

// Debounce window: AI agents write files in bursts. 250 ms keeps reloads
// responsive while coalescing rapid successive writes into a single refresh.
const debounce = 250 * time.Millisecond

type Watcher struct {
	fsw  *fsnotify.Watcher
	out  chan struct{}
	done chan struct{}
	root string
}

// Start begins watching root recursively. The returned Watcher emits on
// Events() whenever the tree has changed (debounced). Close() stops all
// goroutines and releases OS resources.
func Start(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:  fsw,
		out:  make(chan struct{}, 1),
		done: make(chan struct{}),
		root: root,
	}
	if err := addRecursive(fsw, root); err != nil {
		// Non-fatal: the watcher may still be useful with partial coverage
		// (e.g. some dir we couldn't stat). Log-worthy but don't abort.
		_ = err
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Events() <-chan struct{} { return w.out }

func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	var timer *time.Timer
	var timerC <-chan time.Time
	armTimer := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(debounce)
		timerC = timer.C
	}
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if shouldSkip(ev.Name) {
				continue
			}
			// If a new directory was created, watch it too so we see its
			// contents. Best-effort: a file written into the dir in the same
			// burst may be missed, but subsequent writes land fine.
			if ev.Op.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = addRecursive(w.fsw, ev.Name)
				}
			}
			armTimer()
		case <-w.fsw.Errors:
			// Drain silently; a transient ENOENT for a deleted watched path
			// is the common case and isn't actionable.
		case <-timerC:
			timerC = nil
			// Non-blocking send: if the previous event hasn't been consumed,
			// collapse this one into it. The consumer will re-check state.
			select {
			case w.out <- struct{}{}:
			default:
			}
		case <-w.done:
			return
		}
	}
}

func addRecursive(fsw *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission / transient errors on a subtree shouldn't kill the walk.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if _, skip := skipDirs[d.Name()]; skip && path != root {
			return fs.SkipDir
		}
		return fsw.Add(path)
	})
}

// shouldSkip filters events whose paths are inside dirs we chose not to watch.
// A new Create event inside a skipped dir can still reach us if the parent is
// watched (fsnotify reports events at the parent's level), so we re-check.
func shouldSkip(path string) bool {
	base := filepath.Base(path)
	if _, skip := skipDirs[base]; skip {
		return true
	}
	// Any segment of the path matching a skipDir name.
	for p := path; p != "" && p != "/"; p = filepath.Dir(p) {
		b := filepath.Base(p)
		if _, skip := skipDirs[b]; skip {
			return true
		}
	}
	return false
}

