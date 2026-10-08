package ui

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/rooty/xwinamp/player"
	"github.com/rooty/xwinamp/playlist"
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

// Dialogs shows system file dialogs. Each method blocks until the user
// answers and returns empty results, not an error, on cancel.
type Dialogs interface {
	OpenFiles() ([]string, error)
	OpenPlaylist() (string, error)
	SavePlaylist() (string, error)
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
	ActShuffle
	ActRepeat
	ActTogglePlaylist
	ActRemove
	ActPlaySelected
	ActLoadPlaylist
	ActSavePlaylist
)

const (
	seekStep   = 5 * time.Second
	volumeStep = 0.05
	// marqueeSpeed is how many ticks one pixel of scrolling takes.
	marqueeSpeed = 3
	// doubleClickTicks is the longest gap between the clicks of a double click.
	doubleClickTicks = 15
)

var buttonActions = map[Button]Action{
	BtnPrev:     ActPrev,
	BtnPlay:     ActPlay,
	BtnPause:    ActPause,
	BtnStop:     ActStop,
	BtnNext:     ActNext,
	BtnOpen:     ActOpen,
	BtnShuffle:  ActShuffle,
	BtnRepeat:   ActRepeat,
	BtnPlaylist: ActTogglePlaylist,
	BtnAdd:      ActOpen,
	BtnRemove:   ActRemove,
	BtnLoad:     ActLoadPlaylist,
	BtnSave:     ActSavePlaylist,
}

// drag is what the mouse is currently holding.
type drag int

const (
	dragNone drag = iota
	dragButton
	dragSeek
	dragVolume
)

// Controller turns input into player and playlist calls and keeps what the
// view shows. It is not safe for concurrent use: Window calls it from the
// game loop only, and background work hands results back through pending.
type Controller struct {
	p       Player
	pl      *playlist.Playlist
	dialogs Dialogs
	// probe returns a track's duration; it runs off the UI thread.
	probe func(path string) (time.Duration, error)
	// pending carries results of background work to apply in Tick.
	pending chan func()

	drag     drag
	pressed  Button  // button held down, if drag == dragButton
	seekFrac float64 // seek bar position while dragging it

	showRemaining bool
	showPlaylist  bool
	ticks         int
	message       string // overrides the track title, e.g. an error

	selected  int // selected playlist row, or -1
	scroll    int // first visible playlist row
	lastClick struct{ row, tick int }
}

// NewController returns a controller for p and pl. dialogs may be nil.
func NewController(p Player, pl *playlist.Playlist, dialogs Dialogs) *Controller {
	return &Controller{
		p:        p,
		pl:       pl,
		dialogs:  dialogs,
		probe:    playlist.Probe,
		pending:  make(chan func(), 64),
		selected: -1,
	}
}

// Playlist returns the playlist the controller drives.
func (c *Controller) Playlist() *playlist.Playlist { return c.pl }

// ShowPlaylist reports whether the playlist panel is shown.
func (c *Controller) ShowPlaylist() bool { return c.showPlaylist }

// SetShowPlaylist shows or hides the playlist panel.
func (c *Controller) SetShowPlaylist(v bool) { c.showPlaylist = v }

// Height returns the logical window height for the current layout.
func (c *Controller) Height() int {
	if c.showPlaylist {
		return Height + PanelHeight
	}
	return Height
}

// Open adds files and folders to the playlist. If nothing is playing it
// starts the first added track.
func (c *Controller) Open(paths []string) {
	first := c.pl.Len()
	if c.pl.AddPaths(paths...) == 0 {
		if len(paths) > 0 {
			c.setMessage("no supported audio in " + filepath.Base(paths[0]))
		}
		return
	}
	c.probeFrom(first)
	if c.p.State() != player.Playing {
		c.playIndex(first)
	}
}

// ProbeDurations finds missing track durations in the background, e.g.
// after a playlist was restored.
func (c *Controller) ProbeDurations() { c.probeFrom(0) }

// probeFrom finds durations of tracks from index i on in the background.
func (c *Controller) probeFrom(i int) {
	var paths []string
	for ; i < c.pl.Len(); i++ {
		if t := c.pl.Track(i); t.Duration == 0 {
			paths = append(paths, t.Path)
		}
	}
	if len(paths) == 0 {
		return
	}
	go func() {
		for _, path := range paths {
			d, err := c.probe(path)
			if err != nil {
				continue
			}
			c.pending <- func() { c.pl.SetDuration(path, d) }
		}
	}()
}

// playIndex loads playlist track i and starts it.
func (c *Controller) playIndex(i int) {
	if i < 0 || i >= c.pl.Len() {
		return
	}
	c.pl.SetCurrent(i)
	c.scrollTo(i)
	c.message, c.ticks = "", 0
	if err := c.p.Load(c.pl.Track(i).Path); err != nil {
		c.setMessage("error: " + err.Error())
		return
	}
	c.reportErr(c.p.Play())
}

