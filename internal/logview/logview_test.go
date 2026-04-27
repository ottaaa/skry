package logview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ottaaa/skry/internal/events"
	"github.com/ottaaa/skry/internal/git"
)

func sampleRows() []git.GraphRow {
	return []git.GraphRow{
		{Graph: "* ", Commit: &git.Commit{Hash: "aaa1", Short: "a1", Subject: "third"}},
		{Graph: "|\\"},
		{Graph: "| * ", Commit: &git.Commit{Hash: "bbb2", Short: "b2", Subject: "feat"}},
		{Graph: "|/"},
		{Graph: "* ", Commit: &git.Commit{Hash: "ccc3", Short: "c3", Subject: "init"}},
	}
}

func TestNewSeedsCommitIndices(t *testing.T) {
	m := New(sampleRows())
	if got := m.CommitCount(); got != 3 {
		t.Errorf("CommitCount = %d, want 3", got)
	}
	if c := m.SelectedCommit(); c == nil || c.Short != "a1" {
		t.Errorf("initial cursor should land on newest commit a1, got %#v", c)
	}
}

func TestGraphCursorSkipsPureGraphRows(t *testing.T) {
	m := New(sampleRows())
	// j three times: a1 -> b2 -> c3 -> stays on c3 (past end)
	want := []string{"b2", "c3", "c3"}
	for i, exp := range want {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = newM
		c := m.SelectedCommit()
		if c == nil || c.Short != exp {
			t.Errorf("step %d: got %#v, want short %s", i, c, exp)
		}
	}
}

func TestGraphCursorEmitsFocusEvent(t *testing.T) {
	m := New(sampleRows())
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newM
	if cmd == nil {
		t.Fatal("expected command emitting LogCommitFocusedMsg")
	}
	msg := cmd()
	focused, ok := msg.(events.LogCommitFocusedMsg)
	if !ok {
		t.Fatalf("expected LogCommitFocusedMsg, got %T", msg)
	}
	if focused.Sha != "bbb2" {
		t.Errorf("Sha = %q, want %q", focused.Sha, "bbb2")
	}
}

func TestSetFilesIgnoresStaleResponses(t *testing.T) {
	m := New(sampleRows())
	// Pretend we focused commit a1 and have a pending request.
	m.markLoading("aaa1")
	// A stale answer for some other commit must be ignored.
	m.SetFiles("zzz9", []git.StatusEntry{{Path: "stale.txt", Status: git.StatusAdded}}, "stale")
	if got := m.SelectedFile(); got != nil {
		t.Errorf("stale SetFiles should be ignored, got %#v", got)
	}
	// The matching answer takes effect.
	m.SetFiles("aaa1", []git.StatusEntry{{Path: "real.go", Status: git.StatusModified}}, "real")
	got := m.SelectedFile()
	if got == nil || got.Path != "real.go" {
		t.Errorf("expected real.go, got %#v", got)
	}
}

func TestFilesPaneEmitsFileFocusedMsg(t *testing.T) {
	m := New(sampleRows())
	m.SetFiles("aaa1",
		[]git.StatusEntry{
			{Path: "a.go", Status: git.StatusModified},
			{Path: "b.go", Status: git.StatusAdded},
		}, "third")
	m.SetFocus(FocusFiles)
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newM
	if cmd == nil {
		t.Fatal("expected file-focus cmd")
	}
	got, ok := cmd().(events.LogFileFocusedMsg)
	if !ok {
		t.Fatalf("expected LogFileFocusedMsg, got %T", cmd())
	}
	if got.Path != "b.go" || got.Sha != "aaa1" {
		t.Errorf("got %#v, want path=b.go sha=aaa1", got)
	}
}

func TestEmitInitialFocusReturnsCommitMsg(t *testing.T) {
	m := New(sampleRows())
	cmd := m.EmitInitialFocus()
	if cmd == nil {
		t.Fatal("expected initial-focus cmd")
	}
	msg, ok := cmd().(events.LogCommitFocusedMsg)
	if !ok {
		t.Fatalf("got %T, want LogCommitFocusedMsg", cmd())
	}
	if msg.Sha != "aaa1" {
		t.Errorf("Sha = %q, want aaa1", msg.Sha)
	}
}

func TestNeighborShasReturnsSurroundingCommits(t *testing.T) {
	rows := sampleRows()
	m := New(rows)
	// Cursor on first commit (a1). Neighbors n=2 should give the two
	// commits below: b2, c3 (no commits above).
	got := m.NeighborShas(2)
	want := map[string]bool{"bbb2": true, "ccc3": true}
	if len(got) != len(want) {
		t.Fatalf("NeighborShas(2) at top: got %v, want %v", got, want)
	}
	for _, sha := range got {
		if !want[sha] {
			t.Errorf("unexpected sha %q in neighbors", sha)
		}
	}
}

func TestSetRowsResetsCursor(t *testing.T) {
	m := New(sampleRows())
	// Move down twice.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if c := m.SelectedCommit(); c == nil || c.Short != "c3" {
		t.Fatalf("setup: expected c3, got %#v", c)
	}
	// Replace rows — cursor must reset.
	m.SetRows(sampleRows()[:1]) // just the first commit
	if c := m.SelectedCommit(); c == nil || c.Short != "a1" {
		t.Errorf("after SetRows: expected a1, got %#v", c)
	}
}

func TestNeighborShasEmpty(t *testing.T) {
	m := New(nil)
	if got := m.NeighborShas(2); len(got) != 0 {
		t.Errorf("NeighborShas on empty: got %v, want []", got)
	}
}

func TestRenderProducesNoEmptyOutput(t *testing.T) {
	m := New(sampleRows())
	m.SetSize(30, 30, 10)
	m.SetFiles("aaa1",
		[]git.StatusEntry{{Path: "x.go", Status: git.StatusModified}},
		"third\n\nbody line")
	if got := m.LeftView(); got == "" {
		t.Error("LeftView should not be empty")
	}
	if got := m.RightView(); got == "" {
		t.Error("RightView should not be empty")
	}
}
