// Command xwinamp-cli plays audio files from the terminal. It exists to check
// the player package on a real machine before the GUI is ready.
//
// Usage:
//
//	xwinamp-cli [-volume 0.8] file.mp3 [file2.flac ...]
//
// While playing, type a command and press Enter:
//
//	p        pause / resume
//	s        stop (rewind)
//	n        next track
//	f / b    forward / back 10 seconds
//	+ / -    volume up / down
//	q        quit
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rooty/xwinamp/player"
)

func main() {
	volume := flag.Float64("volume", 1, "initial volume, 0..1")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-volume v] file...\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	p, err := player.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audio init:", err)
		os.Exit(1)
	}
	defer p.Close()
	p.SetVolume(*volume)

	cmds := make(chan string)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			cmds <- strings.TrimSpace(sc.Text())
		}
		close(cmds)
	}()

	for _, path := range flag.Args() {
		if !playTrack(p, path, cmds) {
			return
		}
	}
}

// playTrack plays one file and returns false if the user asked to quit.
func playTrack(p *player.Player, path string, cmds <-chan string) bool {
	if err := p.Load(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}
	if err := p.Play(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}
	fmt.Printf("▶ %s [%s]\n", filepath.Base(path), formatDur(p.Duration()))

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-p.Finished():
			fmt.Println()
			return true
		case <-tick.C:
			fmt.Printf("\r%s %s / %s  vol %3.0f%%  ", p.State(),
				formatDur(p.Position()), formatDur(p.Duration()), p.Volume()*100)
		case c, ok := <-cmds:
			if !ok {
				// stdin closed (e.g. piped input): just keep playing to the end.
				cmds = nil
				continue
			}
			switch c {
			case "p":
				p.TogglePause()
			case "s":
				p.Stop()
			case "n":
				fmt.Println()
				return true
			case "f":
				p.Seek(p.Position() + 10*time.Second)
			case "b":
				p.Seek(p.Position() - 10*time.Second)
			case "+":
				p.SetVolume(p.Volume() + 0.1)
			case "-":
				p.SetVolume(p.Volume() - 0.1)
			case "q":
				fmt.Println()
				return false
			}
		}
	}
}

func formatDur(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
