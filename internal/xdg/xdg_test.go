package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateHomeEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/skry-test-state")
	if got := StateHome(); got != "/tmp/skry-test-state" {
		t.Errorf("StateHome() = %q, want /tmp/skry-test-state", got)
	}
}

func TestStateHomeDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no HOME resolvable on this runner")
	}
	want := filepath.Join(home, ".local", "state")
	if got := StateHome(); got != want {
		t.Errorf("StateHome() = %q, want %q", got, want)
	}
}

func TestAppStateDirCreates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir, err := AppStateDir("log")
	if err != nil {
		t.Fatalf("AppStateDir: %v", err)
	}
	want := filepath.Join(tmp, "skry", "log")
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("expected directory at %s, err=%v", dir, err)
	}
}
