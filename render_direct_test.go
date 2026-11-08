package eui

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestDrawItemAtFreshButtonUsesNormalColor(t *testing.T) {
	ctx := newTestContext(t)

	if os.Getenv("EUI_RENDER_DIRECT_TEST") == "" {
		t.Skip("set EUI_RENDER_DIRECT_TEST=1 to render direct items")
	}
	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	previousTheme, previousRenderNow := ctx.currentTheme, ctx.renderNow
	t.Cleanup(func() {
		ctx.currentTheme = previousTheme
		ctx.renderNow = previousRenderNow
	})

	theme := *baseTheme
	theme.Button.Color = NewColor(17, 34, 51, 255)
	theme.Button.ClickColor = NewColor(119, 136, 153, 255)
	ctx.currentTheme = &theme
	ctx.renderNow = time.Time{}

	g := &directButtonColorGame{ctx: ctx, theme: &theme}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	if !ctx.renderNow.IsZero() {
		t.Fatalf("DrawItemAt left render timestamp set: %v", ctx.renderNow)
	}
}

type directButtonColorGame struct {
	ctx *Context

	theme *Theme
	done  bool
	err   error
}

func (*directButtonColorGame) Layout(_, _ int) (int, int) { return 20, 20 }

func (g *directButtonColorGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *directButtonColorGame) Draw(_ *ebiten.Image) {
	ctx := g.ctx

	if g.done {
		return
	}
	defer func() { g.done = true }()
	button, _ := ctx.NewButton()
	button.Size = Point{X: 20, Y: 20}
	button.Fillet = 0
	canvas := ebiten.NewImage(20, 20)
	defer canvas.Deallocate()
	ctx.DrawItemAt(canvas, button, Point{})
	r, green, b, a := canvas.At(10, 10).RGBA()
	got := NewColor(uint8(r>>8), uint8(green>>8), uint8(b>>8), uint8(a>>8))
	if got != g.theme.Button.Color {
		g.err = fmt.Errorf("fresh direct button color = %v, want normal color %v", got, g.theme.Button.Color)
	}
}
