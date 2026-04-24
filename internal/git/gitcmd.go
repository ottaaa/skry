package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func run(ctx context.Context, dir string, args ...string) (string, error) {
	b, err := runBytes(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func runBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func TopLevel(dir string) (string, error) {
	out, err := run(context.Background(), dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CurrentBranch(dir string) (string, error) {
	out, err := run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
