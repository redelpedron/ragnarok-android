#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Goro Android Build Script
# =============================================================================
# Prerequisites:
#   - Go 1.25+  (with android/arm64 support)
#   - Android NDK (r27+ recommended)
#   - Android SDK with build-tools 35+
#   - goro source checked out at ../goro (or adjust GORO_PATH)
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# --- Configuration ---
ANDROID_API_LEVEL="28"
ANDROID_ARCH="arm64"
ANDROID_ABI="arm64-v8a"
NDK_VERSION="27.0.12077973"

# Try to auto-detect NDK path
if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
    ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$NDK_VERSION"
fi

if [[ ! -d "$ANDROID_NDK_HOME" ]]; then
    echo "ERROR: Android NDK not found at $ANDROID_NDK_HOME"
    echo "Set ANDROID_NDK_HOME or ANDROID_HOME environment variable."
    exit 1
fi

# Toolchain
TOOLCHAIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"
CC="$TOOLCHAIN/bin/aarch64-linux-android${ANDROID_API_LEVEL}-clang"
CXX="$TOOLCHAIN/bin/aarch64-linux-android${ANDROID_API_LEVEL}-clang++"
SYSROOT="$TOOLCHAIN/sysroot"

# Output
OUT_DIR="$PROJECT_ROOT/android/app/src/main/jniLibs/$ANDROID_ABI"
mkdir -p "$OUT_DIR"

# --- Go build ---
echo "==> Building goro for android/$ANDROID_ARCH..."

# We build the Go binary as a shared library that NativeActivity can load.
# The entry point must be ANativeActivity_onCreate (provided by android_native_app_glue
# or gomobile).  Here we use the raw NDK approach.

export CGO_ENABLED=1
export GOOS=android
export GOARCH=$ANDROID_ARCH
export CC=$CC
export CXX=$CXX

# Link against android_native_app_glue and log
# The Go code includes #cgo directives, but we ensure flags here too.
export CGO_CFLAGS="-I$SYSROOT/usr/include -I$ANDROID_NDK_HOME/sources/android/native_app_glue"
export CGO_LDFLAGS="-L$SYSROOT/usr/lib -landroid -llog -landroid_native_app_glue"

cd "$PROJECT_ROOT/go-bridge"

# Build as shared library (.so)
# -buildmode=c-shared would export C symbols; but NativeActivity expects
#   ANativeActivity_onCreate which android_native_app_glue provides.
#   Our Go main() blocks forever, letting the C glue drive callbacks.
#
# Alternative: use gomobile bind / build which handles much of this.
go build -o "$OUT_DIR/libgoro.so" -buildmode=c-shared ./main_android.go

echo "==> libgoro.so built: $OUT_DIR/libgoro.so"
file "$OUT_DIR/libgoro.so"

# --- Gradle build ---
echo "==> Building APK with Gradle..."
cd "$PROJECT_ROOT/android"

if [[ -x "./gradlew" ]]; then
    ./gradlew assembleDebug
else
    echo "WARN: gradlew not found. Skipping APK build."
    echo "      Run 'gradle wrapper' in the android/ directory first."
fi

echo "==> Done."
echo "    APK should be at: android/app/build/outputs/apk/debug/app-debug.apk"
