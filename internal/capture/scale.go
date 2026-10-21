package capture

import (
	"image"

	"golang.org/x/image/draw"
)

// Downscale shrinks img so its longer side is at most maxPx, keeping the
// aspect ratio. maxPx <= 0 or a smaller image returns img unchanged.
func Downscale(img image.Image, maxPx int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxPx <= 0 || max(w, h) <= maxPx {
		return img
	}
	if w >= h {
		w, h = maxPx, max(1, h*maxPx/w)
	} else {
		w, h = max(1, w*maxPx/h), maxPx
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
