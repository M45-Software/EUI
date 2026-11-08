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

func TestRenderRuntimeUpdates(t *testing.T) {
	ctx := newTestContext(t)

	capture := os.Getenv("EUI_RUNTIME_CAPTURE")
	if capture == "" {
		t.Skip("set EUI_RUNTIME_CAPTURE to a PNG path; run alone")
	}
	win := ctx.keyboardFixture(t)
	win.Position = Point{X: 15, Y: 15}
	win.Size = Point{X: 550, Y: 440}
	win.Title = "Runtime state updates"
	label := ctx.NewWrappedLabel("Initial label.", 420)
	check, _ := ctx.NewCheckbox()
	check.Text = "Updated without emitting user events"
	check.Size = Point{X: 440, Y: 30}
	progress, _ := ctx.NewProgressBar()
	progress.Size = Point{X: 440, Y: 18}
	dd, _ := ctx.NewDropdown()
	dd.Size = Point{X: 440, Y: 28}
	dd.SetOptions([]string{"Initial option"})
	button := ctx.NewActionButton("A disabled subtree", nil)
	button.Size = Point{X: 300, Y: 30}
	disabled := ctx.NewColumn(button)
	input, _ := ctx.NewInput()
	input.Text = "Draft"
	input.Size = Point{X: 420, Y: 32}
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 440, Y: 100}
	firstPage := ctx.NewColumn(ctx.NewActionButton("First page action", nil))
	firstPage.Name = "First"
	secondPage := ctx.NewColumn(ctx.NewLabel("Second page content"))
	secondPage.Name = "Second"
	tabs.Tabs = []*ItemData{firstPage, secondPage}
	root := ctx.NewColumn(label, check, progress, dd, disabled, input, tabs)
	win.AddItem(root)
	win.Refresh()
	g := &runtimeRenderGame{ctx: ctx, win: win, label: label, check: check, progress: progress, dropdown: dd, disabled: disabled, input: input, capture: capture, tabs: tabs}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type runtimeRenderGame struct {
	ctx *Context

	win                                                     *windowData
	label, check, progress, dropdown, disabled, input, tabs *ItemData
	capture                                                 string
	done                                                    bool
	err                                                     error
}

func (*runtimeRenderGame) Layout(int, int) (int, int) { return 600, 500 }
func (g *runtimeRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *runtimeRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *runtimeRenderGame) render(screen *ebiten.Image) error {
	ctx := g.ctx

	draw := func() { screen.Fill(color.RGBA{R: 15, G: 20, B: 30, A: 255}); ctx.Draw(screen) }
	before := make([]byte, 600*500*4)
	after := make([]byte, len(before))
	for _, palette := range []string{"AccentDark", "Paper"} {
		if err := ctx.LoadTheme(palette); err != nil {
			return err
		}
		g.label.SetWrappedText("Initial label.")
		draw()
		update := func(name string, change func()) error {
			screen.ReadPixels(before)
			count := g.win.repaintStats.Count
			change()
			if !g.win.Dirty {
				return fmt.Errorf("%s: %s did not invalidate window", palette, name)
			}
			draw()
			screen.ReadPixels(after)
			if bytes.Equal(before, after) {
				return fmt.Errorf("%s: %s did not update rendered pixels", palette, name)
			}
			if g.win.repaintStats.Count != count+1 {
				return fmt.Errorf("%s: %s repainted an unexpected number of times", palette, name)
			}
			return nil
		}
		if err := update("checkbox", func() { g.check.SetChecked(!g.check.Checked) }); err != nil {
			return err
		}
		if err := update("progress", func() {
			if g.progress.Value == .75 {
				g.progress.SetValue(.25)
			} else {
				g.progress.SetValue(.75)
			}
		}); err != nil {
			return err
		}
		if err := update("options", func() { g.dropdown.SetOptions([]string{palette + " refreshed option"}) }); err != nil {
			return err
		}
		oldY := g.input.DrawRect.Y0
		if err := update("wrapped text", func() {
			g.label.SetWrappedText("Long text wraps and moves the controls below it without a manual window refresh. " + palette)
		}); err != nil {
			return err
		}
		if g.input.DrawRect.Y0 == oldY {
			return fmt.Errorf("%s: wrapped text did not move sibling controls", palette)
		}
		if err := update("disabled subtree", func() { g.disabled.SetDisabled(!g.disabled.Disabled) }); err != nil {
			return err
		}
		// Enabling/disabling must visibly change the child, not just its parent frame.
		g.disabled.SetDisabled(false)
		draw()
		child := g.disabled.Contents[0]
		r := child.DrawRect.getRectangle()
		regionBefore := screen.RecyclableSubImage(r)
		pixels := make([]byte, r.Dx()*r.Dy()*4)
		regionBefore.ReadPixels(pixels)
		regionBefore.Recycle()
		g.disabled.SetDisabled(true)
		draw()
		regionAfter := screen.RecyclableSubImage(r)
		changed := make([]byte, len(pixels))
		regionAfter.ReadPixels(changed)
		regionAfter.Recycle()
		if bytes.Equal(pixels, changed) {
			return fmt.Errorf("%s: child of disabled container did not change appearance", palette)
		}
		g.disabled.SetDisabled(false)
		draw()
		if err := update("hidden subtree", func() { g.disabled.SetVisible(false) }); err != nil {
			return err
		}
		if g.disabled.Contents[0].DrawRect != (Rect{}) {
			return fmt.Errorf("hidden child retained hit bounds")
		}
		if err := update("shown subtree", func() { g.disabled.SetVisible(true) }); err != nil {
			return err
		}
		// A caller-provided item cache must be retired when a setter dirties it.
		oldCache := ctx.uiRenderTargets.Acquire(int(g.input.GetSize().X), int(g.input.GetSize().Y), false)
		oldCache.Fill(color.RGBA{R: 255, A: 255})
		g.input.Render = oldCache
		g.input.Dirty = false
		g.win.Dirty = true
		draw()
		if err := update("cached input", func() { g.input.UpdateText("New value replaces the stale item cache: " + palette) }); err != nil {
			return err
		}
		if g.input.Render != nil || g.input.Dirty {
			return fmt.Errorf("updated input retained a stale render cache or dirty flag")
		}
		g.tabs.SetActiveTab(0)
		draw()
		g.tabs.Tabs[0].SetVisible(false)
		g.tabs.Tabs[1].SetDisabled(true)
		draw()
		if g.tabs.ActiveTab != -1 || g.tabs.Tabs[0].Contents[0].DrawRect != (Rect{}) {
			return fmt.Errorf("unavailable pages retained active content")
		}
		g.tabs.Tabs[0].SetVisible(true)
		g.tabs.Tabs[1].SetDisabled(false)
		g.tabs.SetActiveTab(1)
		draw()
		if g.tabs.ActiveTab != 1 || g.tabs.Tabs[1].Contents[0].DrawRect.Y1 <= g.tabs.Tabs[1].Contents[0].DrawRect.Y0 {
			return fmt.Errorf("restored active page failed to render")
		}

	}
	if err := ctx.LoadTheme("AccentDark"); err != nil {
		return err
	}
	g.label.SetWrappedText("Runtime setters repaint and reflow this window automatically.")
	g.check.SetChecked(true)
	g.dropdown.SetOptions([]string{"Updated options are selected silently", "Another option"})
	g.dropdown.SetSelected(0)
	g.progress.SetValue(.75)
	g.disabled.SetDisabled(true)
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
