package main

import (
	"testing"

	"github.com/rooty/xwinamp/playlist"
	"github.com/rooty/xwinamp/ui"
)

type fakeVolume struct{ v float64 }

func (f *fakeVolume) Volume() float64     { return f.v }
func (f *fakeVolume) SetVolume(v float64) { f.v = v }

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()

	pl := playlist.New()
	pl.AddPaths("/music/a.mp3", "/music/b.flac")
	pl.SetCurrent(1)
	pl.Repeat = true
	c := ui.NewController(nil, pl, nil)
	c.SetShowPlaylist(true)
	if err := saveState(dir, c, &fakeVolume{0.4}); err != nil {
		t.Fatal(err)
	}

	pl2 := playlist.New()
	c2 := ui.NewController(nil, pl2, nil)
	vol := &fakeVolume{1}
	if err := loadState(dir, c2, vol); err != nil {
		t.Fatal(err)
	}
	if pl2.Len() != 2 || pl2.Current() != 1 || !pl2.Repeat || pl2.Shuffle {
		t.Fatalf("len %d current %d repeat %v shuffle %v", pl2.Len(), pl2.Current(), pl2.Repeat, pl2.Shuffle)
	}
	if vol.v != 0.4 || !c2.ShowPlaylist() {
		t.Fatalf("volume %v showPlaylist %v", vol.v, c2.ShowPlaylist())
	}
}

func TestLoadStateMissingIsFine(t *testing.T) {
	c := ui.NewController(nil, playlist.New(), nil)
	if err := loadState(t.TempDir(), c, &fakeVolume{1}); err != nil {
		t.Fatal(err)
	}
}
