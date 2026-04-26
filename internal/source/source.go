// Package source resolves what skry should display from CLI invocation:
// either an existing git repository on disk, or a stream piped over
// stdin. For stdin we materialize a temp directory, init a git repo, and
// commit the bytes as a single file so the rest of skry sees a normal
// repo-rooted view.
//
// The unified-diff special case (git diff | skry -) is intentionally not
// detected here yet — it would need a unified-diff → SplitDiff renderer
// and is tracked as a follow-up. For now, piping `git diff` produces a
// plain text View of the diff output, which is still useful for review.
package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Resolved is what main passes to app.New: an absolute path to a
// directory that contains a git repo skry can browse.
type Resolved struct {
	Path string
	// Cleanup runs when skry exits; for stdin sources this removes the
	// temp directory. Always non-nil so callers can `defer r.Cleanup()`.
	Cleanup func()
}

// FromArg interprets the command-line positional argument. "" or "."
// means cwd; "-" means read stdin and produce a temp repo holding it;
// any other value is taken as a directory path on disk.
func FromArg(ctx context.Context, arg string) (*Resolved, error) {
	if arg == "-" {
		return fromStdin(ctx)
	}
	if arg == "" {
		arg = "."
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return nil, fmt.Errorf("source: resolve path %q: %w", arg, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("source: stat %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source: %q is not a directory", abs)
	}
	return &Resolved{Path: abs, Cleanup: func() {}}, nil
}

// fromStdin reads all of stdin, picks a sensible filename based on the
// content prefix, materializes a single-file git repo in $TMPDIR, and
// returns its path.
func fromStdin(ctx context.Context) (*Resolved, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("source: read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("source: stdin is empty")
	}

	dir, err := os.MkdirTemp("", "skry-stdin-*")
	if err != nil {
		return nil, fmt.Errorf("source: mkdtemp: %w", err)
	}

	name := pickStdinFilename(data)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("source: write %q: %w", name, err)
	}
	if err := initRepoWithFile(ctx, dir, name); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	return &Resolved{
		Path:    dir,
		Cleanup: func() { _ = os.RemoveAll(dir) },
	}, nil
}

// pickStdinFilename inspects the first few bytes to choose a useful
// extension. Picking a known extension lets chroma syntax-highlight the
// content correctly in View mode (e.g. `.diff` for diff output).
func pickStdinFilename(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("diff --git ")) ||
		bytes.HasPrefix(data, []byte("--- ")):
		return "stdin.diff"
	case bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")):
		return "stdin.json"
	case bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")):
		return "stdin.xml"
	default:
		return "stdin.txt"
	}
}

// initRepoWithFile runs `git init && git add file && git commit` inside
// dir so the materialized stdin shows up as a clean tracked file. Errors
// surface verbatim — the caller wraps with the user-facing message.
func initRepoWithFile(ctx context.Context, dir, file string) error {
	steps := [][]string{
		{"init", "-q", "-b", "main"},
		// Override identity so this works even if the user has no global
		// git config; per-invocation values are scoped to this repo.
		{"-c", "user.email=skry@stdin", "-c", "user.name=skry", "add", file},
		{"-c", "user.email=skry@stdin", "-c", "user.name=skry", "commit", "-q", "-m", "stdin"},
	}
	for _, args := range steps {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("source: git %v: %w (%s)", args, err, bytes.TrimSpace(out))
		}
	}
	return nil
}