// Do runs an action.
func (c *Controller) Do(a Action) {
	switch a {
	case ActPrev:
		if i, ok := c.pl.Prev(); ok {
			c.playIndex(i)
		} else {
			c.p.Seek(0)
		}
	case ActNext:
		if i, ok := c.pl.Next(); ok {
			c.playIndex(i)
		}
	case ActPlay:
		switch {
		case c.p.Path() == "":
			c.playFirst()
		case c.p.State() == player.Playing:
			c.p.Seek(0) // Winamp restarts the track on Play while playing.
		default:
			c.reportErr(c.p.Play())
		}
	case ActPlaySelected:
		if c.selected >= 0 {
			c.playIndex(c.selected)
		} else {
			c.Do(ActPlay)
		}
	case ActPause:
		switch c.p.State() {
		case player.Playing:
			c.p.Pause()
		case player.Paused:
			c.reportErr(c.p.Play())
		}
	case ActStop:
		c.p.Stop()
	case ActSeekBack:
		c.p.Seek(c.p.Position() - seekStep)
	case ActSeekForward:
		c.p.Seek(c.p.Position() + seekStep)
	case ActVolumeUp:
		c.p.SetVolume(c.p.Volume() + volumeStep)
	case ActVolumeDown:
		c.p.SetVolume(c.p.Volume() - volumeStep)
	case ActShuffle:
		c.pl.Shuffle = !c.pl.Shuffle
	case ActRepeat:
		c.pl.Repeat = !c.pl.Repeat
	case ActTogglePlaylist:
		c.showPlaylist = !c.showPlaylist
	case ActRemove:
		c.removeSelected()
	case ActOpen:
		c.openDialog()
	case ActLoadPlaylist:
		c.loadDialog()
	case ActSavePlaylist:
		c.saveDialog()
	}
}

// playFirst starts the selected track, or the first one.
func (c *Controller) playFirst() {
	switch {
	case c.selected >= 0:
		c.playIndex(c.selected)
	case c.pl.Len() > 0:
		c.playIndex(max(c.pl.Current(), 0))
	default:
		c.reportErr(player.ErrNoTrack)
	}
}

func (c *Controller) removeSelected() {
	i := c.selected
	if i < 0 || i >= c.pl.Len() {
		return
	}
	c.pl.Remove(i)
	// Keep a row selected so Delete can be pressed repeatedly.
	c.selected = min(i, c.pl.Len()-1)
	c.scroll = max(0, min(c.scroll, c.pl.Len()-visibleRows))
}

// async runs a blocking dialog off the UI thread and applies its result in Tick.
func (c *Controller) async(dialog func() (func(), error)) {
	if c.dialogs == nil {
		c.setMessage("file dialogs are not available")
		return
	}
	go func() {
		apply, err := dialog()
		if err != nil {
			c.pending <- func() { c.setMessage("error: " + err.Error()) }
			return
		}
		if apply != nil {
			c.pending <- apply
		}
	}()
}

func (c *Controller) openDialog() {
	c.async(func() (func(), error) {
		paths, err := c.dialogs.OpenFiles()
		if len(paths) == 0 {
			return nil, err
		}
		return func() { c.Open(paths) }, err
	})
}

func (c *Controller) loadDialog() {
	c.async(func() (func(), error) {
		path, err := c.dialogs.OpenPlaylist()
		if path == "" {
			return nil, err
		}
		return func() {
			if err := c.pl.LoadFile(path); err != nil {
				c.setMessage("error: " + err.Error())
				return
			}
			c.selected, c.scroll = -1, 0
			c.probeFrom(0)
			c.showPlaylist = true
		}, err
	})
}

func (c *Controller) saveDialog() {
	c.async(func() (func(), error) {
		path, err := c.dialogs.SavePlaylist()
		if path == "" {
			return nil, err
		}
		if !strings.EqualFold(filepath.Ext(path), ".m3u") {
			path += ".m3u"
		}
		// Snapshot on the UI thread, write in the background.
		return func() {
			snapshot := playlist.New()
			snapshot.Add(c.pl.Tracks()...)
			go func() {
				if err := snapshot.SaveFile(path); err != nil {
					c.pending <- func() { c.setMessage("error: " + err.Error()) }
				}
			}()
		}, err
	})
}

func (c *Controller) setMessage(m string) {
	c.message, c.ticks = m, 0
}

func (c *Controller) reportErr(err error) {
	if errors.Is(err, player.ErrNoTrack) {
		c.setMessage("no file: drop audio files onto the window")
	} else if err != nil {
		c.setMessage("error: " + err.Error())
	}
}

// MouseDown starts a click or a drag at p (logical coordinates).
func (c *Controller) MouseDown(p image.Point) {
	switch b := buttonAt(p, c.showPlaylist); {
	case b != NoButton:
		c.drag, c.pressed = dragButton, b
	case p.In(seekRect.Inset(-2)) && c.p.Duration() > 0:
		c.drag, c.seekFrac = dragSeek, sliderFrac(seekRect, p.X)
	case p.In(volumeRect.Inset(-2)):
		c.drag = dragVolume
		c.p.SetVolume(sliderFrac(volumeRect, p.X))
	case p.In(timeRect):
		c.showRemaining = !c.showRemaining
	case c.showPlaylist && p.In(listRect):
		c.clickRow(c.scroll + (p.Y-listRect.Min.Y)/rowHeight)
	}
}

