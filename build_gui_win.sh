#!/bin/bash
# build_gui_win.sh — Builds optimized Windows executables from WSL2/Linux
#
# Requirements:
#   sudo dnf install -y mingw64-gcc upx
#   go install github.com/akavel/rsrc@latest
#
# Usage:
#   ./build_gui_win.sh          # builds both GUI and CLI
#   ./build_gui_win.sh gui      # builds GUI only
#   ./build_gui_win.sh cli      # builds CLI only

set -euo pipefail

WIN_DL="/mnt/c/Users/Mike/Downloads"
GOPATH_BIN="$(go env GOPATH)/bin"

build_size() {
  local file="$1"
  echo "$(du -sh "$file" | cut -f1)"
}

build_cli() {
  local EXE="buildREFrameworkWinCLI.exe"
  echo "==> Building CLI: $EXE"

  local before
  GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o "$EXE" \
    buildREFrameworkWinCLI.go
  before=$(build_size "$EXE")

  upx --best "$EXE" >/dev/null
  local after
  after=$(build_size "$EXE")
  echo "    Size: $before → $after"

  if [ -d "$WIN_DL" ]; then
    cp -v "$EXE" "$WIN_DL/"
  fi
}

build_gui() {
  local EXE="buildREFrameworkWinGUI.exe"
  echo "==> Building GUI: $EXE"

  echo "    Generating resources (manifest)..."
  "$GOPATH_BIN/rsrc" -manifest app.manifest -o rsrc.syso

  local before
  CC=x86_64-w64-mingw32-gcc \
    CGO_ENABLED=1 \
    GOOS=windows \
    GOARCH=amd64 \
    go build \
      -ldflags="-H windowsgui -s -w" \
      -o "$EXE" \
      buildREFrameworkWinGUI.go
  before=$(build_size "$EXE")

  upx --best "$EXE" >/dev/null
  local after
  after=$(build_size "$EXE")
  echo "    Size: $before → $after"

  if [ -d "$WIN_DL" ]; then
    cp -v "$EXE" "$WIN_DL/"
  fi
}

MODE="${1:-both}"

case "$MODE" in
  gui)  build_gui ;;
  cli)  build_cli ;;
  both) build_gui; echo ""; build_cli ;;
  *)    echo "Usage: $0 [gui|cli|both]"; exit 1 ;;
esac

echo ""
echo "==> Done!"
