//go:build android

// Package ui draws simple on-screen touch controls (a virtual joystick on
// the left, an action button on the right) directly with OpenGL ES 2.
//
// This is intentionally independent of go-bridge/input/touch_mapper.go,
// which maps touches to *goro's* input.Input type. That type comes from
// the upstream github.com/kivutar/goro module (cloned fresh in CI, not
// vendored in this repo), and its exact API wasn't available to verify
// while writing this, so wiring touch_mapper.go in is left as follow-up
// work rather than guessed at here. This package only draws the controls
// and reports what the user is doing with them (JoystickState, pressed
// bool) - main_android.go decides what that means for the game once the
// input.Input wiring exists.
package ui

import (
	"encoding/binary"
	"math"

	"golang.org/x/mobile/exp/f32"
	"golang.org/x/mobile/gl"
)

const circleSegments = 24

const vertexShaderSrc = `#version 100
attribute vec2 position;
void main() {
	gl_Position = vec4(position, 0.0, 1.0);
}
`

const fragmentShaderSrc = `#version 100
precision mediump float;
uniform vec4 color;
void main() {
	gl_FragColor = color;
}
`

// JoystickState describes the current state of the left virtual joystick.
// All coordinates are screen-space fractions: 0..1 across the width/height,
// with (0,0) at the top-left, matching Android touch.Event coordinates.
type JoystickState struct {
	Active             bool
	CenterX, CenterY   float32 // where the thumb was first pressed down
	CurrentX, CurrentY float32 // where the thumb is now, while dragging
}

// Overlay renders the virtual joystick (base + thumb) and action button.
type Overlay struct {
	glctx    gl.Context
	program  gl.Program
	posAttr  gl.Attrib
	colorUni gl.Uniform
	buf      gl.Buffer

	screenW, screenH float32
}

// New compiles the overlay's shader program. Call once, after the GL
// context exists (e.g. in onStart(), same place dataLoader is used).
func New(glctx gl.Context) *Overlay {
	o := &Overlay{glctx: glctx}

	vs := glctx.CreateShader(gl.VERTEX_SHADER)
	glctx.ShaderSource(vs, vertexShaderSrc)
	glctx.CompileShader(vs)

	fs := glctx.CreateShader(gl.FRAGMENT_SHADER)
	glctx.ShaderSource(fs, fragmentShaderSrc)
	glctx.CompileShader(fs)

	o.program = glctx.CreateProgram()
	glctx.AttachShader(o.program, vs)
	glctx.AttachShader(o.program, fs)
	glctx.LinkProgram(o.program)

	o.posAttr = glctx.GetAttribLocation(o.program, "position")
	o.colorUni = glctx.GetUniformLocation(o.program, "color")
	o.buf = glctx.CreateBuffer()

	return o
}

// SetScreenSize must be called whenever the surface resizes (main_android.go's
// onResize already knows w/h - just forward them here), so the circles stay
// round instead of stretching with the screen's aspect ratio.
func (o *Overlay) SetScreenSize(w, h int) {
	o.screenW, o.screenH = float32(w), float32(h)
}

// Draw renders the joystick and action button on top of whatever was
// already drawn this frame. Call at the end of onDraw().
func (o *Overlay) Draw(js JoystickState, buttonPressed bool) {
	if o.screenW == 0 || o.screenH == 0 {
		return // SetScreenSize hasn't been called yet
	}
	glctx := o.glctx
	glctx.Enable(gl.BLEND)
	glctx.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	glctx.UseProgram(o.program)

	const (
		baseRadius   = 0.10 // fraction of screen height
		thumbRadius  = 0.045
		buttonRadius = 0.08
	)

	// Joystick base: fixed, bottom-left.
	baseCX, baseCY := o.toNDC(0.18, 0.82)
	o.drawCircle(baseCX, baseCY, baseRadius, 0.9, 0.9, 0.9, 0.25)

	// Joystick thumb: follows the drag, clamped to the base's radius.
	thumbCX, thumbCY := baseCX, baseCY
	if js.Active {
		cx, cy := o.toNDC(js.CenterX, js.CenterY)
		tx, ty := o.toNDC(js.CurrentX, js.CurrentY)
		dx, dy := tx-cx, ty-cy
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		if dist > baseRadius && dist > 0 {
			dx, dy = dx/dist*baseRadius, dy/dist*baseRadius
		}
		thumbCX, thumbCY = cx+dx, cy+dy
	}
	o.drawCircle(thumbCX, thumbCY, thumbRadius, 1, 1, 1, 0.6)

	// Action button hidden for now - it isn't wired to anything, so a
	// visible-but-dead button was just confusing. touch handling in
	// main_android.go still tracks right-side taps; only the drawing is
	// disabled. Re-enable this once it's wired into real input.
	_ = buttonPressed
	_ = buttonRadius
}

// DrawDebugToggle renders a small, always-visible dim circle in the
// top-right corner as a discoverable tap target for toggling the debug
// panel. Deliberately placed away from the joystick (bottom-left) and
// action button (bottom-right) zones so it can't be confused with gameplay
// controls. See main_android.go's onTouch for the matching hit zone.
func (o *Overlay) DrawDebugToggle() {
	if o.screenW == 0 || o.screenH == 0 {
		return
	}
	glctx := o.glctx
	glctx.Enable(gl.BLEND)
	glctx.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	glctx.UseProgram(o.program)

	cx, cy := o.toNDC(0.93, 0.06)
	o.drawCircle(cx, cy, 0.035, 0.6, 0.6, 0.9, 0.35)
}

// toNDC converts a screen-space fraction (0..1, y-down, matching touch
// events) into GL normalized device coordinates (-1..1, y-up).
func (o *Overlay) toNDC(fx, fy float32) (x, y float32) {
	return fx*2 - 1, 1 - fy*2
}

func (o *Overlay) drawCircle(cx, cy, radius, r, g, b, a float32) {
	glctx := o.glctx
	aspect := o.screenW / o.screenH // width/height, e.g. ~0.5 in portrait

	verts := make([]float32, 0, (circleSegments+2)*2)
	verts = append(verts, cx, cy) // fan center
	for i := 0; i <= circleSegments; i++ {
		theta := 2 * math.Pi * float64(i) / float64(circleSegments)
		// Divide the x offset by aspect so the circle stays round instead
		// of stretching to match the screen's (non-square) proportions.
		vx := cx + radius/aspect*float32(math.Cos(theta))
		vy := cy + radius*float32(math.Sin(theta))
		verts = append(verts, vx, vy)
	}

	glctx.BindBuffer(gl.ARRAY_BUFFER, o.buf)
	glctx.BufferData(gl.ARRAY_BUFFER, f32.Bytes(binary.LittleEndian, verts...), gl.DYNAMIC_DRAW)
	glctx.EnableVertexAttribArray(o.posAttr)
	glctx.VertexAttribPointer(o.posAttr, 2, gl.FLOAT, false, 0, 0)
	glctx.Uniform4f(o.colorUni, r, g, b, a)
	glctx.DrawArrays(gl.TRIANGLE_FAN, 0, circleSegments+2)
	glctx.DisableVertexAttribArray(o.posAttr)
}
