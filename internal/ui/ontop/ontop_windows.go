package ontop

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procFindWindowW          = user32.NewProc("FindWindowW")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
	procShowWindow           = user32.NewProc("ShowWindow")
	procGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	procSetLayeredAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procMonitorFromPoint     = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW      = user32.NewProc("GetMonitorInfoW")
	procGetDpiForWindow      = user32.NewProc("GetDpiForWindow")
	procSetDisplayAffinity   = user32.NewProc("SetWindowDisplayAffinity")
)

const (
	gwlExStyle        = ^uintptr(19) // -20
	wsExTransparent   = 0x00000020
	wsExToolWindow    = 0x00000080 // no taskbar button
	wsExLayered       = 0x00080000
	wsExNoActivate    = 0x08000000
	hwndTopmost       = ^uintptr(0) // -1
	swpNoSize         = 0x0001
	swpNoMove         = 0x0002
	swpNoActivate     = 0x0010
	swpShowWindow     = 0x0040
	swpFrameChanged   = 0x0020
	swHide            = 0
	swShowNoActivate  = 4
	lwaAlpha          = 0x2
	wdaExcludeCapture = 0x11 // WDA_EXCLUDEFROMCAPTURE, Windows 10 2004+
	monitorNearest    = 2
)

type rect32 struct{ Left, Top, Right, Bottom int32 }

// call runs a Win32 function and returns its result. The error from
// LazyProc.Call is GetLastError, which is only meaningful when the result
// says the call failed; window styling and placement are best effort, so
// callers that care check the result instead.
func call(p *syscall.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}

// find returns our process's top-level window with this title.
func find(title string) uintptr {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	call(procGetWindowThreadPID, hwnd, uintptr(unsafe.Pointer(&pid)))
	if int(pid) != os.Getpid() {
		return 0
	}
	return hwnd
}

func place(title string, r Rect, opts Options) bool {
	hwnd := find(title)
	if hwnd == 0 {
		return false
	}
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	ex |= wsExToolWindow | wsExNoActivate
	if opts.ClickThrough {
		if ex&wsExLayered == 0 {
			ex |= wsExLayered
			call(procSetWindowLongPtrW, hwnd, gwlExStyle, ex)
			// A layered window is invisible until it gets attributes.
			call(procSetLayeredAttributes, hwnd, 0, 255, lwaAlpha)
		}
		ex |= wsExTransparent
	} else {
		ex &^= wsExTransparent
	}
	call(procSetWindowLongPtrW, hwnd, gwlExStyle, ex)
	// Keep our overlay out of screenshots and screen recordings.
	call(procSetDisplayAffinity, hwnd, wdaExcludeCapture)

	// Sizes are logical; Windows positions in physical pixels.
	scale := 1.0
	if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi != 0 {
		scale = float64(dpi) / 96
	}
	call(procSetWindowPos, hwnd, hwndTopmost,
		uintptr(int32(r.X)), uintptr(int32(r.Y)),
		uintptr(int32(r.W*scale)), uintptr(int32(r.H*scale)),
		swpNoActivate|swpFrameChanged)
	return true
}

func setVisible(title string, visible bool) bool {
	hwnd := find(title)
	if hwnd == 0 {
		return false
	}
	if !visible {
		call(procShowWindow, hwnd, swHide)
		return true
	}
	call(procShowWindow, hwnd, swShowNoActivate)
	call(procSetWindowPos, hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
	return true
}

// background: tool windows (set in place) already stay off the taskbar.
func background() {}

const excludesFromCapture = true

// screen returns the work area (without taskbar) of the monitor under the
// mouse, in physical pixels.
func screen() Rect {
	var pt struct{ X, Y int32 }
	if call(procGetCursorPos, uintptr(unsafe.Pointer(&pt))) == 0 {
		return Rect{W: 1280, H: 720}
	}
	mon, _, _ := procMonitorFromPoint.Call(uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32, monitorNearest)
	info := struct {
		Size    uint32
		Monitor rect32
		Work    rect32
		Flags   uint32
	}{}
	info.Size = uint32(unsafe.Sizeof(info))
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&info))); r == 0 {
		return Rect{W: 1280, H: 720}
	}
	w := info.Work
	return Rect{X: float64(w.Left), Y: float64(w.Top), W: float64(w.Right - w.Left), H: float64(w.Bottom - w.Top)}
}

func debug(title string) string {
	hwnd := find(title)
	if hwnd == 0 {
		return "window not found"
	}
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	return fmt.Sprintf("hwnd=0x%x exstyle=0x%x", hwnd, ex)
}
