package player

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/generators"
	"github.com/gopxl/beep/v2/wav"
)

// fakeOutput stands in for the speaker: tests pull samples with drain.
type fakeOutput struct {
	mu      sync.Mutex
	streams []beep.Streamer
}

func (f *fakeOutput) Lock()   { f.mu.Lock() }
func (f *fakeOutput) Unlock() { f.mu.Unlock() }
func (f *fakeOutput) Play(s beep.Streamer) {
	f.mu.Lock()
	f.streams = append(f.streams, s)
	f.mu.Unlock()
}
func (f *fakeOutput) Clear() { f.mu.Lock(); f.streams = nil; f.mu.Unlock() }

// drain mixes n output samples like the speaker does and returns them.
func (f *fakeOutput) drain(n int) [][2]float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][2]float64, n)
	buf := make([][2]float64, n)
	alive := f.streams[:0]
	for _, s := range f.streams {
		got, ok := s.Stream(buf)
		for i := range got {
			out[i][0] += buf[i][0]
			out[i][1] += buf[i][1]
		}
		if ok {
			alive = append(alive, s)
		}
	}
	f.streams = alive
	return out
}

// writeTone writes a mono-ish 440 Hz WAV of length d at rate sr and returns its path.
func writeTone(t *testing.T, sr beep.SampleRate, d time.Duration) string {
	t.Helper()
	tone, err := generators.SineTone(sr, 440)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tone.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	format := beep.Format{SampleRate: sr, NumChannels: 2, Precision: 2}
	if err := wav.Encode(f, beep.Take(sr.N(d), tone), format); err != nil {
		t.Fatal(err)
	}
	return path
}

func peak(samples [][2]float64) float64 {
	var m float64
	for _, s := range samples {
		m = max(m, math.Abs(s[0]))
	}
	return m
}

func newTestPlayer(t *testing.T, path string) (*Player, *fakeOutput) {
	t.Helper()
	out := &fakeOutput{}
	p := NewWithOutput(out)
	t.Cleanup(func() { p.Close() })
	if err := p.Load(path); err != nil {
		t.Fatal(err)
	}
	return p, out
}

func TestOpenUnsupported(t *testing.T) {
	_, _, err := Open("song.xyz")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("got %v, want ErrUnsupportedFormat", err)
	}
}

func TestOpenCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.mp3")
	if err := os.WriteFile(path, []byte("not an mp3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(path); err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestIsSupported(t *testing.T) {
	for path, want := range map[string]bool{
		"a.mp3": true, "A.MP3": true, "b.flac": true, "c.wav": true, "d.ogg": true,
		"e.m4a": false, "noext": false,
	} {
		if got := IsSupported(path); got != want {
			t.Errorf("IsSupported(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLoadReportsDuration(t *testing.T) {
	p, _ := newTestPlayer(t, writeTone(t, SampleRate, 2*time.Second))
	if got := p.Duration().Round(10 * time.Millisecond); got != 2*time.Second {
		t.Fatalf("Duration = %v, want 2s", got)
	}
	if p.State() != Stopped {
		t.Fatalf("State = %v after Load, want stopped", p.State())
	}
}

func TestPlayPauseResume(t *testing.T) {
	p, out := newTestPlayer(t, writeTone(t, SampleRate, time.Second))

	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	if peak(out.drain(1000)) == 0 {
		t.Fatal("expected sound while playing")
	}
	pos := p.Position()

	p.Pause()
	if p.State() != Paused {
		t.Fatalf("State = %v, want paused", p.State())
	}
	if peak(out.drain(1000)) != 0 {
		t.Fatal("expected silence while paused")
	}
	if p.Position() != pos {
		t.Fatal("position moved while paused")
	}

	if err := p.TogglePause(); err != nil {
		t.Fatal(err)
	}
	if p.State() != Playing || peak(out.drain(1000)) == 0 {
		t.Fatal("expected playback to resume")
	}
}

func TestPlaysToEndAndSignalsFinished(t *testing.T) {
	p, out := newTestPlayer(t, writeTone(t, SampleRate, 100*time.Millisecond))
	p.Play()
	out.drain(SampleRate.N(200 * time.Millisecond))

	select {
	case <-p.Finished():
	default:
		t.Fatal("Finished not signalled")
	}
	if p.State() != Stopped {
		t.Fatalf("State = %v, want stopped", p.State())
	}

	// Play again restarts from the beginning.
	p.Play()
	if peak(out.drain(1000)) == 0 {
		t.Fatal("expected replay from start")
	}
}

func TestStopRewindsAndSilences(t *testing.T) {
	p, out := newTestPlayer(t, writeTone(t, SampleRate, time.Second))
	p.Play()
	out.drain(5000)
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	if p.Position() != 0 || p.State() != Stopped {
		t.Fatalf("after Stop: pos %v state %v", p.Position(), p.State())
	}
	if peak(out.drain(1000)) != 0 {
		t.Fatal("expected silence after Stop")
	}
	select {
	case <-p.Finished():
		t.Fatal("Stop must not signal Finished")
	default:
	}
}

func TestSeekClamps(t *testing.T) {
	p, _ := newTestPlayer(t, writeTone(t, SampleRate, time.Second))

	p.Seek(500 * time.Millisecond)
	if got := p.Position().Round(time.Millisecond); got != 500*time.Millisecond {
		t.Fatalf("Position = %v, want 500ms", got)
	}
	p.Seek(-time.Second)
	if p.Position() != 0 {
		t.Fatalf("Position = %v, want 0", p.Position())
	}
	p.Seek(time.Hour)
	if p.Position() >= p.Duration() {
		t.Fatalf("Position %v not clamped below Duration %v", p.Position(), p.Duration())
	}
}

func TestVolume(t *testing.T) {
	p, out := newTestPlayer(t, writeTone(t, SampleRate, time.Second))
	p.Play()
	full := peak(out.drain(2000))

	p.SetVolume(0.5)
	half := peak(out.drain(2000))
	want := full * math.Pow(2, -0.5*volumeRange)
	if math.Abs(half-want) > 0.01 {
		t.Fatalf("peak at 0.5 = %.4f, want %.4f", half, want)
	}

	p.SetVolume(-1)
	if p.Volume() != 0 || peak(out.drain(2000)) != 0 {
		t.Fatal("volume 0 must mute")
	}
	p.SetVolume(7)
	if p.Volume() != 1 {
		t.Fatalf("Volume = %v, want clamped to 1", p.Volume())
	}
}

func TestResamplesOtherRates(t *testing.T) {
	// A 1s track at 22050 Hz must still last about 1s at the 44100 Hz output.
	p, out := newTestPlayer(t, writeTone(t, 22050, time.Second))
	p.Play()
	out.drain(SampleRate.N(900 * time.Millisecond))
	if p.State() != Playing {
		t.Fatal("track ended too early: not resampled")
	}
	out.drain(SampleRate.N(200 * time.Millisecond))
	if p.State() != Stopped {
		t.Fatal("track should have ended by 1.1s")
	}
}

func TestNoTrack(t *testing.T) {
	p := NewWithOutput(&fakeOutput{})
	if err := p.Play(); !errors.Is(err, ErrNoTrack) {
		t.Fatalf("Play: %v", err)
	}
	if err := p.Stop(); !errors.Is(err, ErrNoTrack) {
		t.Fatalf("Stop: %v", err)
	}
	if err := p.Seek(0); !errors.Is(err, ErrNoTrack) {
		t.Fatalf("Seek: %v", err)
	}
}
