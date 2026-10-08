package ui

import (
	"errors"

	"github.com/ncruces/zenity"
)

// SystemDialogs shows native file dialogs: AppleScript on macOS, the
// Windows API on Windows, zenity or kdialog on Linux.
type SystemDialogs struct{}

var audioFilter = zenity.FileFilters{
	{Name: "Audio", Patterns: []string{"*.mp3", "*.wav", "*.flac", "*.ogg"}, CaseFold: true},
}

var playlistFilter = zenity.FileFilters{
	{Name: "Playlist", Patterns: []string{"*.m3u", "*.m3u8"}, CaseFold: true},
}

func (SystemDialogs) OpenFiles() ([]string, error) {
	paths, err := zenity.SelectFileMultiple(zenity.Title("Add files"), audioFilter)
	return paths, ignoreCancel(err)
}

func (SystemDialogs) OpenPlaylist() (string, error) {
	path, err := zenity.SelectFile(zenity.Title("Load playlist"), playlistFilter)
	return path, ignoreCancel(err)
}

func (SystemDialogs) SavePlaylist() (string, error) {
	path, err := zenity.SelectFileSave(zenity.Title("Save playlist"),
		zenity.Filename("playlist.m3u"), zenity.ConfirmOverwrite(), playlistFilter)
	return path, ignoreCancel(err)
}

func ignoreCancel(err error) error {
	if errors.Is(err, zenity.ErrCanceled) {
		return nil
	}
	return err
}
