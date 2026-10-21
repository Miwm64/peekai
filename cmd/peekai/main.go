// Command peekai captures the screen, sends it (or its OCR text) to an AI
// model and shows the answer in an on-top popup or ticker.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"os"
	"os/signal"

	"github.com/Miwm64/peekai/internal/capture"
	"github.com/Miwm64/peekai/internal/config"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "peekai:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config.json (default: user config dir)")
	showVersion := flag.Bool("version", false, "print version and exit")
	once := flag.Bool("once", false, "capture the screen once, save it and exit")
	out := flag.String("out", "capture.png", "where -once saves the screenshot")
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
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return runOnce(ctx, cfg, *out)
	}
	return nil
}

// runOnce captures the screen and saves it as a PNG.
func runOnce(ctx context.Context, cfg config.Config, out string) error {
	capturer, err := capture.New(cfg.Capture)
	if err != nil {
		return err
	}
	img, err := capturer.Capture(ctx)
	if err != nil {
		return fmt.Errorf("capture: %w", err)
	}
	img = capture.Downscale(img, cfg.AI.MaxImagePx)

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	b := img.Bounds()
	fmt.Printf("saved %dx%d screenshot: %s\n", b.Dx(), b.Dy(), out)
	return nil
}
