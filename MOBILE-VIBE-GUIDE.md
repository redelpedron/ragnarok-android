package main

import (
	"fmt"
	"log"

	"github.com/kivutar/goro-android-port/assets"
	"github.com/kivutar/goro-android-port/debug"
	"github.com/kivutar/goro-android-port/ui"
	"github.com/kivutar/goro/res"
	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
)

var (
	glctx      gl.Context
	dataLoader = assets.New("") // dataDir empty -> falls through to GORO_DATA_DIR, then defaultDataDir
	grfArchive *res.GRF

	overlay          *ui.Overlay
	debugPanel       *debug.Panel
	loginScreen      *ui.LoginScreen
	showingLogin     = true // no character-select/gameplay screen exists yet, so this is the only screen
	screenW, screenH int

	joystick             ui.JoystickState
	joystickTracking     bool
	joystickSeq          touch.Sequence
	buttonPressed        bool
	buttonSeq            touch.Sequence
	debugToggleTracking  bool
	debugToggleSeq       touch.Sequence
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
			case key.Event:
				onKey(e)
			}
		}
	})
}

func onStart() {
	log.Println("Goro: surface created")

	overlay = ui.New(glctx)
	debugPanel = debug.New(glctx)
	loginScreen = ui.NewLogin(glctx)
	loginScreen.SetOnSubmit(handleLoginSubmit)
	if screenW != 0 || screenH != 0 {
		loginScreen.SetScreenSize(screenW, screenH) // onStart can re-fire after onResize already ran
	}

	if grfArchive == nil { // onStart re-fires on every resume, not just launch
		loadGRF()
	}

	// TODO: Initialize goro game state here.
	// gogpu.NewApp() does NOT work on Android yet (windowing unreleased).
	// Options:
	//   1. Render with raw OpenGL ES (this scaffold)
	//   2. Wait for gogpu Android platform release
	//   3. Use gogpu's Rust backend (-tags rust) + wgpu-native for Android
}

// handleLoginSubmit fires when the login screen's button is tapped (or
// Enter is pressed from the password field). Real network login isn't
// wired yet - goro's session/network login API wasn't available to verify
// here (same reason touch_mapper.go's input wiring was left as a TODO
// instead of guessed at), so this only gives the tester visible feedback
// that the tap registered. Never log the password.
func handleLoginSubmit(username, password string) {
	log.Printf("Goro: login submitted, user=%q (network login not wired yet)", username)
	loginScreen.SetStatus("Login not wired to network yet - see handleLoginSubmit")
}

func onKey(e key.Event) {
	if showingLogin && loginScreen != nil {
		loginScreen.HandleKey(e)
	}
}

// loadGRF locates data.grf via the asset loader, then parses it with
// goro's own res.OpenGRF - the real, tested GRF reader from
// github.com/kivutar/goro/res (handles the DES-encrypted entry types and
// Korean filename encoding for real; nothing here is guessed).
func loadGRF() {
	path, err := dataLoader.ResolvePath("data.grf")
	if err != nil {
		log.Printf("Goro: %v", err)
		return
	}
	archive, err := res.OpenGRF(path)
	if err != nil {
		log.Printf("Goro: data.grf found at %s but failed to parse: %v", path, err)
		return
	}
	grfArchive = archive
	log.Printf("Goro: data.grf opened OK, %d files", archive.Count())
	names := archive.Names()
	for i := 0; i < len(names) && i < 5; i++ {
		log.Printf("Goro: sample entry: %s", names[i])
	}
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
	if debugPanel != nil {
		debugPanel.SetScreenSize(w, h)
	}
	if loginScreen != nil {
		loginScreen.SetScreenSize(w, h)
	}
}

func onDraw() {
	// Minimal GL ES frame — proves the pipeline works
	glctx.ClearColor(0.1, 0.2, 0.3, 1.0)
	glctx.Clear(gl.COLOR_BUFFER_BIT)

	// TODO: Replace with goro's render loop once upstream Android support lands.
	// For now, this renders a dark-blue screen so you know the APK works.

	if showingLogin && loginScreen != nil {
		loginScreen.Draw()
	} else if overlay != nil {
		// Joystick/action-button controls only make sense once there's a
		// game to control - hidden while the login screen is up.
		overlay.Draw(joystick, buttonPressed)
	}
	if overlay != nil {
		overlay.DrawDebugToggle()
	}

	if debugPanel != nil {
		debugPanel.Tick()
		debugPanel.Draw(debugLines())
	}
}

// debugLines is the content shown in the debug panel. Kept in
// main_android.go (not the debug package) since it's the one place that
// knows about GRF/game state - the debug package itself stays generic.
func debugLines() []string {
	grfStatus := "data.grf: not loaded"
	if grfArchive != nil {
		grfStatus = fmt.Sprintf("data.grf: %d files", grfArchive.Count())
	}
	return []string{
		fmt.Sprintf("FPS: %.0f", debugPanel.FPS()),
		fmt.Sprintf("Screen: %dx%d", screenW, screenH),
		grfStatus,
		fmt.Sprintf("Joystick active: %v", joystick.Active),
		fmt.Sprintf("Button pressed: %v", buttonPressed),
	}
}

func onTouch(e touch.Event) {
	log.Printf("Touch: %v at (%.0f, %.0f)", e.Type, e.X, e.Y)
	if screenW == 0 || screenH == 0 {
		return // haven't had a resize event yet, nothing to normalize against
	}

	// Top-right corner = debug panel toggle, checked first so it can't be
	// confused with the joystick/button zones below it.
	fx := e.X / float32(screenW)
	fy := e.Y / float32(screenH)

	if fx > 0.85 && fy < 0.12 {
		switch e.Type {
		case touch.TypeBegin:
			debugToggleTracking = true
			debugToggleSeq = e.Sequence
		case touch.TypeEnd:
			if debugToggleTracking && e.Sequence == debugToggleSeq {
				debugToggleTracking = false
				if debugPanel != nil {
					debugPanel.Toggle()
				}
			}
		}
		return
	}

	if showingLogin {
		if e.Type == touch.TypeBegin && loginScreen != nil {
			loginScreen.HandleTouchBegin(fx, fy)
		}
		return // no joystick/gameplay touches while the login screen is up
	}

	// Left half of the screen = virtual joystick, right half = action
	// button. Tracked per-finger by touch.Sequence so both can be held at
	// once without one interrupting the other.

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

	// TODO: touch_mapper.go currently references *goro/input.Input, which
	// doesn't exist. The real type is *goro/input.State (confirmed against
	// actual upstream source), with SetTouch/SetKey/SetMouseButton methods
	// that fit this exactly - deliberately left unwired this round to keep
	// the test cycle focused. See the "Wire up real GRF file parsing" task
	// in Asana for details.
}
