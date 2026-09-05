package main

import (
	"log"

	"github.com/kivutar/goro-android-port/assets"
	"github.com/kivutar/goro-android-port/ui"
	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
)

var (
	glctx      gl.Context
	dataLoader = assets.New("") // dataDir empty -> falls through to GORO_DATA_DIR, then defaultDataDir

	overlay          *ui.Overlay
	screenW, screenH int

	joystick         ui.JoystickState
	joystickTracking bool
	joystickSeq      touch.Sequence
	buttonPressed    bool
	buttonSeq        touch.Sequence
)

func main() {
	app.Main(func(a app.App) {
		for e := range a.Events() {
			switch e := a.Filter(e).(type) {
			case lifecycle.Event:
				switch e.Crosses(lifecycle.StageVisible) {
				case lifecycle.CrossOn:
					glctx, _ = e.DrawContext.(gl.Context)
					onStart()
				case lifecycle.CrossOff:
					onStop()
					glctx = nil
				}
			case size.Event:
				onResize(e.WidthPx, e.HeightPx)
			case paint.Event:
				if glctx != nil {
					onDraw()
					a.Publish()
				}
			case touch.Event:
				onTouch(e)
			}
		}
	})
}

func onStart() {
	log.Println("Goro: surface created")

	// Smoke-test GRF resolution (go-bridge/assets/loader.go). This just
	// confirms the file can be opened - it doesn't load it into the engine
	// yet. Look for "Goro: data.grf" in logcat after each run.
	if f, err := dataLoader.Open("data.grf"); err != nil {
		log.Printf("Goro: %v", err)
	} else {
		f.Close()
		log.Println("Goro: data.grf found and opened OK")
	}

	overlay = ui.New(glctx)

	// TODO: Initialize goro game state here.
	// gogpu.NewApp() does NOT work on Android yet (windowing unreleased).
	// Options:
	//   1. Render with raw OpenGL ES (this scaffold)
	//   2. Wait for gogpu Android platform release
	//   3. Use gogpu's Rust backend (-tags rust) + wgpu-native for Android
}

func onStop() {
	log.Println("Goro: surface destroyed")
}

func onResize(w, h int) {
	log.Printf("Goro: resize %dx%d", w, h)
	screenW, screenH = w, h
	if glctx != nil {
		glctx.Viewport(0, 0, w, h)
	}
	if overlay != nil {
		overlay.SetScreenSize(w, h)
	}
}

func onDraw() {
	// Minimal GL ES frame — proves the pipeline works
	glctx.ClearColor(0.1, 0.2, 0.3, 1.0)
	glctx.Clear(gl.COLOR_BUFFER_BIT)

	// TODO: Replace with goro's render loop once upstream Android support lands.
	// For now, this renders a dark-blue screen so you know the APK works.

	if overlay != nil {
		overlay.Draw(joystick, buttonPressed)
	}
}

func onTouch(e touch.Event) {
	log.Printf("Touch: %v at (%.0f, %.0f)", e.Type, e.X, e.Y)
	if screenW == 0 || screenH == 0 {
		return // haven't had a resize event yet, nothing to normalize against
	}

	// Left half of the screen = virtual joystick, right half = action
	// button. Tracked per-finger by touch.Sequence so both can be held at
	// once without one interrupting the other.
	fx := e.X / float32(screenW)
	fy := e.Y / float32(screenH)

	switch e.Type {
	case touch.TypeBegin:
		if fx > 0.5 {
			buttonPressed = true
			buttonSeq = e.Sequence
		} else {
			joystickTracking = true
			joystickSeq = e.Sequence
			joystick = ui.JoystickState{Active: true, CenterX: fx, CenterY: fy, CurrentX: fx, CurrentY: fy}
		}
	case touch.TypeMove:
		if joystickTracking && e.Sequence == joystickSeq {
			joystick.CurrentX, joystick.CurrentY = fx, fy
		}
	case touch.TypeEnd:
		if e.Sequence == joystickSeq {
			joystickTracking = false
			joystick.Active = false
		}
		if e.Sequence == buttonSeq {
			buttonPressed = false
		}
	}

	// TODO: touch_mapper.go already has the logic to turn this into real
	// goro input (arrow keys / mouse clicks) via *goro/input.Input, but
	// that type's exact API wasn't available to verify from here - wire it
	// in once someone can confirm input.New()'s signature against the
	// actual github.com/kivutar/goro source. For now this just drives the
	// on-screen overlay so the controls are visible and touchable.
}
