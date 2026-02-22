#!/bin/bash
# build_gui_win.sh — Builds the optimized Windows GUI exe from WSL2/Linux
# Requires: mingw64-gcc (dnf install mingw64-gcc), upx (dnf install upx)
set -euo pipefail

EXE="buildREFrameworkWinGUI.exe"
GOFILE="buildREFrameworkWinGUI.go"

echo "==> Generating resources (manifest)..."
"$(go env GOPATH)/bin/rsrc" -manifest app.manifest -o rsrc.syso

echo "==> Cross-compiling for Windows amd64 (stripped)..."
CC=x86_64-w64-mingw32-gcc \
  CGO_ENABLED=1 \
  GOOS=windows \
  GOARCH=amd64 \
  go build \
    -ldflags="-H windowsgui -s -w" \
    -o "$EXE" \
    "$GOFILE"

BEFORE=$(stat -c%s "$EXE")
echo "==> Compressing with UPX..."
upx --best "$EXE"
AFTER=$(stat -c%s "$EXE")

echo ""
echo "==> Done!"
printf "    Size: %.1f MB → %.1f MB (%.0f%% reduction)\n" \
  "$(echo "$BEFORE / 1048576" | bc -l)" \
  "$(echo "$AFTER / 1048576" | bc -l)" \
  "$(echo "scale=1; (1 - $AFTER / $BEFORE) * 100" | bc -l)"

# Optional: copy to Windows Downloads
WIN_DL="/mnt/c/Users/Mike/Downloads"
if [ -d "$WIN_DL" ]; then
  cp -v "$EXE" "$WIN_DL/"
  echo "==> Copied to $WIN_DL"
fi
