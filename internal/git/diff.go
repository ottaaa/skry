package git

import (
	"context"
	"strings"
)

// HeadFile returns file contents at HEAD. Returns empty string without error
// when the path does not exist at HEAD (e.g. newly added or untracked files).
func HeadFile(dir, path string) (string, error) {
	out, err := run(context.Background(), dir, "show", "HEAD:"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not in") || strings.Contains(msg, "unknown revision") {
			return "", nil
		}
		return "", err
	}
	return out, nil
}
