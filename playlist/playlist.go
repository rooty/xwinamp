// Package playlist keeps the ordered list of tracks, the current position
// and the repeat/shuffle modes. It does no audio and no UI.
package playlist

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rooty/xwinamp/player"
)

// Track is one playlist entry.
type Track struct {
	Path  string
	Title string
	// Duration is 0 while unknown.
	Duration time.Duration
}

// NewTrack returns a track titled after its file name.
func NewTrack(path string) Track {
	name := filepath.Base(path)
	return Track{Path: path, Title: strings.TrimSuffix(name, filepath.Ext(name))}
}

// maxHistory bounds how far Prev can walk back in shuffle mode.
const maxHistory = 100

// Playlist is an ordered list of tracks with a current position.
// It is not safe for concurrent use.
type Playlist struct {
	tracks  []Track
	current int // -1 when nothing is selected for playback

	Repeat  bool
	Shuffle bool

	history []int // previously played indices, for Prev in shuffle mode
	rand    func(n int) int
}

// New returns an empty playlist.
func New() *Playlist {
	return &Playlist{current: -1, rand: rand.IntN}
}

// Len returns the number of tracks.
func (p *Playlist) Len() int { return len(p.tracks) }

// Track returns track i.
func (p *Playlist) Track(i int) Track { return p.tracks[i] }

// Tracks returns a copy of all tracks.
func (p *Playlist) Tracks() []Track { return slices.Clone(p.tracks) }

// Current returns the index of the current track, or -1.
func (p *Playlist) Current() int { return p.current }

// SetCurrent makes track i current, e.g. when the user picks it.
func (p *Playlist) SetCurrent(i int) {
	if i < 0 || i >= len(p.tracks) {
		return
	}
	if p.current >= 0 && p.current != i {
		p.pushHistory(p.current)
	}
	p.current = i
}

// Add appends tracks for the given files and returns how many were added.
// Unsupported files are skipped.
func (p *Playlist) Add(tracks ...Track) int {
	n := 0
	for _, t := range tracks {
		if player.IsSupported(t.Path) {
			p.tracks = append(p.tracks, t)
			n++
		}
	}
	return n
}

// AddPaths appends files and, recursively, the supported files inside
// directories. Files from a directory are added in path order.
// It returns how many tracks were added.
func (p *Playlist) AddPaths(paths ...string) int {
	n := 0
	for _, path := range paths {
		for _, f := range expand(path) {
			n += p.Add(NewTrack(f))
		}
	}
	return n
}

// expand returns path itself for a file, or the sorted supported files
// under it for a directory.
func expand(path string) []string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return []string{path}
	}
	var files []string
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && player.IsSupported(p) {
			files = append(files, p)
		}
		return nil // skip unreadable entries, keep walking
	})
	slices.SortFunc(files, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	return files
}

// Remove deletes track i, keeping the current track current if it remains.
func (p *Playlist) Remove(i int) {
	if i < 0 || i >= len(p.tracks) {
		return
	}
	p.tracks = slices.Delete(p.tracks, i, i+1)
	switch {
	case p.current == i:
		p.current = -1
	case p.current > i:
		p.current--
	}
	// Re-map history, dropping the removed index.
	h := p.history[:0]
	for _, j := range p.history {
		switch {
		case j < i:
			h = append(h, j)
		case j > i:
			h = append(h, j-1)
		}
	}
	p.history = h
}

// Clear removes all tracks.
func (p *Playlist) Clear() {
	p.tracks, p.current, p.history = nil, -1, nil
}

// SetDuration records the duration of every track with this path.
func (p *Playlist) SetDuration(path string, d time.Duration) {
	for i := range p.tracks {
		if p.tracks[i].Path == path {
			p.tracks[i].Duration = d
		}
	}
}

// TotalDuration returns the sum of known durations and whether all are known.
func (p *Playlist) TotalDuration() (time.Duration, bool) {
	var total time.Duration
	complete := true
	for _, t := range p.tracks {
		if t.Duration == 0 {
			complete = false
		}
		total += t.Duration
	}
	return total, complete
}

// Next moves to the track that should play next and returns its index.
// It returns false at the end of the list when Repeat is off.
func (p *Playlist) Next() (int, bool) {
	n := len(p.tracks)
	if n == 0 {
		return -1, false
	}
	var next int
	switch {
	case p.Shuffle && n > 1:
		// Any track but the current one.
		next = p.rand(n - 1)
		if p.current >= 0 && next >= p.current {
			next++
		}
	case p.Shuffle:
		next = 0
		if p.current == 0 && !p.Repeat {
			return -1, false
		}
	case p.current+1 < n:
		next = p.current + 1
	case p.Repeat:
		next = 0
	default:
		return -1, false
	}
	p.SetCurrent(next)
	return next, true
}

// Prev moves to the previous track: the previously played one in shuffle
// mode, otherwise the one above. It returns false if there is none.
func (p *Playlist) Prev() (int, bool) {
	n := len(p.tracks)
	if n == 0 {
		return -1, false
	}
	var prev int
	switch {
	case p.Shuffle:
		if len(p.history) == 0 {
			return -1, false
		}
		prev = p.history[len(p.history)-1]
		p.history = p.history[:len(p.history)-1]
		p.current = prev
		return prev, true
	case p.current > 0:
		prev = p.current - 1
	case p.Repeat:
		prev = n - 1
	default:
		return -1, false
	}
	p.SetCurrent(prev)
	return prev, true
}

func (p *Playlist) pushHistory(i int) {
	p.history = append(p.history, i)
	if len(p.history) > maxHistory {
		p.history = p.history[len(p.history)-maxHistory:]
	}
}

// Probe returns the duration of the audio file at path.
// For MP3 this reads the whole file, so call it off the UI thread.
func Probe(path string) (time.Duration, error) {
	s, format, err := player.Open(path)
	if err != nil {
		return 0, err
	}
	defer s.Close()
	return format.SampleRate.D(s.Len()), nil
}
