package eui

import (
	"testing"
	"time"
)

func TestClickFeedbackDefersHiddenPagesUntilSelected(t *testing.T) {
	ctx := newTestContext(t)

	now := time.Now()
	win := ctx.NewWindow()
	visible := &itemData{ctx: ctx, ItemType: ITEM_BUTTON, ParentWindow: win, Clicked: now.Add(-2 * clickFlash)}
	hidden := &itemData{ctx: ctx, ItemType: ITEM_BUTTON, ParentWindow: win, Clicked: now.Add(-2 * clickFlash)}
	flow := &itemData{ctx: ctx, ItemType: ITEM_FLOW, Tabs: []*itemData{
		{ctx: ctx, Contents: []*itemData{visible}},
		{ctx: ctx, Contents: []*itemData{hidden}},
	}}
	clearExpiredClicksAt([]*itemData{flow}, now)
	if !visible.Clicked.IsZero() || hidden.Clicked.IsZero() {
		t.Fatal("click cleanup must visit only the visible page")
	}
	win.Dirty = false
	flow.ActiveTab = 1
	clearExpiredClicksAt([]*itemData{flow}, now)
	if !hidden.Clicked.IsZero() || !win.Dirty {
		t.Fatal("selecting a page must clear expired feedback before painting it")
	}
}
