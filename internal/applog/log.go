// Package applog wires charmbracelet/log to a per-OS cache directory so
// background events (watcher errors, editor.Open failures, autosave
// failures, etc.) can be inspected post-mortem without polluting the TUI.
//
// Path resolution uses muesli/go-app-paths for OS conventions:
//
//	macOS:   ~/Library/Caches/skry/skry.log
//	Linux:   ~/.cache/skry/skry.log         (or $XDG_CACHE_HOME/skry/...)
//	Windows: %LOCALAPPDATA%\skry\Cache\skry.log
//
// The log is append-only and grows unbounded — same as glow. Rotate or
// delete by hand if it becomes unwieldy:
//
//	rm "$(skry --log-path)"  // (--log-path is reserved for a future flag)
package applog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	gap "github.com/muesli/go-app-paths"
)

// Logger wraps charmbracelet/log and the underlying file handle so the
// caller can defer Close() at program exit.
type Logger struct {
	inner *log.Logger
	file  *os.File
	path  string
}

// Setup opens (or creates) the log file at the OS-conventional cache path
// and returns a Logger configured at DebugLevel. Failures here are
// surfaced as errors — callers are expected to fall back to a nil logger
// (the watcher and autosave handlers tolerate that already).
func Setup() (*Logger, error) {
	scope := gap.NewScope(gap.User, "skry")
	dir, err := scope.CacheDir()
	if err != nil {
		return nil, fmt.Errorf("applog: resolve cache dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("applog: mkdir %q: %w", dir, err)
	}
	path := filepath.Join(dir, "skry.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("applog: open %q: %w", path, err)
	}
	inner := log.New(f)
	inner.SetLevel(log.DebugLevel)
	inner.SetReportTimestamp(true)
	return &Logger{inner: inner, file: f, path: path}, nil
}

// Log adapts the watcher.Logger contract (Log(msg, kv...)) to the
// charmbracelet/log API. nil-safe so callers can wire it without
// per-call nil checks.
func (l *Logger) Log(msg string, kv ...any) {
	if l == nil {
		return
	}
	l.inner.Info(msg, kv...)
}

// Path returns the absolute path of the active log file. Useful for
// telling the user where to look.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close flushes and closes the underlying file. Safe to call on nil.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("applog: close: %w", err)
	}
	l.file = nil
	return nil
}
