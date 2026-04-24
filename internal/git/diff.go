package git

import (
	"context"
	"strings"
)

// HeadFile returns file contents at HEAD. Returns empty string without error
// when the path does not exist at HEAD (e.g. newly added or untracked files).
func HeadFile(dir, path string) (string, error) {
	b, err := HeadFileBytes(dir, path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HeadFileBytes is the raw-bytes variant of HeadFile. Callers that need to
// inspect binary content should use this to avoid UTF-8 round-tripping.
func HeadFileBytes(dir, path string) ([]byte, error) {
	out, err := runBytes(context.Background(), dir, "show", "HEAD:"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not in") || strings.Contains(msg, "unknown revision") {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}
