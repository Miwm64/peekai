package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Validate reports every invalid value at once.
func (c Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if !slices.Contains([]string{ModeImage, ModeOCR}, c.Mode) {
		add("mode: want %q or %q, got %q", ModeImage, ModeOCR, c.Mode)
	}

	seen := map[string]string{}
	for action, key := range c.Hotkeys.Bindings() {
		norm := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		switch {
		case norm == "":
			add("hotkeys.%s: must not be empty", action)
		case seen[norm] != "":
			add("hotkeys.%s: %q is already used by %s", action, key, seen[norm])
		default:
			seen[norm] = action
		}
	}

	if d := c.Capture.Display; d != "active" && d != "all" {
		if n, err := strconv.Atoi(d); err != nil || n < 0 {
			add("capture.display: want \"active\", \"all\" or a display index, got %q", d)
		}
	}

	if c.Mode == ModeOCR && c.OCR.Engine == "" {
		add("ocr.engine: required when mode is %q", ModeOCR)
	}

	if c.AI.Provider == "" {
		add("ai.provider: must not be empty")
	}
	if c.AI.Model == "" {
		add("ai.model: must not be empty")
	}
	for i, alt := range c.AI.Alternatives {
		if alt.Provider == "" || alt.Model == "" {
			add("ai.alternatives[%d]: provider and model are required", i)
		}
	}
	if c.AI.Timeout <= 0 {
		add("ai.timeout: must be positive")
	}
	if c.AI.MaxImagePx < 0 {
		add("ai.max_image_px: must be >= 0 (0 = no downscale)")
	}

	if len(c.Prompts.List) == 0 {
		add("prompts.list: needs at least one prompt")
	} else if _, ok := c.Prompts.List[c.Prompts.Active]; !ok {
		add("prompts.active: %q is not in prompts.list", c.Prompts.Active)
	}

	t := c.UI.Ticker
	if t.Speed <= 0 || t.Height <= 0 {
		add("ui.ticker: speed and height must be positive")
	}
	if t.Position != "bottom" && t.Position != "top" {
		add("ui.ticker.position: want \"bottom\" or \"top\", got %q", t.Position)
	}
	if t.Start != "right" && t.Start != "left" {
		add("ui.ticker.start: want \"right\" or \"left\", got %q", t.Start)
	}
	p := c.UI.Popup
	if p.Width <= 0 || p.MaxHeight <= 0 {
		add("ui.popup: width and max_height must be positive")
	}
	if p.AutoHide < 0 {
		add("ui.popup.autohide: must be >= 0")
	}
	for name, o := range map[string]float64{"ui.ticker.opacity": t.Opacity, "ui.popup.opacity": p.Opacity} {
		if o <= 0 || o > 1 {
			add("%s: must be in (0, 1], got %v", name, o)
		}
	}

	slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	return errors.Join(errs...)
}
