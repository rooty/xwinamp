#!/usr/bin/env bash
# Builds dist/xWinAmp.app (universal arm64 + amd64) and dist/xWinAmp-macos.zip.
# Runs on macOS only: it needs lipo, sips, iconutil and ditto.
#
#   VERSION=1.2.3 scripts/build-app.sh
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.0.0-dev}"
VERSION="${VERSION#v}"
APP=dist/xWinAmp.app
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

rm -rf "$APP" dist/xWinAmp-macos.zip
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "Building binaries..."
for arch in arm64 amd64; do
	GOOS=darwin GOARCH=$arch CGO_ENABLED=1 \
		go build -trimpath -ldflags "-s -w" -o "$WORK/xwinamp-$arch" ./cmd/xwinamp
done
lipo -create -output "$APP/Contents/MacOS/xwinamp" "$WORK/xwinamp-arm64" "$WORK/xwinamp-amd64"

echo "Building icon..."
go run ./tools/mkicon -size 1024 -o "$WORK/icon.png"
ICONSET="$WORK/xwinamp.iconset"
mkdir "$ICONSET"
for size in 16 32 128 256 512; do
	sips -z $size $size "$WORK/icon.png" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
	sips -z $((size * 2)) $((size * 2)) "$WORK/icon.png" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns -o "$APP/Contents/Resources/xwinamp.icns" "$ICONSET"

cat >"$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>xWinAmp</string>
	<key>CFBundleDisplayName</key>
	<string>xWinAmp</string>
	<key>CFBundleIdentifier</key>
	<string>io.github.rooty.xwinamp</string>
	<key>CFBundleExecutable</key>
	<string>xwinamp</string>
	<key>CFBundleIconFile</key>
	<string>xwinamp</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${VERSION}</string>
	<key>CFBundleVersion</key>
	<string>${VERSION}</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.music</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST
plutil -lint "$APP/Contents/Info.plist" >/dev/null

# Ad-hoc signature: required for arm64 binaries; not a Developer ID signature,
# so Gatekeeper still asks on first launch (right click -> Open).
codesign --force --deep --sign - "$APP"

ditto -c -k --keepParent "$APP" dist/xWinAmp-macos.zip
echo "Built $APP ($VERSION) and dist/xWinAmp-macos.zip"
