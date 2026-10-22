// Package config defines peekai's config.json schema (version 2) and its
// defaults. peekai never writes the file except to create it on first run;
// fields named "default" are startup values that shortcuts change only in
// the running app.
package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// Version is the config format this build reads.
const Version = 2

const (
	ModeImage = "image"
	ModeOCR   = "ocr"
)

type Config struct {
	Version int     `json:"version"`
	Capture Capture `json:"capture"`
	OCR     OCR     `json:"ocr"`
	AI      AI      `json:"ai"`
	Prompts Prompts `json:"prompts"`
	Hotkeys Hotkeys `json:"hotkeys"`
	UI      UI      `json:"ui"`
}

type Capture struct {
	// Mode at startup: "image" sends the screenshot, "ocr" only its text.
	Mode string `json:"mode"`
	// Display is "mouse" (under the mouse), "main", "all" or an index
	// like "1".
	Display string `json:"display"`
	// MaxImagePx is the longest side of the screenshot sent to the AI;
	// 0 sends it at full size.
	MaxImagePx int `json:"max_image_px"`
}

type OCR struct {
	Engine string `json:"engine"`
	Lang   string `json:"lang"`
	// Path to the engine binary; empty means look it up in PATH.
	Path string `json:"path"`
}

type AI struct {
	// DefaultModel is the name of the model to start with.
	DefaultModel string `json:"default_model"`
	// AutoClear starts a fresh AI conversation on every capture. When
	// false, earlier prompts and answers are sent along as context until
	// the clear shortcut. It never clears the windows.
	AutoClear bool     `json:"auto_clear"`
	Timeout   Duration `json:"timeout"`
	// Providers and Models are lists so their order is kept: cycle_model
	// goes through Models in file order.
	Providers []Provider `json:"providers"`
	Models    []Model    `json:"models"`
}

// Provider is an OpenAI-compatible API endpoint.
type Provider struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// KeyEnv names the environment variable holding the API key; empty
	// means the provider needs no key.
	KeyEnv string `json:"key_env,omitempty"`
}

// Model is one model of a provider, with what peekai should know about it.
type Model struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	// Model is the provider's model ID.
	Model string `json:"model"`
	// Vision means the model reads images; without it, image mode is
	// refused for this model.
	Vision      bool   `json:"vision"`
	Free        bool   `json:"free"`
	Description string `json:"description,omitempty"`
	// ReasoningEffort ("none", "low", "medium", "high") is sent only when
	// set; "none" makes thinking models like Gemini Flash answer quickly.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// ResolvedModel is a model with its provider and the shared settings, all
// a client needs.
type ResolvedModel struct {
	Model
	Provider Provider
	Timeout  Duration
}

// ResolvedModels returns the models in file order, each with its provider.
// It assumes a validated config.
func (c Config) ResolvedModels() []ResolvedModel {
	providers := map[string]Provider{}
	for _, p := range c.AI.Providers {
		providers[p.Name] = p
	}
	out := make([]ResolvedModel, 0, len(c.AI.Models))
	for _, m := range c.AI.Models {
		out = append(out, ResolvedModel{Model: m, Provider: providers[m.Provider], Timeout: c.AI.Timeout})
	}
	return out
}

type Prompts struct {
	// Default is the prompt to start with.
	Default string            `json:"default"`
	List    map[string]string `json:"list"`
}

type Hotkeys struct {
	// Capture takes a screenshot and asks the AI; it never opens a window.
	Capture string `json:"capture"`
	// TogglePopup / ToggleTicker are the only ways windows open or close.
	TogglePopup  string `json:"toggle_popup"`
	ToggleTicker string `json:"toggle_ticker"`
	HideAll      string `json:"hide_all"`
	CyclePrompt  string `json:"cycle_prompt"`
	// CycleModel switches to the next model in ai.models.
	CycleModel string `json:"cycle_model"`
	ToggleMode string `json:"toggle_mode"`
	CopyResult string `json:"copy_result"`
	// ReloadConfig re-reads config.json without restarting.
	ReloadConfig string `json:"reload_config"`
	// Clear forgets the last answer and the AI context, and cancels a
	// running analysis.
	Clear string `json:"clear"`
	Quit  string `json:"quit"`
}

