package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseLogOutput(t *testing.T) {
	in := "aaa111\x1fa111\x1fAlice\x1f2026-04-01\x1fadd foo\n" +
		"bbb222\x1fb222\x1fBob\x1f2026-04-02\x1ffix bar\n"
	got := parseLogOutput(in)
	want := []Commit{
		{Hash: "aaa111", Short: "a111", Author: "Alice", Date: "2026-04-01", Subject: "add foo"},
		{Hash: "bbb222", Short: "b222", Author: "Bob", Date: "2026-04-02", Subject: "fix bar"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseLogOutputIgnoresBlankAndMalformed(t *testing.T) {
	in := "\n" +
		"notenough\x1fparts\n" +
		"aaa\x1fa\x1fC\x1f2026\x1fok\n"
	got := parseLogOutput(in)
	if len(got) != 1 {
		t.Fatalf("want 1 valid commit, got %d (%#v)", len(got), got)
	}
}

func TestParseGraphOutput(t *testing.T) {
	// Simulated output of `git log --graph --pretty=format:'\x1e%H\x1f%h\x1f%an\x1f%ad\x1f%s'`
	// for a small history with one merge.
	mark := "\x1e"
	in := "* " + mark + "aaa111\x1fa111\x1fAlice\x1f2026-04-03\x1fMerge branch 'feat'\n" +
		"|\\\n" +
		"| * " + mark + "bbb222\x1fb222\x1fBob\x1f2026-04-02\x1ffeat: x\n" +
		"|/\n" +
		"* " + mark + "ccc333\x1fc333\x1fCarol\x1f2026-04-01\x1finit\n"
	rows := parseGraphOutput(in)
	if len(rows) != 5 {
		t.Fatalf("want 5 rows, got %d: %#v", len(rows), rows)
	}
	if rows[0].Commit == nil || rows[0].Commit.Short != "a111" {
		t.Errorf("row 0 should be commit a111, got %#v", rows[0])
	}
	if rows[0].Graph != "* " {
		t.Errorf("row 0 graph = %q, want %q", rows[0].Graph, "* ")
	}
	if rows[1].Commit != nil {
		t.Errorf("row 1 should be pure graph, got commit %#v", rows[1].Commit)
	}
	if rows[1].Graph != "|\\" {
		t.Errorf("row 1 graph = %q, want %q", rows[1].Graph, "|\\")
	}
	if rows[2].Commit == nil || rows[2].Commit.Short != "b222" {
		t.Errorf("row 2 should be commit b222, got %#v", rows[2])
	}
	if rows[2].Graph != "| * " {
		t.Errorf("row 2 graph = %q, want %q", rows[2].Graph, "| * ")
	}
	if rows[3].Commit != nil || rows[3].Graph != "|/" {
		t.Errorf("row 3 should be pure graph |/, got %#v", rows[3])
	}
	if rows[4].Commit == nil || rows[4].Commit.Short != "c333" {
		t.Errorf("row 4 should be commit c333, got %#v", rows[4])
	}
}

func TestShowNameStatusAndLogGraphLive(t *testing.T) {
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
	write := func(p, body string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	write("b.txt", "two\n")
	run("add", "-A")
	run("commit", "-q", "-m", "init")

	write("a.txt", "ONE\n") // M
	write("c.txt", "three\n")
	run("rm", "-q", "b.txt")
	run("add", "-A")
	run("commit", "-q", "-m", "second")

	rows, err := LogGraph(dir, 0)
	if err != nil {
		t.Fatalf("LogGraph: %v", err)
	}
	commitRows := 0
	for _, r := range rows {
		if r.Commit != nil {
			commitRows++
		}
	}
	if commitRows != 2 {
		t.Errorf("want 2 commit rows, got %d (rows=%#v)", commitRows, rows)
	}
	// Newest first.
	first := rows[0].Commit
	if first == nil || first.Subject != "second" {
		t.Errorf("expected newest commit 'second' first, got %#v", first)
	}

	entries, err := ShowNameStatus(dir, first.Hash)
	if err != nil {
		t.Fatalf("ShowNameStatus: %v", err)
	}
	want := map[string]Status{"a.txt": StatusModified, "b.txt": StatusDeleted, "c.txt": StatusAdded}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for _, e := range entries {
		if got, ok := want[e.Path]; !ok || got != e.Status {
			t.Errorf("entry %q status %v not in want %+v", e.Path, e.Status, want)
		}
	}

	body, err := CommitMessage(dir, first.Hash)
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if body != "second" {
		t.Errorf("CommitMessage = %q, want %q", body, "second")
	}
}

func TestShowMetaCombinedReturnsBodyAndStatus(t *testing.T) {
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
	write := func(p, body string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	run("add", "-A")
	run("commit", "-q", "-m", "first commit\n\nWith a multi-paragraph\n\nbody.")
	write("a.txt", "two\n")
	write("b.txt", "new\n")
	run("add", "-A")
	run("commit", "-q", "-m", "second\n\nadds b")

	body, entries, err := ShowMetaCombined(dir, "HEAD")
	if err != nil {
		t.Fatalf("ShowMetaCombined: %v", err)
	}
	if body != "second\n\nadds b" {
		t.Errorf("body = %q, want %q", body, "second\n\nadds b")
	}
	want := map[string]Status{"a.txt": StatusModified, "b.txt": StatusAdded}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for _, e := range entries {
		if got, ok := want[e.Path]; !ok || got != e.Status {
			t.Errorf("entry %q status %v not in want %+v", e.Path, e.Status, want)
		}
	}

	// Body with internal blank lines must round-trip.
	body, _, err = ShowMetaCombined(dir, "HEAD~")
	if err != nil {
		t.Fatalf("ShowMetaCombined HEAD~: %v", err)
	}
	if body != "first commit\n\nWith a multi-paragraph\n\nbody." {
		t.Errorf("body with blank lines: got %q", body)
	}
}

func TestIsUnbornBranchErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated error", errors.New("boom"), false},
		{"no commits yet (empty repo)", errors.New("git log: fatal: your current branch 'main' does not have any commits yet"), true},
		{"bad default revision", errors.New("fatal: bad default revision 'HEAD'"), true},
		{"unknown revision", errors.New("fatal: unknown revision 'HEAD'"), true},
		{"partial match in embedded error", errors.New("wrapper: fatal: bad default revision 'HEAD': extra"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isUnbornBranchErr(c.err); got != c.want {
				t.Errorf("isUnbornBranchErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
