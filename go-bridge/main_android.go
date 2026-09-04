//go:build android

package main

import (
	"log"
	"runtime"
	"unsafe"

	"github.com/gogpu/gogpu"
	"github.com/kivutar/goro/game"
)

/*
#cgo LDFLAGS: -landroid -llog
#include <android/native_window.h>
#include <android/native_window_jni.h>
#include <android/input.h>
#include <android/log.h>
#include <stdlib.h>

// Forward declarations for callbacks implemented in Go
extern void onSurfaceCreated(ANativeWindow* window);
extern void onSurfaceDestroyed();
extern void onTouchEvent(int32_t action, float x, float y, int32_t pointerId);
extern void onKeyEvent(int32_t action, int32_t keyCode);
extern void onResize(int width, int height);

static void handle_input(struct android_app* app, AInputEvent* event) {
	int32_t type = AInputEvent_getType(event);
	if (type == AINPUT_EVENT_TYPE_MOTION) {
		int32_t action = AMotionEvent_getAction(event);
		int32_t pointerIndex = (action & AMOTION_EVENT_ACTION_POINTER_INDEX_MASK)
			>> AMOTION_EVENT_ACTION_POINTER_INDEX_SHIFT;
		int32_t pointerId = AMotionEvent_getPointerId(event, pointerIndex);
		float x = AMotionEvent_getX(event, pointerIndex);
		float y = AMotionEvent_getY(event, pointerIndex);
		action = action & AMOTION_EVENT_ACTION_MASK;
		onTouchEvent(action, x, y, pointerId);
	} else if (type == AINPUT_EVENT_TYPE_KEY) {
		int32_t action = AKeyEvent_getAction(event);
		int32_t keyCode = AKeyEvent_getKeyCode(event);
		onKeyEvent(action, keyCode);
	}
}

static void handle_cmd(struct android_app* app, int32_t cmd) {
	switch (cmd) {
		case APP_CMD_INIT_WINDOW:
			if (app->window != NULL) {
				onSurfaceCreated(app->window);
			}
			break;
		case APP_CMD_TERM_WINDOW:
			onSurfaceDestroyed();
			break;
		case APP_CMD_WINDOW_RESIZED:
			onResize(ANativeWindow_getWidth(app->window), ANativeWindow_getHeight(app->window));
			break;
		case APP_CMD_GAINED_FOCUS:
			// Resume audio, etc.
			break;
		case APP_CMD_LOST_FOCUS:
			// Pause audio, etc.
			break;
	}
}
*/
import "C"

var (
	appCtx    *gogpu.App
	gameState *game.Game
	surfaceW  int
	surfaceH  int
)

//export onSurfaceCreated
func onSurfaceCreated(window unsafe.Pointer) {
	// window is *C.ANativeWindow
	// gogpu/wgpu Android backend accepts ANativeWindow directly for surface creation
	w := int(C.ANativeWindow_getWidth((*C.ANativeWindow)(window)))
	h := int(C.ANativeWindow_getHeight((*C.ANativeWindow)(window)))
	surfaceW, surfaceH = w, h

	log.Printf("[Goro] Surface created: %dx%d", w, h)

	// Initialize gogpu with the native window handle
	// NOTE: This API shape depends on the exact gogpu version; adjust as needed.
	cfg := gogpu.DefaultConfig().
		WithTitle("Goro").
		WithSize(w, h).
		WithNativeWindow(uintptr(window)) // hypothetical API

	appCtx = gogpu.NewApp(cfg)

	// Initialize goro game state
	gameState = game.New(appCtx)

	go runLoop()
}

//export onSurfaceDestroyed
func onSurfaceDestroyed() {
	log.Println("[Goro] Surface destroyed")
	if appCtx != nil {
		appCtx.Destroy()
		appCtx = nil
	}
}

//export onResize
func onResize(width, height C.int) {
	surfaceW, surfaceH = int(width), int(height)
	if appCtx != nil {
		appCtx.Resize(int(width), int(height))
	}
}

//export onTouchEvent
func onTouchEvent(action C.int32_t, x, y C.float, pointerId C.int32_t) {
	if gameState == nil {
		return
	}

	// Map Android touch events to goro's mouse/keyboard input system
	// goro likely uses gogpu's input abstraction or its own
	const (
		ACTION_DOWN       = 0
		ACTION_UP         = 1
		ACTION_MOVE       = 2
		ACTION_POINTER_DOWN = 5
		ACTION_POINTER_UP   = 6
	)

	switch int32(action) {
	case ACTION_DOWN, ACTION_POINTER_DOWN:
		gameState.Input().MouseDown(float32(x), float32(y), 0) // left button
	case ACTION_UP, ACTION_POINTER_UP:
		gameState.Input().MouseUp(float32(x), float32(y), 0)
	case ACTION_MOVE:
		gameState.Input().MouseMove(float32(x), float32(y))
	}
}

//export onKeyEvent
func onKeyEvent(action, keyCode C.int32_t) {
	if gameState == nil {
		return
	}
	// Map Android key codes to goro key codes
	// This is a minimal mapping; expand as needed
	keyMap := map[int32]int{
		4:   27,  // KEYCODE_BACK -> ESC
		19:  265, // KEYCODE_DPAD_UP
		20:  264, // KEYCODE_DPAD_DOWN
		21:  263, // KEYCODE_DPAD_LEFT
		22:  262, // KEYCODE_DPAD_RIGHT
		23:  257, // KEYCODE_DPAD_CENTER -> ENTER
		66:  257, // KEYCODE_ENTER
		67:  259, // KEYCODE_DEL -> BACKSPACE
	}

	if goroKey, ok := keyMap[int32(keyCode)]; ok {
		if action == 0 { // ACTION_DOWN
			gameState.Input().KeyDown(goroKey)
		} else if action == 1 { // ACTION_UP
			gameState.Input().KeyUp(goroKey)
		}
	}
}

func runLoop() {
	for appCtx != nil && appCtx.Running() {
		gameState.Update()
		gameState.Draw()
		appCtx.Present()
	}
}

func main() {
	// Lock the main thread — Android's NativeActivity requires this
	runtime.LockOSThread()

	// The android_main entry point is provided by the NDK's native_app_glue.
	// We link against it and let it call our onSurfaceCreated callback.
	//
	// If using gomobile, the entry point is different (app.Main).
	// This scaffold uses the raw NDK approach for maximum control.

	// Prevent Go from exiting; the NDK event loop keeps us alive.
	select {}
}
