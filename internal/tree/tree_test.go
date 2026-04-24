package tree

import (
	"testing"

	"github.com/ottaaa/peek/internal/git"
)

func TestSetFilesBuildsTree(t *testing.T) {
	m := New()
	m.SetFiles(
		[]string{"a/b.go", "a/c.go", "d.go"},
		map[string]git.Status{"a/b.go": git.StatusModified, "d.go": git.StatusAdded},
	)
	if m.root == nil {
		t.Fatal("root nil")
	}
	if len(m.root.Children) != 2 {
		t.Fatalf("want 2 top-level children (a/ and d.go), got %d", len(m.root.Children))
	}
	// Dirs sort before files.
	if !m.root.Children[0].IsDir || m.root.Children[0].Name != "a" {
		t.Errorf("want first child to be dir 'a', got %+v", m.root.Children[0])
	}
	// Parent dir status propagates up (modified is the only leaf status under a/).
	if m.root.Children[0].Status != git.StatusModified {
		t.Errorf("parent status: got %q want M", m.root.Children[0].Status)
	}
}

func TestFlatModeShowsOnlyChanged(t *testing.T) {
	m := New()
	m.SetFiles(
		[]string{"a/b.go", "a/c.go", "d.go", "e/f/g.go"},
		map[string]git.Status{"a/b.go": git.StatusModified, "d.go": git.StatusAdded},
	)
	m.ToggleFlat()
	if len(m.rows) != 2 {
		t.Fatalf("flat rows: want 2, got %d (%+v)", len(m.rows), m.rows)
	}
	paths := []string{m.rows[0].node.Path, m.rows[1].node.Path}
	if paths[0] != "a/b.go" || paths[1] != "d.go" {
		t.Errorf("flat row paths sorted: got %v, want [a/b.go d.go]", paths)
	}
}

func TestHigherStatusPriority(t *testing.T) {
	if higher(git.StatusModified, git.StatusDeleted) != git.StatusDeleted {
		t.Error("deleted should outrank modified")
	}
	if higher(git.StatusUntracked, git.StatusNone) != git.StatusUntracked {
		t.Error("untracked should outrank none")
	}
	if higher(git.StatusAdded, git.StatusModified) != git.StatusAdded {
		t.Error("added should outrank modified")
	}
}
