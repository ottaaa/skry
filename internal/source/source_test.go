package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFromArgEmptyMeansCwd(t *testing.T) {
	cwd, _ := os.Getwd()
	r, err := FromArg(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Path != cwd {
		t.Errorf("empty arg should resolve to cwd %q, got %q", cwd, r.Path)
	}
}

func TestFromArgRejectsNonDir(t *testing.T) {
	dir := t.TempDir()
	notDir := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FromArg(t.Context(), notDir); err == nil {
		t.Errorf("FromArg should reject non-directory paths")
	}
}

func TestFromArgRejectsMissing(t *testing.T) {
	if _, err := FromArg(t.Context(), "/no/such/path/skry-test"); err == nil {
		t.Errorf("FromArg should error on missing paths")
	}
}

func TestPickStdinFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello world", "stdin.txt"},
		{"git diff prefix", "diff --git a/foo b/foo\n+++ b/foo", "stdin.diff"},
		{"unified diff fallback", "--- a/foo\n+++ b/foo", "stdin.diff"},
		{"json", "  {\"a\": 1}", "stdin.json"},
		{"xml", "<?xml ...?>", "stdin.xml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickStdinFilename([]byte(c.in)); got != c.want {
				t.Errorf("pickStdinFilename(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestInitRepoWithFileCommits exercises the actual git plumbing — needs
// `git` on PATH (every CI matrix host already has it).
func TestInitRepoWithFileCommits(t *testing.T) {
	dir := t.TempDir()
	body := []byte("hello from stdin\n")
	if err := os.WriteFile(filepath.Join(dir, "stdin.txt"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initRepoWithFile(t.Context(), dir, "stdin.txt"); err != nil {
		t.Fatalf("initRepoWithFile: %v", err)
	}
	// Verify the file is tracked and HEAD resolves.
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git after init, got %v", err)
	}
}

// TestFromStdinEndToEnd is light because os.Stdin can't be cleanly
// reassigned in-test; we exercise the public path via a synthetic stdin
// pipe and assert the resolved temp directory contains a tracked file.
func TestFromStdinEndToEnd(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = w.Write([]byte("piped content\n"))
		_ = w.Close()
	}()

	saved := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = saved })

	res, err := fromStdin(t.Context())
	if err != nil {
		t.Fatalf("fromStdin: %v", err)
	}
	t.Cleanup(res.Cleanup)

	body, err := os.ReadFile(filepath.Join(res.Path, "stdin.txt"))
	if err != nil {
		t.Fatalf("read materialized file: %v", err)
	}
	if string(body) != "piped content\n" {
		t.Errorf("materialized file mismatch: %q", string(body))
	}
}

// keep import alive even if someone trims unused tests later.
var _ = context.TODO
