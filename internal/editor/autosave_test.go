package editor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg builds a tea.KeyMsg for a single rune. Ensures we exercise the
// same path as a real keypress in handleKey (rune branch).
func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestAutosaveVersionBumpsOnInsert(t *testing.T) {
	e := loadTempFile(t, "hello")
	before := e.autosaveVersion
	e.col = 5
	e.insertRune('!')
	if e.autosaveVersion != before+1 {
		t.Errorf("insertRune should bump autosaveVersion by 1: before=%d after=%d", before, e.autosaveVersion)
	}
}

func TestAutosaveVersionBumpsOnBackspace(t *testing.T) {
	e := loadTempFile(t, "hello")
	before := e.autosaveVersion
	e.col = 5
	e.backspace()
	if e.autosaveVersion != before+1 {
		t.Errorf("backspace should bump autosaveVersion: before=%d after=%d", before, e.autosaveVersion)
	}
}

func TestAutosaveVersionDoesNotBumpOnNoOpBackspace(t *testing.T) {
	e := loadTempFile(t, "hello")
	before := e.autosaveVersion
	// Backspace at row 0 col 0 is a no-op — must not bump.
	e.row, e.col = 0, 0
	e.backspace()
	if e.autosaveVersion != before {
		t.Errorf("no-op backspace should not bump autosaveVersion: before=%d after=%d", before, e.autosaveVersion)
	}
}

func TestAutosaveVersionBumpsOnUndoRedo(t *testing.T) {
	e := loadTempFile(t, "hello")
	e.col = 5
	e.insertRune('!') // bumps once
	v1 := e.autosaveVersion
	e.Undo()
	if e.autosaveVersion <= v1 {
		t.Errorf("Undo should bump autosaveVersion: was=%d after=%d", v1, e.autosaveVersion)
	}
	v2 := e.autosaveVersion
	e.Redo()
	if e.autosaveVersion <= v2 {
		t.Errorf("Redo should bump autosaveVersion: was=%d after=%d", v2, e.autosaveVersion)
	}
}

func TestAutosaveUpdateReturnsTickOnEdit(t *testing.T) {
	e := loadTempFile(t, "hello")
	e.col = 5
	cmd := e.Update(keyMsg('!'))
	if cmd == nil {
		t.Fatal("Update should return a Tick Cmd after a content-changing keystroke")
	}
}

func TestAutosaveUpdateReturnsNilOnMotion(t *testing.T) {
	e := loadTempFile(t, "hello")
	e.col = 5
	cmd := e.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if cmd != nil {
		t.Errorf("pure motion (←) should not arm autosave")
	}
}

// dispatchTick simulates the Bubble Tea program loop sending an
// AutosaveTickMsg back into editor.Model.Update. Returns the resulting
// (model, cmd, msg-from-cmd) so tests can assert on each.
func dispatchTick(t *testing.T, m Model, version int) (Model, tea.Msg) {
	t.Helper()
	newM, cmd := m.Update(AutosaveTickMsg{Version: version})
	if cmd == nil {
		return newM, nil
	}
	return newM, cmd()
}

