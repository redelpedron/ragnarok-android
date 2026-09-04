# Troubleshooting

## Build errors

### `undefined: wgpu.SurfaceDescriptorFromAndroidNativeWindow`
The gogpu Android surface API is still in preview. Make sure you are on the
latest `main` branch of `github.com/gogpu/wgpu` and that the patch applied
cleanly.

### `cannot find -landroid_native_app_glue`
The NDK ships `native_app_glue` as source, not a prebuilt library. You may need
to compile it first:

```bash
$ANDROID_NDK_HOME/sources/android/native_app_glue/android_native_app_glue.c
```

Or switch to the gomobile build path which avoids this.

### `runtime: unknown gc mode` or similar on device
Ensure you are building with `GOOS=android GOARCH=arm64`. Mixing `amd64`
(target) and `arm64` (device) binaries is a common mistake.

## Runtime errors

### Black screen on launch
- Check `adb logcat | grep GoroNative` for Vulkan errors.
- Verify the device supports Vulkan 1.0+ (`adb shell dumpsys gpu`).
- Try forcing the software backend (if gogpu has one) to rule out driver bugs.

### `data.grf not found`
- The file picker should appear on first launch. If not, grant storage
  permissions manually in Android Settings → Apps → Goro RO → Permissions.
- Place `data.grf` in `/sdcard/Android/data/com.goro.android/files/` as a
  fallback.

### Touch not responding
- Ensure `GoroNativeActivity` is not being killed by the system for ANR.
- Check that `onTouchEvent` is being called in `adb logcat`.

## Performance

- Enable GPU profiling: `adb shell dumpsys gfxinfo com.goro.android`
- If frame times are high, reduce render resolution or MSAA.
- Mali GPUs are sensitive to bandwidth; prefer compressed textures.