// Bindings returns action name → shortcut for every hotkey.
func (h Hotkeys) Bindings() map[string]string {
	return map[string]string{
		"capture":       h.Capture,
		"toggle_popup":  h.TogglePopup,
		"toggle_ticker": h.ToggleTicker,
		"hide_all":      h.HideAll,
		"cycle_prompt":  h.CyclePrompt,
		"cycle_model":   h.CycleModel,
		"toggle_mode":   h.ToggleMode,
		"copy_result":   h.CopyResult,
		"reload_config": h.ReloadConfig,
		"clear":         h.Clear,
		"quit":          h.Quit,
	}
}

type UI struct {
	// Theme is the look at startup: a built-in theme or one from Themes.
	Theme  string        `json:"theme"`
	Themes []CustomTheme `json:"themes"`
	Popup  Popup         `json:"popup"`
	Ticker Ticker        `json:"ticker"`
	Status Status        `json:"status"`
}

// BuiltinThemes are the themes that need no definition in ui.themes.
var BuiltinThemes = []string{"dark"}

// CustomTheme is a user-defined look based on another theme. Colors,
// fonts and per-window overrides come with the theme step.
type CustomTheme struct {
	Name string `json:"name"`
	Base string `json:"base"`
}

type Popup struct {
	Width     int      `json:"width"`
	MaxHeight int      `json:"max_height"`
	Opacity   float64  `json:"opacity"`
	AutoHide  Duration `json:"autohide"` // 0 = never
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

// Status is the small pill showing "thinking…" and notices like
// "mode: ocr" without a shortcut. Off by default.
type Status struct {
	Enabled bool `json:"enabled"`
}

// Default returns the config written on first run.
func Default() Config {
	return Config{
		Version: Version,
		Capture: Capture{Mode: ModeImage, Display: "mouse", MaxImagePx: 1600},
		OCR:     OCR{Engine: "tesseract_cli", Lang: "eng"},
		AI: AI{
			DefaultModel: "qwen-local",
			Timeout:      Duration(2 * time.Minute), // local models can take >60s on first load
			Providers: []Provider{
				{Name: "ollama", URL: "http://localhost:11434/v1"},
				{Name: "grok", URL: "https://api.x.ai/v1", KeyEnv: "XAI_API_KEY"},
				{Name: "routerai", URL: "https://routerai.ru/api/v1", KeyEnv: "ROUTERAI_API_KEY"},
			},
			Models: []Model{
				{
					Name: "qwen-local", Provider: "ollama", Model: "qwen2.5vl:7b",
					Vision: true, Free: true, Description: "local, private, slower",
				},
				{
					Name: "grok", Provider: "grok", Model: "grok-4.7",
					Vision: true, Description: "xAI, prepaid credits",
				},
				{
					Name: "deepseek-flash", Provider: "routerai", Model: "deepseek/deepseek-v4.1-flash",
					Vision: true, Description: "cheap, via RouterAI",
				},
			},
		},
		Prompts: Prompts{
			Default: "explain",
			List: map[string]string{
				"explain":   "Explain what is on this screen concisely.",
				"translate": "Translate all text to English.",
				"answer":    "Answer the question shown on screen. Be brief.",
			},
		},
		Hotkeys: Hotkeys{
			Capture:      "Ctrl+Shift+S",
			TogglePopup:  "Ctrl+Shift+1",
			ToggleTicker: "Ctrl+Shift+2",
			HideAll:      "Ctrl+Shift+H",
			CyclePrompt:  "Ctrl+Shift+N",
			CycleModel:   "Ctrl+Shift+P",
			ToggleMode:   "Ctrl+Shift+M",
			CopyResult:   "Ctrl+Shift+C",
			ReloadConfig: "Ctrl+Shift+R",
			Clear:        "Ctrl+Shift+X",
			Quit:         "Ctrl+Shift+Q",
		},
		UI: UI{
			Theme:  "dark",
			Themes: []CustomTheme{},
			Popup:  Popup{Width: 520, MaxHeight: 600, Opacity: 0.95},
			Ticker: Ticker{Speed: 120, Height: 44, Opacity: 0.85, Position: "bottom", Start: "right"},
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
