package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"

	"github.com/rooty/xwinamp/player"
)

// The built-in skin: colours in the spirit of the classic Winamp one.
var (
	colBody      = color.RGBA{0x2a, 0x2a, 0x3a, 0xff}
	colBevelHi   = color.RGBA{0x5a, 0x5a, 0x78, 0xff}
	colBevelLo   = color.RGBA{0x12, 0x12, 0x1c, 0xff}
	colTitle     = color.RGBA{0x1c, 0x1c, 0x58, 0xff}
	colTitleLine = color.RGBA{0x40, 0x40, 0x90, 0xff}
	colTitleText = color.RGBA{0xd8, 0xd8, 0xff, 0xff}
	colScreen    = color.RGBA{0x00, 0x00, 0x00, 0xff}
	colLCD       = color.RGBA{0x00, 0xe0, 0x00, 0xff}
	colLCDDim    = color.RGBA{0x0c, 0x30, 0x0c, 0xff}
	colLCDInfo   = color.RGBA{0x00, 0xa0, 0x00, 0xff}
	colGroove    = color.RGBA{0x10, 0x10, 0x18, 0xff}
	colKnob      = color.RGBA{0xc8, 0xc0, 0x90, 0xff}
	colBtnFace   = color.RGBA{0xb8, 0xb8, 0xc8, 0xff}
	colBtnDown   = color.RGBA{0x90, 0x90, 0xa0, 0xff}
	colBtnHi     = color.RGBA{0xee, 0xee, 0xf6, 0xff}
	colBtnLo     = color.RGBA{0x50, 0x50, 0x60, 0xff}
	colIcon      = color.RGBA{0x20, 0x20, 0x30, 0xff}
)

var font = text.NewGoXFace(basicfont.Face7x13)

func drawWindow(dst *ebiten.Image, v View) {
	dst.Fill(colBody)
	bevel(dst, dst.Bounds(), colBevelHi, colBevelLo)

	drawTitleBar(dst)
	drawDisplay(dst, v)
	drawMarquee(dst, v)
	drawText(dst, infoRect, v.Info, colLCDInfo, 0)
	drawVolume(dst, v.Volume)
	drawSeek(dst, v)
	for b, r := range buttonRects {
		drawButton(dst, b, r, v.Pressed == b)
	}
}

func drawTitleBar(dst *ebiten.Image) {
	r := titleBarRect.Inset(1)
	fillRect(dst, r, colTitle)
	// Winamp-style grooves on both sides of the title.
	for y := r.Min.Y + 3; y < r.Max.Y-2; y += 3 {
		fillRect(dst, image.Rect(r.Min.X+4, y, r.Max.X-4, y+1), colTitleLine)
	}
	const title = "xWinAmp"
	w, _ := text.Measure(title, font, 0)
	tr := image.Rect(Width/2-int(w)/2-4, r.Min.Y, Width/2+int(w)/2+4, r.Max.Y)
	fillRect(dst, tr, colTitle)
	drawText(dst, tr, title, colTitleText, 4)
}

func drawDisplay(dst *ebiten.Image, v View) {
	fillRect(dst, displayRect, colScreen)
	bevel(dst, displayRect.Inset(-1), colBevelLo, colBevelHi)

	// Play state icon.
	x, y := float32(displayRect.Min.X+8), float32(timeRect.Min.Y+3)
	switch v.State {
	case player.Playing:
		fillPath(dst, colLCD, x, y, x+8, y+5, x, y+10)
	case player.Paused:
		vector.FillRect(dst, x, y+1, 3, 9, colLCD, false)
		vector.FillRect(dst, x+5, y+1, 3, 9, colLCD, false)
	default:
		vector.FillRect(dst, x, y+1, 8, 8, colLCD, false)
	}

	drawLCDTime(dst, timeRect, v.Time)
}

func drawMarquee(dst *ebiten.Image, v View) {
	fillRect(dst, marqueeRect, colScreen)
	bevel(dst, marqueeRect.Inset(-1), colBevelLo, colBevelHi)

	area := dst.SubImage(marqueeRect.Inset(2)).(*ebiten.Image)
	w, _ := text.Measure(v.Title, font, 0)
	if int(w) <= area.Bounds().Dx() {
		drawText(area, area.Bounds(), v.Title, colLCD, 0)
		return
	}
	// Scroll in a loop with a gap between the end and the next start.
	const gap = 30
	period := int(w) + gap
	off := v.Scroll % period
	r := area.Bounds()
	drawText(area, r.Sub(image.Pt(off, 0)), v.Title, colLCD, 0)
	drawText(area, r.Add(image.Pt(period-off, 0)), v.Title, colLCD, 0)
}

func drawVolume(dst *ebiten.Image, vol float64) {
	r := volumeRect
	groove := image.Rect(r.Min.X, r.Min.Y+2, r.Max.X, r.Max.Y-2)
	fillRect(dst, groove, colGroove)
	// Fill from green to red like the Winamp volume bar.
	fill := groove
	fill.Max.X = fill.Min.X + int(vol*float64(groove.Dx()))
	fillRect(dst, fill.Inset(1), lerpColor(colLCD, color.RGBA{0xe0, 0x30, 0x00, 0xff}, vol))

	kx := r.Min.X + int(vol*float64(r.Dx()-6))
	knob := image.Rect(kx, r.Min.Y-1, kx+6, r.Max.Y+1)
	fillRect(dst, knob, colKnob)
	bevel(dst, knob, colBtnHi, colBtnLo)
}

