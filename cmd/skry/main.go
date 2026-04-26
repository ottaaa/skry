package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ottaaa/skry/internal/app"
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

	p := tea.NewProgram(app.New(src.Path), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skry: %v\n", err)
		os.Exit(1)
	}
}
