package hotkey

import "golang.design/x/hotkey"

// On X11, Mod1 is Alt and Mod4 is Super.
func sysMods(mods []string) []hotkey.Modifier {
	m := map[string]hotkey.Modifier{Ctrl: hotkey.ModCtrl, Shift: hotkey.ModShift, Alt: hotkey.Mod1, Super: hotkey.Mod4}
	out := make([]hotkey.Modifier, 0, len(mods))
	for _, name := range mods {
		out = append(out, m[name])
	}
	return out
}
