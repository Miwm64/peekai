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

	OutputPopup  = "popup"
	OutputTicker = "ticker"
)

type Config struct {
	Mode    string  `json:"mode"`
	Output  string  `json:"output"`
	Hotkeys Hotkeys `json:"hotkeys"`
	Capture Capture `json:"capture"`
	OCR     OCR     `json:"ocr"`
	AI      AI      `json:"ai"`
	Prompts Prompts `json:"prompts"`
	UI      UI      `json:"ui"`
}

type Hotkeys struct {
	Capture       string `json:"capture"`
	CapturePopup  string `json:"capture_popup"`
	CaptureTicker string `json:"capture_ticker"`
	TogglePopup   string `json:"toggle_popup"`
	ToggleTicker  string `json:"toggle_ticker"`
	HideAll       string `json:"hide_all"`
	CyclePrompt   string `json:"cycle_prompt"`
	ToggleMode    string `json:"toggle_mode"`
	CopyResult    string `json:"copy_result"`
	Quit          string `json:"quit"`
}

// Bindings returns action name → shortcut for every hotkey.
func (h Hotkeys) Bindings() map[string]string {
	return map[string]string{
		"capture":        h.Capture,
		"capture_popup":  h.CapturePopup,
		"capture_ticker": h.CaptureTicker,
		"toggle_popup":   h.TogglePopup,
		"toggle_ticker":  h.ToggleTicker,
		"hide_all":       h.HideAll,
		"cycle_prompt":   h.CyclePrompt,
		"toggle_mode":    h.ToggleMode,
		"copy_result":    h.CopyResult,
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
	Provider   string   `json:"provider"`
	BaseURL    string   `json:"base_url"`
	Model      string   `json:"model"`
	APIKeyEnv  string   `json:"api_key_env"`
	Timeout    Duration `json:"timeout"`
	MaxImagePx int      `json:"max_image_px"`
}

type Prompts struct {
	Active string            `json:"active"`
	List   map[string]string `json:"list"`
}

type UI struct {
	Ticker Ticker `json:"ticker"`
	Popup  Popup  `json:"popup"`
}

type Ticker struct {
	Speed    int     `json:"speed"` // pixels per second
	Height   int     `json:"height"`
	Opacity  float64 `json:"opacity"`
	Position string  `json:"position"` // "bottom" | "top"
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
		Mode:   ModeImage,
		Output: OutputPopup,
		Hotkeys: Hotkeys{
			Capture:       "Ctrl+Shift+S",
			CapturePopup:  "Ctrl+Shift+P",
			CaptureTicker: "Ctrl+Shift+T",
			TogglePopup:   "Ctrl+Shift+1",
			ToggleTicker:  "Ctrl+Shift+2",
			HideAll:       "Ctrl+Shift+H",
			CyclePrompt:   "Ctrl+Shift+N",
			ToggleMode:    "Ctrl+Shift+M",
			CopyResult:    "Ctrl+Shift+C",
			Quit:          "Ctrl+Shift+Q",
		},
		Capture: Capture{Display: "active"},
		OCR:     OCR{Engine: "tesseract_cli", Lang: "eng"},
		AI: AI{
			Provider:   "openai_compat",
			BaseURL:    "http://localhost:11434/v1",
			Model:      "qwen2.5vl:7b",
			APIKeyEnv:  "AI_API_KEY",
			Timeout:    Duration(60 * time.Second),
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
			Ticker: Ticker{Speed: 120, Height: 36, Opacity: 0.85, Position: "bottom"},
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
