package ui

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rooty/xwinamp/player"
	"github.com/rooty/xwinamp/playlist"
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

// newTestController returns a controller whose durations are probed by a stub.
func newTestController(p *fakePlayer) *Controller {
	c := NewController(p, playlist.New(), nil)
	c.probe = func(string) (time.Duration, error) { return 2 * time.Minute, nil }
	return c
}

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
	c := newTestController(p)
	c.Open([]string{"/tmp/cover.jpg", "/music/Song Name.mp3", "/music/other.flac"})

	if p.path != "/music/Song Name.mp3" || p.state != player.Playing {
		t.Fatalf("path %q state %v", p.path, p.state)
	}
	if c.Playlist().Len() != 2 {
		t.Fatalf("playlist len %d, want 2", c.Playlist().Len())
	}
	if v := c.View(); v.Title != "1. Song Name (3:00)" || v.Info != "MP3  playing" {
		t.Fatalf("title %q info %q", v.Title, v.Info)
	}
}

func TestOpenReportsErrors(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)

	c.Open([]string{"/tmp/notes.txt"})
	if got := c.View().Title; got != "no supported audio in notes.txt" {
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
	c := newTestController(p)
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
		t.Fatalf("prev on the first track should restart it, pos %v", p.pos)
	}
}

func TestButtonFiresOnlyWhenReleasedOverIt(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
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
	c := newTestController(newFakePlayer())
	click(c, center(BtnPlay))
	if got := c.View().Title; !strings.HasPrefix(got, "no file") {
		t.Fatalf("title %q", got)
	}
}

func TestSeekBarDrag(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
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
	c := newTestController(p)
	c.MouseDown(image.Pt(seekRect.Min.X+50, seekRect.Min.Y+5))
	c.MouseUp(image.Pt(seekRect.Min.X+50, seekRect.Min.Y+5))
	if p.pos != 0 {
		t.Fatal("seek without track")
	}
}

func TestVolumeSlider(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
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
	c := newTestController(p)
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
	c := newTestController(p)
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
	c := newTestController(newFakePlayer())
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
		area := image.Rect(0, 0, Width, Height)
		if inPanel(a) {
			area = panelRect
		}
		if !ra.In(area) {
			t.Errorf("button %d outside its area", a)
		}
	}
}

// openTracks opens n fake tracks named t1.mp3, t2.mp3, ...
func openTracks(c *Controller, n int) {
	var paths []string
	for i := 1; i <= n; i++ {
		paths = append(paths, fmt.Sprintf("/music/t%d.mp3", i))
	}
	c.Open(paths)
}

func TestNextPrevFollowPlaylist(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	openTracks(c, 3)

	click(c, center(BtnNext))
	if p.path != "/music/t2.mp3" || p.state != player.Playing {
		t.Fatalf("after next: %q %v", p.path, p.state)
	}
	click(c, center(BtnPrev))
	if p.path != "/music/t1.mp3" {
		t.Fatalf("after prev: %q", p.path)
	}
}

func TestOpenWhilePlayingOnlyAppends(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	openTracks(c, 1)
	c.Open([]string{"/music/later.mp3"})
	if p.path != "/music/t1.mp3" || c.Playlist().Len() != 2 {
		t.Fatalf("path %q len %d", p.path, c.Playlist().Len())
	}
}

func TestAutoAdvanceAndStopAtEnd(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	openTracks(c, 2)

	p.state = player.Stopped
	p.finished <- struct{}{}
	c.Tick()
	if p.path != "/music/t2.mp3" || p.state != player.Playing {
		t.Fatalf("after first track ended: %q %v", p.path, p.state)
	}

	p.state = player.Stopped
	p.finished <- struct{}{}
	c.Tick()
	if p.path != "/music/t2.mp3" || p.state != player.Stopped {
		t.Fatalf("after last track ended: %q %v", p.path, p.state)
	}
}

func TestRepeatAndShuffleButtons(t *testing.T) {
	c := newTestController(newFakePlayer())
	click(c, center(BtnRepeat))
	click(c, center(BtnShuffle))
	if v := c.View(); !v.Repeat || !v.Shuffle {
		t.Fatalf("repeat %v shuffle %v", v.Repeat, v.Shuffle)
	}
	c.Do(ActShuffle)
	if c.View().Shuffle {
		t.Fatal("shuffle should toggle off")
	}
}

func TestPlaylistPanelToggle(t *testing.T) {
	c := newTestController(newFakePlayer())
	if c.Height() != Height {
		t.Fatal("panel shown by default")
	}
	// Panel buttons are dead while the panel is hidden.
	if buttonAt(center(BtnAdd), false) != NoButton {
		t.Fatal("hidden panel button is clickable")
	}
	click(c, center(BtnPlaylist))
	if c.Height() != Height+PanelHeight || !c.View().ShowPlaylist {
		t.Fatal("panel not shown")
	}
}

func rowPoint(row int) image.Point {
	return image.Pt(listRect.Min.X+20, listRect.Min.Y+row*rowHeight+5)
}

