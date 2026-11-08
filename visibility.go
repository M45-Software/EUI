package eui

import "time"

// Visibility is inherited by descendants, including controls in a hidden flow.
func (item *itemData) isInvisible() bool {
	for it := item; it != nil; it = it.Parent {
		if it.Invisible {
			return true
		}
	}
	return false
}

// A hidden subtree must not retain hit rectangles, focus, or popup overlays
// from the previous frame. Requested sizes remain intact for showing it again.
func (item *itemData) clearHiddenState() {
	ctx := item.ui()

	if item == nil {
		return
	}
	item.CancelComposition()
	item.DrawRect = rect{}
	item.Hovered, item.Focused = false, false
	item.selecting = false
	item.SelectStart, item.SelectEnd = item.CursorPos, item.CursorPos
	item.dragStartInit = false
	item.Clicked = time.Time{}
	if item.textEdit != nil {
		item.textEdit.scrollDrag = PART_NONE
	}
	if ctx.keyboardSlider == item {
		ctx.keyboardSlider = nil
	}
	if ctx.focusedItem == item {
		ctx.focusedItem = nil
	}
	if ctx.activeItem == item {
		ctx.activeItem = nil
	}
	if ctx.hoveredItem == item {
		ctx.hoveredItem = nil
	}
	if ctx.selectedTextItem == item {
		ctx.selectedTextItem = nil
	}
	if ctx.dragFlow == item {
		ctx.dragFlow, ctx.dragWin, ctx.dragPart = nil, nil, PART_NONE
	}
	for _, child := range item.Contents {
		child.clearHiddenState()
	}
	for _, tab := range item.Tabs {
		tab.clearHiddenState()
	}
	if item.ItemType == ITEM_DROPDOWN {
		item.dismissRuntimeMenu()
	}
}

// Disabled state, like visibility, is inherited without overwriting descendants'
// own flags; re-enabling a parent preserves individually disabled controls.
func (item *itemData) isDisabled() bool {
	for it := item; it != nil; it = it.Parent {
		if it.Disabled {
			return true
		}
	}
	return false
}
