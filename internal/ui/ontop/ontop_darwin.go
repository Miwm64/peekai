package ontop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static int peekaiPlace(const char *title, double x, double y, double w, double h, int clickThrough) {
	NSString *t = [NSString stringWithUTF8String:title];
	NSRect screen = [[NSScreen screens][0] frame];
	for (NSWindow *win in [NSApp windows]) {
		if (![[win title] isEqualToString:t]) continue;
		[win setLevel:NSStatusWindowLevel];
		NSWindowCollectionBehavior b = NSWindowCollectionBehaviorCanJoinAllSpaces |
			NSWindowCollectionBehaviorFullScreenAuxiliary |
			NSWindowCollectionBehaviorStationary |
			NSWindowCollectionBehaviorIgnoresCycle;
		if (@available(macOS 13.0, *)) {
			// Lets the window into other apps' fullscreen Spaces.
			b |= NSWindowCollectionBehaviorCanJoinAllApplications;
		}
		[win setCollectionBehavior:b];
		[win setHidesOnDeactivate:NO];
		[win setIgnoresMouseEvents:clickThrough ? YES : NO];
		// Keep our overlay out of screenshots and screen recordings.
		[win setSharingType:NSWindowSharingNone];
		// Cocoa's origin is bottom-left of the main screen.
		[win setFrame:NSMakeRect(x, screen.size.height - y - h, w, h) display:YES];
		return 1;
	}
	return 0;
}

// peekaiSetVisible shows or hides our window without activating the app,
// so it appears over the current app (also fullscreen ones) and focus
// stays where it is.
static int peekaiSetVisible(const char *title, int visible) {
	NSString *t = [NSString stringWithUTF8String:title];
	for (NSWindow *win in [NSApp windows]) {
		if (![[win title] isEqualToString:t]) continue;
		if (visible) [win orderFrontRegardless];
		else [win orderOut:nil];
		return 1;
	}
	return 0;
}

// peekaiDebug describes a window's overlay-relevant state.
static const char *peekaiDebug(const char *title) {
	NSString *t = [NSString stringWithUTF8String:title];
	for (NSWindow *win in [NSApp windows]) {
		if (![[win title] isEqualToString:t]) continue;
		NSString *s = [NSString stringWithFormat:
			@"macOS %@, policy=%ld (1=accessory), behavior=0x%lx, level=%ld, visible=%d, onActiveSpace=%d, class=%@",
			[[NSProcessInfo processInfo] operatingSystemVersionString],
			(long)[NSApp activationPolicy], (unsigned long)[win collectionBehavior],
			(long)[win level], [win isVisible], [win isOnActiveSpace], NSStringFromClass([win class])];
		return strdup([s UTF8String]);
	}
	return strdup("window not found");
}

static void peekaiAccessory(void) {
	// No Dock icon or menu bar; showing a window doesn't activate the app.
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

// peekaiScreen returns the visible area of the display under the mouse,
// in top-left-origin coordinates relative to the main display.
static NSRect peekaiScreen(void) {
	NSPoint mouse = [NSEvent mouseLocation];
	NSScreen *target = [NSScreen screens][0];
	for (NSScreen *s in [NSScreen screens]) {
		if (NSMouseInRect(mouse, [s frame], NO)) { target = s; break; }
	}
	CGFloat mainH = [[NSScreen screens][0] frame].size.height;
	NSRect vis = [target visibleFrame];
	return NSMakeRect(vis.origin.x, mainH - vis.origin.y - vis.size.height, vis.size.width, vis.size.height);
}
*/
import "C"

import "unsafe"

func place(title string, r Rect, opts Options) bool {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	ct := C.int(0)
	if opts.ClickThrough {
		ct = 1
	}
	// NoFocus: Cocoa windows at status level don't take key focus unless
	// clicked; nothing extra to do.
	return C.peekaiPlace(t, C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H), ct) == 1
}

func background() { C.peekaiAccessory() }

const excludesFromCapture = true

func debug(title string) string {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	s := C.peekaiDebug(t)
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}

func setVisible(title string, visible bool) bool {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	v := C.int(0)
	if visible {
		v = 1
	}
	return C.peekaiSetVisible(t, v) == 1
}

func screen() Rect {
	s := C.peekaiScreen()
	return Rect{X: float64(s.origin.x), Y: float64(s.origin.y), W: float64(s.size.width), H: float64(s.size.height)}
}
