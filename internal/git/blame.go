package git

import (
	"context"
	"strings"
	"time"
)

type BlameLine struct {
	Hash    string
	Short   string
	Author  string
	Date    string
	Summary string
	Line    int // 1-based, in the current file
	Text    string
}

// Blame runs `git blame --porcelain <path>` and returns one entry per line.
// `rev` may be empty (blame the working tree) or a revision like "HEAD".
func Blame(dir, rev, path string) ([]BlameLine, error) {
	args := []string{"blame", "--porcelain"}
	if rev != "" {
		args = append(args, rev)
	}
	args = append(args, "--", path)
	out, err := run(context.Background(), dir, args...)
	if err != nil {
		if isUnbornBranchErr(err) || strings.Contains(err.Error(), "no such path") {
			return nil, nil
		}
		return nil, err
	}
	return parseBlameOutput(out), nil
}

func parseBlameOutput(out string) []BlameLine {
	type meta struct {
		author  string
		when    string
		summary string
	}
	commitMeta := map[string]*meta{}
	var lines []BlameLine

	scanner := strings.Split(out, "\n")
	var cur BlameLine
	var curCommit *meta
	for i := 0; i < len(scanner); i++ {
		line := scanner[i]
		if line == "" {
			continue
		}
		// Header: "<40 hex> <orig line> <final line> [<num lines>]"
		if isHashLine(line) {
			fields := strings.Fields(line)
			cur = BlameLine{Hash: fields[0]}
			if len(fields[0]) >= 8 {
				cur.Short = fields[0][:8]
			}
			if len(fields) >= 3 {
				cur.Line = atoi(fields[2])
			}
			if m, ok := commitMeta[cur.Hash]; ok {
				curCommit = m
			} else {
				curCommit = &meta{}
				commitMeta[cur.Hash] = curCommit
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "author "):
			curCommit.author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-time "):
			curCommit.when = formatUnixDate(strings.TrimPrefix(line, "author-time "))
		case strings.HasPrefix(line, "summary "):
			curCommit.summary = strings.TrimPrefix(line, "summary ")
		case strings.HasPrefix(line, "\t"):
			cur.Author = curCommit.author
			cur.Date = curCommit.when
			cur.Summary = curCommit.summary
			cur.Text = strings.TrimPrefix(line, "\t")
			lines = append(lines, cur)
		}
	}
	return lines
}

func isHashLine(line string) bool {
	if len(line) < 42 {
		return false
	}
	if line[40] != ' ' {
		return false
	}
	for i := 0; i < 40; i++ {
		c := line[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func formatUnixDate(s string) string {
	secs := int64(atoi(s))
	if secs == 0 {
		return ""
	}
	return time.Unix(secs, 0).Format("2006-01-02")
}
