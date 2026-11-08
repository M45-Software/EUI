package eui

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderGamepadNavigation(t *testing.T) {
	path := os.Getenv("EUI_GAMEPAD_CAPTURE")
	if path == "" {
		t.Skip("set EUI_GAMEPAD_CAPTURE to a PNG path; run alone")
	}
	g := &gamepadRenderGame{path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type gamepadRenderGame struct {
	path string
	done bool
	err  error
}

func (*gamepadRenderGame) Layout(int, int) (int, int) { return 900, 700 }
func (g *gamepadRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *gamepadRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.check(screen)
	}
}
func (g *gamepadRenderGame) check(screen *ebiten.Image) error {
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
func (g *gamepadRenderGame) checkCase(screen *ebiten.Image, theme string, scale float32) error {
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
	ctx.SetScreenSize(900, 700)
	ctx.SetGamepadNavigation(true)
	frame := InputFrame{Time: time.Unix(100, 0), PointerX: -100, PointerY: -100, Gamepads: []GamepadInput{{ID: 0}}}
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
	win := ctx.NewWindow()
	win.Title = "Controller navigation"
	win.Position = Point{X: 20, Y: 20}
	win.AutoSize = true
	win.Padding = 12
	a := ctx.NewActionButton("Start", nil)
	b := ctx.NewActionButton("Settings", nil)
	c := ctx.NewActionButton("Load", nil)
	d := ctx.NewActionButton("Quit", nil)
	buttons := ctx.NewColumn(ctx.NewRow(a, b), ctx.NewRow(c, d))
	buttons.Gap = 10
	for _, button := range []*ItemData{a, b, c, d} {
		button.Size = Point{X: 140, Y: 30}
	}
	hint := ctx.NewWrappedLabel("", 290)
	root := ctx.NewColumn(buttons, hint)
	root.Gap = 20
	win.AddItem(root)
	win.MarkOpen()
	ctx.Focus(a)
	if err = step(); err != nil {
		return err
	}
	frame.Gamepads[0].Buttons = []ebiten.StandardGamepadButton{padRight}
	if err = step(); err != nil {
		return err
	}
	frame.Gamepads[0].Buttons = nil
	if err = step(); err != nil {
		return err
	}
	if ctx.FocusedItem() != b {
		return fmt.Errorf("right did not select the rendered Settings button")
	}
	before := make([]byte, 900*700*4)
	screen.ReadPixels(before)
	ctx.Focus(b)
	screen.Clear()
	ctx.Draw(screen)
	after := make([]byte, len(before))
	screen.ReadPixels(after)
	if !bytes.Equal(before, after) {
		return fmt.Errorf("gamepad focus pixels differ from keyboard focus")
	}
	frame.Gamepads[0].Buttons = []ebiten.StandardGamepadButton{padDown}
	if err = step(); err != nil {
		return err
	}
	if ctx.FocusedItem() != d {
		return fmt.Errorf("down did not select the rendered Quit button")
	}
	var lines []string
	for _, prompt := range ctx.NavigationPrompts() {
		lines = append(lines, prompt.Label+": "+prompt.Action)
	}
	hint.SetWrappedText(strings.Join(lines, "\n"))
	frame.Gamepads[0].Buttons = nil
	return step()
}
