package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"

	"github.com/Miwm64/peekai/internal/config"
)

// tesseract runs the tesseract binary, piping a PNG through stdin/stdout.
type tesseract struct {
	path string
	lang string
}

func newTesseract(cfg config.OCR) (*tesseract, error) {
	path := cfg.Path
	if path == "" {
		path = "tesseract"
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("tesseract not found (%w): install it (brew install tesseract / apt install tesseract-ocr / UB-Mannheim build on Windows) or set ocr.path", err)
	}
	lang := cfg.Lang
	if lang == "" {
		lang = "eng"
	}
	return &tesseract{path: resolved, lang: lang}, nil
}

func (t *tesseract) Recognize(ctx context.Context, img image.Image) (string, error) {
	var in bytes.Buffer
	if err := png.Encode(&in, img); err != nil {
		return "", err
	}
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, t.path, "stdin", "stdout", "-l", t.lang)
	cmd.Stdin = &in
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && stderr.Len() > 0 {
			return "", fmt.Errorf("tesseract: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("tesseract: %w", err)
	}
	return cleanText(out.String()), nil
}

// cleanText trims each line and collapses runs of blank lines.
func cleanText(s string) string {
	var b strings.Builder
	blank := false
	for line := range strings.Lines(s) {
		line = strings.TrimSpace(strings.TrimRight(line, "\f"))
		if line == "" {
			blank = b.Len() > 0
			continue
		}
		if blank {
			b.WriteString("\n")
			blank = false
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
