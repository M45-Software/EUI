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

func TestRenderInteractionReplay(t *testing.T) {
	ctx := newTestContext(t)

	capture := os.Getenv("EUI_INTERACTION_CAPTURE")
	if capture == "" {
		t.Skip("set EUI_INTERACTION_CAPTURE to a PNG path; run alone")
	}
	win := ctx.keyboardFixture(t)
	win.Position = Point{X: 20, Y: 20}
	win.Size = Point{X: 500, Y: 350}
	button := ctx.NewActionButton("Activate", nil)
	button.Position = Point{}
	button.Size = Point{X: 160, Y: 32}
	editor, _ := ctx.NewTextArea()
	editor.Position = Point{}
	editor.Size = Point{X: 350, Y: 120}
	root := ctx.NewColumn(button, editor)
	root.Gap = 12
	win.AddItem(root)
	g := &interactionRenderGame{ctx: ctx, button: button, editor: editor, capture: capture, frame: InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}}
	previous := ctx.SetInputSource(func() InputFrame { return g.frame })
	t.Cleanup(func() { ctx.SetInputSource(previous) })
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type interactionRenderGame struct {
	ctx *Context

	button, editor *ItemData
	frame          InputFrame
	capture        string
	done           bool
	err            error
}

func (*interactionRenderGame) Layout(int, int) (int, int) { return 640, 480 }
func (g *interactionRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *interactionRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *interactionRenderGame) render(screen *ebiten.Image) error {
	ctx := g.ctx

	step := func(advance time.Duration) error {
		g.frame.Time = g.frame.Time.Add(advance)
		if err := ctx.Update(); err != nil {
			return err
		}
		screen.Clear()
		ctx.Draw(screen)
		g.frame.Text = ""
		return nil
	}
	before, after := make([]byte, 640*480*4), make([]byte, 640*480*4)
	differs := func(r Rect) bool {
		x0, y0, x1, y1 := max(0, int(r.X0)), max(0, int(r.Y0)), min(640, int(r.X1)), min(480, int(r.Y1))
		for y := y0; y < y1; y++ {
			start, end := (y*640+x0)*4, (y*640+x1)*4
			if !bytes.Equal(before[start:end], after[start:end]) {
				return true
			}
		}
		return false
	}
	if err := step(time.Second / 60); err != nil {
		return err
	}
	p := itemCenter(g.button)
	g.frame.PointerX, g.frame.PointerY = int(p.X), int(p.Y)
	g.frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButton0}
	calls := 0
	g.button.Handler.Handle = func(UIEvent) { calls++ }
	if err := step(time.Second / 60); err != nil {
		return err
	}
	g.frame.MouseButtons = nil
	g.frame.PointerX, g.frame.PointerY = -100, -100
	if err := step(time.Second / 60); err != nil {
		return err
	}
	screen.ReadPixels(before)
	if err := step(clickFlash + time.Millisecond); err != nil {
		return err
	}
	screen.ReadPixels(after)
	if calls != 1 || !g.button.Clicked.IsZero() || !differs(g.button.DrawRect) {
		return fmt.Errorf("replayed click flash did not expire visually on the injected clock")
	}
	g.frame.Keys = []ebiten.Key{ebiten.KeyTab}
	if err := step(time.Second / 60); err != nil {
		return err
	}
	g.frame.Keys = nil
	g.frame.Text = "Editable café"
	if err := step(time.Second / 60); err != nil {
		return err
	}
	if ctx.FocusedItem() != g.editor || g.editor.Text != "Editable café" || !g.editor.textEdit.caretOn {
		return fmt.Errorf("replayed focus/text sequence failed")
	}
	screen.ReadPixels(before)
	if err := step(600 * time.Millisecond); err != nil {
		return err
	}
	screen.ReadPixels(after)
	if g.editor.textEdit.caretOn || !differs(g.editor.DrawRect) {
		return fmt.Errorf("caret pixels did not follow injected clock")
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
