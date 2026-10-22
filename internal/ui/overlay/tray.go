package overlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/Miwm64/peekai/internal/ui"
)

// Tray menu item IDs are controller action names, so a click is passed on
// unchanged.
const trayStatusID = "status"

// Actions delivers tray menu clicks as action names (e.g. "reload_config").
func (o *Overlay) Actions() <-chan string { return o.actions }

// startTray creates the tray icon. UI thread only. A missing tray (some
// Linux desktops) is not fatal: everything still works by hotkey.
func (o *Overlay) startTray() {
	if err := o.tray.start(o, trayMenu(ui.Content{}), trayTooltip(ui.Content{})); err != nil {
		fmt.Fprintln(os.Stderr, "peekai: no tray icon:", err)
	}
}

// updateTray shows c in the tray menu and tooltip. UI thread only.
func (o *Overlay) updateTray(c ui.Content) {
	o.tray.update(o, trayMenu(c), trayTooltip(c))
}

// onTrayAction passes a menu click to the controller.
func (o *Overlay) onTrayAction(id string) {
	if id == trayStatusID || id == "" {
		return
	}
	select {
	case o.actions <- id:
	default: // controller busy reloading; drop the click
	}
}

func trayStatus(c ui.Content) string {
	if c.Model == "" {
		return "starting…"
	}
	return c.Header()
}

func trayTooltip(c ui.Content) string { return "peekai — " + trayStatus(c) }

// trayMenu is the tray menu for c: status, a Models submenu, show/hide,
// reload and quit. Item IDs are controller action names.
func trayMenu(c ui.Content) []gui.NativeMenuItemCfg {
	menu := []gui.NativeMenuItemCfg{
		{ID: trayStatusID, Text: trayStatus(c), Disabled: true},
		{Separator: true},
	}
	if len(c.Models) > 0 {
		menu = append(menu,
			gui.NativeMenuItemCfg{Text: "Models", Submenu: modelItems(c)},
			gui.NativeMenuItemCfg{Separator: true})
	}
	return append(menu,
		gui.NativeMenuItemCfg{ID: "toggle_popup", Text: "Show / hide popup"},
		gui.NativeMenuItemCfg{ID: "toggle_ticker", Text: "Show / hide running line"},
		gui.NativeMenuItemCfg{Separator: true},
		gui.NativeMenuItemCfg{ID: "reload_config", Text: "Reload config"},
		gui.NativeMenuItemCfg{ID: "quit", Text: "Quit peekai"},
	)
}

// modelItems lists the configured models as "name — description", with
// free / no-vision marks and a check on the current one. Their IDs are
// "model:<name>" actions.
func modelItems(c ui.Content) []gui.NativeMenuItemCfg {
	items := make([]gui.NativeMenuItemCfg, 0, len(c.Models))
	for _, m := range c.Models {
		text := m.Name
		var tags []string
		if m.Free {
			tags = append(tags, "free")
		}
		if !m.Vision {
			tags = append(tags, "text only")
		}
		if len(tags) > 0 {
			text += " (" + strings.Join(tags, ", ") + ")"
		}
		if m.Description != "" {
			text += " — " + m.Description
		}
		items = append(items, gui.NativeMenuItemCfg{ID: "model:" + m.Name, Text: text, Checked: m.Name == c.Model})
	}
	return items
}

// trayIcon is a small eye: blue disc, white eye, dark pupil. Coloured
// rather than a black template so it shows on light and dark menu bars.
var trayIcon = func() []byte {
	const n = 44
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	blue := color.NRGBA{R: 76, G: 154, B: 255, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	dark := color.NRGBA{R: 20, G: 24, B: 32, A: 255}
	c := float64(n-1) / 2
	for y := range n {
		for x := range n {
			dx, dy := float64(x)-c, float64(y)-c
			d2 := dx*dx + dy*dy
			// The eye's white is a flattened circle.
			ey := dy * 1.7
			switch {
			case d2 <= 5*5:
				img.Set(x, y, dark)
			case dx*dx+ey*ey <= 15*15:
				img.Set(x, y, white)
			case d2 <= 21*21:
				img.Set(x, y, blue)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}()
