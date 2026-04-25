package git

import (
	"context"
	"strings"
)

type Branch struct {
	Name    string
	Current bool
}

func Branches(dir string) ([]Branch, error) {
	out, err := run(context.Background(), dir, "branch", "--list", "--format=%(HEAD)%00%(refname:short)")
	if err != nil {
		return nil, err
	}
	return parseBranchesOutput(out), nil
}

func parseBranchesOutput(out string) []Branch {
	var res []Branch
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		res = append(res, Branch{
			Current: strings.TrimSpace(parts[0]) == "*",
			Name:    strings.TrimSpace(parts[1]),
		})
	}
	return res
}

// Switch runs `git switch <name>`. Callers should check WorkingDirty first
// unless they mean to let git's own protection fail the switch.
func Switch(dir, name string) error {
	_, err := run(context.Background(), dir, "switch", name)
	return err
}

// SwitchForce discards uncommitted changes (`git switch --discard-changes`).
func SwitchForce(dir, name string) error {
	_, err := run(context.Background(), dir, "switch", "--discard-changes", name)
	return err
}

// WorkingDirty reports whether the working tree has unstaged or untracked
// (non-ignored) changes.
func WorkingDirty(dir string) (bool, error) {
	out, err := run(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}
