package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ottaaa/skry/internal/git"
)

func TestScopeFilterStripsAndKeepsOnlyMatchingPaths(t *testing.T) {
	in := []string{
		"README.md",
		"services/devops/cmd/main.go",
		"services/devops/internal/x.go",
		"services/billing/foo.go",
		"services/devops-extra/sneaky.go", // must not match the scope prefix
	}
	got := scopeFilter("services/devops", slices.Clone(in))
	want := []string{"cmd/main.go", "internal/x.go"}
	if !slices.Equal(got, want) {
		t.Errorf("scopeFilter mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestScopeFilterEmptyScopeIsPassthrough(t *testing.T) {
	in := []string{"a.go", "sub/b.go"}
	got := scopeFilter("", slices.Clone(in))
	if !slices.Equal(got, in) {
		t.Errorf("empty scopeDir should not change input, got %v want %v", got, in)
	}
}

func TestScopeFilterStatusesStripsPrefix(t *testing.T) {
	entries := []git.StatusEntry{
		{Path: "services/devops/cmd/main.go", Status: git.StatusModified},
		{Path: "services/billing/foo.go", Status: git.StatusModified},
		{Path: "services/devops/added.go", Status: git.StatusAdded},
	}
	got := scopeFilterStatuses("services/devops", entries)
	wantPaths := []string{"cmd/main.go", "added.go"}
	if len(got) != len(wantPaths) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(wantPaths), got)
	}
	for i, e := range got {
		if e.Path != wantPaths[i] {
			t.Errorf("entry %d: got path %q, want %q", i, e.Path, wantPaths[i])
		}
	}
}

func TestExpandScope(t *testing.T) {
	cases := []struct {
		scope, in, want string
	}{
		{"", "cmd/main.go", "cmd/main.go"},
		{"services/devops", "cmd/main.go", "services/devops/cmd/main.go"},
		{"services/devops", "", ""},
	}
	for _, c := range cases {
		if got := expandScope(c.scope, c.in); got != c.want {
			t.Errorf("expandScope(%q,%q) = %q, want %q", c.scope, c.in, got, c.want)
		}
	}
}

// TestLoadRepoScopedTreeUserScenario reproduces the user-reported workflow:
// invoking skry on /repo/services/devops should produce a tree whose entries
// are relative to services/devops (not the whole repo).
func TestLoadRepoScopedTreeUserScenario(t *testing.T) {
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
	mustWrite := func(p string) {
		t.Helper()
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	mustWrite("README.md")
	mustWrite("services/billing/foo.go")
	mustWrite("services/devops/cmd/main.go")
	run("add", "-A")
	run("commit", "-q", "-m", "init")

	files, err := git.ListFiles(dir)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	files = scopeFilter("services/devops", files)

	if !slices.Contains(files, "cmd/main.go") {
		t.Errorf("scoped tree missing cmd/main.go, got %v", files)
	}
	for _, f := range files {
		if f == "README.md" || f == "services/billing/foo.go" {
			t.Errorf("scoped tree leaked out-of-scope entry %q", f)
		}
	}
}
