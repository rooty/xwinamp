package ui

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/rooty/xwinamp/player"
)

// fakePlayer records calls and keeps just enough state for the controller.
type fakePlayer struct {
	path     string
	state    player.State
	pos, dur time.Duration
	volume   float64
	loadErr  error
	finished chan struct{}
}

func newFakePlayer() *fakePlayer {
	return &fakePlayer{volume: 1, finished: make(chan struct{}, 1)}
}

func (f *fakePlayer) Load(path string) error {
	if f.loadErr != nil {
		return f.loadErr
	}
	f.path, f.state, f.pos, f.dur = path, player.Stopped, 0, 3*time.Minute
	return nil
}

func (f *fakePlayer) Play() error {
	if f.path == "" {
		return player.ErrNoTrack
	}
	f.state = player.Playing
	return nil
}

func (f *fakePlayer) Pause() {
	if f.state == player.Playing {
		f.state = player.Paused
	}
}

func (f *fakePlayer) Stop() error { f.state, f.pos = player.Stopped, 0; return nil }

func (f *fakePlayer) Seek(d time.Duration) error {
	f.pos = max(0, min(d, f.dur))
	return nil
}

func (f *fakePlayer) Position() time.Duration   { return f.pos }
func (f *fakePlayer) Duration() time.Duration   { return f.dur }
func (f *fakePlayer) State() player.State       { return f.state }
func (f *fakePlayer) Path() string              { return f.path }
func (f *fakePlayer) Volume() float64           { return f.volume }
func (f *fakePlayer) SetVolume(v float64)       { f.volume = max(0, min(v, 1)) }
func (f *fakePlayer) Finished() <-chan struct{} { return f.finished }

func center(b Button) image.Point {
	r := buttonRects[b]
	return r.Min.Add(r.Size().Div(2))
}

func click(c *Controller, p image.Point) {
	c.MouseDown(p)
	c.MouseUp(p)
}

func TestOpenPlaysFirstSupportedFile(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"/tmp/cover.jpg", "/music/Song Name.mp3", "/music/other.flac"})

	if p.path != "/music/Song Name.mp3" || p.state != player.Playing {
		t.Fatalf("path %q state %v", p.path, p.state)
	}
	if v := c.View(); v.Title != "Song Name (3:00)" || v.Info != "MP3  playing" {
		t.Fatalf("title %q info %q", v.Title, v.Info)
	}
}

func TestOpenReportsErrors(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)

	c.Open([]string{"/tmp/notes.txt"})
	if got := c.View().Title; got != "unsupported file: notes.txt" {
		t.Fatalf("title %q", got)
	}

	p.loadErr = errors.New("boom")
	c.Open([]string{"/music/a.mp3"})
	if got := c.View().Title; got != "error: boom" {
		t.Fatalf("title %q", got)
	}
}

func TestButtons(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"a.mp3"})

	click(c, center(BtnPause))
	if p.state != player.Paused {
		t.Fatalf("after pause: %v", p.state)
	}
	click(c, center(BtnPause))
	if p.state != player.Playing {
		t.Fatalf("pause again should resume: %v", p.state)
	}

	p.pos = time.Minute
	click(c, center(BtnStop))
	if p.state != player.Stopped || p.pos != 0 {
		t.Fatalf("after stop: %v at %v", p.state, p.pos)
	}

	click(c, center(BtnPlay))
	if p.state != player.Playing {
		t.Fatalf("after play: %v", p.state)
	}

	p.pos = time.Minute
	click(c, center(BtnPrev))
	if p.pos != 0 {
		t.Fatalf("prev should restart the track, pos %v", p.pos)
	}
}

func TestButtonFiresOnlyWhenReleasedOverIt(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"a.mp3"})

	c.MouseDown(center(BtnStop))
	if c.View().Pressed != BtnStop {
		t.Fatal("stop should look pressed")
	}
	c.MouseUp(center(BtnPlay))
	if p.state != player.Playing {
		t.Fatal("stop fired although released elsewhere")
	}
	if c.View().Pressed != NoButton {
		t.Fatal("button still pressed after release")
	}
}

