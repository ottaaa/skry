package watcher

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// fileDebounce is shorter than the recursive watcher's because per-file
// events are scoped — the user is staring at one file expecting near-
// instant feedback when an external editor / AI agent saves it.
const fileDebounce = 80 * time.Millisecond

// FileWatcher tracks a single absolute path and emits a debounced signal
// when that file changes. Editors typically save atomically (write a
// temp + rename), which invalidates an inotify watch on the file itself,
// so we watch the parent directory and filter events by basename.
//
// Lifecycle is independent of the recursive Watcher: callers create one
// FileWatcher per pane that wants its own fast-path, and call Watch(abs)
// to swap targets when the open file changes. An empty Watch("") stops
// emitting until a new path is set.
type FileWatcher struct {
	fsw  *fsnotify.Watcher
	out  chan struct{}
	done chan struct{}
	log  Logger

	mu       sync.Mutex
	watching string // absolute path of the currently-watched file
	watchDir string // parent dir we Add()'d in fsnotify
}

// StartFile creates a FileWatcher with no current target. Pair every
// successful StartFile with a Close to release the OS-level fsnotify
// resources.
func StartFile(log Logger) (*FileWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("filewatcher: new fsnotify: %w", err)
	}
	fw := &FileWatcher{
		fsw:  fsw,
		out:  make(chan struct{}, 1),
		done: make(chan struct{}),
		log:  log,
	}
	go fw.loop()
	return fw, nil
}

// Events delivers debounced "the watched file changed" signals. The
// channel is non-blocking on the sender side: bursts coalesce into one
// pending signal.
func (w *FileWatcher) Events() <-chan struct{} { return w.out }

// Watch swaps the file under watch. Pass "" to stop emitting. The
// previous parent-dir add is removed so we don't accumulate watches over
// time as the user navigates.
func (w *FileWatcher) Watch(absPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.watching == absPath {
		return
	}
	if w.watchDir != "" {
		_ = w.fsw.Remove(w.watchDir)
		w.watchDir = ""
	}
	w.watching = absPath
	if absPath == "" {
		return
	}
	dir := filepath.Dir(absPath)
	if err := w.fsw.Add(dir); err != nil {
		if w.log != nil {
			w.log.Log("filewatcher: add failed", "dir", dir, "err", err.Error())
		}
		w.watching = ""
		return
	}
	w.watchDir = dir
}

// Close stops the loop and releases fsnotify. Idempotent.
func (w *FileWatcher) Close() error {
	select {
	case <-w.done:
		return nil
	default:
	}
	close(w.done)
	if err := w.fsw.Close(); err != nil {
		return fmt.Errorf("filewatcher: close: %w", err)
	}
	return nil
}

func (w *FileWatcher) loop() {
	var timer *time.Timer
	var timerC <-chan time.Time
	arm := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(fileDebounce)
		timerC = timer.C
	}
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !w.matches(ev.Name) {
				continue
			}
			arm()
		case err := <-w.fsw.Errors:
			if err != nil && w.log != nil {
				w.log.Log("filewatcher: fsnotify error", "err", err.Error())
			}
		case <-timerC:
			timerC = nil
			// Non-blocking send: coalesce.
			select {
			case w.out <- struct{}{}:
			default:
			}
		case <-w.done:
			return
		}
	}
}

// matches returns true when the event path is the file we are currently
// watching. Done under the lock because Watch can swap targets concurrently.
func (w *FileWatcher) matches(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watching != "" && name == w.watching
}
