// Package theme turns ui.theme / ui.themes from the config into concrete
// colors and sizes. It knows nothing about the GUI toolkit.
package theme

import (
	"image/color"

	"github.com/Miwm64/peekai/internal/config"
)

// Palette is a resolved theme.
type Palette struct {
	Name string
	// Dark tells widget toolkits which base look to start from.
	Dark bool

	Background color.NRGBA // window background (its alpha is ignored; see Opacity)
	Surface    color.NRGBA // cards, inputs, sidebar
	Border     color.NRGBA
	Text       color.NRGBA
	Dim        color.NRGBA // title line, labels, hints
	Accent     color.NRGBA // primary buttons, selection
	AccentText color.NRGBA // text on Accent
	Thinking   color.NRGBA
	Answer     color.NRGBA
	Failed     color.NRGBA

	FontSize float32 // answer text
	Mono     bool
	Opacity  float64 // overlay window background, (0, 1]
	Radius   float32 // popup corners
}

func hex(s string) color.NRGBA {
	c, err := config.ParseColor(s)
	if err != nil {
		panic(err) // presets are constants
	}
	return c
}

// Presets are the built-in themes except "system", which picks light or dark.
var Presets = map[string]Palette{
	// Ollama-like: white, near-black text and buttons, soft grey surfaces.
	"light": {
		Name: "light", Background: hex("#FFFFFF"), Surface: hex("#F4F4F5"), Border: hex("#E4E4E7"),
		Text: hex("#18181B"), Dim: hex("#71717A"), Accent: hex("#18181B"), AccentText: hex("#FFFFFF"),
		Thinking: hex("#D97706"), Answer: hex("#16A34A"), Failed: hex("#DC2626"),
		FontSize: 15, Opacity: 0.97, Radius: 14,
	},
	"dark": {
		Name: "dark", Dark: true, Background: hex("#18181B"), Surface: hex("#27272A"), Border: hex("#3F3F46"),
		Text: hex("#F4F4F5"), Dim: hex("#A1A1AA"), Accent: hex("#F4F4F5"), AccentText: hex("#18181B"),
		Thinking: hex("#FBBF24"), Answer: hex("#4ADE80"), Failed: hex("#F87171"),
		FontSize: 15, Opacity: 0.95, Radius: 14,
	},
	// Frosted dark, see-through.
	"glass": {
		Name: "glass", Dark: true, Background: hex("#0F0F14"), Surface: hex("#1C1C24"), Border: hex("#FFFFFF26"),
		Text: hex("#F5F5F7"), Dim: hex("#B4B4BE"), Accent: hex("#F5F5F7"), AccentText: hex("#0F0F14"),
		Thinking: hex("#FCD34D"), Answer: hex("#86EFAC"), Failed: hex("#FCA5A5"),
		FontSize: 15, Opacity: 0.55, Radius: 18,
	},
	"high-contrast": {
		Name: "high-contrast", Dark: true, Background: hex("#000000"), Surface: hex("#000000"), Border: hex("#FFFFFF"),
		Text: hex("#FFFFFF"), Dim: hex("#FFD60A"), Accent: hex("#FFD60A"), AccentText: hex("#000000"),
		Thinking: hex("#FFD60A"), Answer: hex("#00FF7F"), Failed: hex("#FF453A"),
		FontSize: 18, Opacity: 1, Radius: 6,
	},
	"terminal": {
		Name: "terminal", Dark: true, Background: hex("#0A0F0A"), Surface: hex("#101810"), Border: hex("#1F5F2F"),
		Text: hex("#39FF6A"), Dim: hex("#1F9944"), Accent: hex("#39FF6A"), AccentText: hex("#0A0F0A"),
		Thinking: hex("#E5FF39"), Answer: hex("#39FF6A"), Failed: hex("#FF5555"),
		FontSize: 14, Mono: true, Opacity: 0.92, Radius: 4,
	},
	// Camouflage: tiny, dim grey text on an almost invisible background,
	// so it blends into whatever is behind and draws no attention.
	"stealth": {
		Name: "stealth", Dark: true, Background: hex("#7F7F7F"), Surface: hex("#2A2A2A"), Border: hex("#00000000"),
		Text: hex("#8E8E8EE0"), Dim: hex("#8E8E8E99"), Accent: hex("#8E8E8E"), AccentText: hex("#1A1A1A"),
		Thinking: hex("#8E8E8EAA"), Answer: hex("#8E8E8EAA"), Failed: hex("#A07070AA"),
		FontSize: 11, Opacity: 0.06, Radius: 6,
	},
}

// Resolve returns the palette for the theme named name. "system" is dark
// or light by systemDark; custom themes from ui.themes start from their
// base (resolved the same way) and override what they set. It assumes a
// validated config; unknown names fall back to system.
func Resolve(ui config.UI, name string, systemDark bool) Palette {
	return resolve(ui, name, systemDark, len(ui.Themes)+1)
}

func resolve(ui config.UI, name string, systemDark bool, depth int) Palette {
	if p, ok := Presets[name]; ok {
		return p
	}
	if depth > 0 {
		for _, t := range ui.Themes {
			if t.Name == name {
				p := resolve(ui, t.Base, systemDark, depth-1)
				p.Name = t.Name
				apply(&p, t)
				return p
			}
		}
	}
	if systemDark {
		return Presets["dark"]
	}
	return Presets["light"]
}

func apply(p *Palette, t config.CustomTheme) {
	set := func(dst *color.NRGBA, v string) {
		if c, err := config.ParseColor(v); v != "" && err == nil {
			*dst = c
		}
	}
	set(&p.Background, t.Background)
	set(&p.Text, t.Text)
	set(&p.Dim, t.Dim)
	set(&p.Accent, t.Accent)
	set(&p.Thinking, t.Thinking)
	set(&p.Answer, t.Answer)
	set(&p.Failed, t.Failed)
	if t.FontSize > 0 {
		p.FontSize = float32(t.FontSize)
	}
	if t.Opacity > 0 {
		p.Opacity = t.Opacity
	}
	if t.Radius > 0 {
		p.Radius = float32(t.Radius)
	}
	if t.Mono {
		p.Mono = true
	}
}

// Names lists the themes to choose from: built-ins, then custom ones.
func Names(ui config.UI) []string {
	names := append([]string(nil), config.BuiltinThemes...)
	for _, t := range ui.Themes {
		names = append(names, t.Name)
	}
	return names
}
