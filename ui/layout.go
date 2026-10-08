// Package ui is the xWinAmp main window. Controller holds the window logic
// and does not depend on Ebitengine, so it is unit tested; Window draws it
// and feeds it input.
package ui

import "image"

// Logical size of the main window, the same as the classic Winamp one.
// It is drawn at this size and scaled up by the window. The playlist panel,
// when shown, adds PanelHeight below it.
const (
	Width       = 275
	Height      = 116
	PanelHeight = 140
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
	BtnShuffle
	BtnRepeat
	BtnPlaylist
	BtnAdd
	BtnRemove
	BtnLoad
	BtnSave
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

	panelRect     = image.Rect(0, Height, Width, Height+PanelHeight)
	listRect      = image.Rect(8, Height+4, 260, Height+108)
	scrollbarRect = image.Rect(261, Height+4, 267, Height+108)
	panelInfoRect = image.Rect(156, Height+114, 267, Height+132)

	buttonRects = map[Button]image.Rectangle{
		BtnPrev:     image.Rect(16, 88, 39, 106),
		BtnPlay:     image.Rect(39, 88, 62, 106),
		BtnPause:    image.Rect(62, 88, 85, 106),
		BtnStop:     image.Rect(85, 88, 108, 106),
		BtnNext:     image.Rect(108, 88, 130, 106),
		BtnOpen:     image.Rect(136, 89, 158, 105),
		BtnShuffle:  image.Rect(166, 89, 200, 105),
		BtnRepeat:   image.Rect(202, 89, 228, 105),
		BtnPlaylist: image.Rect(236, 89, 260, 105),

		BtnAdd:    image.Rect(8, Height+114, 38, Height+132),
		BtnRemove: image.Rect(40, Height+114, 70, Height+132),
		BtnLoad:   image.Rect(72, Height+114, 108, Height+132),
		BtnSave:   image.Rect(110, Height+114, 146, Height+132),
	}

	// buttonLabels are drawn as text; the others get icons.
	buttonLabels = map[Button]string{
		BtnShuffle:  "SHUF",
		BtnRepeat:   "REP",
		BtnPlaylist: "PL",
		BtnAdd:      "ADD",
		BtnRemove:   "REM",
		BtnLoad:     "LOAD",
		BtnSave:     "SAVE",
	}
)

// rowHeight is the height of one playlist row; listRect holds visibleRows.
const (
	rowHeight   = 13
	visibleRows = 8
)

// inPanel reports whether b belongs to the playlist panel.
func inPanel(b Button) bool { return buttonRects[b].Min.Y >= Height }

// buttonAt returns the button under p, or NoButton. Panel buttons only
// count when the panel is shown.
func buttonAt(p image.Point, panel bool) Button {
	for b, r := range buttonRects {
		if p.In(r) && (panel || !inPanel(b)) {
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
