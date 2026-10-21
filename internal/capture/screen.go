package capture

import (
	"context"
	"fmt"
	"image"
	"runtime"

	"github.com/kbinani/screenshot"

	"github.com/Miwm64/peekai/internal/config"
)

// New returns the Capturer for cfg.
func New(cfg config.Capture) (Capturer, error) {
	return &screen{display: cfg.Display}, nil
}

// screen captures real displays via kbinani/screenshot.
type screen struct {
	display string
}

func (s *screen) Capture(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := screenshot.NumActiveDisplays()
	bounds := make([]image.Rectangle, n)
	for i := range n {
		bounds[i] = screenshot.GetDisplayBounds(i)
	}
	r, err := rectFor(s.display, bounds)
	if err != nil {
		return nil, err
	}
	img, err := screenshot.CaptureRect(r)
	if err != nil && runtime.GOOS == "darwin" {
		return nil, fmt.Errorf("%w (allow Screen Recording for this app in System Settings → Privacy & Security)", err)
	}
	return img, err
}
