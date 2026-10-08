package playlist

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func newList(names ...string) *Playlist {
	p := New()
	for _, n := range names {
		p.Add(NewTrack("/music/" + n + ".mp3"))
	}
	return p
}

func titles(p *Playlist) []string {
	var out []string
	for _, t := range p.Tracks() {
		out = append(out, t.Title)
	}
	return out
}

func TestAddSkipsUnsupported(t *testing.T) {
	p := New()
	n := p.Add(NewTrack("a.mp3"), NewTrack("cover.jpg"), NewTrack("b.FLAC"))
	if n != 2 || p.Len() != 2 {
		t.Fatalf("added %d, len %d", n, p.Len())
	}
	if p.Current() != -1 {
		t.Fatal("new playlist must have no current track")
	}
}

func TestAddPathsWalksDirectoriesSorted(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"b.mp3", "A.mp3", "notes.txt", "sub/c.flac", "sub/deep/d.ogg"} {
		path := filepath.Join(dir, f)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, nil, 0o644)
	}
	p := New()
	if n := p.AddPaths(dir, "/elsewhere/x.wav"); n != 5 {
		t.Fatalf("added %d, want 5", n)
	}
	want := []string{"A", "b", "c", "d", "x"}
	if got := titles(p); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextPrevLinear(t *testing.T) {
	p := newList("a", "b", "c")
	for _, want := range []int{0, 1, 2} {
		if i, ok := p.Next(); !ok || i != want {
			t.Fatalf("Next = %d %v, want %d", i, ok, want)
		}
	}
	if _, ok := p.Next(); ok {
		t.Fatal("Next past the end without repeat")
	}
	if i, ok := p.Prev(); !ok || i != 1 {
		t.Fatalf("Prev = %d %v", i, ok)
	}
	p.SetCurrent(0)
	if _, ok := p.Prev(); ok {
		t.Fatal("Prev before the start without repeat")
	}
}

func TestRepeatWraps(t *testing.T) {
	p := newList("a", "b")
	p.Repeat = true
	p.SetCurrent(1)
	if i, ok := p.Next(); !ok || i != 0 {
		t.Fatalf("Next = %d %v, want wrap to 0", i, ok)
	}
	if i, ok := p.Prev(); !ok || i != 1 {
		t.Fatalf("Prev = %d %v, want wrap to 1", i, ok)
	}
}

func TestShuffleNeverRepeatsCurrentAndPrevGoesBack(t *testing.T) {
	p := newList("a", "b", "c", "d")
	p.Shuffle = true
	var played []int
	for range 50 {
		prev := p.Current()
		i, ok := p.Next()
		if !ok || i == prev {
			t.Fatalf("Next = %d %v after %d", i, ok, prev)
		}
		played = append(played, i)
	}
	for k := len(played) - 2; k >= len(played)-5; k-- {
		if i, ok := p.Prev(); !ok || i != played[k] {
			t.Fatalf("Prev = %d %v, want %d", i, ok, played[k])
		}
	}
}

func TestShuffleSingleTrack(t *testing.T) {
	p := newList("a")
	p.Shuffle = true
	if i, ok := p.Next(); !ok || i != 0 {
		t.Fatalf("Next = %d %v", i, ok)
	}
	if _, ok := p.Next(); ok {
		t.Fatal("single track without repeat must end")
	}
}

func TestRemoveKeepsCurrent(t *testing.T) {
	p := newList("a", "b", "c")
	p.SetCurrent(2)
	p.Remove(0)
	if p.Current() != 1 || p.Track(p.Current()).Title != "c" {
		t.Fatalf("current %d", p.Current())
	}
	p.Remove(1)
	if p.Current() != -1 {
		t.Fatal("removing the current track must clear current")
	}
	p.Remove(5) // out of range: no panic
	if got := titles(p); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDurations(t *testing.T) {
	p := newList("a", "b")
	if _, complete := p.TotalDuration(); complete {
		t.Fatal("unknown durations reported complete")
	}
	p.SetDuration("/music/a.mp3", time.Minute)
	p.SetDuration("/music/b.mp3", 30*time.Second)
	if d, complete := p.TotalDuration(); !complete || d != 90*time.Second {
		t.Fatalf("total %v %v", d, complete)
	}
}

func TestReadM3U(t *testing.T) {
	src := "\xef\xbb\xbf#EXTM3U\n" +
		"#EXTINF:215,Artist - Song\n" +
		"songs/one.mp3\n" +
		"\n" +
		"# a comment\n" +
		"/abs/two.flac\n" +
		"#EXTINF:-1,Radio\n" +
		"http://example.com/stream.mp3\n" +
		"three.wav\n"
	tracks, err := ReadM3U(strings.NewReader(src), "/lists")
	if err != nil {
		t.Fatal(err)
	}
	want := []Track{
		{Path: filepath.FromSlash("/lists/songs/one.mp3"), Title: "Artist - Song", Duration: 215 * time.Second},
		{Path: filepath.FromSlash("/abs/two.flac"), Title: "two"},
		{Path: filepath.FromSlash("/lists/three.wav"), Title: "three"},
	}
	if !slices.Equal(tracks, want) {
		t.Fatalf("got %+v\nwant %+v", tracks, want)
	}
}

func TestM3URoundTrip(t *testing.T) {
	p := newList("a", "b")
	p.SetDuration("/music/a.mp3", 61*time.Second)
	path := filepath.Join(t.TempDir(), "list.m3u")
	if err := p.SaveFile(path); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("#EXTINF:61,a\n/music/a.mp3\n#EXTINF:-1,b\n")) {
		t.Fatalf("unexpected file:\n%s", data)
	}

	q := New()
	q.Add(NewTrack("old.mp3"))
	if err := q.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(q.Tracks(), p.Tracks()) {
		t.Fatalf("got %+v, want %+v", q.Tracks(), p.Tracks())
	}
}
