package main

import (
	"log"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
)

var (
	glctx gl.Context
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
	if glctx != nil {
		glctx.Viewport(0, 0, w, h)
	}
}

func onDraw() {
	// Minimal GL ES frame — proves the pipeline works
	glctx.ClearColor(0.1, 0.2, 0.3, 1.0)
	glctx.Clear(gl.COLOR_BUFFER_BIT)

	// TODO: Replace with goro's render loop once upstream Android support lands.
	// For now, this renders a dark-blue screen so you know the APK works.
}

func onTouch(e touch.Event) {
	// Map touch to goro input
	log.Printf("Touch: %v at (%.0f, %.0f)", e.Type, e.X, e.Y)

	// Left half = virtual joystick (arrow keys)
	// Right half = mouse click
	// See the old touch_mapper.go for the full mapping logic.
}