func drawSeek(dst *ebiten.Image, v View) {
	r := seekRect
	fillRect(dst, r, colGroove)
	bevel(dst, r, colBevelLo, colBevelHi)
	if !v.HasTrack {
		return
	}
	const kw = 28
	kx := r.Min.X + int(v.Seek*float64(r.Dx()-kw))
	knob := image.Rect(kx, r.Min.Y, kx+kw, r.Max.Y)
	fillRect(dst, knob, colKnob)
	bevel(dst, knob, colBtnHi, colBtnLo)
}

func drawButton(dst *ebiten.Image, b Button, r image.Rectangle, down bool) {
	face, hi, lo := colBtnFace, colBtnHi, colBtnLo
	if down {
		face, hi, lo = colBtnDown, colBtnLo, colBtnHi
	}
	fillRect(dst, r, face)
	bevel(dst, r, hi, lo)

	c := r.Min.Add(r.Size().Div(2))
	if down {
		c = c.Add(image.Pt(1, 1))
	}
	x, y := float32(c.X), float32(c.Y)
	switch b {
	case BtnPrev:
		vector.FillRect(dst, x-5, y-4, 2, 8, colIcon, false)
		fillPath(dst, colIcon, x+4, y-4, x+4, y+4, x-3, y)
	case BtnPlay:
		fillPath(dst, colIcon, x-3, y-5, x+4, y, x-3, y+5)
	case BtnPause:
		vector.FillRect(dst, x-4, y-4, 3, 8, colIcon, false)
		vector.FillRect(dst, x+1, y-4, 3, 8, colIcon, false)
	case BtnStop:
		vector.FillRect(dst, x-4, y-4, 8, 8, colIcon, false)
	case BtnNext:
		fillPath(dst, colIcon, x-4, y-4, x-4, y+4, x+3, y)
		vector.FillRect(dst, x+3, y-4, 2, 8, colIcon, false)
	case BtnOpen:
		fillPath(dst, colIcon, x-5, y+1, x, y-5, x+5, y+1)
		vector.FillRect(dst, x-5, y+3, 10, 2, colIcon, false)
	}
}

// drawLCDTime draws s ("m:ss", "-m:ss") right-aligned in r with
// seven-segment digits.
func drawLCDTime(dst *ebiten.Image, r image.Rectangle, s string) {
	x := r.Max.X
	for i := len(s) - 1; i >= 0; i-- {
		switch ch := s[i]; {
		case ch == ':':
			x -= 5
			vector.FillRect(dst, float32(x+1), float32(r.Min.Y+4), 2, 2, colLCD, false)
			vector.FillRect(dst, float32(x+1), float32(r.Min.Y+10), 2, 2, colLCD, false)
		case ch == '-':
			x -= 11
			drawSegments(dst, x, r.Min.Y, segG)
		case ch >= '0' && ch <= '9':
			x -= 11
			drawSegments(dst, x, r.Min.Y, digitSegments[ch-'0'])
		}
	}
}

// Seven-segment bits: a top, b top-right, c bottom-right, d bottom,
// e bottom-left, f top-left, g middle.
const (
	segA = 1 << iota
	segB
	segC
	segD
	segE
	segF
	segG
)

var digitSegments = [10]int{
	segA | segB | segC | segD | segE | segF,
	segB | segC,
	segA | segB | segG | segE | segD,
	segA | segB | segG | segC | segD,
	segF | segG | segB | segC,
	segA | segF | segG | segC | segD,
	segA | segF | segG | segE | segC | segD,
	segA | segB | segC,
	segA | segB | segC | segD | segE | segF | segG,
	segA | segB | segC | segD | segF | segG,
}

// Segment rectangles in a 9x15 cell.
var segmentRects = map[int]image.Rectangle{
	segA: image.Rect(1, 0, 8, 2),
	segB: image.Rect(7, 1, 9, 7),
	segC: image.Rect(7, 8, 9, 14),
	segD: image.Rect(1, 13, 8, 15),
	segE: image.Rect(0, 8, 2, 14),
	segF: image.Rect(0, 1, 2, 7),
	segG: image.Rect(1, 6, 8, 8),
}

func drawSegments(dst *ebiten.Image, x, y, on int) {
	for seg, r := range segmentRects {
		c := colLCDDim
		if on&seg != 0 {
			c = colLCD
		}
		fillRect(dst, r.Add(image.Pt(x, y)), c)
	}
}

// drawText draws s in r, vertically centred, starting padX pixels in.
func drawText(dst *ebiten.Image, r image.Rectangle, s string, c color.Color, padX int) {
	if s == "" {
		return
	}
	_, h := text.Measure(s, font, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(r.Min.X+padX), float64(r.Min.Y)+(float64(r.Dy())-h)/2)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, font, op)
}

func fillRect(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}

// bevel draws a 1px 3D edge: hi on top/left, lo on bottom/right.
func bevel(dst *ebiten.Image, r image.Rectangle, hi, lo color.Color) {
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), hi)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), hi)
	fillRect(dst, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), lo)
	fillRect(dst, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), lo)
}

// fillPath fills the polygon given as x, y pairs.
func fillPath(dst *ebiten.Image, c color.Color, xy ...float32) {
	var p vector.Path
	p.MoveTo(xy[0], xy[1])
	for i := 2; i < len(xy); i += 2 {
		p.LineTo(xy[i], xy[i+1])
	}
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(dst, &p, nil, op)
}

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 0xff}
}
