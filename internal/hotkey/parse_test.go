package hotkey

import (
	"strings"
	"testing"

	"github.com/Miwm64/peekai/internal/config"
)

func TestParse(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Ctrl+Shift+S", "ctrl+shift+s"},
		{"shift + ctrl + s", "ctrl+shift+s"},
		{"Cmd+Option+F5", "alt+super+f5"},
		{"Win+Alt+Return", "alt+super+enter"},
		{"ctrl+ctrl+1", "ctrl+1"},
		{"Ctrl+Escape", "ctrl+esc"},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got.String() != tt.want {
			t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ in, want string }{
		{"S", "need at least one modifier"},
		{"", "need at least one modifier"},
		{"Hyper+S", `unknown modifier "hyper"`},
		{"Ctrl+Shift+PgUp", `unknown key "pgup"`},
		{"Ctrl+", `unknown key ""`},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) err = %v, want %q", tt.in, err, tt.want)
		}
	}
}

func TestDefaultHotkeysParse(t *testing.T) {
	for action, s := range config.Default().Hotkeys.Bindings() {
		if _, err := Parse(s); err != nil {
			t.Errorf("default %s: %v", action, err)
		}
	}
}

func TestEveryKeyNameHasSystemKey(t *testing.T) {
	for _, k := range keyNames {
		if _, ok := sysKeys[k]; !ok {
			t.Errorf("key %q has no system key code", k)
		}
	}
}
