package editor

import (
	"os"
	"path/filepath"
	"testing"
)

// loadTempFile writes content to a fresh tempfile and loads it into an
// editMode, returning the populated editor.
func loadTempFile(t *testing.T, content string) *editMode {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEditMode()
	if err := e.Load(p, "f.txt", content); err != nil {
		t.Fatal(err)
	}
	e.SetSize(80, 24)
	return &e
}

func TestUndoInsertRune(t *testing.T) {
	e := loadTempFile(t, "hello")
	e.col = 5
	e.insertRune('!')
	if got := e.Value(); got != "hello!" {
		t.Fatalf("after insert: %q", got)
	}
	if !e.Undo() {
		t.Fatal("Undo returned false")
	}
	if got := e.Value(); got != "hello" {
		t.Errorf("after undo: %q, want \"hello\"", got)
	}
	if e.dirty {
		t.Errorf("dirty should be false after undoing to saved state")
	}
	if !e.Redo() {
		t.Fatal("Redo returned false")
	}
	if got := e.Value(); got != "hello!" {
		t.Errorf("after redo: %q, want \"hello!\"", got)
	}
	if !e.dirty {
		t.Errorf("dirty should be true after redo past saved state")
	}
}

func TestUndoSplitLineAndBackspace(t *testing.T) {
	e := loadTempFile(t, "abcdef")
	e.row, e.col = 0, 3
	e.splitLine()
	if e.Value() != "abc\ndef" {
		t.Fatalf("after split: %q", e.Value())
	}
	if e.row != 1 || e.col != 0 {
		t.Errorf("cursor after split: row=%d col=%d", e.row, e.col)
	}
	e.backspace() // merges lines again
	if e.Value() != "abcdef" {
		t.Fatalf("after backspace: %q", e.Value())
	}
	// Two undos should step us back through each op.
	if !e.Undo() {
		t.Fatal("first Undo")
	}
	if e.Value() != "abc\ndef" {
		t.Errorf("after 1st undo: %q", e.Value())
	}
	if !e.Undo() {
		t.Fatal("second Undo")
	}
	if e.Value() != "abcdef" {
		t.Errorf("after 2nd undo: %q", e.Value())
	}
}

func TestUndoEmptyStackIsNoop(t *testing.T) {
	e := loadTempFile(t, "x")
	if e.Undo() {
		t.Error("Undo on empty stack should return false")
	}
	if e.Redo() {
		t.Error("Redo on empty stack should return false")
	}
}

func TestNewEditDiscardsRedo(t *testing.T) {
	e := loadTempFile(t, "ab")
	e.col = 2
	e.insertRune('c') // "abc"
	_ = e.Undo()      // back to "ab", redo has "abc"
	e.insertRune('X') // "abX" — should drop the redo stack
	if len(e.redo) != 0 {
		t.Errorf("redo stack should be cleared after a fresh edit, got len=%d", len(e.redo))
	}
	if e.Redo() {
		t.Error("Redo should be a no-op after redo history was dropped")
	}
}

func TestUndoBoundsToCap(t *testing.T) {
	e := loadTempFile(t, "")
	for range undoCap + 50 {
		e.insertRune('a')
	}
	if len(e.undo) != undoCap {
		t.Errorf("undo stack capped: got %d, want %d", len(e.undo), undoCap)
	}
}

func TestBackspaceAtStartIsNoop(t *testing.T) {
	e := loadTempFile(t, "hi")
	e.row, e.col = 0, 0
	e.backspace() // no-op
	if e.dirty {
		t.Error("dirty should not be set by a no-op backspace")
	}
	if len(e.undo) != 0 {
		t.Errorf("undo should be empty (no-op must not push snapshot), got %d", len(e.undo))
	}
}
