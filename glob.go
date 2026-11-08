package eui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	clickFlash         = time.Millisecond * 100
	scaleCheckInterval = time.Second
)

func (ctx *Context) initializeWhiteImage() {
	ctx.whiteImage.Fill(color.White)
}

// constants moved to const.go

func (ctx *Context) deviceScaleCheckDue(now time.Time) bool {
	return now.Sub(ctx.lastScaleCheck) >= scaleCheckInterval
}

// Layout reports the dimensions for the game's screen.
// Pass Ebiten's outside size values to this from your Layout function.
func (ctx *Context) Layout(outsideWidth, outsideHeight int) (int, int) {
	ctx.mustAlive()

	scale := 1.0
	if ctx.AutoHiDPI {
		scale = ctx.lastDeviceScale
		now := time.Now()
		if ctx.deviceScaleCheckDue(now) {
			scale = ebiten.Monitor().DeviceScaleFactor()
			ctx.lastDeviceScale = scale
			ctx.lastScaleCheck = now
		}
	}

	scaledW := int(float64(outsideWidth) * scale)
	scaledH := int(float64(outsideHeight) * scale)
	// outsideWidth/outsideHeight are in display-independent points. The game
	// renders at the physical framebuffer size, so the UI must use the same
	// device factor or it becomes physically tiny on Retina and other HiDPI
	// displays. userUIScale remains the user's cross-display preference.
	effectiveScale := ctx.userUIScale * float32(scale)
	if effectiveScale != ctx.uiScale {
		ctx.SetUIScale(effectiveScale)
	}

	if scaledW != ctx.screenWidth || scaledH != ctx.screenHeight {
		ctx.SetScreenSize(scaledW, scaledH)
	}
	return scaledW, scaledH
}
