package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ottaaa/skry/internal/app"
	"github.com/ottaaa/skry/internal/git"
	"github.com/ottaaa/skry/internal/source"
	"github.com/ottaaa/skry/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: skry [flags] [path|-]\n\n"+
				"  path  open the given git repository (default: current directory)\n"+
				"  -     read content from stdin and open it as a single-file repo\n\n"+
				"Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	arg := ""
	if flag.NArg() > 0 {
		arg = flag.Arg(0)
	}

	ctx := context.Background()
	src, err := source.FromArg(ctx, arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skry: %v\n", err)
		os.Exit(1)
	}
	defer src.Cleanup()

	// Bubble Tea reads keystrokes from os.Stdin by default. When the user
	// piped content via `skry -` we've already drained stdin in
	// source.FromArg, so the program needs /dev/tty to receive input.
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if arg == "-" {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Fprintf(os.Stderr, "skry: open /dev/tty: %v\n", err)
			os.Exit(1)
		}
		defer tty.Close()
		opts = append(opts, tea.WithInput(tty))
	}

	// If src.Path is a subdirectory of a git repo, scope the tree to it: the
	// app uses repoRoot (= git toplevel) for git operations but only shows
	// files beneath scopeDir. When src.Path *is* the toplevel (or git can't
	// resolve one — e.g. stdin's freshly-init'd repo), scopeDir stays empty.
	repoRoot := src.Path
	scopeDir := ""
	if top, err := git.TopLevel(src.Path); err == nil && top != "" {
		topAbs, _ := filepath.Abs(top)
		if topAbs != "" && topAbs != src.Path {
			if rel, err := filepath.Rel(topAbs, src.Path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				repoRoot = topAbs
				scopeDir = filepath.ToSlash(rel)
			}
		}
	}

	p := tea.NewProgram(app.New(repoRoot, scopeDir), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skry: %v\n", err)
		os.Exit(1)
	}
}
