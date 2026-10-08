package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rooty/xwinamp/ui"
)

// volumer is the part of the player whose state is saved.
type volumer interface {
	Volume() float64
	SetVolume(v float64)
}

// settings is what is remembered between runs besides the playlist.
type settings struct {
	Volume       float64 `json:"volume"`
	Shuffle      bool    `json:"shuffle"`
	Repeat       bool    `json:"repeat"`
	ShowPlaylist bool    `json:"showPlaylist"`
	Current      int     `json:"current"`
}

const (
	settingsFile = "settings.json"
	playlistFile = "playlist.m3u"
)

// stateDir returns the per-user directory for xWinAmp state, e.g.
// ~/Library/Application Support/xwinamp on macOS, creating it if needed.
func stateDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "xwinamp")
	return dir, os.MkdirAll(dir, 0o755)
}

// loadState restores the playlist and settings. Missing files are not an error.
func loadState(dir string, c *ui.Controller, p volumer) error {
	pl := c.Playlist()
	err := pl.LoadFile(filepath.Join(dir, playlistFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	data, err := os.ReadFile(filepath.Join(dir, settingsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	s := settings{Volume: 1, Current: -1}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	p.SetVolume(s.Volume)
	pl.Shuffle, pl.Repeat = s.Shuffle, s.Repeat
	pl.SetCurrent(s.Current)
	c.SetShowPlaylist(s.ShowPlaylist)
	return nil
}

func saveState(dir string, c *ui.Controller, p volumer) error {
	pl := c.Playlist()
	if err := pl.SaveFile(filepath.Join(dir, playlistFile)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings{
		Volume:       p.Volume(),
		Shuffle:      pl.Shuffle,
		Repeat:       pl.Repeat,
		ShowPlaylist: c.ShowPlaylist(),
		Current:      pl.Current(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, settingsFile), data, 0o644)
}
