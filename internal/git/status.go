package git

import (
	"context"
	"strings"
)

type Status string

const (
	StatusNone      Status = ""
	StatusModified  Status = "M"
	StatusAdded     Status = "A"
	StatusDeleted   Status = "D"
	StatusRenamed   Status = "R"
	StatusUntracked Status = "?"
)

type StatusEntry struct {
	Path   string
	Status Status
}

func Statuses(dir string) ([]StatusEntry, error) {
	out, err := run(context.Background(), dir, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	raw := strings.TrimRight(out, "\x00")
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "\x00")
	var entries []StatusEntry
	for i := 0; i < len(parts); i++ {
		rec := parts[i]
		if len(rec) < 4 {
			continue
		}
		x := rec[0]
		y := rec[1]
		path := rec[3:]
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			// -z rename format stores the from-path in the next NUL token
			i++
		}
		entries = append(entries, StatusEntry{Path: path, Status: decode(x, y)})
	}
	return entries, nil
}

func decode(x, y byte) Status {
	if x == '?' && y == '?' {
		return StatusUntracked
	}
	switch {
	case x == 'R' || y == 'R':
		return StatusRenamed
	case x == 'D' || y == 'D':
		return StatusDeleted
	case x == 'A' || y == 'A':
		return StatusAdded
	case x == 'M' || y == 'M':
		return StatusModified
	}
	return StatusNone
}
