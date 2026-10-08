package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// ErrUnsupportedFormat is returned when a file extension is not recognised.
var ErrUnsupportedFormat = errors.New("unsupported audio format")

// SupportedExtensions lists file extensions (lower case, with dot) that Open can decode.
var SupportedExtensions = []string{".mp3", ".wav", ".flac", ".ogg"}

// IsSupported reports whether the file at path has a supported extension.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range SupportedExtensions {
		if e == ext {
			return true
		}
	}
	return false
}

// Open decodes the audio file at path, choosing the decoder by extension.
// The returned streamer owns the file and must be closed by the caller.
func Open(path string) (beep.StreamSeekCloser, beep.Format, error) {
	if !IsSupported(path) {
		return nil, beep.Format{}, fmt.Errorf("%s: %w", path, ErrUnsupportedFormat)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}

	var (
		s      beep.StreamSeekCloser
		format beep.Format
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		s, format, err = mp3.Decode(f)
	case ".wav":
		s, format, err = wav.Decode(f)
	case ".flac":
		s, format, err = flac.Decode(f)
	case ".ogg":
		s, format, err = vorbis.Decode(f)
	}
	if err != nil {
		f.Close()
		return nil, beep.Format{}, fmt.Errorf("decode %s: %w", path, err)
	}

	// Every decoder closes f when the returned streamer is closed.
	return s, format, nil
}
