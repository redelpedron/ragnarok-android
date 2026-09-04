//go:build android

package input

import (
	"github.com/kivutar/goro/input"
)

// TouchMapper translates Android touch events into goro input events.
// It also provides on-screen virtual controls for RO gameplay.
type TouchMapper struct {
	gameInput *input.Input

	// Virtual joystick state
	joystickActive bool
	joystickCenterX, joystickCenterY float32
	joystickCurrentX, joystickCurrentY float32
}

func NewTouchMapper(gi *input.Input) *TouchMapper {
	return &TouchMapper{gameInput: gi}
}

func (tm *TouchMapper) HandleTouch(action int, x, y float32, pointerID int) {
	// Split screen: left half = joystick, right half = buttons

	switch action {
	case 0, 5: // DOWN / POINTER_DOWN
		if x < 0.5 { // left side: joystick
			tm.joystickActive = true
			tm.joystickCenterX = x
			tm.joystickCenterY = y
			tm.joystickCurrentX = x
			tm.joystickCurrentY = y
		} else {
			tm.gameInput.MouseDown(x, y, 0)
		}

	case 1, 6: // UP / POINTER_UP
		if tm.joystickActive && pointerID == 0 {
			tm.joystickActive = false
			tm.gameInput.KeyUp(262)
			tm.gameInput.KeyUp(263)
			tm.gameInput.KeyUp(264)
			tm.gameInput.KeyUp(265)
		} else {
			tm.gameInput.MouseUp(x, y, 0)
		}

	case 2: // MOVE
		if tm.joystickActive {
			tm.joystickCurrentX = x
			tm.joystickCurrentY = y
			tm.updateJoystickKeys()
		} else {
			tm.gameInput.MouseMove(x, y)
		}
	}
}

func (tm *TouchMapper) updateJoystickKeys() {
	dx := tm.joystickCurrentX - tm.joystickCenterX
	dy := tm.joystickCurrentY - tm.joystickCenterY
	const threshold = 0.05

	if dx > threshold {
		tm.gameInput.KeyDown(262)
	} else {
		tm.gameInput.KeyUp(262)
	}
	if dx < -threshold {
		tm.gameInput.KeyDown(263)
	} else {
		tm.gameInput.KeyUp(263)
	}
	if dy > threshold {
		tm.gameInput.KeyDown(264)
	} else {
		tm.gameInput.KeyUp(264)
	}
	if dy < -threshold {
		tm.gameInput.KeyDown(265)
	} else {
		tm.gameInput.KeyUp(265)
	}
}
