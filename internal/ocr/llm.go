package ocr

import (
	"context"
	"errors"
	"image"

	"github.com/Miwm64/peekai/internal/ai"
)

const llmPrompt = "Transcribe all text visible in this image exactly as written, " +
	"preserving line breaks. Output only the text, with no commentary."

// llm asks a vision model to transcribe the image.
type llm struct {
	provider ai.Provider
}

func (l *llm) Recognize(ctx context.Context, img image.Image) (string, error) {
	if l.provider == nil {
		return "", errors.New("llm ocr needs an AI provider")
	}
	text, err := l.provider.Analyze(ctx, ai.Request{Prompt: llmPrompt, Image: img})
	if err != nil {
		return "", err
	}
	return cleanText(text), nil
}
