#!/bin/bash
# Build Argus.app — a proper macOS menu bar app bundle
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$SCRIPT_DIR/.."
APP_NAME="Argus"
APP_DIR="$ROOT/dist/${APP_NAME}.app"

echo "Building Argus macOS app bundle..."

# 1. Build the menubar binary
cd "$ROOT"
go build -ldflags="-s -w" -o "$ROOT/dist/argus-menubar-bin" ./cmd/argus-menubar/

# Also build the CLI tool
go build -ldflags="-s -w" -o "$ROOT/dist/argus" ./cmd/argus/

# 2. Create .app bundle structure
rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# 3. Copy menubar binary into bundle
# NOTE: macOS filesystem is case-insensitive — do NOT copy argus (lowercase) alongside Argus,
#       they would be the same file and the second copy would overwrite the first.
#       The CLI binary is installed separately to /usr/local/bin/argus.
cp "$ROOT/dist/argus-menubar-bin" "$APP_DIR/Contents/MacOS/Argus"

# 4. Write Info.plist
# LSUIElement=1 hides the Dock icon (menu bar only app)
cat > "$APP_DIR/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.cortexark.argus</string>
    <key>CFBundleName</key>
    <string>Argus</string>
    <key>CFBundleDisplayName</key>
    <string>Argus — AI Privacy Monitor</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleExecutable</key>
    <string>Argus</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2025 Argus Contributors. MIT License.</string>
    <key>NSPrincipalClass</key>
    <string>NSApplication</string>
</dict>
</plist>
PLIST

echo ""
echo "✓  Built: $APP_DIR"
echo ""
echo "To install:"
echo "  cp -r '$APP_DIR' /Applications/"
echo "  open /Applications/Argus.app"
echo ""
echo "Or run directly:"
echo "  open '$APP_DIR'"
