package eui

import (
	"github.com/hajimehoshi/ebiten/v2"
)

const touchScrollScale = .05

// PointerPosition returns the current native pointer, or the latest replayed
// pointer when a custom source is installed. A sole touch (or lowest ID in a
// multi-touch gesture) takes precedence over the mouse. UI handlers use their
// latched frame, so calls from an embedding host do not resample UI input.
func (ctx *Context) PointerPosition() (int, int) {
	ctx.mustAlive()

	if ctx.inputSource != nil {
		return ctx.sampledInput.x, ctx.sampledInput.y
	}
	ids := ebiten.AppendTouchIDs(nil)
	if len(ids) > 0 {
		first := ids[0]
		for _, id := range ids[1:] {
			if id < first {
				first = id
			}
		}
		return ebiten.TouchPosition(first)
	}
	return ebiten.CursorPosition()
}
func (ctx *Context) pointerWheel() (float64, float64) {
	return ctx.sampledInput.wheelX, ctx.sampledInput.wheelY
}
func (ctx *Context) pointerJustPressed() bool  { return ctx.sampledInput.primaryJust }
func (ctx *Context) pointerPressed() bool      { return ctx.sampledInput.primaryDown }
func (ctx *Context) pointerPressDuration() int { return ctx.sampledInput.primaryTicks }

// CursorShape returns the most recent shape requested by this context's input
// handling. It can be used by hosts drawing their own pointer.
func (ctx *Context) CursorShape() ebiten.CursorShapeType { ctx.mustAlive(); return ctx.cursorShape }
