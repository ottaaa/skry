package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestListIgnoredFiles(t *testing.T) {
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
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("results/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "results/sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "results/sub/report.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "debug.log"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".gitignore", "kept.txt")
	run("commit", "-q", "-m", "init")

	// ListFiles must NOT see ignored entries.
	files, err := ListFiles(dir)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if slices.Contains(files, "results/sub/report.md") || slices.Contains(files, "debug.log") {
		t.Errorf("ListFiles should hide gitignored entries, got %v", files)
	}

	// ListIgnoredFiles must return them.
	ignored, err := ListIgnoredFiles(dir)
	if err != nil {
		t.Fatalf("ListIgnoredFiles: %v", err)
	}
	if !slices.Contains(ignored, "results/sub/report.md") {
		t.Errorf("ListIgnoredFiles missing results/sub/report.md, got %v", ignored)
	}
	if !slices.Contains(ignored, "debug.log") {
		t.Errorf("ListIgnoredFiles missing debug.log, got %v", ignored)
	}
	if slices.Contains(ignored, "kept.txt") {
		t.Errorf("ListIgnoredFiles should not include tracked kept.txt, got %v", ignored)
	}
}
