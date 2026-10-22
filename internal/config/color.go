package config

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// ParseColor reads "#RRGGBB" or "#RRGGBBAA".
func ParseColor(s string) (color.NRGBA, error) {
	hex, ok := strings.CutPrefix(s, "#")
	if !ok || (len(hex) != 6 && len(hex) != 8) {
		return color.NRGBA{}, fmt.Errorf("want a color like #1E1E2E or #1E1E2EF0, got %q", s)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("want a color like #1E1E2E or #1E1E2EF0, got %q", s)
	}
	if len(hex) == 6 {
		v = v<<8 | 0xFF
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}
