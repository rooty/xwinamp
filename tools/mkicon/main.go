// Command mkicon draws the xWinAmp app icon and writes it as a PNG.
//
//	go run ./tools/mkicon -size 1024 -o icon.png
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

var (
	colBg     = color.RGBA{0x2a, 0x2a, 0x3a, 0xff}
	colBorder = color.RGBA{0x5a, 0x5a, 0x78, 0xff}
	colScreen = color.RGBA{0x00, 0x00, 0x00, 0xff}
	colLCD    = color.RGBA{0x00, 0xe0, 0x00, 0xff}
)

func main() {
	size := flag.Int("size", 1024, "icon size in pixels")
	out := flag.String("o", "icon.png", "output PNG file")
	flag.Parse()

	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, draw(*size)); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

// draw renders the icon: a rounded dark body like the player window with a
// black screen and a green play triangle, in the macOS icon grid (the body
// covers about 80% of the canvas).
func draw(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	margin := s * 0.1
	body := rect{margin, margin, s - margin, s - margin}
	screen := body.inset(s * 0.09)
	tri := screen.inset(s * 0.14)

	for y := range size {
		for x := range size {
			// Sample at the pixel centre with 4x4 supersampling for smooth edges.
			var r, g, b, a float64
			for sy := range 4 {
				for sx := range 4 {
					px := float64(x) + (float64(sx)+0.5)/4
					py := float64(y) + (float64(sy)+0.5)/4
					c, ok := sample(px, py, s, body, screen, tri)
					if ok {
						r += float64(c.R)
						g += float64(c.G)
						b += float64(c.B)
						a += 255
					}
				}
			}
			if a > 0 {
				// Premultiplied: colours are summed only where covered.
				img.SetRGBA(x, y, color.RGBA{uint8(r / 16), uint8(g / 16), uint8(b / 16), uint8(a / 16)})
			}
		}
	}
	return img
}

func sample(x, y, s float64, body, screen, tri rect) (color.RGBA, bool) {
	radius := s * 0.18
	if !body.containsRounded(x, y, radius) {
		return color.RGBA{}, false
	}
	if !body.inset(s*0.02).containsRounded(x, y, radius*0.9) {
		return colBorder, true
	}
	if !screen.containsRounded(x, y, s*0.04) {
		return colBg, true
	}
	// Play triangle pointing right, centred in the screen.
	t := tri
	if x >= t.x0+t.w()*0.08 && math.Abs(y-t.cy())*2 <= t.h()*(1-(x-t.x0-t.w()*0.08)/(t.w()*0.84)) {
		return colLCD, true
	}
	return colScreen, true
}

type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) w() float64  { return r.x1 - r.x0 }
func (r rect) h() float64  { return r.y1 - r.y0 }
func (r rect) cy() float64 { return (r.y0 + r.y1) / 2 }

func (r rect) inset(d float64) rect { return rect{r.x0 + d, r.y0 + d, r.x1 - d, r.y1 - d} }

func (r rect) containsRounded(x, y, rad float64) bool {
	if x < r.x0 || x > r.x1 || y < r.y0 || y > r.y1 {
		return false
	}
	cx := math.Max(r.x0+rad, math.Min(x, r.x1-rad))
	cy := math.Max(r.y0+rad, math.Min(y, r.y1-rad))
	return math.Hypot(x-cx, y-cy) <= rad
}