// editorOnFile returns a Model rooted at the temp dir holding `name` with
// `content`, already in ModeEdit. The chroma extension hint comes from
// `name`.
func editorOnFile(t *testing.T, name, content string) Model {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	m.SetSize(80, 24)
	if err := m.Open(name, ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.edit.Load(abs, name, content); err != nil {
		t.Fatalf("edit.Load: %v", err)
	}
	m.mode = ModeEdit
	return m
}

func TestAutosaveTickWithMatchingVersionWritesFile(t *testing.T) {
	m := editorOnFile(t, "f.txt", "hello")
	m.edit.col = 5
	m.edit.insertRune('!') // bumps to version 1, sets dirty=true
	v := m.edit.autosaveVersion

	m, msg := dispatchTick(t, m, v)
	auto, ok := msg.(AutoSavedMsg)
	if !ok {
		t.Fatalf("expected AutoSavedMsg, got %T", msg)
	}
	if auto.Err != nil {
		t.Errorf("autosave should succeed, got err=%v", auto.Err)
	}
	if m.edit.dirty {
		t.Errorf("dirty should clear after successful autosave")
	}
	// Disk content should reflect the new buffer.
	got, _ := os.ReadFile(m.absPath)
	if string(got) != "hello!" {
		t.Errorf("on-disk = %q, want %q", string(got), "hello!")
	}
}

func TestAutosaveTickWithStaleVersionIsNoop(t *testing.T) {
	m := editorOnFile(t, "f.txt", "hello")
	m.edit.col = 5
	m.edit.insertRune('!')
	staleV := m.edit.autosaveVersion
	m.edit.insertRune('?') // bumps version, supersedes the earlier tick

	_, msg := dispatchTick(t, m, staleV)
	if msg != nil {
		t.Errorf("stale tick must be a no-op (no AutoSavedMsg), got %T: %+v", msg, msg)
	}
	// The buffer is still dirty waiting for its own (current-version) tick.
	if !m.edit.dirty {
		t.Errorf("dirty should remain true: stale tick must not save")
	}
}

func TestAutosaveTickWhenCleanIsNoop(t *testing.T) {
	m := editorOnFile(t, "f.txt", "hello")
	v := m.edit.autosaveVersion
	// dirty stays false; even if a tick somehow fires with this version
	// the autosave must not redundantly write.
	_, msg := dispatchTick(t, m, v)
	if msg != nil {
		t.Errorf("clean buffer must not autosave, got %+v", msg)
	}
}

func TestAutosaveTickOutsideEditModeIsNoop(t *testing.T) {
	m := editorOnFile(t, "f.txt", "hello")
	m.edit.col = 5
	m.edit.insertRune('!')
	v := m.edit.autosaveVersion
	m.mode = ModeView // user already left Edit

	_, msg := dispatchTick(t, m, v)
	if msg != nil {
		t.Errorf("tick outside ModeEdit must be a no-op, got %+v", msg)
	}
}

// TestAutosaveDelay sanity-checks the configured delay is in the right
// ballpark (1.5s as per Q2). This is mostly a guard against accidental
// edits to the constant in unrelated PRs.
func TestAutosaveDelay(t *testing.T) {
	if AutosaveDelay < 500*time.Millisecond || AutosaveDelay > 5*time.Second {
		t.Errorf("AutosaveDelay = %v, expected 500ms..5s", AutosaveDelay)
	}
}

func TestEscFlushesSave(t *testing.T) {
	m := editorOnFile(t, "f.txt", "hello")
	m.edit.col = 5
	m.edit.insertRune('!')
	if !m.edit.dirty {
		t.Fatal("buffer should be dirty before Esc")
	}
	// Drive the Esc through the public Update path (the real Bubble Tea
	// flow) so we exercise the same handler as a real keypress.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if newM.mode != ModeView {
		t.Errorf("Esc should transition to ModeView, got %v", newM.mode)
	}
	if newM.edit.dirty {
		t.Errorf("Esc must flush save, leaving dirty=false")
	}
	got, _ := os.ReadFile(newM.absPath)
	if string(got) != "hello!" {
		t.Errorf("file should contain flushed content %q, got %q", "hello!", string(got))
	}
}

func TestOpenFlushesPreviousEditBuffer(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("AAA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("BBB"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(dir)
	m.SetSize(80, 24)
	if err := m.Open("a.txt", ""); err != nil {
		t.Fatal(err)
	}
	if err := m.edit.Load(a, "a.txt", "AAA"); err != nil {
		t.Fatal(err)
	}
	m.mode = ModeEdit
	m.edit.col = 3
	m.edit.insertRune('!') // dirty edit on a.txt

	// Switching files via Open must flush a's buffer to disk first.
	if err := m.Open("b.txt", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(a)
	if string(got) != "AAA!" {
		t.Errorf("a.txt should have been flushed before opening b: got %q, want %q", string(got), "AAA!")
	}
}
