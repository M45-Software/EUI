package eui

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestContextMenuStartsAtAnchorAndMatchesHitTargets(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldHeight, oldWidth, oldScale := ctx.screenHeight, ctx.screenWidth, ctx.uiScale
	t.Cleanup(func() {
		ctx.screenHeight, ctx.screenWidth, ctx.uiScale = oldHeight, oldWidth, oldScale
		ctx.CloseContextMenus()
	})
	ctx.screenHeight, ctx.screenWidth = 1200, 1200
	for _, scale := range []float32{1, 1.3, 2} {
		ctx.uiScale = scale
		ctx.CloseContextMenus()
		selected := -1
		anchor := point{X: 100, Y: 200} // The bottom-left corner of a source button.
		menu := ctx.ShowContextMenu([]string{"Flute", "Harp", "Bass"}, anchor.X, anchor.Y, func(index int) {
			selected = index
		})
		bounds, visible := ctx.dropdownOpenRect(menu, anchor)
		if bounds.X0 != anchor.X || bounds.Y0 != anchor.Y || visible != 3 {
			t.Fatalf("%.1fx context menu left a gap at its anchor: %+v", scale, bounds)
		}
		rowHeight := ctx.dropdownOptionHeight(menu)
		pointer := point{X: bounds.X0 + 1, Y: bounds.Y0 + rowHeight*1.5}
		ctx.handleContextMenus(pointer, false)
		if menu.HoverIndex != 1 {
			t.Fatalf("%.1fx menu hover does not match the second visible row", scale)
		}
		ctx.handleContextMenus(pointer, true)
		if selected != 1 || ctx.ContextMenusOpen() {
			t.Fatalf("%.1fx menu click did not select and close the second row", scale)
		}
	}
}

func TestHoverContextMenuDismissesOutsideSourceAndMenu(t *testing.T) {
	ctx := newTestContext(t)

	ctx.CloseContextMenus()
	t.Cleanup(ctx.CloseContextMenus)
	menu := &itemData{ctx: ctx, Options: []string{"hello"}, Open: true, Size: point{X: 100, Y: 20}, DrawRect: rect{X0: 50, Y0: 30}}
	menu.DismissOnPointerLeave(Rect{X0: 20, Y0: 20, X1: 60, Y1: 40})
	regular := &itemData{ctx: ctx, Options: []string{"Copy"}, Open: true}
	ctx.contextMenus = append(ctx.contextMenus, menu, regular)
	ctx.dismissContextMenusOutside(point{X: 25, Y: 25})
	if !menu.Open || len(ctx.contextMenus) != 2 {
		t.Fatal("hover menu closed over its source")
	}
	bounds, _ := ctx.dropdownOpenRect(menu, point{X: 50, Y: 30})
	ctx.dismissContextMenusOutside(point{X: bounds.X1 - 1, Y: bounds.Y0 + 1})
	if !menu.Open || len(ctx.contextMenus) != 2 {
		t.Fatal("hover menu closed while choosing a suggestion")
	}
	ctx.dismissContextMenusOutside(point{X: bounds.X1 + 20, Y: bounds.Y1 + 20})
	if menu.Open || len(ctx.contextMenus) != 1 || ctx.contextMenus[0] != regular || !regular.Open {
		t.Fatal("leaving the hover menu did not dismiss only that menu")
	}
}

func TestContextMenuSelectionCanOpenFollowUpMenu(t *testing.T) {
	ctx := newTestContext(t)

	ctx.CloseContextMenus()
	t.Cleanup(ctx.CloseContextMenus)

	first := &itemData{ctx: ctx, contextMenu: true, Options: []string{"Next"}, Open: true, Size: point{X: 100, Y: 20}}
	first.DrawRect.X0 = 20
	first.DrawRect.Y0 = 20
	first.OnSelect = func(int) {
		ctx.contextMenus = append(ctx.contextMenus, &itemData{ctx: ctx, Options: []string{"Done"}, Open: true, Size: point{X: 100, Y: 20}})
	}
	ctx.contextMenus = append(ctx.contextMenus, first)
	r, _ := ctx.dropdownOpenRect(first, point{X: first.DrawRect.X0, Y: first.DrawRect.Y0})
	ctx.handleContextMenus(point{X: r.X0 + 1, Y: r.Y0 + 1}, true)

	if len(ctx.contextMenus) != 1 || ctx.contextMenus[0] == first || !ctx.contextMenus[0].Open {
		t.Fatalf("follow-up context menu was not kept open: %#v", ctx.contextMenus)
	}
}
