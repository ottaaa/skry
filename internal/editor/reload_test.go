package editor

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReloadPreservesScrollWhenContentUnchanged(t *testing.T) {
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

	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < 50; i++ {
		m, _ = m.Update(jKey)
	}
	scrollBefore := m.view.vp.YOffset
	if scrollBefore == 0 {
		t.Fatalf("expected non-zero scroll before reload")
	}

	m.Reload()
	if m.view.vp.YOffset != scrollBefore {
		t.Errorf("Reload with unchanged content reset scroll: was %d, now %d", scrollBefore, m.view.vp.YOffset)
	}

	// Simulate spurious watcher events.
	for i := 0; i < 10; i++ {
		m.Reload()
	}
	if m.view.vp.YOffset != scrollBefore {
		t.Errorf("repeated spurious Reloads reset scroll: was %d, now %d", scrollBefore, m.view.vp.YOffset)
	}
}

func TestReloadPreservesScrollOnRealChange(t *testing.T) {
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

	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < 50; i++ {
		m, _ = m.Update(jKey)
	}
	scrollBefore := m.view.vp.YOffset

	// Simulate external append.
	if err := os.WriteFile(abs, []byte(makeNumberedLines(220)), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Reload()
	if m.view.vp.YOffset != scrollBefore {
		t.Errorf("Reload with append did not preserve scroll: was %d, now %d", scrollBefore, m.view.vp.YOffset)
	}
}
