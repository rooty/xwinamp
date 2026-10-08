# xWinAmp

A Winamp-style audio player written in Go. macOS first, Linux and Windows later.

Status: step 1 of 4, the audio core. There is no GUI yet; a terminal player is provided to try it out.

## Layout

- `player/` - audio core: load a track, play, pause, stop, seek, volume. No UI code.
- `cmd/xwinamp-cli/` - terminal player built on `player`.

Supported formats: MP3, WAV, FLAC, OGG Vorbis.

## Try it

```sh
go run ./cmd/xwinamp-cli ~/Music/song.mp3 ~/Music/other.flac
```

While playing, type a command and press Enter: `p` pause/resume, `s` stop,
`n` next, `f`/`b` seek ±10s, `+`/`-` volume, `q` quit.

## Build requirements

- Go 1.24+
- macOS and Windows: nothing else (no cgo needed).
- Linux: ALSA headers and cgo, e.g. `sudo apt install libasound2-dev`.

## Tests

```sh
go test -race ./...
```

Tests use a fake audio output, so they run without a sound card.
