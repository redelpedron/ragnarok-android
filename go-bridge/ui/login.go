//go:build android

// Package ui (this file): a basic login screen, rendered the same way
// debug.Panel renders its text - draw everything into a CPU-side
// image.RGBA with golang.org/x/image/font/basicfont, upload it as one
// texture, draw it as a single textured quad. No new dependencies, same
// technique already proven in debug/debug.go.
//
// Text input caveat, verified against the actual package docs rather than
// assumed: golang.org/x/mobile/event/key states plainly that on-screen
// software keyboards do not send key events under this event model. So
// HandleKey below works today with a physical/Bluetooth keyboard, but
// nothing here pops the Android soft keyboard - that needs a JNI bridge
// (show a transparent EditText from the Java side, forward its text back
// to Go) which doesn't exist yet. Left as a TODO rather than guessed at,
// same way touch_mapper.go's *goro/input.Input mismatch was left as a
// TODO instead of quietly papered over.
package ui

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/exp/f32"
	"golang.org/x/mobile/gl"
)

type loginField int

const (
	fieldNone loginField = iota
	fieldUsername
	fieldPassword
)

const (
	loginPanelW = 600
	loginPanelH = 420
	loginMaxLen = 24 // comfortably under the ~74 glyphs that fit a 520px-wide box at 7px/glyph

	fieldBoxX = 40
	fieldBoxW = 520
	fieldBoxH = 56

	usernameBoxY = 110
	passwordBoxY = 190
	buttonBoxY   = 290
	buttonBoxH   = 64

	titleY   = 50
	statusY  = 380
)

var (
	colPanelBg   = color.RGBA{18, 24, 38, 235}
	colBoxBg     = color.RGBA{40, 48, 66, 255}
	colBoxBorder = color.RGBA{90, 100, 120, 255}
	colBoxActive = color.RGBA{210, 170, 90, 255} // warm gold, matches classic RO UI accents
	colButtonBg  = color.RGBA{120, 70, 30, 255}
	colButtonHi  = color.RGBA{170, 110, 50, 255}
	colText      = color.RGBA{235, 235, 235, 255}
	colTextDim   = color.RGBA{160, 165, 175, 255}
)

const loginVertexShaderSrc = `#version 100
attribute vec2 position;
attribute vec2 texcoord;
varying vec2 vTexcoord;
void main() {
	gl_Position = vec4(position, 0.0, 1.0);
	vTexcoord = texcoord;
}
`

const loginFragmentShaderSrc = `#version 100
precision mediump float;
varying vec2 vTexcoord;
uniform sampler2D tex;
void main() {
	gl_FragColor = texture2D(tex, vTexcoord);
}
`

// LoginScreen is a basic username/password/login-button screen, centered
// on the display. It owns its own GL program/texture, same pattern as
// Overlay and debug.Panel - each screen is self-contained.
type LoginScreen struct {
	glctx   gl.Context
	program gl.Program
	posAttr gl.Attrib
	uvAttr  gl.Attrib
	texUni  gl.Uniform
	buf     gl.Buffer
	tex     gl.Texture
	img     *image.RGBA

	screenW, screenH float32

	Username string
	Password string
	active   loginField

	status string // e.g. "Connecting..." - caller sets this via SetStatus

	cursorOn      bool
	cursorBlinkAt time.Time

	onSubmit func(username, password string)
}

// NewLogin compiles the login screen's shader and allocates its canvas.
// Call once, after the GL context exists - same place as ui.New and
// debug.New are called in onStart().
func NewLogin(glctx gl.Context) *LoginScreen {
	l := &LoginScreen{glctx: glctx, active: fieldUsername, cursorOn: true, cursorBlinkAt: time.Now()}

	vs := glctx.CreateShader(gl.VERTEX_SHADER)
	glctx.ShaderSource(vs, loginVertexShaderSrc)
	glctx.CompileShader(vs)
	fs := glctx.CreateShader(gl.FRAGMENT_SHADER)
	glctx.ShaderSource(fs, loginFragmentShaderSrc)
	glctx.CompileShader(fs)

	l.program = glctx.CreateProgram()
	glctx.AttachShader(l.program, vs)
	glctx.AttachShader(l.program, fs)
	glctx.LinkProgram(l.program)

	l.posAttr = glctx.GetAttribLocation(l.program, "position")
	l.uvAttr = glctx.GetAttribLocation(l.program, "texcoord")
	l.texUni = glctx.GetUniformLocation(l.program, "tex")
	l.buf = glctx.CreateBuffer()

	l.tex = glctx.CreateTexture()
	glctx.BindTexture(gl.TEXTURE_2D, l.tex)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	l.img = image.NewRGBA(image.Rect(0, 0, loginPanelW, loginPanelH))
	glctx.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, loginPanelW, loginPanelH, gl.RGBA, gl.UNSIGNED_BYTE, l.img.Pix)

	return l
}

