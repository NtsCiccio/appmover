#!/usr/bin/env bash
set -euo pipefail

APP_NAME="appmover"
OUTPUT_DIR="dist"
OUTPUT="${OUTPUT_DIR}/${APP_NAME}.exe"
ICON_PATH="internal/tray/icon.ico"
VERSION="${1:-dev}"

if [ ! -f "$ICON_PATH" ]; then
  echo "Error: missing $ICON_PATH (required by //go:embed in tray.go)" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "Building for Windows (amd64), version ${VERSION}..."
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -X appmover/internal/version.Version=${VERSION}" -o "$OUTPUT" ./cmd/appmover

echo "Done: $OUTPUT"