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

// DefaultPath returns config.json next to the peekai executable (or next
// to peekai.app on macOS), so the app is portable. Under `go run` the
// binary lives in a temporary build dir, so the current directory is used
// instead.
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
	// In a macOS app bundle the binary sits in peekai.app/Contents/MacOS;
	// keep the config next to the .app where people can find it.
	dir := filepath.Dir(exe)
	if bundle := filepath.Dir(filepath.Dir(dir)); strings.HasSuffix(bundle, ".app") &&
		filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" {
		dir = filepath.Dir(bundle)
	}
	return filepath.Join(dir, FileName), nil
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
	if err := checkVersion(data); err != nil {
		return Config{}, err
	}
	cfg := Default()
	// Lists and maps from the file replace the defaults. Decoding into a
	// non-empty slice would reuse its elements, so a model missing "model"
	// would silently inherit the default model's ID.
	def := cfg
	cfg.Prompts.List = nil
	cfg.AI.Providers, cfg.AI.Models, cfg.UI.Themes = nil, nil, nil

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Prompts.List == nil {
		cfg.Prompts.List = def.Prompts.List
	}
	if cfg.AI.Providers == nil {
		cfg.AI.Providers = def.AI.Providers
	}
	if cfg.AI.Models == nil {
		cfg.AI.Models = def.AI.Models
	}
	if cfg.UI.Themes == nil {
		cfg.UI.Themes = def.UI.Themes
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

// ErrOldFormat is returned for a config.json from before version 2.
var ErrOldFormat = errors.New(`old config format (version 1): ` +
	`see "Migrating to version 2" in the README, or delete the file to get new defaults`)

// checkVersion rejects version 1 files before the strict decode, so the
// user sees how to migrate instead of `unknown field "mode"`.
func checkVersion(data []byte) error {
	var probe struct {
		Version *int            `json:"version"`
		Mode    json.RawMessage `json:"mode"`
		AI      struct {
			Provider     json.RawMessage `json:"provider"`
			Alternatives json.RawMessage `json:"alternatives"`
		} `json:"ai"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err // the strict decode would fail the same way
	}
	if probe.Mode != nil || probe.AI.Provider != nil || probe.AI.Alternatives != nil ||
		probe.Version == nil || *probe.Version < Version {
		return ErrOldFormat
	}
	if *probe.Version > Version {
		return fmt.Errorf("config.json is version %d, but this peekai reads version %d: update peekai", *probe.Version, Version)
	}
	return nil
}
