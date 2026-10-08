package ui

import (
	"image"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// keyActions are the classic Winamp hotkeys.
var keyActions = map[ebiten.Key]Action{
	ebiten.KeyZ:          ActPrev,
	ebiten.KeyX:          ActPlay,
	ebiten.KeyC:          ActPause,
	ebiten.KeySpace:      ActPause,
	ebiten.KeyV:          ActStop,
	ebiten.KeyB:          ActNext,
	ebiten.KeyL:          ActOpen,
	ebiten.KeyArrowLeft:  ActSeekBack,
	ebiten.KeyArrowRight: ActSeekForward,
	ebiten.KeyArrowUp:    ActVolumeUp,
	ebiten.KeyArrowDown:  ActVolumeDown,
}

// Window is the Ebitengine game that shows the main window.
type Window struct {
	c *Controller
}

// NewWindow returns a window driven by c.
func NewWindow(c *Controller) *Window {
	return &Window{c: c}
}

// Run opens the window at the given scale and blocks until it is closed.
func (w *Window) Run(scale int) error {
	ebiten.SetWindowTitle("xWinAmp")
	ebiten.SetWindowSize(Width*scale, Height*scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Keep pixels crisp when scaled up.
	ebiten.SetScreenFilterEnabled(false)
	// Nothing animates fast; save CPU.
	ebiten.SetTPS(30)
	return ebiten.RunGame(w)
}

// Update implements ebiten.Game.
func (w *Window) Update() error {
	if files := droppedPaths(); len(files) > 0 {
		w.c.Open(files)
	}

	for k, a := range keyActions {
		if inpututil.IsKeyJustPressed(k) || isKeyRepeated(k, a) {
			w.c.Do(a)
		}
	}

	p := image.Pt(ebiten.CursorPosition())
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		w.c.MouseDown(p)
	case inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft):
		w.c.MouseUp(p)
	case ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft):
		w.c.MouseMove(p)
	}

	w.c.Tick()
	return nil
}

// isKeyRepeated makes seek and volume keys repeat while held.
func isKeyRepeated(k ebiten.Key, a Action) bool {
	switch a {
	case ActSeekBack, ActSeekForward, ActVolumeUp, ActVolumeDown:
		d := inpututil.KeyPressDuration(k)
		return d > 15 && d%3 == 0
	}
	return false
}

// droppedPaths returns the real paths of files dropped onto the window this tick.
func droppedPaths() []string {
	fsys := ebiten.DroppedFiles()
	if fsys == nil {
		return nil
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue // folders come with the playlist
		}
		if ap, ok := e.(ebiten.AbsPather); ok {
			paths = append(paths, ap.AbsPath())
		}
	}
	return paths
}

// Draw implements ebiten.Game.
func (w *Window) Draw(screen *ebiten.Image) {
	drawWindow(screen, w.c.View())
}

// Layout implements ebiten.Game: always draw at the logical size and let
// Ebitengine scale it to the window.
func (w *Window) Layout(_, _ int) (int, int) {
	return Width, Height
}
