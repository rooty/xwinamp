package ui

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/rooty/xwinamp/player"
)

// Player is the part of *player.Player the window uses.
type Player interface {
	Load(path string) error
	Play() error
	Pause()
	Stop() error
	Seek(d time.Duration) error
	Position() time.Duration
	Duration() time.Duration
	State() player.State
	Path() string
	Volume() float64
	SetVolume(v float64)
	Finished() <-chan struct{}
}

// Action is a command from a button or a hotkey.
type Action int

const (
	ActPrev Action = iota
	ActPlay
	ActPause
	ActStop
	ActNext
	ActOpen
	ActSeekBack
	ActSeekForward
	ActVolumeUp
	ActVolumeDown
)

const (
	seekStep   = 5 * time.Second
	volumeStep = 0.05
	// marqueeSpeed is how many ticks one pixel of scrolling takes.
	marqueeSpeed = 3
)

var buttonActions = map[Button]Action{
	BtnPrev:  ActPrev,
	BtnPlay:  ActPlay,
	BtnPause: ActPause,
	BtnStop:  ActStop,
	BtnNext:  ActNext,
	BtnOpen:  ActOpen,
}

// drag is what the mouse is currently holding.
type drag int

const (
	dragNone drag = iota
	dragButton
	dragSeek
	dragVolume
)

// Controller turns input into player calls and keeps what the view shows.
// It is not safe for concurrent use; Window calls it from the game loop only.
type Controller struct {
	p Player

	drag     drag
	pressed  Button  // button held down, if drag == dragButton
	seekFrac float64 // seek bar position while dragging it

	showRemaining bool
	ticks         int
	message       string // overrides the track title, e.g. an error
}

// NewController returns a controller for p.
func NewController(p Player) *Controller {
	return &Controller{p: p}
}

// Open loads the first supported file in paths and starts playing it.
// Later files are ignored until the playlist exists.
func (c *Controller) Open(paths []string) {
	for _, path := range paths {
		if !player.IsSupported(path) {
			continue
		}
		if err := c.p.Load(path); err != nil {
			c.message = "error: " + err.Error()
			return
		}
		c.message = ""
		c.ticks = 0
		if err := c.p.Play(); err != nil {
			c.message = "error: " + err.Error()
		}
		return
	}
	if len(paths) > 0 {
		c.message = "unsupported file: " + filepath.Base(paths[0])
	}
}

// Do runs an action.
func (c *Controller) Do(a Action) {
	switch a {
	case ActPrev:
		// No playlist yet: restart the current track.
		c.p.Seek(0)
	case ActPlay:
		if c.p.State() == player.Playing {
			c.p.Seek(0) // Winamp restarts the track on Play while playing.
			return
		}
		c.reportErr(c.p.Play())
	case ActPause:
		switch c.p.State() {
		case player.Playing:
			c.p.Pause()
		case player.Paused:
			c.reportErr(c.p.Play())
		}
	case ActStop:
		c.p.Stop()
	case ActNext:
		// No playlist yet: nothing to skip to.
	case ActOpen:
		c.message = "drop audio files onto the window"
		c.ticks = 0
	case ActSeekBack:
		c.p.Seek(c.p.Position() - seekStep)
	case ActSeekForward:
		c.p.Seek(c.p.Position() + seekStep)
	case ActVolumeUp:
		c.p.SetVolume(c.p.Volume() + volumeStep)
	case ActVolumeDown:
		c.p.SetVolume(c.p.Volume() - volumeStep)
	}
}

func (c *Controller) reportErr(err error) {
	if errors.Is(err, player.ErrNoTrack) {
		c.message = "no file: drop audio files onto the window"
		c.ticks = 0
	} else if err != nil {
		c.message = "error: " + err.Error()
	}
}

// MouseDown starts a click or a drag at p (logical coordinates).
func (c *Controller) MouseDown(p image.Point) {
	switch {
	case buttonAt(p) != NoButton:
		c.drag, c.pressed = dragButton, buttonAt(p)
	case p.In(seekRect.Inset(-2)) && c.p.Duration() > 0:
		c.drag, c.seekFrac = dragSeek, sliderFrac(seekRect, p.X)
	case p.In(volumeRect.Inset(-2)):
		c.drag = dragVolume
		c.p.SetVolume(sliderFrac(volumeRect, p.X))
	case p.In(timeRect):
		c.showRemaining = !c.showRemaining
	}
}

// MouseMove updates a drag in progress.
func (c *Controller) MouseMove(p image.Point) {
	switch c.drag {
	case dragSeek:
		c.seekFrac = sliderFrac(seekRect, p.X)
	case dragVolume:
		c.p.SetVolume(sliderFrac(volumeRect, p.X))
	}
}

// MouseUp finishes a click or drag at p.
func (c *Controller) MouseUp(p image.Point) {
	switch c.drag {
	case dragButton:
		// Like any button: fires only if released over the same button.
		if buttonAt(p) == c.pressed {
			c.Do(buttonActions[c.pressed])
		}
	case dragSeek:
		c.seekFrac = sliderFrac(seekRect, p.X)
		c.p.Seek(time.Duration(c.seekFrac * float64(c.p.Duration())))
	}
	c.drag, c.pressed = dragNone, NoButton
}

// Tick advances animations; call once per game update.
func (c *Controller) Tick() {
	c.ticks++
	select {
	case <-c.p.Finished():
		// No playlist yet: stay stopped at the end.
	default:
	}
}

// View is everything the window needs to draw one frame.
type View struct {
	State    player.State
	Time     string  // "m:ss" or "-m:ss" when showing remaining time
	Title    string  // marquee text
	Scroll   int     // marquee offset in pixels
	Info     string  // second display line
	Seek     float64 // seek knob position in [0, 1]
	HasTrack bool
	Volume   float64
	Pressed  Button
}

// View returns the current view state.
func (c *Controller) View() View {
	pos, dur := c.p.Position(), c.p.Duration()
	v := View{
		State:    c.p.State(),
		HasTrack: dur > 0,
		Volume:   c.p.Volume(),
		Scroll:   c.ticks / marqueeSpeed,
	}
	if c.drag == dragButton {
		v.Pressed = c.pressed
	}

	switch {
	case c.drag == dragSeek:
		v.Seek = c.seekFrac
		pos = time.Duration(c.seekFrac * float64(dur))
	case dur > 0:
		v.Seek = float64(pos) / float64(dur)
	}

	if c.showRemaining && dur > 0 {
		v.Time = "-" + formatTime(dur-pos)
	} else {
		v.Time = formatTime(pos)
	}

	switch {
	case c.message != "":
		v.Title = c.message
	case c.p.Path() != "":
		name := filepath.Base(c.p.Path())
		v.Title = fmt.Sprintf("%s (%s)", strings.TrimSuffix(name, filepath.Ext(name)), formatTime(dur))
	default:
		v.Title = "xWinAmp - drop audio files here"
	}

	if c.drag == dragVolume {
		v.Info = fmt.Sprintf("volume %d%%", int(v.Volume*100+0.5))
	} else if c.p.Path() != "" {
		v.Info = strings.ToUpper(strings.TrimPrefix(filepath.Ext(c.p.Path()), ".")) + "  " + v.State.String()
	}
	return v
}

func formatTime(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
