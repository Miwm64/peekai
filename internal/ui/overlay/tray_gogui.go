//go:build !darwin

package overlay

import "github.com/go-gui-org/go-gui/gui"

// trayState uses go-gui's tray on Windows and Linux, where its submenus
// work.
type trayState struct{ h *gui.SystemTrayHandle }

func (t *trayState) start(o *Overlay, menu []gui.NativeMenuItemCfg, tooltip string) error {
	h, err := o.app.SetSystemTray(t.cfg(o, menu, tooltip))
	if err != nil {
		return err
	}
	t.h = h
	return nil
}

func (t *trayState) update(o *Overlay, menu []gui.NativeMenuItemCfg, tooltip string) {
	if t.h != nil {
		o.app.UpdateSystemTray(t.h, t.cfg(o, menu, tooltip))
	}
}

func (t *trayState) cfg(o *Overlay, menu []gui.NativeMenuItemCfg, tooltip string) gui.SystemTrayCfg {
	return gui.SystemTrayCfg{Tooltip: tooltip, IconPNG: trayIcon, Menu: menu, OnAction: o.onTrayAction}
}
