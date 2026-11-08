package eui

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderResponsiveLayout(t *testing.T) {
	ctx := newTestContext(t)

	capture := os.Getenv("EUI_LAYOUT_CAPTURE")
	if capture == "" {
		t.Skip("set EUI_LAYOUT_CAPTURE to a PNG path; run alone")
	}
	win := ctx.keyboardFixture(t)
	win.Position = Point{X: 20, Y: 20}
	win.Size = Point{X: 460, Y: 240}
	win.Title = "Responsive layout"
	first, middle, last := ctx.layoutBox(60, 28), ctx.layoutBox(80, 28), ctx.layoutBox(50, 40)
	first.Text, middle.Text, last.Text = "First", "Fill", "Last"
	middle.FillWidth = true
	middle.MinSize.X = 80
	middle.MaxSize.X = 140
	row := ctx.NewRow(first, middle, last)
	row.Size = Point{X: 400, Y: 90}
	row.Fixed = true
	row.Filled = true
	row.Color = NewColor(10, 50, 80, 255)
	row.ContentPadding = Insets{Left: 10, Top: 10, Right: 10, Bottom: 10}
	row.Gap = 10
	row.MainAlign = LayoutCenter
	row.CrossAlign = LayoutEnd
	win.AddItem(row)
	g := &responsiveRenderGame{ctx: ctx, win: win, row: row, first: first, middle: middle, last: last, capture: capture}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type responsiveRenderGame struct {
	ctx *Context

	win                      *WindowData
	row, first, middle, last *ItemData
	capture                  string
	done                     bool
	err                      error
}

func (*responsiveRenderGame) Layout(int, int) (int, int) { return 1200, 800 }
func (g *responsiveRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *responsiveRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *responsiveRenderGame) render(screen *ebiten.Image) error {
	ctx := g.ctx

	oldScale := ctx.uiScale
	defer ctx.SetUIScale(oldScale)
	for _, palette := range []string{"AccentDark", "Paper"} {
		if err := ctx.LoadTheme(palette); err != nil {
			return err
		}
		for _, scale := range []float32{1, 1.5, 2} {
			ctx.SetUIScale(scale)
			g.first.SetVisible(true)
			g.win.Refresh()
			screen.Clear()
			ctx.Draw(screen)
			for _, check := range []struct {
				item *ItemData
				x, y float32
			}{{g.first, 65, 52}, {g.middle, 135, 52}, {g.last, 285, 40}} {
				got := Point{X: check.item.DrawRect.X0 - g.row.DrawRect.X0, Y: check.item.DrawRect.Y0 - g.row.DrawRect.Y0}
				want := Point{X: check.x * scale, Y: check.y * scale}
				if math.Abs(float64(got.X-want.X)) > .01 || math.Abs(float64(got.Y-want.Y)) > .01 {
					return fmt.Errorf("%s at %gx: rendered offset %v, want %v", palette, scale, got, want)
				}
			}
			click := Point{X: g.last.DrawRect.X0 + 10*scale, Y: g.last.DrawRect.Y0 + 10*scale}
			calls := 0
			g.last.Handler.Handle = func(ev UIEvent) {
				if ev.Type == EventClick {
					calls++
				}
			}
			ctx.activeItem = nil
			if !g.last.clickItem(click, true) || calls != 1 {
				return fmt.Errorf("aligned button's hit target did not follow its pixels")
			}
			// The gap must show the flow's background, rather than the neighboring
			// button surface or a stale pre-layout widget cache.
			x := int(g.row.DrawRect.X0 + 130*scale)
			y := int(g.row.DrawRect.Y0 + 60*scale)
			if screen.At(x, y) != g.row.Color.ToRGBA() {
				return fmt.Errorf("%s %gx: inter-child gap painted incorrectly", palette, scale)
			}
			g.first.SetVisible(false)
			screen.Clear()
			ctx.Draw(screen)
			if math.Abs(float64(g.middle.DrawRect.X0-g.row.DrawRect.X0-100*scale)) > .01 {
				return fmt.Errorf("hidden child retained a layout gap")
			}
		}
	}
	file, err := os.Create(g.capture)
	if err != nil {
		return err
	}
	if err = png.Encode(file, screen); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
