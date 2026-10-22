// Package config defines peekai's config.json schema and its defaults.
package config

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	ModeImage = "image"
	ModeOCR   = "ocr"
)

type Config struct {
	Mode string `json:"mode"`
	// AutoClear starts a fresh AI conversation on every capture. When
	// false, earlier prompts and answers are sent along as context until
	// the clear shortcut. It never clears the windows.
	AutoClear bool    `json:"auto_clear"`
	Hotkeys   Hotkeys `json:"hotkeys"`
	Capture   Capture `json:"capture"`
	OCR       OCR     `json:"ocr"`
	AI        AI      `json:"ai"`
	Prompts   Prompts `json:"prompts"`
	UI        UI      `json:"ui"`
}

type Hotkeys struct {
	// Capture takes a screenshot and asks the AI; it never opens a window.
	Capture string `json:"capture"`
	// TogglePopup / ToggleTicker are the only ways windows open or close.
	TogglePopup  string `json:"toggle_popup"`
	ToggleTicker string `json:"toggle_ticker"`
	HideAll      string `json:"hide_all"`
	CyclePrompt  string `json:"cycle_prompt"`
	// CycleProvider switches between ai and ai.alternatives.
	CycleProvider string `json:"cycle_provider"`
	ToggleMode    string `json:"toggle_mode"`
	CopyResult    string `json:"copy_result"`
	// ReloadConfig re-reads config.json without restarting.
	ReloadConfig string `json:"reload_config"`
	// Clear forgets the last answer and cancels a running analysis.
	Clear string `json:"clear"`
	Quit  string `json:"quit"`
}

// Bindings returns action name → shortcut for every hotkey.
func (h Hotkeys) Bindings() map[string]string {
	return map[string]string{
		"capture":        h.Capture,
		"toggle_popup":   h.TogglePopup,
		"toggle_ticker":  h.ToggleTicker,
		"hide_all":       h.HideAll,
		"cycle_prompt":   h.CyclePrompt,
		"cycle_provider": h.CycleProvider,
		"toggle_mode":    h.ToggleMode,
		"copy_result":    h.CopyResult,
		"reload_config":  h.ReloadConfig,
		"clear":          h.Clear,
		"quit":           h.Quit,
	}
}

type Capture struct {
	// Display is "active", "all" or a display index like "0".
	Display string `json:"display"`
}

type OCR struct {
	Engine string `json:"engine"`
	Lang   string `json:"lang"`
	// Path to the engine binary; empty means look it up in PATH.
	Path string `json:"path"`
}

type AI struct {
	// Provider is a preset ("ollama", "routerai", "grok", "openrouter",
	// "requesty", "gemini") or "openai_compat" for any other
	// OpenAI-compatible server.
	Provider string `json:"provider"`
	// BaseURL and APIKeyEnv override the preset; empty means use its default.
	BaseURL    string   `json:"base_url"`
	Model      string   `json:"model"`
	APIKeyEnv  string   `json:"api_key_env"`
	Timeout    Duration `json:"timeout"`
	MaxImagePx int      `json:"max_image_px"`
	// Alternatives are more providers to switch to with cycle_provider.
	// Timeout and MaxImagePx are shared.
	Alternatives []AIAlternative `json:"alternatives"`
}

// AIAlternative is another provider/model; empty fields work as in AI.
type AIAlternative struct {
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url,omitempty"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

// Options returns the main provider followed by the alternatives, each as
// a complete AI config.
func (a AI) Options() []AI {
	opts := []AI{a}
	for _, alt := range a.Alternatives {
		o := a
		o.Provider, o.BaseURL, o.Model, o.APIKeyEnv = alt.Provider, alt.BaseURL, alt.Model, alt.APIKeyEnv
		o.Alternatives = nil
		opts = append(opts, o)
	}
	opts[0].Alternatives = nil
	return opts
}

type Prompts struct {
	Active string            `json:"active"`
	List   map[string]string `json:"list"`
}

type UI struct {
	Ticker Ticker `json:"ticker"`
	Popup  Popup  `json:"popup"`
	Status Status `json:"status"`
}

// Status is the small pill showing "thinking…" and notices like
// "mode: ocr" without a shortcut. Off by default.
type Status struct {
	Enabled bool `json:"enabled"`
}

type Ticker struct {
	Speed    int     `json:"speed"` // pixels per second
	Height   int     `json:"height"`
	Opacity  float64 `json:"opacity"`
	Position string  `json:"position"` // "bottom" | "top"
	// Start is where the text begins: "right" enters at the right edge,
	// "left" starts already visible at the left. It then moves left.
	Start string `json:"start"`
}

type Popup struct {
	Width     int      `json:"width"`
	MaxHeight int      `json:"max_height"`
	Opacity   float64  `json:"opacity"`
	AutoHide  Duration `json:"autohide"` // 0 = never
}

// Default returns the config written on first run.
func Default() Config {
	return Config{
		Mode: ModeImage,
		Hotkeys: Hotkeys{
			Capture:       "Ctrl+Shift+S",
			TogglePopup:   "Ctrl+Shift+1",
			ToggleTicker:  "Ctrl+Shift+2",
			HideAll:       "Ctrl+Shift+H",
			CyclePrompt:   "Ctrl+Shift+N",
			CycleProvider: "Ctrl+Shift+P",
			ToggleMode:    "Ctrl+Shift+M",
			CopyResult:    "Ctrl+Shift+C",
			ReloadConfig:  "Ctrl+Shift+R",
			Clear:         "Ctrl+Shift+X",
			Quit:          "Ctrl+Shift+Q",
		},
		Capture: Capture{Display: "active"},
		OCR:     OCR{Engine: "tesseract_cli", Lang: "eng"},
		AI: AI{
			Provider:   "ollama",
			Model:      "qwen2.5vl:7b",
			Timeout:    Duration(2 * time.Minute), // local models can take >60s on first load
			MaxImagePx: 1600,
		},
		Prompts: Prompts{
			Active: "explain",
			List: map[string]string{
				"explain":   "Explain what is on this screen concisely.",
				"translate": "Translate all text to English.",
				"answer":    "Answer the question shown on screen. Be brief.",
			},
		},
		UI: UI{
			Ticker: Ticker{Speed: 120, Height: 44, Opacity: 0.85, Position: "bottom", Start: "right"},
			Popup:  Popup{Width: 520, MaxHeight: 600, Opacity: 0.95},
		},
	}
}

// Duration is a time.Duration stored in JSON as a string like "60s".
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"30s\": %w", err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}
