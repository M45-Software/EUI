package eui

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderModalLayers(t *testing.T) {
	path := os.Getenv("EUI_MODAL_CAPTURE")
	if path == "" {
		t.Skip("set EUI_MODAL_CAPTURE to a PNG path; run alone")
	}
	game := &modalRenderGame{path: path}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type modalRenderGame struct {
	path string
	done bool
	err  error
}

func (*modalRenderGame) Layout(int, int) (int, int) { return 900, 600 }
func (g *modalRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *modalRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.verify(screen)
	}
}
func (g *modalRenderGame) verify(screen *ebiten.Image) error {
	for _, theme := range []string{"AccentDark", "Paper"} {
		for _, scale := range []float32{1, 1.5, 2} {
			if err := g.checkCase(screen, theme, scale); err != nil {
				return err
			}
		}
	}
	file, err := os.Create(g.path)
	if err != nil {
		return err
	}
	if err = png.Encode(file, screen); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
func (g *modalRenderGame) checkCase(screen *ebiten.Image, theme string, scale float32) error {
	ctx, err := NewContext()
	if err != nil {
		return err
	}
	defer ctx.Dispose()
	if err = ctx.LoadTheme(theme); err != nil {
		return err
	}
	ctx.AutoHiDPI = false
	ctx.SetUIScale(scale)
	ctx.SetScreenSize(900, 600)
	frame := InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}
	ctx.SetInputSource(func() InputFrame { return frame })
	step := func() error {
		frame.Time = frame.Time.Add(time.Second / 60)
		if err := ctx.Update(); err != nil {
			return err
		}
		screen.Clear()
		ctx.Draw(screen)
		return nil
	}
	pixel := func(x, y int) [4]uint32 { r, g, b, a := screen.At(x, y).RGBA(); return [4]uint32{r, g, b, a} }
	base := ctx.NewWindow()
	base.Position = Point{X: 20 / scale, Y: 20 / scale}
	base.Size = Point{X: 740 / scale, Y: 520 / scale}
	base.Movable = false
	base.Resizable = false
	base.Title = "Background / blocked while modal"
	base.BGColor = Color{R: 130, G: 140, B: 150, A: 255}
	behind := ctx.NewActionButton("Background action", nil)
	behind.Position = Point{}
	base.AddItem(ctx.NewColumn(behind))
	base.MarkOpen()
	if err = step(); err != nil {
		return err
	}
	ctx.Focus(behind)
	background := pixel(100, 400)
	first := ctx.NewWindow()
	first.Position = Point{X: 180 / scale, Y: 100 / scale}
	first.Size = Point{X: 460 / scale, Y: 380 / scale}
	first.Title = "Parent dialog / blocked by child"
	first.Movable = false
	first.Resizable = false
	button := ctx.NewActionButton("Parent action", nil)
	button.Position = Point{}
	first.AddItem(ctx.NewColumn(button))
	first.MarkOpenModal()
	if err = step(); err != nil {
		return err
	}
	dimmed := pixel(100, 400)
	if dimmed[0] >= background[0] || dimmed[1] >= background[1] {
		return fmt.Errorf("%s %gx: modal backdrop did not dim the background", theme, scale)
	}
	parent := pixel(250, 420)
	second := ctx.NewWindow()
	second.Position = Point{X: 360 / scale, Y: 230 / scale}
	second.Size = Point{X: 330 / scale, Y: 230 / scale}
	second.Title = "Top modal / interactive"
	second.Movable = false
	second.Resizable = false
	selected := ctx.NewActionButton("Confirm", nil)
	selected.Position = Point{}
	selected.Size.X = 90
	second.AddItem(ctx.NewColumn(selected))
	second.DefaultButton = selected
	second.MarkOpenModal()
	if err = step(); err != nil {
		return err
	}
	nested := pixel(250, 420)
	if nested[0] >= parent[0] && nested[1] >= parent[1] {
		return fmt.Errorf("%s %gx: nested backdrop did not dim parent", theme, scale)
	}
	top := pixel(500, 420)
	// A newly opened ordinary window and explicit z-order changes cannot cover
	// the modal even when they occupy the same rectangle in the retained list.
	utility := ctx.NewWindow()
	utility.Position = second.Position
	utility.Size = second.Size
	utility.BGColor = ColorRed
	utility.Title = "Ordinary utility"
	utility.MarkOpen()
	utility.BringForward()
	second.ToBack()
	if err = step(); err != nil {
		return err
	}
	if pixel(500, 420) != top || ctx.activeWindow != second {
		return fmt.Errorf("%s %gx: ordinary window covered active modal", theme, scale)
	}
	hit := itemCenter(selected)
	if !selected.DrawRect.containsPoint(hit) || windowAtPointer(ctx.windowsFrontToBack(), hit) != second {
		return fmt.Errorf("%s %gx: modal hit targets differ from rendered layer", theme, scale)
	}
	// Actually click the drawn target, checking input and drawing share geometry.
	calls := 0
	selected.Handler.Handle = func(UIEvent) { calls++ }
	frame.PointerX, frame.PointerY = int(hit.X), int(hit.Y)
	frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButton0}
	if err = step(); err != nil {
		return err
	}
	frame.MouseButtons = nil
	if err = step(); err != nil {
		return err
	}
	if calls != 1 {
		return fmt.Errorf("%s %gx: rendered modal target did not activate once", theme, scale)
	}
	return nil
}
