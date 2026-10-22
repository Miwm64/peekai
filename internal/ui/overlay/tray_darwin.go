//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void peekaiTraySet(const void *png, int pngLen, const char *tooltip, int n,
	char **ids, char **texts, int *flags, int *parents);
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// trayState draws the macOS tray menu natively. go-gui's macOS tray builds
// the top-level menu from its flattened item list, so submenu items would
// also show after the last top-level item.
type trayState struct{}

// trayOwner receives clicks; there is one overlay per process.
var trayOwner *Overlay

//export peekaiTrayClicked
func peekaiTrayClicked(id *C.char) {
	if trayOwner != nil {
		trayOwner.onTrayAction(C.GoString(id))
	}
}

func (trayState) start(o *Overlay, menu []gui.NativeMenuItemCfg, tooltip string) error {
	trayOwner = o
	setNativeTray(menu, tooltip)
	return nil
}

func (trayState) update(_ *Overlay, menu []gui.NativeMenuItemCfg, tooltip string) {
	setNativeTray(menu, tooltip)
}

// Item flags shared with tray_darwin.m.
const (
	flagSeparator = 1 << iota
	flagDisabled
	flagChecked
	flagSubmenu
)

type flatItem struct {
	id, text      string
	flags, parent int
}

// flatten lists items depth-first, parents before their children, each
// with the index of its parent item (-1 for the top level).
func flatten(items []gui.NativeMenuItemCfg, parent int, out []flatItem) []flatItem {
	for _, it := range items {
		f := flatItem{id: it.ID, text: it.Text, parent: parent}
		if it.Separator {
			f.flags |= flagSeparator
		}
		if it.Disabled {
			f.flags |= flagDisabled
		}
		if it.Checked {
			f.flags |= flagChecked
		}
		if len(it.Submenu) > 0 {
			f.flags |= flagSubmenu
		}
		out = append(out, f)
		if len(it.Submenu) > 0 {
			out = flatten(it.Submenu, len(out)-1, out)
		}
	}
	return out
}

func setNativeTray(menu []gui.NativeMenuItemCfg, tooltip string) {
	items := flatten(menu, -1, nil)
	n := len(items)
	if n == 0 {
		return
	}
	ids := make([]*C.char, n)
	texts := make([]*C.char, n)
	flags := make([]C.int, n)
	parents := make([]C.int, n)
	for i, it := range items {
		ids[i], texts[i] = C.CString(it.id), C.CString(it.text)
		flags[i], parents[i] = C.int(it.flags), C.int(it.parent)
	}
	defer func() {
		for i := range items {
			C.free(unsafe.Pointer(ids[i]))
			C.free(unsafe.Pointer(texts[i]))
		}
	}()
	tip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(tip))

	C.peekaiTraySet(unsafe.Pointer(&trayIcon[0]), C.int(len(trayIcon)), tip, C.int(n),
		&ids[0], &texts[0], &flags[0], &parents[0])
}
