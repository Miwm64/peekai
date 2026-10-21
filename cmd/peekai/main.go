// Command peekai captures the screen, sends it (or its OCR text) to an AI
// model and shows the answer in an on-top popup or ticker.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/signal"
	"time"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/capture"
	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/ocr"
	"github.com/Miwm64/peekai/internal/pipeline"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "peekai:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config.json (default: next to the peekai executable)")
	showVersion := flag.Bool("version", false, "print version and exit")
	once := flag.Bool("once", false, "analyze the screen once, print the answer and exit")
	mode := flag.String("mode", "", "override config mode for -once: image | ocr")
	promptName := flag.String("prompt", "", "override the active prompt for -once (name from prompts.list)")
	out := flag.String("out", "", "also save the screenshot sent by -once to this PNG file")
	flag.Parse()

	if *showVersion {
		fmt.Println("peekai", version)
		return nil
	}

	path := *configPath
	if path == "" {
		var err error
		if path, err = config.DefaultPath(); err != nil {
			return err
		}
	}

	cfg, created, err := config.Load(path)
	if err != nil {
		return err
	}
	if created {
		fmt.Println("created default config:", path)
	}
	fmt.Printf("config: %s (mode=%s, output=%s, provider=%s, model=%s)\n",
		path, cfg.Mode, cfg.Output, cfg.AI.Provider, cfg.AI.Model)

	if *once {
		if *mode != "" {
			cfg.Mode = *mode
		}
		if *promptName != "" {
			cfg.Prompts.Active = *promptName
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return runOnce(ctx, cfg, *out)
	}
	return nil
}

// runOnce analyzes the screen once and prints the answer.
func runOnce(ctx context.Context, cfg config.Config, out string) error {
	capturer, err := capture.New(cfg.Capture)
	if err != nil {
		return err
	}
	provider, err := ai.New(cfg.AI)
	if err != nil {
		return err
	}
	p := &pipeline.Pipeline{Capturer: capturer, AI: provider, MaxImagePx: cfg.AI.MaxImagePx}
	// Only build OCR when used, so image mode works without tesseract.
	if cfg.Mode == config.ModeOCR {
		if p.OCR, err = ocr.New(cfg.OCR, provider); err != nil {
			return err
		}
	}

	fmt.Printf("asking %s (%s), mode %s, prompt %q…\n", cfg.AI.Provider, cfg.AI.Model, cfg.Mode, cfg.Prompts.Active)
	res, err := p.Run(ctx, pipeline.Options{Mode: cfg.Mode, Prompt: cfg.Prompts.List[cfg.Prompts.Active]})
	if out != "" && res.Image != nil {
		if err := savePNG(out, res.Image); err != nil {
			return err
		}
		fmt.Println("saved screenshot:", out)
	}
	if res.Text != "" {
		fmt.Printf("ocr text (%d chars):\n%s\n", len(res.Text), preview(res.Text, 400))
	}
	if err != nil {
		return err
	}
	fmt.Printf("\n%s\n\n(%s)\n", res.Answer, res.Elapsed.Round(100*time.Millisecond))
	return nil
}

// preview shortens s to at most n runes.
func preview(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
