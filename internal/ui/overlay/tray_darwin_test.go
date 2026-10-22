//go:build darwin

package overlay

import (
	"reflect"
	"testing"

	"github.com/Miwm64/peekai/internal/config"
	"github.com/Miwm64/peekai/internal/ui"
)

func TestTrayMenuModelsOnlyInSubmenu(t *testing.T) {
	c := ui.Content{Model: "b", Prompt: "p", Mode: "image", Models: []ui.ModelInfo{
		{Name: "a", Free: true, Vision: true}, {Name: "b", Vision: true, Description: "desc"},
	}}
	items := flatten(New(config.Default().UI).trayMenu(c), -1, nil)

	var top, models []string
	modelsIdx := -1
	for i, it := range items {
		switch it.parent {
		case -1:
			top = append(top, it.text)
			if it.text == "Models" {
				modelsIdx = i
			}
		case modelsIdx:
			models = append(models, it.text)
		}
	}
	if want := []string{"a (free)", "b — desc"}; !reflect.DeepEqual(models, want) {
		t.Errorf("Models submenu = %q, want %q", models, want)
	}
	for _, text := range top {
		if text == "a (free)" || text == "b — desc" {
			t.Errorf("model %q also at top level: %q", text, top)
		}
	}
	if last := top[len(top)-1]; last != "Quit peekai" {
		t.Errorf("last top-level item = %q", last)
	}
	for _, it := range items {
		if it.text == "b — desc" && it.flags&flagChecked == 0 {
			t.Error("current model b is not checked")
		}
	}
}
