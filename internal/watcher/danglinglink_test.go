package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStartContinuesPastDanglingSymlink ensures a broken symlink in one
// subtree does not abort the recursive walk and leave later subtrees
// unwatched. Real-world repro: /Users/ota/sumally/sumally-web has
// .cursor/commands/pr.md → ../../.claude/commands/pr.md (target absent).
// Before the fix, addRecursive halted at .cursor/, so changes under
// alphabetically-later directories (e.g. ccp-search-poc/) were silently
// dropped.
func TestStartContinuesPastDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	mustMkdir := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// .cursor/ comes before later/ alphabetically, so a halt here would have
	// hidden later/.
	mustMkdir(".cursor/commands")
	mustMkdir("later/inner")
	if err := os.Symlink("does-not-exist", filepath.Join(root, ".cursor/commands/dangling")); err != nil {
		t.Fatal(err)
	}

	w, err := Start(root, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Touch a file under later/inner/. If Start gave up before reaching it,
	// no event will fire.
	target := filepath.Join(root, "later/inner/touched.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
	case <-time.After(2 * time.Second):
		t.Fatalf("watcher missed write under later/inner — addRecursive halted at the dangling symlink")
	}
}