func TestPlayWithoutTrackShowsHint(t *testing.T) {
	c := NewController(newFakePlayer())
	click(c, center(BtnPlay))
	if got := c.View().Title; !strings.HasPrefix(got, "no file") {
		t.Fatalf("title %q", got)
	}
}

func TestSeekBarDrag(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"a.mp3"})

	y := seekRect.Min.Y + 5
	c.MouseDown(image.Pt(seekRect.Min.X, y))
	mid := image.Pt((seekRect.Min.X+seekRect.Max.X-1)/2, y)
	c.MouseMove(mid)
	if p.pos != 0 {
		t.Fatal("seek must wait for release")
	}
	if v := c.View(); v.Time != "1:30" {
		t.Fatalf("preview time %q, want 1:30", v.Time)
	}
	c.MouseUp(mid)
	// One pixel of the bar is about 0.7s of a 3 minute track.
	if d := p.pos - 90*time.Second; d < -time.Second || d > time.Second {
		t.Fatalf("pos %v, want about 1m30s", p.pos)
	}

	click(c, image.Pt(seekRect.Max.X-1, y))
	if p.pos != p.dur {
		t.Fatalf("right end: pos %v, want %v", p.pos, p.dur)
	}
}

func TestSeekBarIgnoredWithoutTrack(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.MouseDown(image.Pt(seekRect.Min.X+50, seekRect.Min.Y+5))
	c.MouseUp(image.Pt(seekRect.Min.X+50, seekRect.Min.Y+5))
	if p.pos != 0 {
		t.Fatal("seek without track")
	}
}

func TestVolumeSlider(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	y := volumeRect.Min.Y + 4

	c.MouseDown(image.Pt(volumeRect.Min.X, y))
	if p.volume != 0 {
		t.Fatalf("volume %v, want 0", p.volume)
	}
	c.MouseMove(image.Pt(volumeRect.Max.X+100, y))
	if p.volume != 1 {
		t.Fatalf("volume %v, want 1 (clamped)", p.volume)
	}
	if got := c.View().Info; got != "volume 100%" {
		t.Fatalf("info %q", got)
	}
	c.MouseUp(image.Pt(volumeRect.Max.X, y))
}

func TestHotkeys(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"a.mp3"})

	c.Do(ActSeekForward)
	c.Do(ActSeekForward)
	c.Do(ActSeekBack)
	if p.pos != seekStep {
		t.Fatalf("pos %v, want %v", p.pos, seekStep)
	}
	c.Do(ActVolumeDown)
	if p.volume != 1-volumeStep {
		t.Fatalf("volume %v", p.volume)
	}
}

func TestTimeDisplayToggle(t *testing.T) {
	p := newFakePlayer()
	c := NewController(p)
	c.Open([]string{"a.mp3"})
	p.pos = 65 * time.Second

	if got := c.View().Time; got != "1:05" {
		t.Fatalf("time %q", got)
	}
	click(c, timeRect.Min.Add(image.Pt(5, 5)))
	if got := c.View().Time; got != "-1:55" {
		t.Fatalf("remaining %q, want -1:55", got)
	}
}

func TestMarqueeScrolls(t *testing.T) {
	c := NewController(newFakePlayer())
	for range marqueeSpeed * 10 {
		c.Tick()
	}
	if got := c.View().Scroll; got != 10 {
		t.Fatalf("scroll %d, want 10", got)
	}
}

func TestButtonsDoNotOverlap(t *testing.T) {
	for a, ra := range buttonRects {
		for b, rb := range buttonRects {
			if a < b && ra.Overlaps(rb) {
				t.Errorf("buttons %d and %d overlap", a, b)
			}
		}
		if !ra.In(image.Rect(0, 0, Width, Height)) {
			t.Errorf("button %d outside the window", a)
		}
	}
}
