// Package ocr extracts text from screenshots.
package ocr

import (
	"context"
	"fmt"
	"image"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/config"
)

// Engine turns an image into text.
type Engine interface {
	Recognize(ctx context.Context, img image.Image) (string, error)
}

// New returns the engine named by cfg.Engine. The llm engine reads text with
// the given AI provider, so it needs a vision-capable model.
func New(cfg config.OCR, provider ai.Provider) (Engine, error) {
	switch cfg.Engine {
	case "tesseract_cli":
		return newTesseract(cfg)
	case "llm":
		return &llm{provider: provider}, nil
	}
	return nil, fmt.Errorf("ocr.engine: unknown %q (want tesseract_cli or llm)", cfg.Engine)
}