// SetScreenSize must be called on resize, same as Overlay/debug.Panel, so
// the panel stays centered at a crisp pixel size instead of stretching.
func (l *LoginScreen) SetScreenSize(w, h int) {
	l.screenW, l.screenH = float32(w), float32(h)
}

// SetOnSubmit registers the callback fired when the Login button is
// tapped. Called with whatever is currently typed in each field.
func (l *LoginScreen) SetOnSubmit(fn func(username, password string)) {
	l.onSubmit = fn
}

// SetStatus sets a one-line status message shown under the button (e.g.
// "Connecting...", "Wrong username or password"). Empty string clears it.
func (l *LoginScreen) SetStatus(s string) {
	l.status = s
}

// panelOrigin returns the panel's top-left corner in real screen pixels
// (same units as touch.Event X/Y), centered on the current screen size.
// Both Draw and HandleTouchBegin derive their coordinates from this one
// function so they can never disagree about where the panel actually is.
func (l *LoginScreen) panelOrigin() (x0, y0 float32) {
	return (l.screenW - loginPanelW) / 2, (l.screenH - loginPanelH) / 2
}

// HandleTouchBegin processes a touch-down at screen-space fraction (fx,
// fy), same convention as main_android.go's onTouch (0..1, y-down).
// Returns true if the touch landed inside the panel at all, so the caller
// knows not to also treat it as a joystick/game touch.
func (l *LoginScreen) HandleTouchBegin(fx, fy float32) bool {
	if l.screenW == 0 || l.screenH == 0 {
		return false
	}
	ox, oy := l.panelOrigin()
	lx, ly := fx*l.screenW-ox, fy*l.screenH-oy
	if lx < 0 || ly < 0 || lx > loginPanelW || ly > loginPanelH {
		return false
	}

	pt := image.Pt(int(lx), int(ly))
	switch {
	case pt.In(usernameRect()):
		l.active = fieldUsername
	case pt.In(passwordRect()):
		l.active = fieldPassword
	case pt.In(buttonRect()):
		if l.onSubmit != nil {
			l.onSubmit(l.Username, l.Password)
		}
	}
	return true
}

func usernameRect() image.Rectangle {
	return image.Rect(fieldBoxX, usernameBoxY, fieldBoxX+fieldBoxW, usernameBoxY+fieldBoxH)
}

func passwordRect() image.Rectangle {
	return image.Rect(fieldBoxX, passwordBoxY, fieldBoxX+fieldBoxW, passwordBoxY+fieldBoxH)
}

func buttonRect() image.Rectangle {
	return image.Rect(fieldBoxX, buttonBoxY, fieldBoxX+fieldBoxW, buttonBoxY+buttonBoxH)
}

// HandleKey handles physical/Bluetooth keyboard input (see the package
// doc comment above for why this doesn't cover the on-screen keyboard).
func (l *LoginScreen) HandleKey(e key.Event) {
	if e.Direction != key.DirPress && e.Direction != key.DirNone {
		return // ignore releases; DirNone covers OS key-repeat
	}
	if l.active == fieldNone {
		return
	}

	switch e.Code {
	case key.CodeDeleteBackspace:
		l.backspace()
		return
	case key.CodeTab:
		l.nextField()
		return
	case key.CodeReturnEnter:
		if l.active == fieldUsername {
			l.active = fieldPassword
		} else if l.onSubmit != nil {
			l.onSubmit(l.Username, l.Password)
		}
		return
	}

	if e.Rune < 0x20 || e.Rune > 0x7e {
		return // basicfont only covers printable ASCII; skip anything outside it
	}
	l.appendRune(e.Rune)
}

func (l *LoginScreen) nextField() {
	if l.active == fieldUsername {
		l.active = fieldPassword
	} else {
		l.active = fieldUsername
	}
}

func (l *LoginScreen) appendRune(r rune) {
	switch l.active {
	case fieldUsername:
		if len([]rune(l.Username)) < loginMaxLen {
			l.Username += string(r)
		}
	case fieldPassword:
		if len([]rune(l.Password)) < loginMaxLen {
			l.Password += string(r)
		}
	}
}

func (l *LoginScreen) backspace() {
	switch l.active {
	case fieldUsername:
		if n := len([]rune(l.Username)); n > 0 {
			l.Username = string([]rune(l.Username)[:n-1])
		}
	case fieldPassword:
		if n := len([]rune(l.Password)); n > 0 {
			l.Password = string([]rune(l.Password)[:n-1])
		}
	}
}

