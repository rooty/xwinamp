// Package ui is the xWinAmp main window. Controller holds the window logic
// and does not depend on Ebitengine, so it is unit tested; Window draws it
// and feeds it input.
package ui

import "image"

// Logical size of the main window, the same as the classic Winamp one.
// It is drawn at this size and scaled up by the window.
const (
	Width  = 275
	Height = 116
)

// Button identifies a clickable button.
type Button int

const (
	NoButton Button = iota
	BtnPrev
	BtnPlay
	BtnPause
	BtnStop
	BtnNext
	BtnOpen
)

// Control rectangles in logical pixels.
var (
	titleBarRect = image.Rect(0, 0, Width, 14)
	displayRect  = image.Rect(9, 20, 102, 62)
	timeRect     = image.Rect(36, 26, 99, 46)
	marqueeRect  = image.Rect(110, 22, 266, 37)
	infoRect     = image.Rect(110, 40, 266, 52)
	volumeRect   = image.Rect(110, 56, 190, 64)
	seekRect     = image.Rect(16, 72, 260, 82)

	buttonRects = map[Button]image.Rectangle{
		BtnPrev:  image.Rect(16, 88, 39, 106),
		BtnPlay:  image.Rect(39, 88, 62, 106),
		BtnPause: image.Rect(62, 88, 85, 106),
		BtnStop:  image.Rect(85, 88, 108, 106),
		BtnNext:  image.Rect(108, 88, 130, 106),
		BtnOpen:  image.Rect(136, 89, 158, 105),
	}
)

// buttonAt returns the button under p, or NoButton.
func buttonAt(p image.Point) Button {
	for b, r := range buttonRects {
		if p.In(r) {
			return b
		}
	}
	return NoButton
}

// sliderFrac maps x to [0, 1] across r.
func sliderFrac(r image.Rectangle, x int) float64 {
	f := float64(x-r.Min.X) / float64(r.Dx()-1)
	return max(0, min(f, 1))
}
