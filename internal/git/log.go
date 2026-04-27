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
	commits := make([]Commit, 0, strings.Count(out, "\n")+1)
	for line := range strings.SplitSeq(out, "\n") {
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

// GraphRow is one rendered line of `git log --graph`. PureGraph rows carry no
// commit data (they show edges like `|/` or `|\`). Commit rows carry both the
// graph prefix and the parsed commit metadata so the renderer can keep them
// aligned.
type GraphRow struct {
	Graph  string  // ASCII graph prefix (e.g. "* ", "|\\ ", "| | ")
	Commit *Commit // nil for pure-graph rows
}

const graphMark = "\x1e" // RS — separates graph prefix from commit fields on commit rows

// LogGraph returns the rendered ancestry of HEAD as a list of GraphRows in
// chronological-descending order (newest first), matching `git log --graph`'s
// natural order. max <= 0 means no limit. Returns nil rows (no error) on an
// unborn branch.
func LogGraph(dir string, max int) ([]GraphRow, error) {
	args := []string{
		"log",
		"--graph",
		"--pretty=format:" + graphMark + "%H%x1f%h%x1f%an%x1f%ad%x1f%s",
		"--date=short",
	}
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
	return parseGraphOutput(out), nil
}

// parseGraphOutput splits each line at graphMark: anything before the mark is
// the graph prefix; anything after is `%H\x1f%h\x1f%an\x1f%ad\x1f%s`. Lines
// without the mark are pure-graph rows.
func parseGraphOutput(out string) []GraphRow {
	var rows []GraphRow
	for line := range strings.SplitSeq(out, "\n") {
		// Trim only the trailing CR (Windows-style); leading spaces are
		// part of the graph prefix and must be preserved.
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		idx := strings.Index(line, graphMark)
		if idx < 0 {
			rows = append(rows, GraphRow{Graph: line})
			continue
		}
		graph := line[:idx]
		rest := line[idx+len(graphMark):]
		parts := strings.Split(rest, "\x1f")
		if len(parts) < 5 {
			rows = append(rows, GraphRow{Graph: line})
			continue
		}
		c := Commit{
			Hash:    parts[0],
			Short:   parts[1],
			Author:  parts[2],
			Date:    parts[3],
			Subject: parts[4],
		}
		rows = append(rows, GraphRow{Graph: graph, Commit: &c})
	}
	return rows
}

// CommitFiles lists the files changed in a single commit (vs its first parent,
// or the full tree for a root commit).
func CommitFiles(dir, sha string) ([]string, error) {
	out, err := run(context.Background(), dir, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil {
		return nil, err
	}
	var res []string
	for l := range strings.SplitSeq(out, "\n") {
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
	b, err := FileAtBytes(dir, rev, path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FileAtBytes is the raw-bytes variant of FileAt.
func FileAtBytes(dir, rev, path string) ([]byte, error) {
	out, err := runBytes(context.Background(), dir, "show", rev+":"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not in") || strings.Contains(msg, "unknown revision") {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

// CommitMessage returns the full commit message body for `sha` — subject
// (first line) plus the rest of the body, exactly as git stores it.
func CommitMessage(dir, sha string) (string, error) {
	out, err := run(context.Background(), dir, "log", "-1", "--pretty=format:%B", sha)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// ShowNameStatus returns the files changed in `sha` paired with their
// status code (M/A/D/R...). Renames and copies report only the new path —
// the user wants to inspect the result, not the source. For a root commit
// (no parent), every file in the tree is reported as Added.
func ShowNameStatus(dir, sha string) ([]StatusEntry, error) {
	out, err := run(context.Background(), dir, "show", "--name-status", "--format=", sha)
	if err != nil {
		return nil, err
	}
	var entries []StatusEntry
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		code := fields[0]
		path := fields[len(fields)-1] // R/C have 3 fields: code, old, new
		var st Status
		switch code[0] {
		case 'M':
			st = StatusModified
		case 'A':
			st = StatusAdded
		case 'D':
			st = StatusDeleted
		case 'R':
			st = StatusRenamed
		case 'C':
			st = StatusRenamed // copy: treat like rename for display purposes
		default:
			continue
		}
		entries = append(entries, StatusEntry{Path: path, Status: st})
	}
	return entries, nil
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
