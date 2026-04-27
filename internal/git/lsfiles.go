package git

import (
	"context"
	"strings"
)

// ListFiles returns tracked + non-ignored untracked files relative to dir.
func ListFiles(dir string) ([]string, error) {
	out, err := run(context.Background(), dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	raw := strings.TrimRight(out, "\x00")
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\x00"), nil
}

// ListIgnoredFiles returns untracked files that .gitignore (or other standard
// excludes) would normally hide. Empty when there are none. Does NOT include
// files that are already covered by ListFiles.
func ListIgnoredFiles(dir string) ([]string, error) {
	out, err := run(context.Background(), dir, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	raw := strings.TrimRight(out, "\x00")
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\x00"), nil
}
