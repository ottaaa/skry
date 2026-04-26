package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWatcherEmitsOnTargetWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(target, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fw, err := StartFile(nil)
	if err != nil {
		t.Fatalf("StartFile: %v", err)
	}
	t.Cleanup(func() { _ = fw.Close() })
	fw.Watch(target)

	if err := os.WriteFile(target, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fw.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("expected an event after writing the watched file")
	}
}

func TestFileWatcherIgnoresSiblings(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "watched.txt")
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(target, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fw, err := StartFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fw.Close() })
	fw.Watch(target)

	// Writing a sibling in the same directory must not produce an event.
	if err := os.WriteFile(other, []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fw.Events():
		t.Fatal("unexpected event for a sibling file")
	case <-time.After(300 * time.Millisecond):
		// Good: silence.
	}
}

func TestFileWatcherSwapsTarget(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := StartFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fw.Close() })

	fw.Watch(a)
	fw.Watch(b) // swap before writing

	// Writing a (the old target) must not fire.
	if err := os.WriteFile(a, []byte("changed-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Writing b (the new target) must fire.
	if err := os.WriteFile(b, []byte("changed-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fw.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("expected an event for the new target")
	}
}

func TestFileWatcherEmptyPathSilencesEvents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(target, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fw, err := StartFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fw.Close() })

	fw.Watch(target)
	fw.Watch("") // disarm

	if err := os.WriteFile(target, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fw.Events():
		t.Fatal("Watch(\"\") must silence events")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestFileWatcherCloseIsIdempotent(t *testing.T) {
	fw, err := StartFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Errorf("second close should be a no-op, got %v", err)
	}
}
