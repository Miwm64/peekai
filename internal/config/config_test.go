package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	cfg, created, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("created = false, want true")
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Error("returned config differs from Default()")
	}

	again, created, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("second Load created the file again")
	}
	if !reflect.DeepEqual(again, Default()) {
		t.Error("written default does not round-trip")
	}
}

func TestParseOverridesKeepDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"mode": "ocr",
		"ai": {"model": "gpt-x", "timeout": "5s"},
		"hotkeys": {"quit": "Alt+Q"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	if cfg.Mode != ModeOCR || cfg.AI.Model != "gpt-x" || cfg.Hotkeys.Quit != "Alt+Q" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if time.Duration(cfg.AI.Timeout) != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", time.Duration(cfg.AI.Timeout))
	}
	if cfg.AI.BaseURL != def.AI.BaseURL || cfg.Hotkeys.Capture != def.Hotkeys.Capture {
		t.Error("unset fields lost their defaults")
	}
}

func TestParsePromptListReplacesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`{"prompts": {"active": "mine", "list": {"mine": "Hi"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"mine": "Hi"}; !reflect.DeepEqual(cfg.Prompts.List, want) {
		t.Errorf("prompts.list = %v, want %v", cfg.Prompts.List, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"unknown field", `{"modee": "ocr"}`, `unknown field "modee"`},
		{"bad mode", `{"mode": "video"}`, "mode:"},
		{"bad output", `{"output": "toast"}`, "output:"},
		{"bad duration", `{"ai": {"timeout": "soon"}}`, "invalid duration"},
		{"duplicate hotkey", `{"hotkeys": {"quit": "ctrl+shift+s"}}`, "already used"},
		{"empty hotkey", `{"hotkeys": {"quit": ""}}`, "hotkeys.quit: must not be empty"},
		{"bad display", `{"capture": {"display": "left"}}`, "capture.display"},
		{"missing active prompt", `{"prompts": {"active": "nope"}}`, "prompts.active"},
		{"bad opacity", `{"ui": {"popup": {"opacity": 1.5}}}`, "ui.popup.opacity"},
		{"bad position", `{"ui": {"ticker": {"position": "left"}}}`, "ui.ticker.position"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestValidateReportsAllErrors(t *testing.T) {
	cfg := Default()
	cfg.Mode = "x"
	cfg.Output = "y"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "mode:") || !strings.Contains(err.Error(), "output:") {
		t.Errorf("want both mode and output errors, got %v", err)
	}
}

func TestLoadReportsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"mode": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to mention %s", err, path)
	}
}
