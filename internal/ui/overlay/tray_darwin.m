//go:build darwin

// Native macOS tray menu with real submenus (see tray_darwin.go).

#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

enum { flagSeparator = 1, flagDisabled = 2, flagChecked = 4, flagSubmenu = 8 };

@interface PeekaiTrayTarget : NSObject
- (void)clicked:(NSMenuItem *)item;
@end

@implementation PeekaiTrayTarget
- (void)clicked:(NSMenuItem *)item {
	NSString *ident = item.representedObject;
	if (ident.length > 0) {
		peekaiTrayClicked((char *)ident.UTF8String);
	}
}
@end

static NSStatusItem *peekaiStatusItem;
static PeekaiTrayTarget *peekaiTarget;

static NSString *str(const char *s) {
	return [NSString stringWithUTF8String:(s != NULL ? s : "")];
}

// peekaiTraySet creates the status item on first call and replaces its menu.
// Items come depth-first with parents before children; parents[i] is the
// index of item i's parent (-1 = top level). Main thread only.
void peekaiTraySet(const void *png, int pngLen, const char *tooltip, int n,
	char **ids, char **texts, int *flags, int *parents) {
	if (peekaiStatusItem == nil) {
		peekaiStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
		peekaiTarget = [PeekaiTrayTarget new];
		NSImage *img = [[NSImage alloc] initWithData:[NSData dataWithBytes:png length:pngLen]];
		img.size = NSMakeSize(18, 18);
		peekaiStatusItem.button.image = img;
	}
	peekaiStatusItem.button.toolTip = str(tooltip);

	NSMenu *root = [NSMenu new];
	root.autoenablesItems = NO;
	NSMutableArray *submenus = [NSMutableArray arrayWithCapacity:n]; // per item: NSMenu or NSNull
	for (int i = 0; i < n; i++) {
		NSMenu *parent = parents[i] < 0 ? root : submenus[parents[i]];
		NSMenuItem *mi;
		if (flags[i] & flagSeparator) {
			mi = [NSMenuItem separatorItem];
		} else {
			mi = [[NSMenuItem alloc] initWithTitle:str(texts[i]) action:@selector(clicked:) keyEquivalent:@""];
			mi.target = peekaiTarget;
			mi.representedObject = str(ids[i]);
			mi.enabled = (flags[i] & flagDisabled) == 0;
			mi.state = (flags[i] & flagChecked) ? NSControlStateValueOn : NSControlStateValueOff;
		}
		id sub = [NSNull null];
		if (flags[i] & flagSubmenu) {
			NSMenu *m = [[NSMenu alloc] initWithTitle:mi.title];
			m.autoenablesItems = NO;
			mi.action = nil;
			mi.submenu = m;
			sub = m;
		}
		[submenus addObject:sub];
		[parent addItem:mi];
	}
	peekaiStatusItem.menu = root;
}
