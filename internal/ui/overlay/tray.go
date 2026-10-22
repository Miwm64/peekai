package overlay

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/Miwm64/peekai/internal/icon"
	"github.com/Miwm64/peekai/internal/ui"
	"github.com/Miwm64/peekai/internal/ui/theme"
)

// Tray menu item IDs are controller action names, so a click is passed on
// unchanged.
const trayStatusID = "status"

// Actions delivers tray menu clicks as action names (e.g. "reload_config").
func (o *Overlay) Actions() <-chan string { return o.actions }

// startTray creates the tray icon. UI thread only. A missing tray (some
// Linux desktops) is not fatal: everything still works by hotkey.
func (o *Overlay) startTray() {
	if err := o.tray.start(o, o.trayMenu(ui.Content{}), trayTooltip(ui.Content{})); err != nil {
		fmt.Fprintln(os.Stderr, "peekai: no tray icon:", err)
	}
}

// updateTray shows c in the tray menu and tooltip. UI thread only.
func (o *Overlay) updateTray(c ui.Content) {
	o.tray.update(o, o.trayMenu(c), trayTooltip(c))
}

// Tray actions the overlay handles itself; everything else goes to the
// controller.
const (
	traySettingsID = "settings"
	trayThemeID    = "theme:"
)

// onTrayAction handles a menu click.
func (o *Overlay) onTrayAction(id string) {
	if id == trayStatusID || id == "" {
		return
	}
	if name, ok := strings.CutPrefix(id, trayThemeID); ok {
		o.mu.Lock()
		o.themeSel = name
		o.mu.Unlock()
		o.applyTheme()
		return
	}
	if id == traySettingsID {
		o.openSettings()
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

// trayMenu is the tray menu for c: status, Models and Theme submenus,
// show/hide, settings, reload and quit. Item IDs are action names.
func (o *Overlay) trayMenu(c ui.Content) []gui.NativeMenuItemCfg {
	menu := []gui.NativeMenuItemCfg{
		{ID: trayStatusID, Text: trayStatus(c), Disabled: true},
		{Separator: true},
	}
	if len(c.Models) > 0 {
		menu = append(menu, gui.NativeMenuItemCfg{Text: "Models", Submenu: modelItems(c)})
	}
	menu = append(menu, gui.NativeMenuItemCfg{Text: "Theme", Submenu: o.themeItems()}, gui.NativeMenuItemCfg{Separator: true})
	return append(menu,
		gui.NativeMenuItemCfg{ID: "toggle_popup", Text: "Show / hide popup"},
		gui.NativeMenuItemCfg{ID: "toggle_ticker", Text: "Show / hide running line"},
		gui.NativeMenuItemCfg{Separator: true},
		gui.NativeMenuItemCfg{ID: traySettingsID, Text: "Settings…"},
		gui.NativeMenuItemCfg{ID: "reload_config", Text: "Reload config"},
		gui.NativeMenuItemCfg{Separator: true},
		gui.NativeMenuItemCfg{ID: "quit", Text: "Quit peekai"},
	)
}

// themeItems lists the themes with a check on the current one.
func (o *Overlay) themeItems() []gui.NativeMenuItemCfg {
	o.mu.Lock()
	names, current := theme.Names(o.uiCfg), o.pal.Name
	system := o.themeSel == "system" || (o.themeSel == "" && o.uiCfg.Theme == "system")
	o.mu.Unlock()
	items := make([]gui.NativeMenuItemCfg, 0, len(names))
	for _, n := range names {
		checked := n == current && !system
		if n == "system" {
			checked = system
		}
		items = append(items, gui.NativeMenuItemCfg{ID: trayThemeID + n, Text: n, Checked: checked})
	}
	return items
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

// trayIcon is the peeking cat, sized for menu bars and notification areas.
var trayIcon = icon.PNG(44, false)
