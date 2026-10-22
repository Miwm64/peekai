// Package pipeline runs one analysis: capture → [OCR] → AI.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/capture"
	"github.com/Miwm64/peekai/internal/config"
)

// Recognizer extracts text from an image (an OCR engine).
type Recognizer interface {
	Recognize(ctx context.Context, img image.Image) (string, error)
}

// Pipeline holds the stages. OCR may be nil when only image mode is used.
type Pipeline struct {
	Capturer   capture.Capturer
	OCR        Recognizer
	AI         ai.Provider
	MaxImagePx int
}

// Options are the per-run choices the user can switch with hotkeys.
type Options struct {
	Mode    string    // config.ModeImage or config.ModeOCR
	Prompt  string    // prompt template
	History []ai.Turn // earlier conversation to send along
}

// Result is what one run produced.
type Result struct {
	Answer  string
	Turn    ai.Turn // this exchange, to append to the history
	Mode    string
	Image   image.Image // the (downscaled) screenshot
	Text    string      // OCR text, ocr mode only
	Elapsed time.Duration
}

var ErrNoOCR = errors.New("ocr mode needs an OCR engine")

// Run captures the screen and asks the AI about it.
func (p *Pipeline) Run(ctx context.Context, opts Options) (Result, error) {
	start := time.Now()
	res := Result{Mode: opts.Mode}

	if opts.Mode == config.ModeOCR && p.OCR == nil {
		return res, ErrNoOCR
	}

	img, err := p.Capturer.Capture(ctx)
	if err != nil {
		return res, fmt.Errorf("capture: %w", err)
	}
	res.Image = capture.Downscale(img, p.MaxImagePx)

	req := ai.Request{}
	switch opts.Mode {
	case config.ModeOCR:
		// OCR the full-resolution image; small text survives better.
		if res.Text, err = p.OCR.Recognize(ctx, img); err != nil {
			return res, fmt.Errorf("ocr: %w", err)
		}
		req.Text = res.Text
	default:
		req.Image = res.Image
	}

	prompt, usesText, err := renderPrompt(opts.Prompt, promptData{Text: res.Text, Mode: opts.Mode, Time: now()})
	if err != nil {
		return res, fmt.Errorf("prompt: %w", err)
	}
	req.Prompt = prompt
	if usesText {
		req.Text = ""
	}
	req.History = opts.History

	if res.Answer, err = p.AI.Analyze(ctx, req); err != nil {
		return res, fmt.Errorf("ai: %w", err)
	}
	res.Turn = req.Turn(res.Answer)
	res.Elapsed = time.Since(start)
	return res, nil
}
