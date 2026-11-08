package eui

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderIndependentContexts(t *testing.T) {
	path := os.Getenv("EUI_CONTEXT_CAPTURE")
	if path == "" {
		t.Skip("set EUI_CONTEXT_CAPTURE to a PNG path; run alone")
	}
	a, b := newTestContext(t), newTestContext(t)
	g := &contextRenderGame{a: a, b: b, path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type contextRenderGame struct {
	a, b *Context
	path string
	done bool
	err  error
}

func (*contextRenderGame) Layout(int, int) (int, int) { return 960, 400 }
func (g *contextRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *contextRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.verify(screen)
	}
}
func (g *contextRenderGame) verify(screen *ebiten.Image) error {
	a, b := g.a, g.b
	if err := b.LoadTheme("Paper"); err != nil {
		return err
	}
	if err := b.LoadStyle("Rounded"); err != nil {
		return err
	}
	b.SetFontSource(b.BoldFontSource())
	b.SetUIScale(1.25)
	b.SetPotatoMode(true)
	left, right := ebiten.NewImage(480, 400), ebiten.NewImage(480, 400)
	defer left.Deallocate()
	defer right.Deallocate()
	var wa, wb *WindowData
	for i, ctx := range []*Context{a, b} {
		ctx.AutoHiDPI = false
		ctx.SetScreenSize(480, 400)
		frame := InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}
		ctx.SetInputSource(func() InputFrame { return frame })
		win := ctx.NewWindow()
		win.Position = Point{X: 10, Y: 10}
		win.Size = Point{X: 450 / ctx.UIScale(), Y: 350 / ctx.UIScale()}
		win.Title = []string{"Context A / dark", "Context B / light + bold"}[i]
		button := ctx.NewActionButton("Independent controls", nil)
		button.Position = Point{}
		win.AddItem(ctx.NewColumn(ctx.NewLabel("Owned windows, fonts, palettes and caches"), button, ctx.NewLabel("café / local input and undo history")))
		win.MarkOpen()
		if err := ctx.Update(); err != nil {
			return err
		}
		if i == 0 {
			wa = win
			ctx.Draw(left)
		} else {
			wb = win
			ctx.Draw(right)
		}
	}
	pa, pb := make([]byte, 480*400*4), make([]byte, 480*400*4)
	left.ReadPixels(pa)
	right.ReadPixels(pb)
	if bytes.Equal(pa, pb) || a.currentTheme == b.currentTheme || a.whiteImage == b.whiteImage {
		return fmt.Errorf("independent appearances/resources were shared")
	}
	if len(a.plainUIText.entries) == 0 || len(b.plainUIText.entries) == 0 {
		return fmt.Errorf("context text caches were not exercised")
	}
	if a.uiRenderTargets.Stats().Active == 0 || b.uiRenderTargets.Stats().Active == 0 {
		return fmt.Errorf("owned window targets were not exercised")
	}
	screen.DrawImage(left, nil)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(480, 0)
	screen.DrawImage(right, op)
	// Application image ownership remains with the caller after context cleanup.
	imageItem, image := a.NewImageItem(8, 8)
	image.Fill(ColorRed)
	defer image.Deallocate()
	wa.AddItem(imageItem)
	a.Dispose()
	if a.uiRenderTargets.Stats().Active != 0 || a.uiRenderTargets.Stats().Free != 0 || wa.Render != nil || wa.Open {
		return fmt.Errorf("disposed context retained window targets")
	}
	r, _, _, alpha := image.At(0, 0).RGBA()
	if r == 0 || alpha == 0 {
		return fmt.Errorf("context disposed the application's image")
	}
	// Force B to repaint after A has returned all of its allocations.
	wb.Refresh()
	right.Clear()
	b.Draw(right)
	after := make([]byte, len(pb))
	right.ReadPixels(after)
	if !bytes.Equal(pb, after) || b.uiRenderTargets.Stats().Active == 0 {
		return fmt.Errorf("disposing A changed B's pixels or target ownership")
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, screen); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
