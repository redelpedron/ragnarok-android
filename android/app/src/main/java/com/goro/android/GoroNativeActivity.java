package com.goro.android;

import android.app.NativeActivity;
import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.Settings;
import android.util.Log;

/**
 * NativeActivity wrapper that bootstraps the Go runtime.
 *
 * The Go binary is built as libgoro.so and loaded automatically by
 * NativeActivity via android:app.lib_name in the manifest.
 *
 * Lifecycle events are forwarded to Go via JNI or (preferably) by
 * hooking into the onNativeWindowCreated / onNativeWindowDestroyed
 * callbacks inside the Go side.
 */
public class GoroNativeActivity extends NativeActivity {
    private static final String TAG = "GoroNative";

    static {
        // libgoro.so is loaded by NativeActivity automatically,
        // but we can force-load dependencies here if needed.
        System.loadLibrary("goro");
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Immersive fullscreen
        getWindow().setFlags(
            WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON,
            WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON
        );

        hideSystemUI();

        // Request data directory permission on first run
        if (!hasDataDirectory()) {
            launchDataPicker();
        }
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) hideSystemUI();
    }

    private void hideSystemUI() {
        View decor = getWindow().getDecorView();
        decor.setSystemUiVisibility(
            View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
            | View.SYSTEM_UI_FLAG_FULLSCREEN
            | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
            | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
            | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
            | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
        );
    }

    private boolean hasDataDirectory() {
        // TODO: check SharedPreferences for configured data path
        return false;
    }

    private void launchDataPicker() {
        Intent intent = new Intent(this, DataPickerActivity.class);
        startActivityForResult(intent, 42);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == 42 && resultCode == RESULT_OK && data != null) {
            Uri uri = data.getData();
            if (uri != null) {
                String path = uri.getPath();
                Log.i(TAG, "Data directory selected: " + path);
                nativeSetDataDir(path);
            }
        }
    }

    // Called from Go via reverse JNI to request a file picker
    public static void requestDataDirPicker() {
        // Implementation would use a static reference or event bus
    }

    // Native methods implemented in Go (via gomobile / exported C symbols)
    private native void nativeSetDataDir(String path);
    private native void nativeOnPause();
    private native void nativeOnResume();
    private native void nativeOnSurfaceDestroyed();
}
