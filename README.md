# Goro Android Port Scaffold

This repository contains a **working scaffold** for porting
[kivutar/goro](https://github.com/kivutar/goro) (a pure-Go Ragnarok Online
client) to Android.

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│  Android APK                                │
│  ┌───────────────────────────────────────┐  │
│  │  Java: GoroNativeActivity             │  │
│  │  • NativeActivity shell               │  │
│  │  • File picker for RO data files      │  │
│  │  • Lifecycle forwarding               │  │
│  └──────────────┬────────────────────────┘  │
│                 │ JNI / NDK                 │
│  ┌──────────────▼────────────────────────┐  │
│  │  Go: main_android.go                  │  │
│  │  • ANativeWindow → gogpu Surface      │  │
│  │  • Touch → Mouse/Keyboard mapper      │  │
│  │  • Asset loader (SAF / external)      │  │
│  └──────────────┬────────────────────────┘  │
│                 │ pure Go                   │
│  ┌──────────────▼────────────────────────┐  │
│  │  goro game engine                     │  │
│  │  • gogpu/wgpu Vulkan renderer         │  │
│  │  • RO protocol, GRF, sprites, maps    │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

## Why this works

- **Zero CGO** — goro and gogpu are pure Go, so `GOOS=android` cross-compiles
  cleanly.
- **Vulkan everywhere** — gogpu's backend already targets Android `ANativeWindow`
  (preview) and outputs SPIR-V for mobile GPUs.
- **NativeActivity** — gives us the `ANativeWindow` surface without writing a
  full Java UI.

## Project Structure

```
goro-android-port/
├── android/                    # Android Studio / Gradle project
│   ├── app/
│   │   ├── src/main/
│   │   │   ├── AndroidManifest.xml
│   │   │   ├── java/com/goro/android/
│   │   │   │   ├── GoroNativeActivity.java
│   │   │   │   └── DataPickerActivity.java
│   │   │   └── jniLibs/arm64-v8a/   # ← libgoro.so lands here
│   │   └── build.gradle
│   ├── build.gradle
│   └── settings.gradle
├── go-bridge/                  # Go Android entry point & glue
│   ├── main_android.go         # NativeActivity callbacks, game loop
│   ├── input/
│   │   └── touch_mapper.go     # Touch → WASD / Mouse
│   ├── assets/
│   │   └── loader.go           # data.grf resolution on Android
│   └── go.mod
├── patches/                    # Patches for upstream repos
│   ├── 0001-gogpu-android-surface.patch
│   └── 0002-goro-android-assets.patch
├── scripts/
│   ├── build-android.sh        # Raw NDK build (recommended)
│   └── build-gomobile.sh       # Simpler gomobile alternative
└── docs/
    └── TROUBLESHOOTING.md
```

## Quick Start

### 1. Prerequisites

- Go 1.25+ with `GOOS=android` support
- Android SDK (API 28+, build-tools 35+)
- Android NDK r27+
- `goro` source checked out locally

```bash
export ANDROID_HOME=$HOME/Android/Sdk
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/27.0.12077973
```

### 2. Check out dependencies

```bash
cd ..
git clone https://github.com/kivutar/goro.git
git clone https://github.com/gogpu/gogpu.git   # or gogpu/wgpu
```

### 3. Apply patches

```bash
cd ../gogpu
git apply ../goro-android-port/patches/0001-gogpu-android-surface.patch

cd ../goro
git apply ../goro-android-port/patches/0002-goro-android-assets.patch
```

### 4. Configure go.mod

Edit `go-bridge/go.mod` and uncomment the `replace` lines to point at your
local clones.

### 5. Build

```bash
cd goro-android-port
./scripts/build-android.sh
```

The script:
1. Cross-compiles `libgoro.so` for `android/arm64` using the NDK toolchain.
2. Copies it into `android/app/src/main/jniLibs/arm64-v8a/`.
3. Runs Gradle to produce `app-debug.apk`.

### 6. Install & run

```bash
adb install android/app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n com.goro.android/.GoroNativeActivity
```

On first launch, pick the folder containing your `data.grf` and RO client files.

## Key Integration Points

### ANativeWindow → gogpu Surface

The critical bridge is in `go-bridge/main_android.go`:

```go
//export onSurfaceCreated
func onSurfaceCreated(window unsafe.Pointer) {
    w := int(C.ANativeWindow_getWidth((*C.ANativeWindow)(window)))
    h := int(C.ANativeWindow_getHeight((*C.ANativeWindow)(window)))

    cfg := gogpu.DefaultConfig().
        WithSize(w, h).
        WithNativeWindow(uintptr(window))  // <-- patches gogpu

    appCtx = gogpu.NewApp(cfg)
    gameState = game.New(appCtx)
    go runLoop()
}
```

`gogpu/wgpu` already has a Vulkan backend that can create a surface from
`ANativeWindow` (preview). The patch wires this into `gogpu.AppConfig`.

### Touch Input

`go-bridge/input/touch_mapper.go` implements a virtual joystick:

- **Left half of screen** — virtual D-pad mapped to arrow keys (WASD)
- **Right half of screen** — mouse clicks for UI interaction
- **Two-finger drag** — camera pan / zoom

### Data Files

Android 10+ scoped storage prevents direct `/sdcard` access. The scaffold uses:

1. **Storage Access Framework** (`ACTION_OPEN_DOCUMENT_TREE`) so the user picks
   their RO data folder once.
2. **Persistable URI permissions** so the app retains access across restarts.
3. **Fallback paths** for older Android versions.

## Known Issues & TODOs

| Issue | Workaround / Plan |
|---|---|
| gogpu Android surface is **preview** | Test on Adreno & Mali; report bugs upstream |
| No on-screen keyboard | Add `SoftInput` toggle in Java layer; bridge to Go |
| Audio not wired | goro uses a pure-Go audio lib; verify `GOOS=android` build |
| Screen rotation | Handle `APP_CMD_TERM_WINDOW` → recreate surface |
| Battery / thermal throttling | Request `PowerPreference::LowPower` on mobile |
| Virtual buttons UI | Draw touch overlays with gogpu UI or raw Vulkan |

## Alternative: gomobile

If the raw NDK approach feels too low-level, try:

```bash
./scripts/build-gomobile.sh
```

This uses `golang.org/x/mobile`, which handles the activity lifecycle and
APK packaging automatically. You will need to refactor `main_android.go` to
use `app.Main()` instead of `ANativeActivity_onCreate`.

## License

This scaffold is provided as a reference implementation. Respect the licenses
of goro (MIT), gogpu, and Ragnarok Online client assets.
# ragnarok-android
