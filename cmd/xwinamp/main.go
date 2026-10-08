// Command xwinamp is the xWinAmp desktop player.
//
// Usage:
//
//	xwinamp [-scale 2] [file]
//
// Files can also be dropped onto the window.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rooty/xwinamp/player"
	"github.com/rooty/xwinamp/ui"
)

func main() {
	scale := flag.Int("scale", 2, "window scale, 1..4")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-scale n] [file...]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	p, err := player.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audio init:", err)
		os.Exit(1)
	}
	defer p.Close()

	c := ui.NewController(p)
	if flag.NArg() > 0 {
		c.Open(flag.Args())
	}
	if err := ui.NewWindow(c).Run(max(1, min(*scale, 4))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
