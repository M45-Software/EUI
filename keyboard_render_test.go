package eui

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run separately: Ebitengine permits one RunGame invocation per process.
func TestRenderKeyboardFocus(t *testing.T) {
	ctx := newTestContext(t)

	capture := os.Getenv("EUI_KEYBOARD_CAPTURE")
	if capture == "" {
		t.Skip("set EUI_KEYBOARD_CAPTURE to a PNG path; run alone")
	}
	win := ctx.keyboardFixture(t)
	win.Size = Point{X: 700, Y: 540}
	win.Title = "Keyboard navigation"
	win.Position = Point{X: 20, Y: 20}
	win.NoScroll = true
	button := ctx.NewActionButton("Activate with Enter or Space", nil)
	check, _ := ctx.NewCheckbox()
	check.Text = "Toggle with Space"
	check.Size = Point{X: 320, Y: 28}
	radio, _ := ctx.NewRadio()
	radio.Text = "Radio group: arrow keys"
	radio.Size = Point{X: 320, Y: 28}
	radio.RadioGroup = "demo"
	slider, _ := ctx.NewSlider()
	slider.Size = Point{X: 320, Y: 28}
	slider.Label = "Slider: arrows, Home and End"
	dropdown, _ := ctx.NewDropdown()
	dropdown.Options = []string{"Choose with arrows", "Confirm with Enter", "Cancel with Escape"}
	dropdown.Size = Point{X: 320, Y: 28}
	input, _ := ctx.NewInput()
	input.Text = "Text editing keeps its shortcuts"
	input.Size = Point{X: 320, Y: 32}
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 440, Y: 120}
	tabs.Tabs = []*ItemData{ctx.NewColumn(ctx.NewLabel("Tab / Shift+Tab moves between controls.")), ctx.NewColumn(ctx.NewLabel("F6 / Shift+F6 moves between windows."))}
	tabs.Tabs[0].Name = "First page"
	tabs.Tabs[1].Name = "Second page"
	root := ctx.NewColumn(button, check, radio, slider, dropdown, input, tabs)
	win.AddItem(root)
	win.Refresh()
	game := &keyboardFocusGame{ctx: ctx, win: win, targets: []*ItemData{button, check, radio, slider, dropdown, input, tabs}, capture: capture}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type keyboardFocusGame struct {
	ctx *Context

	win     *windowData
	targets []*itemData
	capture string
	done    bool
	err     error
}

func (*keyboardFocusGame) Layout(int, int) (int, int) { return 760, 600 }
func (g *keyboardFocusGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *keyboardFocusGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *keyboardFocusGame) render(screen *ebiten.Image) error {
	ctx := g.ctx

	draw := func() { screen.Fill(color.RGBA{R: 20, G: 25, B: 34, A: 255}); g.win.Dirty = true; ctx.Draw(screen) }
	for _, scale := range []float32{1, 1.5, 2} {
		ctx.uiScale = scale
		width, height := int(760*scale), int(600*scale)
		ctx.SetScreenSize(width, height)
		canvas := ebiten.NewImage(width, height)
		err := g.checkFocusPixels(canvas)
		canvas.Deallocate()
		if err != nil {
			return fmt.Errorf("scale %v: %w", scale, err)
		}
	}
	ctx.uiScale = 1
	ctx.SetScreenSize(760, 600)

	scrollWin := ctx.NewWindow()
	scrollWin.Size = Point{X: 220, Y: 220}
	scrollWin.Position = Point{X: 500, Y: 40}
	scrollWin.Padding = 0
	scrollWin.BorderPad = 0
	scrollWin.TitleHeight = 0
	flow := ctx.NewColumn()
	flow.Fixed = true
	flow.Scrollable = true
	flow.Size = Point{X: 180, Y: 70}
	var last *ItemData
	for i := 0; i < 10; i++ {
		last = ctx.NewActionButton(fmt.Sprint(i), nil)
		last.Size = Point{X: 140, Y: 24}
		last.Position = Point{}
		flow.AddItem(last)
	}
	spacer := ctx.NewLabel("")
	spacer.Size = Point{X: 1, Y: 200}
	scrollWin.AddItem(ctx.NewColumn(spacer, flow))
	scrollWin.MarkOpen()
	ctx.Focus(last)
	draw()
	if last.DrawRect.Y1-last.DrawRect.Y0 < last.GetSize().Y-.5 || last.DrawRect.X1-last.DrawRect.X0 < last.GetSize().X-.5 {
		return fmt.Errorf("focused target remained clipped: bounds=%v size=%v", last.DrawRect, last.GetSize())
	}
	scrollWin.RemoveWindow()
	if err := ctx.LoadTheme("AccentDark"); err != nil {
		return err
	}
	if err := ctx.LoadStyle("Default"); err != nil {
		return err
	}
	ctx.Focus(g.targets[len(g.targets)-1])
	draw()
	file, err := os.Create(g.capture)
	if err != nil {
		return err
	}
	if err := png.Encode(file, screen); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (g *keyboardFocusGame) checkFocusPixels(screen *ebiten.Image) error {
	ctx := g.ctx

	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	draw := func() { screen.Fill(color.RGBA{R: 20, G: 25, B: 34, A: 255}); g.win.Dirty = true; ctx.Draw(screen) }
	before := make([]byte, width*height*4)
	after := make([]byte, len(before))
	for _, palette := range []string{"AccentDark", "Paper"} {
		if err := ctx.LoadTheme(palette); err != nil {
			return err
		}
		for _, style := range []string{"Default", "Borderless", "Outline", "HighContrast"} {
			if err := ctx.LoadStyle(style); err != nil {
				return err
			}
			for _, target := range g.targets {
				ctx.Focus(target)
				ctx.focusVisible = false
				draw()
				screen.ReadPixels(before)
				ctx.focusVisible = true
				draw()
				screen.ReadPixels(after)
				if bytes.Equal(before, after) {
					return fmt.Errorf("%s/%s: no focus pixels for control %v", palette, style, target.ItemType)
				}
				bounds := target.DrawRect
				if len(target.Tabs) > 0 {
					bounds = target.Tabs[target.ActiveTab].DrawRect
				}
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						i := (y*width + x) * 4
						if !bytes.Equal(before[i:i+4], after[i:i+4]) && !bounds.containsPoint(Point{X: float32(x), Y: float32(y)}) {
							return fmt.Errorf("focus drew outside control %v at %d,%d", target.ItemType, x, y)
						}
					}
				}
			}
		}
	}
	return nil
}
