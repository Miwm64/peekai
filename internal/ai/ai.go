// Package ai sends a prompt plus screen text or image to an AI model.
package ai

import (
	"context"
	"image"
)

// Request is one analysis call. Text and Image are optional; usually one of
// them is set depending on the capture mode.
type Request struct {
	Prompt string
	Text   string      // OCR text of the screen
	Image  image.Image // the screenshot
}

// Provider returns the model's answer for a request.
type Provider interface {
	Analyze(ctx context.Context, req Request) (string, error)
}

// userText joins the prompt and the OCR text into one message.
func userText(req Request) string {
	if req.Text == "" {
		return req.Prompt
	}
	return req.Prompt + "\n\nScreen text:\n" + req.Text
}
