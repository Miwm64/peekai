package ontop

import (
	"fmt"
	"image"
	"os"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xfixes"
	"github.com/jezek/xgb/xproto"
	"github.com/kbinani/screenshot"
)

// Wayland doesn't let apps place their windows or keep them on top, so run
// go-gui on X11 (XWayland on Wayland desktops).
func init() {
	if os.Getenv("GOGUI_WAYLAND") == "" {
		os.Setenv("GOGUI_WAYLAND", "0")
	}
}

var (
	connOnce sync.Once
	conn     *xgb.Conn
	connErr  error
)

func x() (*xgb.Conn, error) {
	connOnce.Do(func() {
		conn, connErr = xgb.NewConn()
		if connErr == nil {
			connErr = xfixes.Init(conn)
			if connErr == nil {
				_, connErr = xfixes.QueryVersion(conn, 5, 0).Reply()
			}
		}
	})
	return conn, connErr
}

func atom(c *xgb.Conn, name string) xproto.Atom {
	r, err := xproto.InternAtom(c, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return r.Atom
}

// find returns our process's managed window with this title.
func find(c *xgb.Conn, title string) xproto.Window {
	root := xproto.Setup(c).DefaultScreen(c).Root
	list, err := xproto.GetProperty(c, false, root, atom(c, "_NET_CLIENT_LIST"), xproto.AtomWindow, 0, 1<<16).Reply()
	if err != nil {
		return 0
	}
	nameAtom, utf8, pidAtom := atom(c, "_NET_WM_NAME"), atom(c, "UTF8_STRING"), atom(c, "_NET_WM_PID")
	for i := 0; i+4 <= len(list.Value); i += 4 {
		w := xproto.Window(xgb.Get32(list.Value[i:]))
		name, err := xproto.GetProperty(c, false, w, nameAtom, utf8, 0, 256).Reply()
		if err != nil || string(name.Value) != title {
			continue
		}
		pid, err := xproto.GetProperty(c, false, w, pidAtom, xproto.AtomCardinal, 0, 1).Reply()
		if err == nil && len(pid.Value) >= 4 && int(xgb.Get32(pid.Value)) != os.Getpid() {
			continue
		}
		return w
	}
	return 0
}

// wmState asks the window manager to add a _NET_WM_STATE flag.
func wmState(c *xgb.Conn, w xproto.Window, state string) {
	root := xproto.Setup(c).DefaultScreen(c).Root
	const add = 1
	ev := xproto.ClientMessageEvent{
		Format: 32, Window: w, Type: atom(c, "_NET_WM_STATE"),
		Data: xproto.ClientMessageDataUnionData32New([]uint32{add, uint32(atom(c, state)), 0, 1, 0}),
	}
	xproto.SendEvent(c, false, root,
		xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect, string(ev.Bytes()))
}

func place(title string, r Rect, opts Options) bool {
	c, err := x()
	if err != nil {
		return false
	}
	w := find(c, title)
	if w == 0 {
		return false
	}
	for _, s := range []string{"_NET_WM_STATE_ABOVE", "_NET_WM_STATE_STICKY", "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_STATE_SKIP_PAGER"} {
		wmState(c, w, s)
	}
	xproto.ConfigureWindow(c, w,
		xproto.ConfigWindowX|xproto.ConfigWindowY|xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
		[]uint32{uint32(int32(r.X)), uint32(int32(r.Y)), uint32(r.W), uint32(r.H)})

	// Click-through: an empty input region lets the mouse reach the window
	// below; nil restores the default.
	region := xfixes.Region(0)
	if opts.ClickThrough {
		if region, err = xfixes.NewRegionId(c); err == nil {
			xfixes.CreateRegion(c, region, nil)
			defer xfixes.DestroyRegion(c, region)
		}
	}
	const shapeInput = 2
	xfixes.SetWindowShapeRegion(c, w, shapeInput, 0, 0, region)
	c.Sync()
	return true
}

func setVisible(title string, visible bool) bool {
	c, err := x()
	if err != nil {
		return false
	}
	w := find(c, title)
	if w == 0 {
		return false
	}
	if visible {
		xproto.MapWindow(c, w)
		xproto.ConfigureWindow(c, w, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeAbove})
		wmState(c, w, "_NET_WM_STATE_ABOVE")
	} else {
		xproto.UnmapWindow(c, w)
	}
	c.Sync()
	return true
}

// background: skip-taskbar/pager (set in place) keep windows out of panels.
func background() {}

// suppressAutoShow: not needed on X11 so far; overlays are hidden on
// their first frame.
func suppressAutoShow() {}

// X11 has no way to keep a window out of screenshots; callers hide the
// overlay while capturing instead.
const excludesFromCapture = false

// screen returns the bounds of the monitor under the mouse.
func screen() Rect {
	var mouse image.Point
	if c, err := x(); err == nil {
		root := xproto.Setup(c).DefaultScreen(c).Root
		if p, err := xproto.QueryPointer(c, root).Reply(); err == nil {
			mouse = image.Pt(int(p.RootX), int(p.RootY))
		}
	}
	n := screenshot.NumActiveDisplays()
	for i := range n {
		if b := screenshot.GetDisplayBounds(i); mouse.In(b) {
			return Rect{X: float64(b.Min.X), Y: float64(b.Min.Y), W: float64(b.Dx()), H: float64(b.Dy())}
		}
	}
	if n > 0 {
		b := screenshot.GetDisplayBounds(0)
		return Rect{X: float64(b.Min.X), Y: float64(b.Min.Y), W: float64(b.Dx()), H: float64(b.Dy())}
	}
	return Rect{W: 1280, H: 720}
}

func debug(title string) string {
	c, err := x()
	if err != nil {
		return "x11: " + err.Error()
	}
	return fmt.Sprintf("x11 window=0x%x", find(c, title))
}
