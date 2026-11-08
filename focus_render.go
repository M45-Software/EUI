package eui

import "github.com/hajimehoshi/ebiten/v2"

// A contrasting halo keeps the focus indicator visible over arbitrary accents,
// icon images, swatches, and both filled and unfilled control surfaces.
func (ctx *Context) drawKeyboardFocus(dst *ebiten.Image, it *itemData, bounds, clip rect) {
	if !ctx.focusVisible || it != ctx.focusedItem || !it.Focused || !focusAvailable(it) {
		return
	}
	clipped := intersectRect(bounds, clip)
	if clipped.X1 <= clipped.X0 || clipped.Y1 <= clipped.Y0 {
		return
	}
	target := dst.RecyclableSubImage(clipped.getRectangle())
	defer target.Recycle()
	scale := max(1, ctx.uiScale)
	width, height := bounds.X1-bounds.X0, bounds.Y1-bounds.Y0
	if width < 6*scale || height < 6*scale {
		return
	}
	// Draw on the true bounds, not the clipped rectangle: scrolling does not
	// create a misleading focus edge in the middle of a partially visible item.
	drawInsideRectBorder(target, bounds.X0+scale, bounds.Y0+scale, width-2*scale, height-2*scale, 3*scale, ColorBlack)
	drawInsideRectBorder(target, bounds.X0+2*scale, bounds.Y0+2*scale, width-4*scale, height-4*scale, scale, ColorWhite)
}
