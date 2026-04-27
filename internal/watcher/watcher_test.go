package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldSkip(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/repo/.git", true},
		{"/repo/.git/objects/ff/12", true},
		{"/repo/src/main.go", false},
		{"/repo/node_modules/pkg/index.js", true},
		{"/repo/dist/app.js", true},
		{"/repo/src/.git-like-but-not", false},
		{"/repo/vendored", false},
		{"/repo/.venv/lib/site-packages/foo.py", true},
		{"/repo/venv/bin/python", true},
		{"/repo/pkg/__pycache__/x.cpython-312.pyc", true},
		{"/repo/.pytest_cache/v/cache/lastfailed", true},
		{"/repo/.gradle/caches/x", true},
	}
	for _, c := range cases {
		if got := shouldSkip(c.path); got != c.want {
			t.Errorf("shouldSkip(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestStartEmitsOnWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := Start(root, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	// Modify a file; expect a debounced notification within a reasonable
	// window (debounce is 250ms, so 2s is plenty of headroom for CI).
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("no event after file modification")
	}
}

func TestStartSkipsGitDir(t *testing.T) {
	root := t.TempDir()
	git := filepath.Join(root, ".git", "objects")
	if err := os.MkdirAll(git, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := Start(root, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	// Writing inside .git should not produce a notification.
	if err := os.WriteFile(filepath.Join(git, "deadbeef"), []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Events():
		t.Fatal("unexpected event for .git write")
	case <-time.After(500 * time.Millisecond):
		// Good: no event.
	}
}

func TestStartDetectsNewSubdir(t *testing.T) {
	root := t.TempDir()
	w, err := Start(root, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Consume the mkdir-triggered event.
	select {
	case <-w.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("no event for mkdir")
	}

	// Now write into the new dir; the watcher should have auto-added it.
	if err := os.WriteFile(filepath.Join(sub, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("no event after write to newly-created subdir")
	}
}
