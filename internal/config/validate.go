package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

var reasoningEfforts = []string{"none", "low", "medium", "high"}

// Validate reports every invalid value at once.
func (c Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if c.Version != Version {
		add("version: want %d, got %d", Version, c.Version)
	}

	// capture
	if !slices.Contains([]string{ModeImage, ModeOCR}, c.Capture.Mode) {
		add("capture.mode: want %q or %q, got %q", ModeImage, ModeOCR, c.Capture.Mode)
	}
	if d := c.Capture.Display; !slices.Contains([]string{"mouse", "main", "all"}, d) {
		if n, err := strconv.Atoi(d); err != nil || n < 0 {
			add("capture.display: want \"mouse\", \"main\", \"all\" or a display index, got %q", d)
		}
	}
	if c.Capture.MaxImagePx < 0 {
		add("capture.max_image_px: must be >= 0 (0 = no downscale)")
	}
	if c.Capture.Mode == ModeOCR && c.OCR.Engine == "" {
		add("ocr.engine: required when capture.mode is %q", ModeOCR)
	}

	// ai
	providers := map[string]bool{}
	for i, p := range c.AI.Providers {
		where := fmt.Sprintf("ai.providers[%d]", i)
		switch {
		case p.Name == "":
			add("%s: name is required", where)
		case providers[p.Name]:
			add("%s: name %q is used twice", where, p.Name)
		default:
			providers[p.Name] = true
		}
		if u, err := url.Parse(p.URL); err != nil || u.Scheme == "" || u.Host == "" {
			add("%s (%s): url must be like https://host/v1, got %q", where, p.Name, p.URL)
		}
	}
	models := map[string]bool{}
	for i, m := range c.AI.Models {
		where := fmt.Sprintf("ai.models[%d]", i)
		switch {
		case m.Name == "":
			add("%s: name is required", where)
		case models[m.Name]:
			add("%s: name %q is used twice", where, m.Name)
		default:
			models[m.Name] = true
		}
		if !providers[m.Provider] {
			add("%s (%s): provider %q is not in ai.providers", where, m.Name, m.Provider)
		}
		if m.Model == "" {
			add("%s (%s): model is required", where, m.Name)
		}
		if m.ReasoningEffort != "" && !slices.Contains(reasoningEfforts, m.ReasoningEffort) {
			add("%s (%s): reasoning_effort: want one of %s, got %q",
				where, m.Name, strings.Join(reasoningEfforts, ", "), m.ReasoningEffort)
		}
	}
	switch {
	case len(c.AI.Models) == 0:
		add("ai.models: needs at least one model")
	case !models[c.AI.DefaultModel]:
		add("ai.default_model: %q is not in ai.models", c.AI.DefaultModel)
	}
	if c.AI.Timeout <= 0 {
		add("ai.timeout: must be positive")
	}

	// prompts
	if len(c.Prompts.List) == 0 {
		add("prompts.list: needs at least one prompt")
	} else if _, ok := c.Prompts.List[c.Prompts.Default]; !ok {
		add("prompts.default: %q is not in prompts.list", c.Prompts.Default)
	}

	// hotkeys
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

	// ui
	themes := slices.Clone(BuiltinThemes)
	for i, t := range c.UI.Themes {
		if t.Name == "" || slices.Contains(themes, t.Name) {
			add("ui.themes[%d]: name %q is empty or already taken", i, t.Name)
			continue
		}
		themes = append(themes, t.Name)
	}
	bases := map[string]string{}
	for i, t := range c.UI.Themes {
		if !slices.Contains(themes, t.Base) || t.Base == t.Name {
			add("ui.themes[%d] (%s): base %q is not another theme", i, t.Name, t.Base)
			continue
		}
		bases[t.Name] = t.Base
	}
	for _, t := range c.UI.Themes {
		// Following base must reach a built-in theme; a loop never does.
		name, steps := t.Name, 0
		for bases[name] != "" && steps <= len(bases) {
			name, steps = bases[name], steps+1
		}
		if steps > len(bases) {
			add("ui.themes (%s): base chain loops back on itself", t.Name)
		}
	}
	if !slices.Contains(themes, c.UI.Theme) {
		add("ui.theme: %q is not a built-in theme (%s) or in ui.themes",
			c.UI.Theme, strings.Join(BuiltinThemes, ", "))
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
