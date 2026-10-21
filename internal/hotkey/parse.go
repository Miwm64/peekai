// Package hotkey parses shortcut strings and listens for global hotkeys.
package hotkey

import (
	"fmt"
	"slices"
	"strings"
)

// Modifier names after normalization.
const (
	Ctrl  = "ctrl"
	Shift = "shift"
	Alt   = "alt"   // Option on macOS
	Super = "super" // Cmd on macOS, Win on Windows
)

var modAliases = map[string]string{
	"ctrl": Ctrl, "control": Ctrl,
	"shift": Shift,
	"alt":   Alt, "option": Alt, "opt": Alt,
	"super": Super, "cmd": Super, "command": Super, "win": Super, "meta": Super,
}

// keyNames are the keys a shortcut may end with.
var keyNames = func() []string {
	k := []string{"space", "enter", "esc", "tab", "delete", "left", "right", "up", "down"}
	for c := 'a'; c <= 'z'; c++ {
		k = append(k, string(c))
	}
	for c := '0'; c <= '9'; c++ {
		k = append(k, string(c))
	}
	for i := 1; i <= 12; i++ {
		k = append(k, fmt.Sprintf("f%d", i))
	}
	return k
}()

var keyAliases = map[string]string{
	"return": "enter", "escape": "esc", "del": "delete",
}

// Shortcut is a parsed hotkey like Ctrl+Shift+S.
type Shortcut struct {
	Mods []string // sorted, normalized
	Key  string   // lowercase: "a".."z", "0".."9", "f1".."f12", "space", ...
}

func (s Shortcut) String() string {
	return strings.Join(append(slices.Clone(s.Mods), s.Key), "+")
}

// Parse reads a shortcut like "Ctrl+Shift+S" or "cmd+alt+f5".
func Parse(s string) (Shortcut, error) {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(s, " ", "")), "+")
	if len(parts) < 2 {
		return Shortcut{}, fmt.Errorf("shortcut %q: need at least one modifier and a key", s)
	}

	var sc Shortcut
	for _, p := range parts[:len(parts)-1] {
		m, ok := modAliases[p]
		if !ok {
			return Shortcut{}, fmt.Errorf("shortcut %q: unknown modifier %q", s, p)
		}
		if !slices.Contains(sc.Mods, m) {
			sc.Mods = append(sc.Mods, m)
		}
	}
	slices.Sort(sc.Mods)

	key := parts[len(parts)-1]
	if a, ok := keyAliases[key]; ok {
		key = a
	}
	if !slices.Contains(keyNames, key) {
		return Shortcut{}, fmt.Errorf("shortcut %q: unknown key %q", s, key)
	}
	sc.Key = key
	return sc, nil
}
