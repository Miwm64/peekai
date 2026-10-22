package theme

import (
	"slices"
	"testing"

	"github.com/Miwm64/peekai/internal/config"
)

func TestEveryBuiltinResolves(t *testing.T) {
	ui := config.Default().UI
	for _, name := range config.BuiltinThemes {
		p := Resolve(ui, name, false)
		if p.FontSize <= 0 || p.Opacity <= 0 || p.Opacity > 1 {
			t.Errorf("%s: %+v", name, p)
		}
	}
}

func TestSystemFollowsAppearance(t *testing.T) {
	ui := config.Default().UI
	if Resolve(ui, "system", true).Name != "dark" || Resolve(ui, "system", false).Name != "light" {
		t.Error("system does not follow appearance")
	}
}

func TestCustomThemeChain(t *testing.T) {
	ui := config.Default().UI
	ui.Themes = []config.CustomTheme{
		{Name: "base2", Base: "stealth", Text: "#112233"},
		{Name: "mine", Base: "base2", FontSize: 20, Opacity: 0.5},
	}
	p := Resolve(ui, "mine", false)
	if p.Name != "mine" || p.Text.R != 0x11 || p.FontSize != 20 || p.Opacity != 0.5 {
		t.Errorf("mine = %+v", p)
	}
	if p.Dim != Presets["stealth"].Dim {
		t.Error("unset colors should come from the base chain")
	}
	if got := Names(ui); !slices.Contains(got, "mine") || got[0] != "system" {
		t.Errorf("names = %v", got)
	}
}
