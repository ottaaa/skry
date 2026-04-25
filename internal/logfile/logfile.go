// Package logfile is a minimal rotating JSON-lines logger that writes to
// $XDG_STATE_HOME/skry/log/. It is used for background events that have no
// good place in the TUI — watcher errors, git command failures, etc.
//
// The file is deliberately simple: size-based rotation, a bounded number of
// backups, and a best-effort retention window. There is no level filter and
// no structured context — callers pass a message and optional key/value
// pairs which are serialized as a single JSON object per line.
package logfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ottaaa/skry/internal/xdg"
)

const (
	// DefaultMaxBytes is the rotation threshold: when the active log file
	// exceeds this size, it is renamed and a fresh file is opened.
	DefaultMaxBytes int64 = 10 * 1024 * 1024 // 10 MiB

	// DefaultMaxBackups is the maximum number of rotated files kept on disk.
	// Excess backups are deleted newest-first after each rotation.
	DefaultMaxBackups = 3

	// DefaultMaxAge is the retention window applied on open: any rotated
	// file older than this is deleted. 0 disables age-based pruning.
	DefaultMaxAge = 7 * 24 * time.Hour
)

// Logger is a thread-safe rotating JSON-lines logger. The zero value is NOT
// usable; construct with Open.
type Logger struct {
	mu         sync.Mutex
	dir        string
	base       string // file name inside dir, e.g. "skry.log"
	f          *os.File
	size       int64
	maxBytes   int64
	maxBackups int
	maxAge     time.Duration
}

// Options configures a Logger. Zero values fall back to package defaults.
type Options struct {
	// Dir is the directory to write into. If empty, defaults to
	// $XDG_STATE_HOME/skry/log (created if missing).
	Dir string
	// Name is the active file name. Defaults to "skry.log".
	Name string
	// MaxBytes overrides DefaultMaxBytes when non-zero.
	MaxBytes int64
	// MaxBackups overrides DefaultMaxBackups when non-zero.
	MaxBackups int
	// MaxAge overrides DefaultMaxAge when non-zero. Set to -1 to disable.
	MaxAge time.Duration
}

// Open creates or opens the logger described by opts. It also runs an
// opportunistic retention pass, deleting backups older than MaxAge.
func Open(opts Options) (*Logger, error) {
	dir := opts.Dir
	if dir == "" {
		d, err := xdg.AppStateDir("log")
		if err != nil {
			return nil, fmt.Errorf("logfile: resolve state dir: %w", err)
		}
		dir = d
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("logfile: mkdir: %w", err)
	}
	name := opts.Name
	if name == "" {
		name = "skry.log"
	}
	l := &Logger{
		dir:        dir,
		base:       name,
		maxBytes:   pickInt64(opts.MaxBytes, DefaultMaxBytes),
		maxBackups: pickInt(opts.MaxBackups, DefaultMaxBackups),
		maxAge:     pickDuration(opts.MaxAge, DefaultMaxAge),
	}
	if err := l.openFile(); err != nil {
		return nil, err
	}
	l.pruneByAge()
	return l, nil
}

// Close flushes and closes the underlying file. Further calls to Log become
// no-ops after Close returns.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// Log writes one JSON object containing a timestamp, a message, and any
// key/value pairs supplied. kv must come in (string, any) pairs; a dangling
// odd element is reported under the key "_malformed". Errors are swallowed
// so callers can log from anywhere without error plumbing.
func (l *Logger) Log(msg string, kv ...any) {
	if l == nil {
		return
	}
	rec := map[string]any{
		"ts":  time.Now().UTC().Format(time.RFC3339Nano),
		"msg": msg,
	}
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		rec[k] = kv[i+1]
	}
	if len(kv)%2 != 0 {
		rec["_malformed"] = kv[len(kv)-1]
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	if l.size+int64(len(b)) > l.maxBytes {
		if err := l.rotateLocked(); err != nil {
			return
		}
	}
	n, _ := l.f.Write(b)
	l.size += int64(n)
}

func (l *Logger) openFile() error {
	p := filepath.Join(l.dir, l.base)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("logfile: open: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("logfile: stat: %w", err)
	}
	l.f = f
	l.size = info.Size()
	return nil
}

// rotateLocked renames the current file to "<base>.<timestamp>" and opens a
// fresh empty file. Must be called with l.mu held.
func (l *Logger) rotateLocked() error {
	if l.f == nil {
		return errors.New("logfile: rotate on closed logger")
	}
	if err := l.f.Close(); err != nil {
		return err
	}
	l.f = nil
	src := filepath.Join(l.dir, l.base)
	ts := time.Now().UTC().Format("20060102T150405")
	dst := filepath.Join(l.dir, l.base+"."+ts)
	if err := os.Rename(src, dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	l.pruneByCount()
	return l.openFile()
}

// pruneByCount keeps at most maxBackups rotated files (oldest removed first).
func (l *Logger) pruneByCount() {
	if l.maxBackups <= 0 {
		return
	}
	backups := l.listBackups()
	if len(backups) <= l.maxBackups {
		return
	}
	// Oldest first.
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime().Before(backups[j].ModTime())
	})
	for _, b := range backups[:len(backups)-l.maxBackups] {
		_ = os.Remove(filepath.Join(l.dir, b.Name()))
	}
}

// pruneByAge deletes rotated files older than maxAge. Called once at Open.
func (l *Logger) pruneByAge() {
	if l.maxAge <= 0 {
		return
	}
	cutoff := time.Now().Add(-l.maxAge)
	for _, b := range l.listBackups() {
		if b.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(l.dir, b.Name()))
		}
	}
}

func (l *Logger) listBackups() []os.FileInfo {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil
	}
	prefix := l.base + "."
	out := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) <= len(prefix) || e.Name()[:len(prefix)] != prefix {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	return out
}

// Path returns the absolute path of the currently-active log file. Useful
// for telling users where to look ("see ~/.local/state/skry/log/skry.log").
func (l *Logger) Path() string {
	return filepath.Join(l.dir, l.base)
}

// Discard returns a writer that drops all input. Handy when callers want to
// disable logging without littering nil-checks.
func Discard() io.Writer { return io.Discard }

func pickInt64(v, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}

func pickInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func pickDuration(v, def time.Duration) time.Duration {
	switch {
	case v == 0:
		return def
	case v < 0:
		return 0
	default:
		return v
	}
}
