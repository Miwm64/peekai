package hotkey

import (
	"fmt"

	"golang.design/x/hotkey"
)

// sysKeys maps key names to the library's key codes.
var sysKeys = func() map[string]hotkey.Key {
	m := map[string]hotkey.Key{
		"space": hotkey.KeySpace, "enter": hotkey.KeyReturn, "esc": hotkey.KeyEscape,
		"tab": hotkey.KeyTab, "delete": hotkey.KeyDelete,
		"left": hotkey.KeyLeft, "right": hotkey.KeyRight, "up": hotkey.KeyUp, "down": hotkey.KeyDown,
		"f1": hotkey.KeyF1, "f2": hotkey.KeyF2, "f3": hotkey.KeyF3, "f4": hotkey.KeyF4,
		"f5": hotkey.KeyF5, "f6": hotkey.KeyF6, "f7": hotkey.KeyF7, "f8": hotkey.KeyF8,
		"f9": hotkey.KeyF9, "f10": hotkey.KeyF10, "f11": hotkey.KeyF11, "f12": hotkey.KeyF12,
	}
	letters := []hotkey.Key{
		hotkey.KeyA, hotkey.KeyB, hotkey.KeyC, hotkey.KeyD, hotkey.KeyE, hotkey.KeyF, hotkey.KeyG,
		hotkey.KeyH, hotkey.KeyI, hotkey.KeyJ, hotkey.KeyK, hotkey.KeyL, hotkey.KeyM, hotkey.KeyN,
		hotkey.KeyO, hotkey.KeyP, hotkey.KeyQ, hotkey.KeyR, hotkey.KeyS, hotkey.KeyT, hotkey.KeyU,
		hotkey.KeyV, hotkey.KeyW, hotkey.KeyX, hotkey.KeyY, hotkey.KeyZ,
	}
	for i, k := range letters {
		m[string(rune('a'+i))] = k
	}
	digits := []hotkey.Key{
		hotkey.Key0, hotkey.Key1, hotkey.Key2, hotkey.Key3, hotkey.Key4,
		hotkey.Key5, hotkey.Key6, hotkey.Key7, hotkey.Key8, hotkey.Key9,
	}
	for i, k := range digits {
		m[fmt.Sprint(i)] = k
	}
	return m
}()
