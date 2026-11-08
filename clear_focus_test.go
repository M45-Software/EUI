package eui

import (
	"golang.org/x/image/font/gofont/goregular"
	"testing"
)

// TestClearFocus verifies ClearFocus removes focus only when the provided
// item currently holds focus.
func TestClearFocus(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldScale, oldFocus, oldSelection := ctx.uiScale, ctx.focusedItem, ctx.selectedTextItem
	t.Cleanup(func() { ctx.uiScale, ctx.focusedItem, ctx.selectedTextItem = oldScale, oldFocus, oldSelection })
	ctx.uiScale = 1
	item, _ := ctx.NewInput()
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 10, Y1: 10}
	item.clickItem(point{X: 1, Y: 1}, true)
	if ctx.focusedItem != item {
		t.Fatalf("expected item to be focused")
	}

	// Clearing focus with a different item should do nothing.
	other, _ := ctx.NewInput()
	ctx.ClearFocus(other)
	if ctx.focusedItem != item {
		t.Fatalf("focus should remain on original item")
	}

	// Clearing focus with the focused item should remove focus.
	ctx.ClearFocus(item)
	if ctx.focusedItem != nil {
		t.Fatalf("focus was not cleared")
	}
}
