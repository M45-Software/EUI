package eui

import (
	"github.com/M45-Software/EUI/renderpool"
	"image"
	"image/color"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// UnmanagedImageCreated is an optional diagnostics hook installed by the
// client when asset-load tracing is enabled.

// SetPotatoMode toggles creation of unmanaged ebiten images.
func (ctx *Context) SetPotatoMode(v bool) {
	ctx.mustAlive()

	if ctx.potatoMode == v {
		return
	}
	ctx.potatoMode = v
	ctx.uiPrimitives.clear()
	ctx.plainUIText.clear()
	if ctx.whiteImage != nil {
		ctx.whiteImage.Deallocate()
	}
	ctx.whiteImage = ctx.newManagedImage(3, 3)
	ctx.whiteImage.Fill(color.White)
}

// SetWindowShadows controls drop shadows for windows and dropdown menus.
func (ctx *Context) SetWindowShadows(v bool) {
	ctx.mustAlive()

	if ctx.windowShadows == v {
		return
	}
	ctx.windowShadows = v
	for _, win := range ctx.windows {
		win.Dirty = true
	}
}

// Managed images let Ebitengine share cached UI textures on its atlases.
// Cache owners recycle replaceable targets with uiRenderTargets.
func (ctx *Context) newManagedImage(w, h int) *ebiten.Image {
	if ctx.potatoMode {
		return ctx.newUnmanagedImage(w, h)
	}
	return ebiten.NewImage(w, h)
}

func (ctx *Context) newUnmanagedImage(w, h int) *ebiten.Image {
	if ctx.UnmanagedImageCreated == nil {
		return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
	}
	started := time.Now()
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
	site := "eui.unknown"
	if pc, _, _, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			site = fn.Name()
		}
	}
	ctx.UnmanagedImageCreated(w, h, time.Since(started), site)
	return img
}

func (ctx *Context) releaseUIRenderTarget(img *ebiten.Image) {
	if img != nil && !ctx.uiRenderTargets.Release(img) {
		img.Deallocate()
	}
}

// RenderTargetPoolStats reports active and idle allocations for diagnostics.
func (ctx *Context) RenderTargetPoolStats() renderpool.Stats {
	ctx.mustAlive()
	return ctx.uiRenderTargets.Stats()
}

// ClearRenderTargetPool releases idle allocations without invalidating windows.
func (ctx *Context) ClearRenderTargetPool() {
	ctx.mustAlive()
	ctx.uiRenderTargets.Clear()
}
