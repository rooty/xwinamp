# xWinAmp

A Winamp-style audio player written in Go. macOS first, Linux and Windows later.

Status: step 2 of 4. The main window plays one file at a time; the playlist comes next.

![xWinAmp main window](docs/screenshot.png)

## Layout

- `player/` - audio core: load a track, play, pause, stop, seek, volume. No UI code.
- `ui/` - main window on Ebitengine. `Controller` holds the logic (tested without a display), `draw.go` is the built-in skin.
- `cmd/xwinamp/` - the desktop player.
- `cmd/xwinamp-cli/` - terminal player built on `player`.

Supported formats: MP3, WAV, FLAC, OGG Vorbis.

## Try it

```sh
go run ./cmd/xwinamp ~/Music/song.mp3
```

Or start it without arguments and drop a file onto the window.

| Key | Action |
| --- | --- |
| Z / X / C / V / B | previous (restart) / play / pause / stop / next |
| Space | pause / resume |
| ← / → | seek ∓5 s |
| ↑ / ↓ | volume ±5 % |

Click the time to switch between elapsed and remaining. `-scale 1..4` sets the window size (default 2).

Terminal player:

```sh
go run ./cmd/xwinamp-cli ~/Music/song.mp3 ~/Music/other.flac
```

While playing, type a command and press Enter: `p` pause/resume, `s` stop,
`n` next, `f`/`b` seek ±10s, `+`/`-` volume, `q` quit.

## Build requirements

- Go 1.25+ (required by Ebitengine 2.10)
- macOS and Windows: nothing else (no cgo needed).
- Linux: cgo plus ALSA and X11/OpenGL headers, e.g.
  `sudo apt install libasound2-dev libgl1-mesa-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxxf86vm-dev`.

## Tests

```sh
go test -race ./...
```

Tests use a fake audio output and a fake player, so they run without a sound card or a display.
