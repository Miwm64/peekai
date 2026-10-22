package capture

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>

static CGPoint peekaiCursor(void) {
	CGEventRef e = CGEventCreate(NULL);
	CGPoint p = CGEventGetLocation(e); // global, top-left origin, like CGDisplayBounds
	CFRelease(e);
	return p;
}
*/
import "C"

import "image"

func cursor() (image.Point, bool) {
	p := C.peekaiCursor()
	return image.Pt(int(p.x), int(p.y)), true
}
