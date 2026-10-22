// Package icon draws peekai's icon: a cat peeking over a ledge. It is drawn
// in code so the tray, the windows and the macOS app share one source at
// any size.
package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"golang.org/x/image/vector"
)

var (
	fur      = color.NRGBA{0xF6, 0xA2, 0x4F, 0xFF}
	furDark  = color.NRGBA{0xD9, 0x7F, 0x2E, 0xFF}
	earPink  = color.NRGBA{0xFF, 0xB8, 0xC2, 0xFF}
	nosePink = color.NRGBA{0xF0, 0x6C, 0x8A, 0xFF}
	eyeWhite = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	pupil    = color.NRGBA{0x2B, 0x22, 0x3A, 0xFF}
	ledge    = color.NRGBA{0x4B, 0x3F, 0x72, 0xFF}
	ledgeTop = color.NRGBA{0x6A, 0x5C, 0x9C, 0xFF}
	tileTop  = color.NRGBA{0xB9, 0xD7, 0xFF, 0xFF}
	tileLow  = color.NRGBA{0xE4, 0xC8, 0xFF, 0xFF}
)

// canvas draws shapes given in a 100×100 design space.
type canvas struct {
	dst   *image.NRGBA
	scale float32
	off   float32 // design-space offset for drawing inside a tile
	k     float32 // shrink factor for the cat inside a tile
}

func (c *canvas) pt(x, y float32) (float32, float32) {
	return (c.off + x*c.k) * c.scale, (c.off + y*c.k) * c.scale
}

func (c *canvas) fill(build func(r *vector.Rasterizer), src image.Image) {
	b := c.dst.Bounds()
	r := vector.NewRasterizer(b.Dx(), b.Dy())
	build(r)
	r.Draw(c.dst, b, src, image.Point{})
}

func (c *canvas) uniform(col color.NRGBA, build func(r *vector.Rasterizer)) {
	c.fill(build, image.NewUniform(col))
}

// ellipse adds an ellipse made of four cubic curves.
func (c *canvas) ellipse(r *vector.Rasterizer, cx, cy, rx, ry float32) {
	const k = 0.5523 // cubic approximation of a quarter circle
	p := func(x, y float32) (float32, float32) { return c.pt(x, y) }
	r.MoveTo(p(cx+rx, cy))
	x1, y1 := p(cx+rx, cy+ry*k)
	x2, y2 := p(cx+rx*k, cy+ry)
	x3, y3 := p(cx, cy+ry)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = p(cx-rx*k, cy+ry)
	x2, y2 = p(cx-rx, cy+ry*k)
	x3, y3 = p(cx-rx, cy)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = p(cx-rx, cy-ry*k)
	x2, y2 = p(cx-rx*k, cy-ry)
	x3, y3 = p(cx, cy-ry)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = p(cx+rx*k, cy-ry)
	x2, y2 = p(cx+rx, cy-ry*k)
	x3, y3 = p(cx+rx, cy)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	r.ClosePath()
}

func (c *canvas) poly(r *vector.Rasterizer, pts ...float32) {
	r.MoveTo(c.pt(pts[0], pts[1]))
	for i := 2; i+1 < len(pts); i += 2 {
		r.LineTo(c.pt(pts[i], pts[i+1]))
	}
	r.ClosePath()
}

// roundRect adds a rectangle with rounded corners.
func (c *canvas) roundRect(r *vector.Rasterizer, x, y, w, h, rad float32) {
	const k = 0.5523
	p := func(x, y float32) (float32, float32) { return c.pt(x, y) }
	r.MoveTo(p(x+rad, y))
	r.LineTo(p(x+w-rad, y))
	a, b := p(x+w-rad+rad*k, y)
	d, e := p(x+w, y+rad-rad*k)
	f, g := p(x+w, y+rad)
	r.CubeTo(a, b, d, e, f, g)
	r.LineTo(p(x+w, y+h-rad))
	a, b = p(x+w, y+h-rad+rad*k)
	d, e = p(x+w-rad+rad*k, y+h)
	f, g = p(x+w-rad, y+h)
	r.CubeTo(a, b, d, e, f, g)
	r.LineTo(p(x+rad, y+h))
	a, b = p(x+rad-rad*k, y+h)
	d, e = p(x, y+h-rad+rad*k)
	f, g = p(x, y+h-rad)
	r.CubeTo(a, b, d, e, f, g)
	r.LineTo(p(x, y+rad))
	a, b = p(x, y+rad-rad*k)
	d, e = p(x+rad-rad*k, y)
	f, g = p(x+rad, y)
	r.CubeTo(a, b, d, e, f, g)
	r.ClosePath()
}

