// Package xdg is a tiny XDG Base Directory helper. skry only needs a state
// directory (for the rotating log file), so we intentionally do not expose
// the full XDG surface — just what callers actually use.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// StateHome returns $XDG_STATE_HOME if set, else the spec default
// "$HOME/.local/state". If $HOME cannot be resolved, falls back to "state"
// relative to the current directory (better than crashing on a container
// with no env vars set).
func StateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "state"
	}
	return filepath.Join(home, ".local", "state")
}

// AppStateDir returns $XDG_STATE_HOME/skry/<sub...>, creating the directory
// (0o700) if it does not exist. Returns the resolved path plus any mkdir
// error; callers should fall back to stderr logging when this fails.
func AppStateDir(sub ...string) (string, error) {
	parts := append([]string{StateHome(), "skry"}, sub...)
	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return dir, fmt.Errorf("xdg: mkdir %q: %w", dir, err)
	}
	return dir, nil
}
