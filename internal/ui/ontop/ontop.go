// Package ontop does what the GUI toolkit cannot: keep our windows above
// other apps (including fullscreen ones), place them, and make them
// click-through. Windows are found by title, so the toolkit stays swappable.
package ontop

// Rect is in screen points, origin at the top-left of the main display;
// other displays have offsets (possibly negative).
type Rect struct{ X, Y, W, H float64 }

// Options for Place.
type Options struct {
	ClickThrough bool // mouse events pass to the app underneath
	NoFocus      bool // showing the window does not steal keyboard focus
}

// Place finds our window titled title, keeps it above all other windows and
// moves it to r. It must run on the UI (main) thread. It reports whether
// the window was found.
func Place(title string, r Rect, opts Options) bool { return place(title, r, opts) }

// Show puts our window titled title on screen above the current app
// without activating peekai (the toolkit's own Show would switch Spaces
// away from fullscreen apps). UI thread only.
func Show(title string) bool { return setVisible(title, true) }

// Hide takes the window off screen. UI thread only.
func Hide(title string) bool { return setVisible(title, false) }

// Debug describes the window's overlay state for troubleshooting.
func Debug(title string) string { return debug(title) }

// Background makes the app an overlay: no Dock/taskbar entry and showing a
// window doesn't take focus from the app you're using. UI thread only.
func Background() { background() }

// Screen returns the usable area (without menu bar and dock/taskbar where
// the OS reports it) of the display the mouse is on, in the same
// coordinates as Place.
func Screen() Rect { return screen() }