func (c *Controller) clickRow(row int) {
	if row >= c.pl.Len() {
		c.selected = -1
		return
	}
	if c.lastClick.row == row && c.ticks-c.lastClick.tick <= doubleClickTicks && c.selected == row {
		c.playIndex(row)
		c.lastClick.row = -1
		return
	}
	c.selected = row
	c.lastClick.row, c.lastClick.tick = row, c.ticks
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
		if buttonAt(p, c.showPlaylist) == c.pressed {
			c.Do(buttonActions[c.pressed])
		}
	case dragSeek:
		c.seekFrac = sliderFrac(seekRect, p.X)
		c.p.Seek(time.Duration(c.seekFrac * float64(c.p.Duration())))
	}
	c.drag, c.pressed = dragNone, NoButton
}

// Wheel scrolls the playlist by rows (positive scrolls down).
func (c *Controller) Wheel(rows int) {
	c.scroll = max(0, min(c.scroll+rows, c.pl.Len()-visibleRows))
}

// scrollTo makes row i visible.
func (c *Controller) scrollTo(i int) {
	switch {
	case i < c.scroll:
		c.scroll = i
	case i >= c.scroll+visibleRows:
		c.scroll = i - visibleRows + 1
	}
}

// Tick advances animations and applies background results; call once per
// game update.
func (c *Controller) Tick() {
	c.ticks++
	for {
		select {
		case apply := <-c.pending:
			apply()
			continue
		case <-c.p.Finished():
			if i, ok := c.pl.Next(); ok {
				c.playIndex(i)
			}
			continue
		default:
		}
		return
	}
}

// Row is one visible playlist line.
type Row struct {
	Text     string // "N. Title"
	Time     string // "m:ss", or "" while unknown
	Current  bool
	Selected bool
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

	Shuffle, Repeat bool
	ShowPlaylist    bool
	Rows            []Row
	// ListPos and ListSize place the scrollbar thumb as fractions of the track.
	ListPos, ListSize float64
	PlaylistInfo      string
}

// View returns the current view state.
func (c *Controller) View() View {
	pos, dur := c.p.Position(), c.p.Duration()
	v := View{
		State:        c.p.State(),
		HasTrack:     dur > 0,
		Volume:       c.p.Volume(),
		Scroll:       c.ticks / marqueeSpeed,
		Shuffle:      c.pl.Shuffle,
		Repeat:       c.pl.Repeat,
		ShowPlaylist: c.showPlaylist,
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
		title := trackTitle(c.p.Path())
		if i := c.pl.Current(); i >= 0 && c.pl.Track(i).Path == c.p.Path() {
			title = fmt.Sprintf("%d. %s", i+1, c.pl.Track(i).Title)
		}
		v.Title = fmt.Sprintf("%s (%s)", title, formatTime(dur))
	case c.pl.Current() >= 0:
		v.Title = fmt.Sprintf("%d. %s", c.pl.Current()+1, c.pl.Track(c.pl.Current()).Title)
	default:
		v.Title = "xWinAmp - drop audio files here"
	}

	if c.drag == dragVolume {
		v.Info = fmt.Sprintf("volume %d%%", int(v.Volume*100+0.5))
	} else if c.p.Path() != "" {
		v.Info = strings.ToUpper(strings.TrimPrefix(filepath.Ext(c.p.Path()), ".")) + "  " + v.State.String()
	}

	if c.showPlaylist {
		c.fillPlaylistView(&v)
	}
	return v
}

func (c *Controller) fillPlaylistView(v *View) {
	n := c.pl.Len()
	for i := c.scroll; i < min(n, c.scroll+visibleRows); i++ {
		t := c.pl.Track(i)
		r := Row{
			Text:     fmt.Sprintf("%d. %s", i+1, t.Title),
			Current:  i == c.pl.Current(),
			Selected: i == c.selected,
		}
		if t.Duration > 0 {
			r.Time = formatTime(t.Duration)
		}
		v.Rows = append(v.Rows, r)
	}
	if n > visibleRows {
		v.ListPos = float64(c.scroll) / float64(n)
		v.ListSize = float64(visibleRows) / float64(n)
	}

	total, complete := c.pl.TotalDuration()
	switch {
	case n == 0:
		v.PlaylistInfo = "empty"
	case complete:
		v.PlaylistInfo = fmt.Sprintf("%d / %s", n, formatLongTime(total))
	default:
		v.PlaylistInfo = fmt.Sprintf("%d / %s+", n, formatLongTime(total))
	}
}

func trackTitle(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func formatTime(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// formatLongTime is formatTime with hours for long playlists.
func formatLongTime(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	if s < 3600 {
		return formatTime(d)
	}
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}
