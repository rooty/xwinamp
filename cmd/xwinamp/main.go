// Command xwinamp is the xWinAmp desktop player.
//
// Usage:
//
//	xwinamp [-scale 2] [file or folder...]
//
// Files and folders can also be dropped onto the window. The playlist and
// settings are restored from the previous run.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rooty/xwinamp/player"
	"github.com/rooty/xwinamp/playlist"
	"github.com/rooty/xwinamp/ui"
)

func main() {
	scale := flag.Int("scale", 2, "window scale, 1..4")
	fresh := flag.Bool("fresh", false, "start with an empty playlist and default settings")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-scale n] [-fresh] [file or folder...]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	p, err := player.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audio init:", err)
		os.Exit(1)
	}
	defer p.Close()

	pl := playlist.New()
	c := ui.NewController(p, pl, ui.SystemDialogs{})

	dir, dirErr := stateDir()
	if dirErr != nil {
		fmt.Fprintln(os.Stderr, "settings will not be saved:", dirErr)
	}
	if dirErr == nil && !*fresh {
		if err := loadState(dir, c, p); err != nil {
			fmt.Fprintln(os.Stderr, "restore previous session:", err)
		}
	}

	c.ProbeDurations()
	if flag.NArg() > 0 {
		c.Open(flag.Args())
	}
	runErr := ui.NewWindow(c).Run(max(1, min(*scale, 4)))

	if dirErr == nil {
		if err := saveState(dir, c, p); err != nil {
			fmt.Fprintln(os.Stderr, "save session:", err)
		}
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}
