package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ottaaa/skry/internal/git"
)

// TestEditorOpenThenGoToLineView covers the search → ModeView path: Open
// loads the file (no working-tree changes) and GoToLine scrolls so the
// requested line is visible.
func TestEditorOpenThenGoToLineView(t *testing.T) {
	dir := t.TempDir()
	rel := "big.txt"
	abs := filepath.Join(dir, rel)
	if err := os.WriteFile(abs, []byte(makeNumberedLines(200)), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(dir)
	m.SetSize(80, 30)
	if err := m.Open(rel, ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if m.mode != ModeView {
		t.Fatalf("expected ModeView, got %v", m.mode)
	}
	m.GoToLine(100)
	if m.view.vp.YOffset == 0 {
		t.Errorf("expected viewport YOffset to advance, got 0")
	}
	out := m.View()
	if !strings.Contains(out, "AAA100") {
		t.Errorf("rendered view should contain AAA100, got first 400 chars:\n%s", firstN(out, 400))
	}
}

// TestEditorOpenThenGoToLineSplitDiff covers the grep → ModeSplit path:
// the file has uncommitted changes, so Open enters SplitDiff and GoToLine
// has to map the working-tree line number to the matching aligned diff row.
func TestEditorOpenThenGoToLineSplitDiff(t *testing.T) {
	dir := t.TempDir()
	lines := makeNumberedLinesSlice(200)
	rel := "big.txt"
	abs := filepath.Join(dir, rel)
	if err := os.WriteFile(abs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("add", rel)
	run("commit", "-q", "-m", "init")

	lines[99] = "MODIFIED line 100"
	if err := os.WriteFile(abs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(dir)
	m.SetSize(80, 30)
	if err := m.Open(rel, git.StatusModified); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if m.mode != ModeSplit {
		t.Fatalf("expected ModeSplit, got %v", m.mode)
	}
	m.GoToLine(100)
	if m.diffTop == 0 {
		t.Errorf("expected diffTop to advance, got 0")
	}
	out := m.View()
	if !strings.Contains(out, "MODIFIED line 100") {
		t.Errorf("rendered SplitDiff should contain MODIFIED line 100, first 800 chars:\n%s", firstN(out, 800))
	}
}

func makeNumberedLines(n int) string {
	return strings.Join(makeNumberedLinesSlice(n), "\n")
}

func makeNumberedLinesSlice(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = "AAA" + itoa(i+1)
	}
	return out
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
