package applog

import (
	"os"
	"strings"
	"testing"
)

// TestSetupCreatesLogFile uses XDG_CACHE_HOME to redirect gap to a temp
// dir so the test doesn't pollute the user's actual cache. (gap honors
// $XDG_CACHE_HOME on Linux; on macOS it falls back to $HOME — covered by
// the home-redirect path below.)
func TestSetupCreatesLogFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp) // covers the macOS path that ignores XDG_CACHE_HOME

	l, err := Setup()
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// File should exist at the resolved path under tmp.
	if !strings.HasPrefix(l.Path(), tmp) {
		t.Errorf("log path %q should be under tmp %q", l.Path(), tmp)
	}
	if _, err := os.Stat(l.Path()); err != nil {
		t.Errorf("log file should exist after Setup: %v", err)
	}
}

func TestLogWritesLine(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	l, err := Setup()
	if err != nil {
		t.Fatal(err)
	}
	l.Log("watcher: addRecursive failed", "root", "/tmp/x", "err", "boom")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	body, err := os.ReadFile(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, want := range []string{"watcher: addRecursive failed", "root=", "/tmp/x", "err=", "boom"} {
		if !strings.Contains(got, want) {
			t.Errorf("log output should contain %q, got:\n%s", want, got)
		}
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger
	l.Log("must not panic")
	if err := l.Close(); err != nil {
		t.Errorf("nil Close should be a no-op, got %v", err)
	}
	if p := l.Path(); p != "" {
		t.Errorf("nil Path should be empty, got %q", p)
	}
}
