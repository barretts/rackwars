package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"rackwars/internal/save"
	"rackwars/internal/tui"
)

// version is replaced by release builds with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

const usage = `RACK WARS - a GPU trading game for your terminal.

Usage:
  rackwars [flags]

Flags:
  --seed N     deal a specific market (default 1). The same seed always deals the
               same market, so a bug report can be re-created exactly. A finished
               run shows its seed and offers to replay it.
  --prototype  play the three-town trading prototype: the phase-3 economy with no
               hosting, allocation, or checkpoints. The default game is the full
               five-market base game.
  --save-dir PATH  use this folder instead of the user configuration directory
  --no-color   draw without color (NO_COLOR does the same)
  --ascii      draw with ASCII glyphs only
  --help       show this help
  --version    print the version and exit

Environment:
  NO_COLOR=1          same as --no-color
  RACKWARS_DEBUG=path append engine and terminal diagnostics to a log file
`

func main() {
	fs := flag.NewFlagSet("rackwars", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), usage) }
	noColor := fs.Bool("no-color", false, "draw without color")
	ascii := fs.Bool("ascii", false, "draw with ASCII glyphs only")
	saveDir := fs.String("save-dir", "", "save folder override")
	seed := fs.Uint64("seed", 1, "deal a specific market")
	prototype := fs.Bool("prototype", false, "play the three-town trading prototype instead of the full game")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("rackwars " + version)
		return
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "rackwars: unexpected argument %q\n\n%s", fs.Arg(0), usage)
		os.Exit(2)
	}

	// A game needs a keyboard and a screen. Without one, say so briefly and
	// leave the terminal alone instead of starting a broken full screen.
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr,
			"RACK WARS needs an interactive terminal. Run it from Terminal.app, iTerm2, "+
				"Windows Terminal, or PowerShell.")
		os.Exit(1)
	}

	if path := os.Getenv("RACKWARS_DEBUG"); path != "" {
		if f, err := tea.LogToFile(path, "rackwars"); err != nil {
			fmt.Fprintln(os.Stderr, "rackwars: cannot open RACKWARS_DEBUG log:", err)
			os.Exit(1)
		} else {
			defer f.Close()
		}
	}

	colorOn := !*noColor && os.Getenv("NO_COLOR") == ""
	chrome := tui.NewChrome(colorOn, *ascii)
	newGame := tui.NewFullRun
	if *prototype {
		newGame = tui.NewRun
	}
	model, err := newGame(chrome, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rackwars: cannot start a run:", err)
		os.Exit(1)
	}
	dir, err := save.Dir(*saveDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rackwars:", err)
		os.Exit(1)
	}
	store, err := save.NewStore(dir, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rackwars:", err)
		os.Exit(1)
	}
	model = model.StartPersistence(tui.SaveTarget{Store: store, RunID: fmt.Sprintf("seed-%d", *seed)})
	final, err := tea.NewProgram(model).Run()
	if ended, ok := final.(tui.Model); ok {
		ended.Close()
		if note := ended.QuitNote(); note != "" {
			fmt.Println(note)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rackwars:", err)
		os.Exit(1)
	}
}
