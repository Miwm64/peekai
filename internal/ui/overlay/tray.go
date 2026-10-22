package overlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

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
	h, err := o.app.SetSystemTray(o.trayCfg(ui.Content{}))
	if err != nil {
		fmt.Fprintln(os.Stderr, "peekai: no tray icon:", err)
		return
	}
	o.tray = h
}

// updateTray shows c in the tray menu and tooltip. UI thread only.
func (o *Overlay) updateTray(c ui.Content) {
	if o.tray != nil {
		o.app.UpdateSystemTray(o.tray, o.trayCfg(c))
	}
}

func (o *Overlay) trayCfg(c ui.Content) gui.SystemTrayCfg {
	status := c.Header()
	if c.Provider == "" {
		status = "starting…"
	}
	return gui.SystemTrayCfg{
		Tooltip: "peekai — " + status,
		IconPNG: trayIcon,
		Menu: []gui.NativeMenuItemCfg{
			{ID: trayStatusID, Text: status, Disabled: true},
			{Separator: true},
			{ID: "toggle_popup", Text: "Show / hide popup"},
			{ID: "toggle_ticker", Text: "Show / hide running line"},
			{Separator: true},
			{ID: "reload_config", Text: "Reload config"},
			{ID: "quit", Text: "Quit peekai"},
		},
		OnAction: func(id string) {
			if id == trayStatusID {
				return
			}
			select {
			case o.actions <- id:
			default: // controller busy reloading; drop the click
			}
		},
	}
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