func TestPlaylistRowsSelectAndDoubleClick(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	c.SetShowPlaylist(true)
	openTracks(c, 3)

	click(c, rowPoint(2))
	v := c.View()
	if len(v.Rows) != 3 || !v.Rows[2].Selected || !v.Rows[0].Current {
		t.Fatalf("rows %+v", v.Rows)
	}
	if v.Rows[1].Text != "2. t2" {
		t.Fatalf("row text %q", v.Rows[1].Text)
	}
	if p.path != "/music/t1.mp3" {
		t.Fatal("single click must not play")
	}

	c.Tick()
	click(c, rowPoint(2))
	if p.path != "/music/t3.mp3" {
		t.Fatalf("double click did not play row 3: %q", p.path)
	}

	// Clicks too far apart are two single clicks.
	click(c, rowPoint(1))
	for range doubleClickTicks + 1 {
		c.Tick()
	}
	click(c, rowPoint(1))
	if p.path != "/music/t3.mp3" {
		t.Fatal("slow clicks played a track")
	}
}

func TestRemoveSelected(t *testing.T) {
	c := newTestController(newFakePlayer())
	c.SetShowPlaylist(true)
	openTracks(c, 3)

	click(c, rowPoint(1))
	c.Do(ActRemove)
	c.Do(ActRemove)
	v := c.View()
	if len(v.Rows) != 1 || v.Rows[0].Text != "1. t1" {
		t.Fatalf("rows %+v", v.Rows)
	}
	c.Do(ActRemove)
	c.Do(ActRemove) // nothing selected anymore: no-op
	if c.Playlist().Len() != 0 || c.View().PlaylistInfo != "empty" {
		t.Fatalf("len %d info %q", c.Playlist().Len(), c.View().PlaylistInfo)
	}
}

func TestScrollAndFollowCurrent(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	c.SetShowPlaylist(true)
	openTracks(c, 20)

	c.Wheel(100)
	v := c.View()
	if v.Rows[0].Text != "13. t13" || len(v.Rows) != visibleRows {
		t.Fatalf("first row %q", v.Rows[0].Text)
	}
	if v.ListSize != float64(visibleRows)/20 {
		t.Fatalf("thumb size %v", v.ListSize)
	}
	c.Wheel(-100)
	if c.View().Rows[0].Text != "1. t1" {
		t.Fatal("did not scroll back to top")
	}

	// Playing a track off screen scrolls to it.
	for range 9 {
		c.Do(ActNext)
	}
	if c.View().Rows[visibleRows-1].Text != "10. t10" {
		t.Fatalf("last row %q", c.View().Rows[visibleRows-1].Text)
	}
}

func TestDurationsProbedInBackground(t *testing.T) {
	c := newTestController(newFakePlayer())
	c.SetShowPlaylist(true)
	openTracks(c, 2)

	deadline := time.Now().Add(2 * time.Second)
	for {
		c.Tick()
		if c.View().PlaylistInfo == "2 / 4:00" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("info %q", c.View().PlaylistInfo)
		}
		time.Sleep(time.Millisecond)
	}
	if got := c.View().Rows[0].Time; got != "2:00" {
		t.Fatalf("row time %q", got)
	}
}

// fakeDialogs answers file dialogs with fixed results.
type fakeDialogs struct {
	files    []string
	loadPath string
	savePath string
}

func (f *fakeDialogs) OpenFiles() ([]string, error)  { return f.files, nil }
func (f *fakeDialogs) OpenPlaylist() (string, error) { return f.loadPath, nil }
func (f *fakeDialogs) SavePlaylist() (string, error) { return f.savePath, nil }

// tickUntil runs Tick until cond holds or a second passes.
func tickUntil(t *testing.T, c *Controller, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		c.Tick()
		time.Sleep(time.Millisecond)
	}
}

func TestDialogs(t *testing.T) {
	p := newFakePlayer()
	c := newTestController(p)
	dir := t.TempDir()
	d := &fakeDialogs{files: []string{"/music/a.mp3", "/music/b.mp3"}, savePath: filepath.Join(dir, "mix")}
	c.dialogs = d

	c.Do(ActOpen)
	tickUntil(t, c, func() bool { return c.Playlist().Len() == 2 })
	if p.path != "/music/a.mp3" {
		t.Fatalf("path %q", p.path)
	}

	c.Do(ActSavePlaylist)
	saved := filepath.Join(dir, "mix.m3u")
	tickUntil(t, c, func() bool { _, err := os.Stat(saved); return err == nil })

	c.Playlist().Clear()
	d.loadPath = saved
	c.Do(ActLoadPlaylist)
	tickUntil(t, c, func() bool { return c.Playlist().Len() == 2 })
}

func TestDialogsUnavailable(t *testing.T) {
	c := newTestController(newFakePlayer())
	c.Do(ActOpen)
	if got := c.View().Title; got != "file dialogs are not available" {
		t.Fatalf("title %q", got)
	}
}

func TestRestoredPlaylistShowsCurrentAndProbes(t *testing.T) {
	c := newTestController(newFakePlayer())
	c.Playlist().AddPaths("/music/a.mp3", "/music/b.mp3")
	c.Playlist().SetCurrent(1)
	if got := c.View().Title; got != "2. b" {
		t.Fatalf("title %q", got)
	}
	c.SetShowPlaylist(true)
	c.ProbeDurations()
	tickUntil(t, c, func() bool { return c.View().PlaylistInfo == "2 / 4:00" })
}