// Draw renders the panel. Call from onDraw() while the login screen is
// the active screen (main_android.go decides that, same division of
// responsibility as Overlay/debug.Panel).
func (l *LoginScreen) Draw() {
	if l.screenW == 0 || l.screenH == 0 {
		return
	}

	if time.Since(l.cursorBlinkAt) > 500*time.Millisecond {
		l.cursorOn = !l.cursorOn
		l.cursorBlinkAt = time.Now()
	}
	l.render()

	glctx := l.glctx
	glctx.Enable(gl.BLEND)
	glctx.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	glctx.UseProgram(l.program)
	glctx.ActiveTexture(gl.TEXTURE0)
	glctx.BindTexture(gl.TEXTURE_2D, l.tex)
	glctx.Uniform1i(l.texUni, 0)

	ox, oy := l.panelOrigin()
	x0 := ox/l.screenW*2 - 1
	y0 := 1 - oy/l.screenH*2
	x1 := (ox+loginPanelW)/l.screenW*2 - 1
	y1 := 1 - (oy+loginPanelH)/l.screenH*2

	verts := []float32{
		x0, y0, 0, 0,
		x1, y0, 1, 0,
		x0, y1, 0, 1,
		x1, y1, 1, 1,
	}
	const stride = 4 * 4
	glctx.BindBuffer(gl.ARRAY_BUFFER, l.buf)
	glctx.BufferData(gl.ARRAY_BUFFER, f32.Bytes(binary.LittleEndian, verts...), gl.DYNAMIC_DRAW)

	glctx.EnableVertexAttribArray(l.posAttr)
	glctx.VertexAttribPointer(l.posAttr, 2, gl.FLOAT, false, stride, 0)
	glctx.EnableVertexAttribArray(l.uvAttr)
	glctx.VertexAttribPointer(l.uvAttr, 2, gl.FLOAT, false, stride, 2*4)

	glctx.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)

	glctx.DisableVertexAttribArray(l.posAttr)
	glctx.DisableVertexAttribArray(l.uvAttr)
}

// render redraws the CPU-side canvas and re-uploads it. Runs every Draw
// call (same as debug.Panel while visible) - the canvas is small and this
// keeps the cursor blink and typed text trivially in sync, at the cost of
// a bit of redundant redraw work; fine for a login screen that isn't up
// during actual gameplay frames.
func (l *LoginScreen) render() {
	draw.Draw(l.img, l.img.Bounds(), image.NewUniform(colPanelBg), image.Point{}, draw.Src)

	drawer := &font.Drawer{Dst: l.img, Face: basicfont.Face7x13}

	drawer.Src = image.NewUniform(colText)
	drawer.Dot = fixed.P(fieldBoxX, titleY)
	drawer.DrawString("GORO - RAGNAROK ONLINE")

	l.drawField(drawer, usernameRect(), "Username", l.Username, false, l.active == fieldUsername)
	l.drawField(drawer, passwordRect(), "Password", l.Password, true, l.active == fieldPassword)

	btn := buttonRect()
	draw.Draw(l.img, btn, image.NewUniform(colButtonBg), image.Point{}, draw.Src)
	drawBorder(l.img, btn, colButtonHi, 2)
	drawer.Src = image.NewUniform(colText)
	// Rough centering: basicfont advances ~7px per glyph.
	label := "LOG IN"
	labelX := btn.Min.X + (btn.Dx()-len(label)*7)/2
	drawer.Dot = fixed.P(labelX, btn.Min.Y+btn.Dy()/2+5)
	drawer.DrawString(label)

	if l.status != "" {
		drawer.Src = image.NewUniform(colTextDim)
		drawer.Dot = fixed.P(fieldBoxX, statusY)
		drawer.DrawString(l.status)
	}

	l.glctx.BindTexture(gl.TEXTURE_2D, l.tex)
	l.glctx.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, loginPanelW, loginPanelH, gl.RGBA, gl.UNSIGNED_BYTE, l.img.Pix)
}

func (l *LoginScreen) drawField(drawer *font.Drawer, box image.Rectangle, label, value string, masked, isActive bool) {
	drawer.Src = image.NewUniform(colTextDim)
	drawer.Dot = fixed.P(box.Min.X, box.Min.Y-8)
	drawer.DrawString(label)

	draw.Draw(l.img, box, image.NewUniform(colBoxBg), image.Point{}, draw.Src)
	borderCol := colBoxBorder
	if isActive {
		borderCol = colBoxActive
	}
	drawBorder(l.img, box, borderCol, 2)

	shown := value
	if masked {
		shown = maskString(value)
	}
	drawer.Src = image.NewUniform(colText)
	drawer.Dot = fixed.P(box.Min.X+10, box.Min.Y+box.Dy()/2+5)
	drawer.DrawString(shown)

	if isActive && l.cursorOn {
		cursorX := drawer.Dot.X.Ceil() + 2
		cur := image.Rect(cursorX, box.Min.Y+8, cursorX+2, box.Max.Y-8)
		draw.Draw(l.img, cur, image.NewUniform(colText), image.Point{}, draw.Src)
	}
}

func maskString(s string) string {
	n := len([]rune(s))
	out := make([]rune, n)
	for i := range out {
		out[i] = '*'
	}
	return string(out)
}

// drawBorder draws a w-pixel-thick rectangular outline of r, clipped to r
// so it never draws outside the field/button box it belongs to.
func drawBorder(img *image.RGBA, r image.Rectangle, col color.Color, w int) {
	u := image.NewUniform(col)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+w), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-w, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-w, r.Min.Y, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
}