// gradient is a vertical two-color gradient over the image height.
type gradient struct {
	top, low color.NRGBA
	h        int
}

func (g gradient) ColorModel() color.Model { return color.NRGBAModel }
func (g gradient) Bounds() image.Rectangle {
	return image.Rect(math.MinInt32, math.MinInt32, math.MaxInt32, math.MaxInt32)
}

func (g gradient) At(_, y int) color.Color {
	t := min(max(float64(y)/float64(g.h), 0), 1)
	mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	return color.NRGBA{mix(g.top.R, g.low.R), mix(g.top.G, g.low.G), mix(g.top.B, g.low.B), 255}
}

// cat draws the peeking cat in design space.
func (c *canvas) cat() {
	// Ears behind the head.
	c.uniform(fur, func(r *vector.Rasterizer) {
		c.poly(r, 22, 46, 26, 14, 46, 34)
		c.poly(r, 78, 46, 74, 14, 54, 34)
	})
	c.uniform(earPink, func(r *vector.Rasterizer) {
		c.poly(r, 27, 40, 29, 23, 40, 34)
		c.poly(r, 73, 40, 71, 23, 60, 34)
	})
	// Head, then the ledge hides its lower half: the cat peeks over it.
	c.uniform(fur, func(r *vector.Rasterizer) { c.ellipse(r, 50, 58, 32, 28) })
	c.uniform(furDark, func(r *vector.Rasterizer) { // forehead stripes
		c.poly(r, 47, 31, 53, 31, 51, 41, 49, 41)
		c.poly(r, 39, 33, 43, 32, 43, 39, 41, 40)
		c.poly(r, 61, 33, 57, 32, 57, 39, 59, 40)
	})
	// Big eyes looking up and to the side, curious.
	c.uniform(eyeWhite, func(r *vector.Rasterizer) {
		c.ellipse(r, 38, 55, 9, 10)
		c.ellipse(r, 62, 55, 9, 10)
	})
	c.uniform(pupil, func(r *vector.Rasterizer) {
		c.ellipse(r, 40.5, 54, 5.5, 6.5)
		c.ellipse(r, 64.5, 54, 5.5, 6.5)
	})
	c.uniform(eyeWhite, func(r *vector.Rasterizer) {
		c.ellipse(r, 42.5, 51, 1.8, 1.8)
		c.ellipse(r, 66.5, 51, 1.8, 1.8)
	})
	c.uniform(nosePink, func(r *vector.Rasterizer) { c.poly(r, 46.5, 66, 53.5, 66, 50, 70) })
	// The ledge with a lighter top edge, and two paws holding on to it.
	c.uniform(ledge, func(r *vector.Rasterizer) { c.roundRect(r, 4, 72, 92, 26, 6) })
	c.uniform(ledgeTop, func(r *vector.Rasterizer) { c.roundRect(r, 4, 72, 92, 5, 2.5) })
	c.uniform(fur, func(r *vector.Rasterizer) {
		c.ellipse(r, 33, 73, 9, 6)
		c.ellipse(r, 67, 73, 9, 6)
	})
	c.uniform(furDark, func(r *vector.Rasterizer) {
		for _, x := range []float32{30, 34, 64, 68} {
			c.roundRect(r, x, 71, 1.2, 5, 0.6)
		}
	})
}

// Render draws the icon at size×size pixels. With tile, the cat sits on a
// rounded gradient square (app icon); without, it stands alone (tray).
func Render(size int, tile bool) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	c := &canvas{dst: dst, scale: float32(size) / 100, k: 1}
	if tile {
		// macOS icon grid: the tile fills ~80% of the canvas.
		c.fill(func(r *vector.Rasterizer) { c.roundRect(r, 10, 10, 80, 80, 18) },
			gradient{top: tileTop, low: tileLow, h: size})
		c.off, c.k = 18, 0.64
	}
	c.cat()
	return dst
}

// PNG returns Render(size, tile) as PNG bytes.
func PNG(size int, tile bool) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, Render(size, tile))
	return buf.Bytes()
}
