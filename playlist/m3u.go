package playlist

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReadM3U parses an M3U or extended M3U playlist. Relative paths are
// resolved against baseDir. Unsupported entries (URLs, other formats) are
// skipped.
func ReadM3U(r io.Reader, baseDir string) ([]Track, error) {
	var (
		tracks []Track
		info   *Track // from the preceding #EXTINF line
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimPrefix(line, "\xef\xbb\xbf") // UTF-8 BOM
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#EXTINF:"):
			info = parseExtInf(strings.TrimPrefix(line, "#EXTINF:"))
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}

		path := filepath.FromSlash(line)
		if strings.Contains(line, "://") {
			info = nil
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		t := NewTrack(path)
		if info != nil {
			if info.Title != "" {
				t.Title = info.Title
			}
			t.Duration = info.Duration
			info = nil
		}
		tracks = append(tracks, t)
	}
	return tracks, sc.Err()
}

// parseExtInf parses "123,Artist - Title" (seconds may be -1 for unknown).
func parseExtInf(s string) *Track {
	secs, title, _ := strings.Cut(s, ",")
	t := &Track{Title: strings.TrimSpace(title)}
	// Attributes like tvg-id="..." may follow the number; take the first field.
	if f := strings.Fields(secs); len(f) > 0 {
		if n, err := strconv.ParseFloat(f[0], 64); err == nil && n > 0 {
			t.Duration = time.Duration(n * float64(time.Second))
		}
	}
	return t
}

// WriteM3U writes tracks as an extended M3U playlist with absolute paths.
func WriteM3U(w io.Writer, tracks []Track) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintln(bw, "#EXTM3U")
	for _, t := range tracks {
		secs := -1
		if t.Duration > 0 {
			secs = int(t.Duration.Round(time.Second) / time.Second)
		}
		fmt.Fprintf(bw, "#EXTINF:%d,%s\n%s\n", secs, t.Title, t.Path)
	}
	return bw.Flush()
}

// LoadFile replaces the playlist with the tracks of an M3U file.
func (p *Playlist) LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tracks, err := ReadM3U(f, filepath.Dir(path))
	if err != nil {
		return err
	}
	p.Clear()
	p.Add(tracks...)
	return nil
}

// SaveFile writes the playlist to an M3U file, replacing it atomically.
func (p *Playlist) SaveFile(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".playlist-*.m3u")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if err := WriteM3U(tmp, p.tracks); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
