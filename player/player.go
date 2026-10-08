// Package player is the audio core of xWinAmp: it loads a track and controls
// playback (play, pause, stop, seek, volume). It knows nothing about the UI.
package player

import (
	"errors"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/speaker"
)

// SampleRate is the fixed output rate; tracks with other rates are resampled.
const SampleRate = beep.SampleRate(44100)

// resampleQuality: 4 is good quality and still cheap enough for real time.
const resampleQuality = 4

// ErrNoTrack is returned by operations that need a loaded track.
var ErrNoTrack = errors.New("no track loaded")

// State is the playback state.
type State int

const (
	Stopped State = iota
	Playing
	Paused
)

func (s State) String() string {
	switch s {
	case Playing:
		return "playing"
	case Paused:
		return "paused"
	default:
		return "stopped"
	}
}

// Output is the audio device. speakerOutput is the real one; tests use a fake.
// Lock/Unlock must guard everything Play'ed streamers touch.
type Output interface {
	Lock()
	Unlock()
	Play(s beep.Streamer)
	Clear()
}

type speakerOutput struct{}

func (speakerOutput) Lock()                { speaker.Lock() }
func (speakerOutput) Unlock()              { speaker.Unlock() }
func (speakerOutput) Play(s beep.Streamer) { speaker.Play(s) }
func (speakerOutput) Clear()               { speaker.Clear() }

// Player plays one track at a time. It is safe for concurrent use.
// All mutable state is guarded by the output lock, because the end-of-track
// callback runs on the audio goroutine with that lock already held.
type Player struct {
	out Output

	path   string
	stream beep.StreamSeekCloser
	format beep.Format
	ctrl   *beep.Ctrl
	vol    *effects.Volume
	state  State
	volume float64

	finished chan struct{}
}

// New opens the system audio device and returns a player.
func New() (*Player, error) {
	if err := speaker.Init(SampleRate, SampleRate.N(100*time.Millisecond)); err != nil {
		return nil, err
	}
	return NewWithOutput(speakerOutput{}), nil
}

// NewWithOutput returns a player that writes to out.
func NewWithOutput(out Output) *Player {
	return &Player{
		out:      out,
		volume:   1,
		finished: make(chan struct{}, 1),
	}
}

// Finished receives a value each time a track plays to its end.
func (p *Player) Finished() <-chan struct{} { return p.finished }

// Load opens path and makes it the current track, stopping any previous one.
// The new track is stopped at position 0; call Play to start it.
func (p *Player) Load(path string) error {
	s, format, err := Open(path)
	if err != nil {
		return err
	}

	p.out.Clear()
	p.out.Lock()
	old := p.stream
	p.path, p.stream, p.format = path, s, format
	p.state = Stopped
	p.ctrl = nil
	p.out.Unlock()

	if old != nil {
		old.Close()
	}
	return nil
}

// Play starts playback from the current position, or resumes after Pause.
// After the track has ended or Stop was called it starts from the beginning.
func (p *Player) Play() error {
	p.out.Lock()
	switch {
	case p.stream == nil:
		p.out.Unlock()
		return ErrNoTrack
	case p.state == Playing:
		p.out.Unlock()
		return nil
	case p.state == Paused && p.ctrl != nil:
		p.ctrl.Paused = false
		p.state = Playing
		p.out.Unlock()
		return nil
	}

	if p.stream.Position() >= p.stream.Len() {
		if err := p.stream.Seek(0); err != nil {
			p.out.Unlock()
			return err
		}
	}
	var src beep.Streamer = p.stream
	if p.format.SampleRate != SampleRate {
		src = beep.Resample(resampleQuality, p.format.SampleRate, SampleRate, src)
	}
	ctrl := &beep.Ctrl{Streamer: src}
	p.vol = &effects.Volume{Streamer: ctrl, Base: 2}
	p.applyVolume()
	p.ctrl = ctrl
	p.state = Playing
	seq := beep.Seq(p.vol, beep.Callback(func() { p.onEnd(ctrl) }))
	p.out.Unlock()

	p.out.Play(seq)
	return nil
}

