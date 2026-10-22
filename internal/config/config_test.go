package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// v2 wraps a config fragment with the version header.
func v2(fragment string) []byte {
	return []byte(`{"version": 2` + fragment + `}`)
}

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
	cfg, err := Parse(v2(`,
		"capture": {"mode": "ocr"},
		"ai": {"timeout": "5s"},
		"hotkeys": {"quit": "Alt+Q"}`))
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	if cfg.Capture.Mode != ModeOCR || cfg.Hotkeys.Quit != "Alt+Q" || time.Duration(cfg.AI.Timeout) != 5*time.Second {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.Capture.Display != def.Capture.Display || !reflect.DeepEqual(cfg.AI.Models, def.AI.Models) {
		t.Error("unset fields lost their defaults")
	}
}

func TestParseListsReplaceDefaults(t *testing.T) {
	cfg, err := Parse(v2(`,
		"ai": {
			"default_model": "g",
			"providers": [{"name": "gemini", "url": "https://generativelanguage.googleapis.com/v1beta/openai", "key_env": "GEMINI_API_KEY"}],
			"models": [{"name": "g", "provider": "gemini", "model": "gemini-3.8-flash", "vision": true, "reasoning_effort": "none"}]
		},
		"prompts": {"default": "mine", "list": {"mine": "Hi"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AI.Providers) != 1 || len(cfg.AI.Models) != 1 || cfg.AI.Models[0].ReasoningEffort != "none" {
		t.Errorf("ai = %+v", cfg.AI)
	}
	if want := map[string]string{"mine": "Hi"}; !reflect.DeepEqual(cfg.Prompts.List, want) {
		t.Errorf("prompts.list = %v, want %v", cfg.Prompts.List, want)
	}
}

// withSuggestions is the default config plus every suggested provider and
// its model, like after adding them all in the settings window.
func withSuggestions() Config {
	cfg := Default()
	for _, s := range ProviderSuggestions {
		cfg.AI.Providers = append(cfg.AI.Providers, s.Provider)
		if s.Model.Name != "" {
			cfg.AI.Models = append(cfg.AI.Models, s.Model)
		}
	}
	cfg.AI.DefaultModel = cfg.AI.Models[0].Name
	return cfg
}

func TestEmptyDefaultsAndSuggestionsAreValid(t *testing.T) {
	cfg := Default()
	if len(cfg.AI.Providers) != 0 || len(cfg.AI.Models) != 0 || cfg.AI.DefaultModel != "" {
		t.Errorf("default ai = %+v, want empty", cfg.AI)
	}
	if err := withSuggestions().Validate(); err != nil {
		t.Errorf("suggestions invalid: %v", err)
	}
}

func TestResolvedModelsKeepOrderAndJoinProviders(t *testing.T) {
	cfg := Default()
	cfg.AI.Providers = []Provider{{Name: "ollama", URL: "http://localhost:11434/v1"}, {Name: "grok", URL: "https://api.x.ai/v1", KeyEnv: "XAI_API_KEY"}}
	cfg.AI.Models = []Model{
		{Name: "qwen-local", Provider: "ollama", Model: "q"},
		{Name: "grok", Provider: "grok", Model: "g"},
		{Name: "deepseek-flash", Provider: "ollama", Model: "d"},
	}
	got := cfg.ResolvedModels()
	var names []string
	for _, m := range got {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "qwen-local,grok,deepseek-flash" {
		t.Errorf("order = %v", names)
	}
	if got[1].Provider.URL != "https://api.x.ai/v1" || got[1].Provider.KeyEnv != "XAI_API_KEY" || got[1].Timeout != cfg.AI.Timeout {
		t.Errorf("grok resolved = %+v", got[1])
	}
}

func TestOldFormatIsRejected(t *testing.T) {
	tests := map[string]string{
		"no version":          `{"capture": {"display": "mouse"}}`,
		"top-level mode":      `{"version": 2, "mode": "image"}`,
		"ai.provider":         `{"version": 2, "ai": {"provider": "ollama"}}`,
		"ai.alternatives":     `{"version": 2, "ai": {"alternatives": []}}`,
		"version 1":           `{"version": 1}`,
		"real version 1 file": `{"mode": "image", "auto_clear": true, "ai": {"provider": "ollama", "model": "m"}}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); !errors.Is(err, ErrOldFormat) {
				t.Errorf("err = %v, want ErrOldFormat", err)
			}
		})
	}
	if _, err := Parse([]byte(`{"version": 3}`)); err == nil || !strings.Contains(err.Error(), "update peekai") {
		t.Errorf("future version: err = %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"unknown field", `, "capturee": {}`, `unknown field "capturee"`},
		{"bad mode", `, "capture": {"mode": "video"}`, "capture.mode:"},
		{"bad display", `, "capture": {"display": "left"}`, "capture.display"},
		{"old display name", `, "capture": {"display": "active"}`, "capture.display"},
		{"bad duration", `, "ai": {"timeout": "soon"}`, "invalid duration"},
		{"unknown default model", `, "ai": {"providers": [{"name": "ollama", "url": "http://a"}], "models": [{"name": "x", "provider": "ollama", "model": "m"}], "default_model": "nope"}`, "ai.default_model"},
		{"model with unknown provider", `, "ai": {"models": [{"name": "x", "provider": "nope", "model": "m"}], "default_model": "x"}`, `provider "nope" is not in ai.providers`},
		{"model without id", `, "ai": {"providers": [{"name": "ollama", "url": "http://a"}], "models": [{"name": "x", "provider": "ollama"}], "default_model": "x"}`, "model is required"},
		{"duplicate model", `, "ai": {"providers": [{"name": "ollama", "url": "http://a"}], "models": [{"name": "x", "provider": "ollama", "model": "a"}, {"name": "x", "provider": "ollama", "model": "b"}], "default_model": "x"}`, `name "x" is used twice`},
		{"duplicate provider", `, "ai": {"providers": [{"name": "p", "url": "http://a"}, {"name": "p", "url": "http://b"}], "models": [{"name": "x", "provider": "p", "model": "m"}], "default_model": "x"}`, `name "p" is used twice`},
		{"bad url", `, "ai": {"providers": [{"name": "p", "url": "localhost:1"}], "models": [{"name": "x", "provider": "p", "model": "m"}], "default_model": "x"}`, "url must be like"},
		{"bad reasoning", `, "ai": {"providers": [{"name": "ollama", "url": "http://a"}], "models": [{"name": "x", "provider": "ollama", "model": "m", "reasoning_effort": "max"}], "default_model": "x"}`, "reasoning_effort"},
		{"duplicate hotkey", `, "hotkeys": {"quit": "ctrl+shift+s"}`, "already used"},
		{"empty hotkey", `, "hotkeys": {"quit": ""}`, "hotkeys.quit: must not be empty"},
		{"missing default prompt", `, "prompts": {"default": "nope"}`, "prompts.default"},
		{"unknown theme", `, "ui": {"theme": "pink"}`, "ui.theme"},
		{"theme loop", `, "ui": {"themes": [{"name": "a", "base": "b"}, {"name": "b", "base": "a"}]}`, "loops back"},
		{"theme bad base", `, "ui": {"themes": [{"name": "a", "base": "nope"}]}`, "not another theme"},
		{"theme bad color", `, "ui": {"themes": [{"name": "a", "base": "dark", "text": "red"}]}`, "a).text: want a color"},
		{"theme bad opacity", `, "ui": {"themes": [{"name": "a", "base": "dark", "opacity": 2}]}`, "a).opacity"},
		{"bad opacity", `, "ui": {"popup": {"opacity": 1.5}}`, "ui.popup.opacity"},
		{"bad start", `, "ui": {"ticker": {"start": "middle"}}`, "ui.ticker.start"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(v2(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestCustomThemeIsAccepted(t *testing.T) {
	cfg, err := Parse(v2(`, "ui": {"theme": "mine", "themes": [{"name": "mine", "base": "stealth", "text": "#CDD6F4", "background": "#1E1E2EF0", "font_size": 17}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Themes[0].FontSize != 17 {
		t.Errorf("themes = %+v", cfg.UI.Themes)
	}
}

func TestParseColor(t *testing.T) {
	c, err := ParseColor("#1E1E2E")
	if err != nil || c.R != 0x1E || c.B != 0x2E || c.A != 0xFF {
		t.Errorf("#1E1E2E = %v, %v", c, err)
	}
	if c, err := ParseColor("#11223344"); err != nil || c.A != 0x44 {
		t.Errorf("#11223344 = %v, %v", c, err)
	}
	for _, bad := range []string{"", "red", "#123", "#GGGGGG", "112233"} {
		if _, err := ParseColor(bad); err == nil {
			t.Errorf("ParseColor(%q) accepted", bad)
		}
	}
}

func TestValidateReportsAllErrors(t *testing.T) {
	cfg := Default()
	cfg.Capture.Mode = "x"
	cfg.UI.Ticker.Position = "y"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "capture.mode:") || !strings.Contains(err.Error(), "ui.ticker.position:") {
		t.Errorf("want both errors, got %v", err)
	}
}

func TestLoadReportsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, v2(`, "capture": {"mode": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to mention %s", err, path)
	}
}

func TestPathFor(t *testing.T) {
	wd := func() (string, error) { return filepath.FromSlash("/work/peekai"), nil }
	tests := []struct {
		name, exe, want string
	}{
		{"installed binary", "/opt/peekai/peekai", "/opt/peekai/config.json"},
		{"windows-style dir", "/apps/peekai/peekai.exe", "/apps/peekai/config.json"},
		{"go run", "/tmp/go-build123/b001/exe/peekai", "/work/peekai/config.json"},
		{"macOS app bundle", "/Applications/peekai.app/Contents/MacOS/peekai", "/Applications/config.json"},
		{"folder named MacOS outside a bundle", "/tools/MacOS/peekai", "/tools/MacOS/config.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pathFor(filepath.FromSlash(tt.exe), wd)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.FromSlash(tt.want); got != want {
				t.Errorf("path = %s, want %s", got, want)
			}
		})
	}
}
