package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the config file kept next to the peekai executable.
const FileName = "config.json"

// DefaultPath returns config.json next to the peekai executable, so the app
// is portable. Under `go run` the binary lives in a temporary build dir, so
// the current directory is used instead.
func DefaultPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return pathFor(exe, os.Getwd)
}

func pathFor(exe string, getwd func() (string, error)) (string, error) {
	if strings.Contains(filepath.ToSlash(exe), "/go-build") {
		dir, err := getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, FileName), nil
	}
	return filepath.Join(filepath.Dir(exe), FileName), nil
}

// Load reads the config at path. If the file does not exist, the default
// config is written there and returned with created = true.
func Load(path string) (cfg Config, created bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		cfg = Default()
		if err := Save(path, cfg); err != nil {
			return Config{}, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}

	cfg, err = Parse(data)
	if err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, false, nil
}

// Parse decodes config JSON on top of the defaults, so missing fields keep
// their default values. Unknown fields are rejected to catch typos.
func Parse(data []byte) (Config, error) {
	cfg := Default()
	// A user-supplied prompt list replaces the defaults instead of merging.
	defaultPrompts := cfg.Prompts.List
	cfg.Prompts.List = nil

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Prompts.List == nil {
		cfg.Prompts.List = defaultPrompts
	}
	return cfg, cfg.Validate()
}

// Save writes cfg as indented JSON, creating parent directories.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
