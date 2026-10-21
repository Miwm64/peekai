// Command peekai captures the screen, sends it (or its OCR text) to an AI
// model and shows the answer in an on-top popup or ticker.
package main

import (
	"flag"
	"fmt"
	"os"

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
	return nil
}
