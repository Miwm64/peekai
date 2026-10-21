// Package capture grabs screen images.
package capture

import (
	"context"
	"fmt"
	"image"
)

// Capturer takes a screenshot.
type Capturer interface {
	Capture(ctx context.Context) (image.Image, error)
}

// rectFor picks the area to capture from the display bounds.
// "active" is the primary display for now; following the cursor needs the
// UI layer and comes later.
func rectFor(display string, bounds []image.Rectangle) (image.Rectangle, error) {
	if len(bounds) == 0 {
		return image.Rectangle{}, fmt.Errorf("no active displays found")
	}
	switch display {
	case "active":
		return bounds[0], nil
	case "all":
		r := bounds[0]
		for _, b := range bounds[1:] {
			r = r.Union(b)
		}
		return r, nil
	}
	var i int
	if _, err := fmt.Sscan(display, &i); err != nil || i < 0 || i >= len(bounds) {
		return image.Rectangle{}, fmt.Errorf("display %q not found (%d displays)", display, len(bounds))
	}
	return bounds[i], nil
}
