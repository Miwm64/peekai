package hotkey

import "golang.design/x/hotkey"

func sysMods(mods []string) []hotkey.Modifier {
	m := map[string]hotkey.Modifier{Ctrl: hotkey.ModCtrl, Shift: hotkey.ModShift, Alt: hotkey.ModOption, Super: hotkey.ModCmd}
	out := make([]hotkey.Modifier, 0, len(mods))
	for _, name := range mods {
		out = append(out, m[name])
	}
	return out
}
