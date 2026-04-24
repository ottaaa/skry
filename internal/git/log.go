package git

import (
	"context"
	"fmt"
	"strings"
)

type Commit struct {
	Hash    string
	Short   string
	Author  string
	Date    string
	Subject string
}

// Log returns up to `max` commits of the current branch (newest first).
// max <= 0 means no limit. Returns an empty slice (no error) when the
// repository has no commits yet.
func Log(dir string, max int) ([]Commit, error) {
	args := []string{"log", "--pretty=format:%H%x1f%h%x1f%an%x1f%ad%x1f%s", "--date=short"}
	if max > 0 {
		args = append(args, fmt.Sprintf("-n%d", max))
	}
	out, err := run(context.Background(), dir, args...)
	if err != nil {
		if isUnbornBranchErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseLogOutput(out), nil
}

// isUnbornBranchErr matches "no commits yet" / "bad default revision 'HEAD'"
// style errors that surface from plumbing commands on an empty repository.
func isUnbornBranchErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not have any commits yet") ||
		strings.Contains(msg, "bad default revision") ||
		strings.Contains(msg, "unknown revision")
}

func parseLogOutput(out string) []Commit {
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Short:   parts[1],
			Author:  parts[2],
			Date:    parts[3],
			Subject: parts[4],
		})
	}
	return commits
}

// CommitFiles lists the files changed in a single commit (vs its first parent,
// or the full tree for a root commit).
func CommitFiles(dir, sha string) ([]string, error) {
	out, err := run(context.Background(), dir, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil {
		return nil, err
	}
	var res []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			res = append(res, l)
		}
	}
	return res, nil
}

// FileAt returns the contents of a file at a given revision. Returns empty
// string (no error) if the path does not exist at that revision.
func FileAt(dir, rev, path string) (string, error) {
	out, err := run(context.Background(), dir, "show", rev+":"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not in") || strings.Contains(msg, "unknown revision") {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

// ParentOf returns the first-parent SHA of rev, or empty string if rev is a
// root commit.
func ParentOf(dir, rev string) (string, error) {
	out, err := run(context.Background(), dir, "rev-parse", rev+"^")
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") ||
			strings.Contains(err.Error(), "ambiguous argument") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}
