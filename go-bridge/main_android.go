package main

import (
	"log"
	"path/filepath"

	"github.com/kivutar/goro-android-port/assets"
	"github.com/kivutar/goro-android-port/ui"
	goroapp "github.com/kivutar/goro/app"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/res"
	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
)

var (
	glctx gl.Context

	// game is goro's real engine object (github.com/kivutar/goro/app.Game).
	// It's created once, in onStart, and already contains a live login mode
	// by the time app.New returns - see the comment there. It has nothing
	// presenting its pixels yet; see the TODO in onDraw.
	game *goroapp.Game

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

	dataDir, err := assets.ResolveDataDir("")
	if err != nil {
		log.Printf("Goro: %v", err)
		overlay = ui.New(glctx)
		return // no data dir yet - keep the overlay up rather than a bare black screen
	}
	log.Printf("Goro: using data dir %s", dataDir)

	// Diagnostic pass with goro's real GRF reader (github.com/kivutar/goro/res),
	// logged separately from app.New below so a single logcat line proves
	// actual GRF parsing happened - which archives were found and what login
	// servers their clientinfo.xml advertises - rather than just "no error".
	// app.New builds its own res.Manager internally from the same dataDir, so
	// this does scan the directory twice; that's a deliberate trade for a
	// debuggable log line, not an oversight.
	if resource, err := res.NewManager(dataDir); err != nil {
		log.Printf("Goro: resource manager: %v", err)
	} else {
		log.Printf("Goro: opened %d archive(s)", len(resource.Archives))
		for _, a := range resource.Archives {
			log.Printf("Goro:   %s - %d files", filepath.Base(a.Path()), a.Count())
		}
		for _, c := range resource.ClientInfo.Connections {
			log.Printf("Goro: login server %q at %s:%d", c.Display, c.Address, c.Port)
		}
	}

	cfg := config.Config{
		DataDir: dataDir,
		Window:  config.WindowConfig{Title: "Ragnarok", Width: screenW, Height: screenH},
		// Audio isn't verified on Android yet - leave it off until that's
		// tested deliberately, rather than as a side effect of this change.
		Audio: config.AudioConfig{Disabled: true},
	}
	g, err := goroapp.New(cfg)
	if err != nil {
		// Most likely cause: dataDir has some .grf file (so ResolveDataDir
		// passed) but not one goro can actually parse, or clientinfo.xml is
		// missing/malformed. The res.Manager diagnostic above should already
		// point at which.
		log.Printf("Goro: engine init failed: %v", err)
	} else {
		game = g
		log.Println("Goro: engine initialized - login mode is live")
		if screenW > 0 && screenH > 0 {
			game.Resize(screenW, screenH)
		}
	}

	// TODO: game now runs the real login screen's logic (see onDraw's
	// game.Update() call), but nothing turns game.Draw's output into pixels
	// yet. render.Run - goro's normal desktop entry point - creates its own
	// window via github.com/gogpu/ui, and that library has no Android
	// backend today (github.com/gogpu/gogpu's App has OnSurfaceAvailable /
	// OnResumed / etc. hooks clearly meant for Android, but no
	// internal/platform/platform_android.go exists yet to back them).
	// Until that lands upstream, or something translates goro's
	// render.Frame command buffer into raw GL ES calls by hand, this
	// overlay is the only thing actually drawn on screen.
	overlay = ui.New(glctx)
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
	if game != nil {
		game.Resize(w, h)
	}
}

func onDraw() {
	// Minimal GL ES frame — proves the pipeline works
	glctx.ClearColor(0.1, 0.2, 0.3, 1.0)
	glctx.Clear(gl.COLOR_BUFFER_BIT)

	if game != nil {
		// Runs the real login-mode state machine for this frame (server
		// ping timers, input handling, the fade-to-world transition, etc).
		// This is the same Update() goro's desktop build calls every frame
		// from render.Run - safe to call with no server connected, since
		// that's also true the instant any desktop instance launches.
		if err := game.Update(); err != nil {
			log.Printf("Goro: update error: %v", err)
		}
		// Deliberately not calling game.Draw() here yet: it would render
		// into a render.Frame command buffer that nothing consumes (see the
		// TODO in onStart), so it'd just be wasted work every frame with no
		// way to log or observe the result.
	}

	// TODO: Replace with goro's render loop once upstream Android support
	// lands, or a hand-written GL ES executor for render.Frame exists.
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