// onEnd runs on the audio goroutine with the output lock held.
func (p *Player) onEnd(ctrl *beep.Ctrl) {
	if p.ctrl != ctrl {
		// A newer Play/Stop/Load replaced this stream.
		return
	}
	p.state = Stopped
	p.ctrl = nil
	select {
	case p.finished <- struct{}{}:
	default:
	}
}

// Pause pauses playback; Play resumes it.
func (p *Player) Pause() {
	p.out.Lock()
	defer p.out.Unlock()
	if p.state == Playing {
		p.ctrl.Paused = true
		p.state = Paused
	}
}

// TogglePause switches between Playing and Paused, or starts a stopped track.
func (p *Player) TogglePause() error {
	if p.State() == Playing {
		p.Pause()
		return nil
	}
	return p.Play()
}

// Stop stops playback and rewinds to the beginning.
func (p *Player) Stop() error {
	p.out.Lock()
	defer p.out.Unlock()
	if p.stream == nil {
		return ErrNoTrack
	}
	if p.ctrl != nil {
		// Detach so the audio goroutine drops this stream and its callback is ignored.
		p.ctrl.Streamer = nil
		p.ctrl = nil
	}
	p.state = Stopped
	return p.stream.Seek(0)
}

// Seek moves to position d, clamped to the track bounds.
func (p *Player) Seek(d time.Duration) error {
	p.out.Lock()
	defer p.out.Unlock()
	if p.stream == nil {
		return ErrNoTrack
	}
	n := p.format.SampleRate.N(d)
	n = max(0, min(n, p.stream.Len()-1))
	return p.stream.Seek(n)
}

// Position returns the current playback position.
func (p *Player) Position() time.Duration {
	p.out.Lock()
	defer p.out.Unlock()
	if p.stream == nil {
		return 0
	}
	return p.format.SampleRate.D(p.stream.Position())
}

// Duration returns the length of the current track.
func (p *Player) Duration() time.Duration {
	p.out.Lock()
	defer p.out.Unlock()
	if p.stream == nil {
		return 0
	}
	return p.format.SampleRate.D(p.stream.Len())
}

// State returns the current playback state.
func (p *Player) State() State {
	p.out.Lock()
	defer p.out.Unlock()
	return p.state
}

// Path returns the path of the current track, or "" if none is loaded.
func (p *Player) Path() string {
	p.out.Lock()
	defer p.out.Unlock()
	return p.path
}

// Volume returns the volume in the range [0, 1].
func (p *Player) Volume() float64 {
	p.out.Lock()
	defer p.out.Unlock()
	return p.volume
}

// SetVolume sets the volume; v is clamped to [0, 1], 0 is mute.
func (p *Player) SetVolume(v float64) {
	p.out.Lock()
	defer p.out.Unlock()
	p.volume = max(0, min(v, 1))
	p.applyVolume()
}

// volumeRange is how many halvings of amplitude the [0, 1] slider spans.
const volumeRange = 6

// applyVolume maps p.volume onto the exponential beep volume. Caller holds the lock.
func (p *Player) applyVolume() {
	if p.vol == nil {
		return
	}
	p.vol.Silent = p.volume == 0
	// 1 → 0 (no change), 0.5 → -3 (1/8 amplitude); feels roughly linear to the ear.
	p.vol.Volume = (p.volume - 1) * volumeRange
}

// Close stops playback and releases the current track.
func (p *Player) Close() error {
	p.out.Clear()
	p.out.Lock()
	s := p.stream
	p.stream, p.ctrl, p.path, p.state = nil, nil, "", Stopped
	p.out.Unlock()
	if s != nil {
		return s.Close()
	}
	return nil
}
