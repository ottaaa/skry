package git

import (
	"context"
	"strings"
)

type Worktree struct {
	Path     string
	Branch   string
	Head     string
	Bare     bool
	Detached bool
}

func Worktrees(dir string) ([]Worktree, error) {
	out, err := run(context.Background(), dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreesOutput(out), nil
}

func parseWorktreesOutput(out string) []Worktree {
	var res []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			res = append(res, cur)
		}
		cur = Worktree{}
	}
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "bare":
			cur.Bare = true
		case line == "detached":
			cur.Detached = true
		}
	}
	flush()
	return res
}
