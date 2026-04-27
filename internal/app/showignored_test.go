package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ottaaa/skry/internal/git"
	"github.com/ottaaa/skry/internal/watcher"
)

// TestAppendIgnoredIncludesUserScenario reproduces the user-reported case:
// skry opened at the repo root, target file lives under
//   ccp-search-poc/results/100k_bq_archive/report.md
// where ccp-search-poc/.gitignore lists `results/`. With showIgnored=true the
// file must surface in the merged tree list — only skipDir paths are dropped.
func TestAppendIgnoredIncludesUserScenario(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustWrite := func(p, body string) {
		t.Helper()
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	mustWrite(".gitignore", "")
	mustWrite("ccp-search-poc/.gitignore", ".venv/\nresults/\n__pycache__/\n")
	mustWrite("ccp-search-poc/README.md", "real")
	mustWrite("ccp-search-poc/results/100k_bq_archive/report.md", "the file the user wanted")
	mustWrite("ccp-search-poc/.venv/lib/python3.12/site-packages/foo.py", "deps")
	run("add", ".gitignore", "ccp-search-poc/.gitignore", "ccp-search-poc/README.md")
	run("commit", "-q", "-m", "init")

	files, err := git.ListFiles(dir)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if slices.Contains(files, "ccp-search-poc/results/100k_bq_archive/report.md") {
		t.Fatalf("default ListFiles should hide ignored report.md, got %v", files)
	}

	seen := make(map[string]bool, len(files))
	for _, f := range files {
		seen[f] = true
	}
	files = appendIgnored(dir, files, seen)

	if !slices.Contains(files, "ccp-search-poc/results/100k_bq_archive/report.md") {
		t.Errorf("appendIgnored missing report.md, got %v", files)
	}
	for _, f := range files {
		if watcher.ShouldSkip(f) {
			t.Errorf("appendIgnored leaked skipDir entry: %q", f)
		}
	}
	for _, f := range files {
		if filepath.Base(f) == "foo.py" {
			t.Errorf(".venv contents leaked: %q (should be filtered by skipDirs)", f)
		}
	}
}
