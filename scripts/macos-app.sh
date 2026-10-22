#!/bin/sh
# Wraps a peekai binary into peekai.app: an overlay-style app (no Dock icon,
# no menu bar) whose macOS permissions belong to "peekai", not the terminal.
#
#   scripts/macos-app.sh <binary> <version> <out-dir>
set -eu

bin=$1
version=$2
out=$3

app="$out/peekai.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS"
cp "$bin" "$app/Contents/MacOS/peekai"

cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>peekai</string>
	<key>CFBundleIdentifier</key>
	<string>io.github.miwm64.peekai</string>
	<key>CFBundleExecutable</key>
	<string>peekai</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${version#v}</string>
	<key>CFBundleVersion</key>
	<string>${version#v}</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST

# Ad-hoc signature: required on Apple Silicon, and lets macOS remember the
# Accessibility / Screen Recording permissions for this build.
codesign --force --sign - "$app"
echo "$app"
