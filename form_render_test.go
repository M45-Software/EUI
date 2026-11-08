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

func TestRenderFormValidation(t *testing.T) {
	path := os.Getenv("EUI_FORM_CAPTURE")
	if path == "" {
		t.Skip("set EUI_FORM_CAPTURE to a PNG path; run alone")
	}
	g := &formRenderGame{path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type formRenderGame struct {
	path string
	done bool
	err  error
}

func (*formRenderGame) Layout(int, int) (int, int) { return 900, 800 }
func (g *formRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *formRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.check(screen)
	}
}
func (g *formRenderGame) check(screen *ebiten.Image) error {
	for _, theme := range []string{"AccentDark", "Paper"} {
		for _, scale := range []float32{1, 1.5, 2} {
			if err := g.checkCase(screen, theme, scale); err != nil {
				return fmt.Errorf("%s/%gx: %w", theme, scale, err)
			}
		}
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, screen)
}
func (g *formRenderGame) checkCase(screen *ebiten.Image, theme string, scale float32) error {
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
	ctx.SetScreenSize(900, 800)
	frame := InputFrame{Time: time.Unix(100, 0), PointerX: -100, PointerY: -100}
	ctx.SetInputSource(func() InputFrame { return frame })
	name := ctx.NewFormField("name", "Scene name", "Garden", Required)
	count := ctx.NewFormField("count", "Particles", "12", Required, IntegerRange(1, 200))
	form := ctx.NewForm(name, count)
	form.View().Size.X = 240
	form.View().MaxSize.X = 240
	win := ctx.NewWindow()
	win.Title = "Validate your scene"
	win.AutoSize = true
	win.Position = Point{X: 20, Y: 20}
	win.Padding = 12
	apply := ctx.NewActionButton("Apply", func() { _ = form.Apply() })
	cancel := ctx.NewActionButton("Cancel", form.Cancel)
	apply.Size.X, cancel.Size.X = 100, 100
	root := ctx.NewColumn(form.View(), ctx.NewRow(cancel, apply))
	root.Gap = 12
	win.AddItem(root)
	win.MarkOpenModal()
	step := func() error {
		frame.Time = frame.Time.Add(time.Second / 60)
		if err := ctx.Update(); err != nil {
			return err
		}
		screen.Clear()
		ctx.Draw(screen)
		return nil
	}
	if err = step(); err != nil {
		return err
	}
	name.Input().UpdateText("")
	count.Input().UpdateText("201")
	if form.Apply() == nil {
		return fmt.Errorf("invalid draft accepted")
	}
	if err = step(); err != nil {
		return err
	}
	for _, field := range form.Fields() {
		r := field.ErrorLabel().DrawRect
		if field.ErrorLabel().Invisible || r.X1 <= r.X0 || r.Y1 <= r.Y0 || r.X0 < win.getPosition().X || r.X1 > win.getPosition().X+win.GetSize().X+1 {
			return fmt.Errorf("error label is hidden or outside window: %+v", r)
		}
		base := screen.At(int(r.X0), int(r.Y0))
		count := 0
		for y := int(r.Y0); y < int(r.Y1); y++ {
			for x := int(r.X0); x < int(r.X1); x++ {
				if screen.At(x, y) != base {
					count++
				}
			}
		}
		if count < 10 {
			return fmt.Errorf("error label has no visible glyphs: %s", field.Name())
		}
	}
	if name.ErrorLabel().DrawRect.Y1 > count.Input().DrawRect.Y0 || count.ErrorLabel().DrawRect.Y1 > apply.DrawRect.Y0 {
		return fmt.Errorf("inline errors overlap next control")
	}
	before := make([]byte, 900*800*4)
	screen.ReadPixels(before)
	x, y := int((cancel.DrawRect.X0+cancel.DrawRect.X1)/2), int((cancel.DrawRect.Y0+cancel.DrawRect.Y1)/2)
	frame.PointerX, frame.PointerY = x, y
	frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButtonLeft}
	if err = step(); err != nil {
		return err
	}
	frame.MouseButtons = nil
	if err = step(); err != nil {
		return err
	}
	if form.Dirty() || name.Error() != "" || count.Error() != "" || !name.ErrorLabel().Invisible || !count.ErrorLabel().Invisible {
		return fmt.Errorf("Cancel hit target did not reset form")
	}
	after := make([]byte, len(before))
	screen.ReadPixels(after)
	if bytes.Equal(before, after) {
		return fmt.Errorf("Cancel retained invalid cached pixels")
	}
	// Leave the invalid state visible in the capture for visual inspection.
	name.Input().UpdateText("")
	count.Input().UpdateText("201")
	_ = form.Apply()
	return step()
}
