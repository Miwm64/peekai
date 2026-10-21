// Command peekai captures the screen, sends it (or its OCR text) to an AI
// model and shows the answer in an on-top popup or ticker.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"maps"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/atotto/clipboard"
	"golang.design/x/hotkey/mainthread"

	"github.com/Miwm64/peekai/internal/ai"
	"github.com/Miwm64/peekai/internal/app"
	"github.com/Miwm64/peekai/internal/capture"
	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/hotkey"
	"github.com/Miwm64/peekai/internal/ocr"
	"github.com/Miwm64/peekai/internal/pipeline"
	"github.com/Miwm64/peekai/internal/ui"
	"github.com/Miwm64/peekai/internal/ui/console"
	"github.com/Miwm64/peekai/internal/ui/overlay"
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
	mode := flag.String("mode", "", "override config mode: image | ocr")
	promptName := flag.String("prompt", "", "override the active prompt (name from prompts.list)")
	out := flag.String("out", "", "with -once: also save the screenshot sent to this PNG file")
	useConsole := flag.Bool("console", false, "print results in the terminal instead of on-screen windows")
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
	if *mode != "" {
		cfg.Mode = *mode
	}
	if *promptName != "" {
		cfg.Prompts.Active = *promptName
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	fmt.Printf("config: %s (mode=%s, output=%s, provider=%s, model=%s)\n",
		path, cfg.Mode, cfg.Output, cfg.AI.Provider, cfg.AI.Model)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *once {
		return runOnce(ctx, cfg, *out)
	}

	listen := func(presenter ui.Presenter) error {
		err := runHotkeys(ctx, cfg, presenter)
		if errors.Is(err, context.Canceled) {
			return nil // Ctrl+C
		}
		return err
	}
	// Global hotkeys and windows both need the OS main thread (the macOS
	// event loop), so the UI owns it and the app runs alongside.
	if *useConsole {
		mainthread.Init(func() { err = listen(console.New(os.Stdout)) })
		return err
	}
	ov := overlay.New(cfg.UI)
	ov.Run(func() error { return listen(ov) }) // exits the process when done
	return nil
}

// build creates the pipeline and the AI provider it uses.
func build(cfg config.Config) (pipeline.Pipeline, ai.Provider, error) {
	capturer, err := capture.New(cfg.Capture)
	if err != nil {
		return pipeline.Pipeline{}, nil, err
	}
	provider, err := ai.New(cfg.AI)
	if err != nil {
		return pipeline.Pipeline{}, nil, err
	}
	return pipeline.Pipeline{Capturer: capturer, AI: provider, MaxImagePx: cfg.AI.MaxImagePx}, provider, nil
}

// runHotkeys keeps running and reacts to the global hotkeys.
func runHotkeys(ctx context.Context, cfg config.Config, presenter ui.Presenter) error {
	p, provider, err := build(cfg)
	if err != nil {
		return err
	}
	listener, err := hotkey.Listen(cfg.Hotkeys.Bindings())
	if err != nil {
		return err
	}
	defer listener.Close()

	c := app.New(cfg, app.Deps{
		Pipeline:  p,
		NewOCR:    func() (pipeline.Recognizer, error) { return ocr.New(cfg.OCR, provider) },
		UI:        presenter,
		Clipboard: clipboard.WriteAll,
	})

	bindings := cfg.Hotkeys.Bindings()
	fmt.Println("listening for hotkeys (Ctrl+C to exit):")
	for _, action := range slices.Sorted(maps.Keys(bindings)) {
		fmt.Printf("  %-15s %s\n", action, bindings[action])
	}
	return c.Run(ctx, listener.Events())
}

// runOnce analyzes the screen once and prints the answer.
func runOnce(ctx context.Context, cfg config.Config, out string) error {
	p, provider, err := build(cfg)
	if err != nil {
		return err
	}
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
