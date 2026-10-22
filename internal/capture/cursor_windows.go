package capture

import (
	"image"
	"syscall"
	"unsafe"
)

var getCursorPos = syscall.NewLazyDLL("user32.dll").NewProc("GetCursorPos")

func cursor() (image.Point, bool) {
	var p struct{ X, Y int32 }
	if r, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&p))); r == 0 {
		return image.Point{}, false
	}
	return image.Pt(int(p.X), int(p.Y)), true
}
