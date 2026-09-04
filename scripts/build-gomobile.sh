#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Simpler gomobile-based build (experimental)
# =============================================================================
# gomobile handles the NDK toolchain, manifest merging, and APK packaging.
# However, it may conflict with NativeActivity's expected entry points.
# Use this if the raw NDK approach above is too brittle.
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "==> Installing gomobile (if needed)..."
go install golang.org/x/mobile/cmd/gomobile@latest
export PATH="$PATH:$(go env GOPATH)/bin"
gomobile init

echo "==> Building gomobile apk..."
cd "$PROJECT_ROOT/go-bridge"

# gomobile build produces a standalone APK with the Go binary embedded.
# It uses the 'app' package event loop instead of NativeActivity.
# You may need to refactor main_android.go to use app.Main().
gomobile build -target=android/arm64 -o "$PROJECT_ROOT/goro-android.apk" .

echo "==> APK: $PROJECT_ROOT/goro-android.apk"
