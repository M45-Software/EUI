package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestInvisibleSubtreeCollapsesAndRestoresWithoutStaleInput(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	oldScale := ctx.UIScale()
	oldW, oldH := ctx.ScreenSize()
	t.Cleanup(func() { ctx.SetUIScale(oldScale); ctx.SetScreenSize(oldW, oldH) })
	ctx.SetScreenSize(1920, 1080)
	for _, scale := range []float32{1, 2} {
		ctx.SetUIScale(scale)
		win := ctx.NewWindow()
		win.AutoSize = true
		before := ctx.NewLabel("Before")
		before.Size = Point{X: 90, Y: 24}
		before.Position = Point{}
		input, _ := ctx.NewInput()
		input.Size = Point{X: 220, Y: 30}
		dropdown, _ := ctx.NewDropdown()
		dropdown.Options = []string{"One", "Two"}
		dropdown.Size = Point{X: 220, Y: 24}
		hidden := ctx.NewColumn(input, dropdown)
		hidden.Position = Point{X: 17, Y: 19}
		after := ctx.NewLabel("After")
		after.Size = Point{X: 90, Y: 24}
		after.Position = Point{}
		root := ctx.NewColumn(before, hidden, after)
		win.AddItem(root)
		win.AddWindow(false)
		win.MarkOpen()
		canvas := ebiten.NewImage(1920, 1080)
		ctx.Draw(canvas)
		expanded := win.GetSize()
		oldHit := Point{X: input.DrawRect.X0 + 5, Y: input.DrawRect.Y0 + 5}
		ctx.Focus(input)
		dropdown.Open = true
		hidden.Invisible = true
		win.Refresh()
		ctx.Draw(canvas)
		collapsed := win.GetSize()
		if root.contentBounds().X != before.GetSize().X || collapsed.Y >= expanded.Y {
			t.Fatalf("hidden content or position gaps still reserve window space: expanded=%v collapsed=%v root=%v", expanded, collapsed, root.contentBounds())
		}
		if after.DrawRect.Y0 != before.DrawRect.Y1 {
			t.Fatal("hidden flow left a gap between visible siblings")
		}
		if hidden.DrawRect != (rect{}) || input.DrawRect != (rect{}) || dropdown.DrawRect != (rect{}) {
			t.Fatal("hidden subtree retained draw rectangles")
		}
		if ctx.focusedItem == input || input.Focused || dropdown.Open {
			t.Fatal("hidden subtree retained focus or a popup")
		}
		if input.clickItem(oldHit, true) || hidden.clickFlows(oldHit, true) {
			t.Fatal("hidden subtree intercepted a click")
		}
		var inputs []*itemData
		collectFocusControls(root.Contents, &inputs)
		if len(inputs) != 0 {
			t.Fatal("keyboard navigation included an input in a hidden flow")
		}
		ctx.Focus(input)
		if ctx.focusedItem == input {
			t.Fatal("programmatic focus entered a hidden flow")
		}
		dropdown.Open = true // Also reject overlays if hidden before a refresh.
		var overlays []openDropdown
		collectItemDropdowns(root.Contents, &overlays)
		if len(overlays) != 0 || ctx.dropdownOpenContains(root.Contents, oldHit) {
			t.Fatal("hidden dropdown retained an overlay")
		}
		hidden.Invisible = false
		dropdown.Open = false
		win.Refresh()
		ctx.Draw(canvas)
		if win.GetSize() != expanded || input.DrawRect.X1 <= input.DrawRect.X0 {
			t.Fatal("showing the subtree did not restore its original size and drawing")
		}
		win.RemoveWindow()
		canvas.Deallocate()
	}
}

func TestInvisibleItemsDoNotContributeToRowsOrWindowBounds(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	first := ctx.NewLabel("First")
	first.Size = Point{X: 70, Y: 24}
	first.Position = Point{}
	hidden := ctx.NewLabel("Hidden")
	hidden.Size, hidden.Position = Point{X: 500, Y: 200}, Point{X: 100, Y: 100}
	hidden.Invisible = true
	row := ctx.NewRow(first, hidden)
	if row.contentBounds() != first.GetSize() {
		t.Fatalf("hidden row item contributed size or spacing: bounds=%v first=%v pos=%v", row.contentBounds(), first.GetSize(), first.Position)
	}
	win := ctx.NewWindow()
	win.AddItem(first)
	win.AddItem(hidden)
	if win.contentBounds() != first.GetSize() {
		t.Fatal("hidden root item contributed to window bounds")
	}
}
