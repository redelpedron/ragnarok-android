//go:build android

// Package debug renders a small toggleable on-screen panel showing basic
// diagnostics (GRF status, screen size, FPS) as real text, using
// golang.org/x/image/font/basicfont so it needs no font asset files and no
// cgo. Deliberately independent of game/GRF internals - callers just pass
// in the lines of text they want shown (see Draw), so this package can be
// reused for any future debug info without changing it.
package debug

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/mobile/exp/f32"
	"golang.org/x/mobile/gl"
)

const (
	panelW     = 480
	panelH     = 260
	margin     = 16
	lineHeight = 16
	textInsetX = 8
	textInsetY = 16
)

const vertexShaderSrc = `#version 100
attribute vec2 position;
attribute vec2 texcoord;
varying vec2 vTexcoord;
void main() {
	gl_Position = vec4(position, 0.0, 1.0);
	vTexcoord = texcoord;
}
`

const fragmentShaderSrc = `#version 100
precision mediump float;
varying vec2 vTexcoord;
uniform sampler2D tex;
void main() {
	gl_FragColor = texture2D(tex, vTexcoord);
}
`

// Panel is a toggleable diagnostic text overlay pinned to the top-left of
// the screen.
type Panel struct {
	glctx   gl.Context
	program gl.Program
	posAttr gl.Attrib
	uvAttr  gl.Attrib
	texUni  gl.Uniform
	buf     gl.Buffer
	tex     gl.Texture
	img     *image.RGBA

	screenW, screenH float32
	Visible          bool

	frameCount     int
	fpsWindowStart time.Time
	fps            float64
}

// New compiles the panel's shader and allocates its texture. Call once,
// after the GL context exists (same place as ui.New and dataLoader).
func New(glctx gl.Context) *Panel {
	p := &Panel{glctx: glctx, fpsWindowStart: time.Now()}

	vs := glctx.CreateShader(gl.VERTEX_SHADER)
	glctx.ShaderSource(vs, vertexShaderSrc)
	glctx.CompileShader(vs)
	fs := glctx.CreateShader(gl.FRAGMENT_SHADER)
	glctx.ShaderSource(fs, fragmentShaderSrc)
	glctx.CompileShader(fs)

	p.program = glctx.CreateProgram()
	glctx.AttachShader(p.program, vs)
	glctx.AttachShader(p.program, fs)
	glctx.LinkProgram(p.program)

	p.posAttr = glctx.GetAttribLocation(p.program, "position")
	p.uvAttr = glctx.GetAttribLocation(p.program, "texcoord")
	p.texUni = glctx.GetUniformLocation(p.program, "tex")
	p.buf = glctx.CreateBuffer()

	p.tex = glctx.CreateTexture()
	glctx.BindTexture(gl.TEXTURE_2D, p.tex)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	glctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	p.img = image.NewRGBA(image.Rect(0, 0, panelW, panelH))
	glctx.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, panelW, panelH, gl.RGBA, gl.UNSIGNED_BYTE, p.img.Pix)

	return p
}

// SetScreenSize must be called on resize so the panel is positioned/sized
// in real pixels instead of stretching with the screen.
func (p *Panel) SetScreenSize(w, h int) {
	p.screenW, p.screenH = float32(w), float32(h)
}

// Toggle flips panel visibility. Wire this to a tap on the debug-toggle
// hit zone (see ui.Overlay.DrawDebugToggle for the matching visual).
func (p *Panel) Toggle() {
	p.Visible = !p.Visible
}

// Tick records a frame for the FPS counter. Call once per onDraw,
// regardless of Visible, so the FPS reading is accurate whenever the
// panel gets toggled back on.
func (p *Panel) Tick() {
	p.frameCount++
	if elapsed := time.Since(p.fpsWindowStart); elapsed >= time.Second {
		p.fps = float64(p.frameCount) / elapsed.Seconds()
		p.frameCount = 0
		p.fpsWindowStart = time.Now()
	}
}

// FPS returns the most recently measured frames-per-second. Callers
// typically fmt.Sprintf this into one of the lines passed to Draw.
func (p *Panel) FPS() float64 {
	return p.fps
}

// Draw renders lines of diagnostic text if the panel is visible. Pass
// whatever the caller wants shown - this package has no opinion on
// content, only on rendering it.
func (p *Panel) Draw(lines []string) {
	if !p.Visible || p.screenW == 0 || p.screenH == 0 {
		return
	}
	p.render(lines)

	glctx := p.glctx
	glctx.Enable(gl.BLEND)
	glctx.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	glctx.UseProgram(p.program)
	glctx.ActiveTexture(gl.TEXTURE0)
	glctx.BindTexture(gl.TEXTURE_2D, p.tex)
	glctx.Uniform1i(p.texUni, 0)

	// Pin to the top-left corner at the panel's real pixel size, so text
	// stays crisp instead of stretching to fill an arbitrary NDC box.
	x0 := -1 + 2*float32(margin)/p.screenW
	y0 := 1 - 2*float32(margin)/p.screenH
	x1 := x0 + 2*float32(panelW)/p.screenW
	y1 := y0 - 2*float32(panelH)/p.screenH

	verts := []float32{
		x0, y0, 0, 0,
		x1, y0, 1, 0,
		x0, y1, 0, 1,
		x1, y1, 1, 1,
	}
	const stride = 4 * 4 // 4 float32s per vertex * 4 bytes
	glctx.BindBuffer(gl.ARRAY_BUFFER, p.buf)
	glctx.BufferData(gl.ARRAY_BUFFER, f32.Bytes(binary.LittleEndian, verts...), gl.DYNAMIC_DRAW)

	glctx.EnableVertexAttribArray(p.posAttr)
	glctx.VertexAttribPointer(p.posAttr, 2, gl.FLOAT, false, stride, 0)
	glctx.EnableVertexAttribArray(p.uvAttr)
	glctx.VertexAttribPointer(p.uvAttr, 2, gl.FLOAT, false, stride, 2*4)

	glctx.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)

	glctx.DisableVertexAttribArray(p.posAttr)
	glctx.DisableVertexAttribArray(p.uvAttr)
}

// render redraws the CPU-side image and re-uploads it. Only happens while
// visible, so it costs nothing when the panel is hidden.
func (p *Panel) render(lines []string) {
	draw.Draw(p.img, p.img.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 200}), image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  p.img,
		Src:  image.NewUniform(color.White),
		Face: basicfont.Face7x13,
	}
	y := textInsetY
	for _, line := range lines {
		d.Dot = fixed.P(textInsetX, y)
		d.DrawString(line)
		y += lineHeight
	}

	p.glctx.BindTexture(gl.TEXTURE_2D, p.tex)
	p.glctx.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, panelW, panelH, gl.RGBA, gl.UNSIGNED_BYTE, p.img.Pix)
}
