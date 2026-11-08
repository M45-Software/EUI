package eui

// Focus geometry is computed from layout, not clipped DrawRect values. An
// offscreen control may never have been drawn, including before the first frame.
func (ctx *Context) revealFocusedControl(target *itemData) {
	win := target.ParentWindow
	if win == nil || !win.Open {
		return
	}
	win.resizeFlows()
	pad := (win.Padding + win.BorderPad) * win.scale()
	viewport := point{X: max(1, win.GetSize().X-2*pad), Y: max(1, win.GetSize().Y-win.GetTitleSize()-2*pad)}
	revealWindow := func(bounds rect) {
		if win.NoScroll {
			return
		}
		old := win.Scroll
		content := win.contentBounds()
		win.Scroll.X = focusScrollAxis(old.X, bounds.X0, bounds.X1, 0, viewport.X, content.X)
		win.Scroll.Y = focusScrollAxis(old.Y, bounds.Y0, bounds.Y1, 0, viewport.Y, content.Y)
		if win.Scroll != old {
			win.markDirty()
		}
	}
	if target.ItemType != ITEM_FLOW && target.PinTo != PIN_TOP_LEFT {
		// Pinned leaves draw relative to the window, outside flow scrolling.
		position := pointSub(target.getPosition(win), win.Scroll)
		size := target.GetSize()
		revealWindow(rect{X0: position.X, Y0: position.Y, X1: position.X + size.X, Y1: position.Y + size.Y})
		return
	}
	for _, root := range win.Contents {
		bounds, found := ctx.focusRectIn(root, target)
		if !found {
			continue
		}
		revealWindow(rectAdd(bounds, pointSub(root.getPosition(win), win.Scroll)))
		return
	}
}

func focusScrollAxis(scroll, start, end, low, high, content float32) float32 {
	if start < low {
		scroll += start - low
	} else if end > high {
		scroll += min(end-high, max(0, start-low))
	}
	return max(0, min(scroll, max(0, content-(high-low))))
}

func (ctx *Context) focusRectIn(it, target *itemData) (rect, bool) {
	if it == nil || it.Invisible || it.Disabled {
		return rect{}, false
	}
	size := it.GetSize()
	if it == target {
		if len(it.Tabs) > 0 && it.ActiveTab >= 0 && it.ActiveTab < len(it.Tabs) {
			layout, _ := ctx.layoutTabs(it, it.themeStyle())
			return layout[it.ActiveTab], true
		}
		return rect{X1: size.X, Y1: size.Y}, true
	}
	for child, position := range it.layoutChildren(size) {
		bounds, found := ctx.focusRectIn(child, target)
		if found {
			bounds = rectAdd(bounds, pointSub(position, it.Scroll))
			if it.Scrollable {
				old := it.Scroll
				content := it.contentBounds()
				origin, area := it.layoutBody(size)
				it.Scroll.X = focusScrollAxis(old.X, bounds.X0, bounds.X1, origin.X, origin.X+area.X, max(0, content.X-size.X)+area.X)
				it.Scroll.Y = focusScrollAxis(old.Y, bounds.Y0, bounds.Y1, origin.Y, origin.Y+area.Y, max(0, content.Y-size.Y)+area.Y)
				bounds = rectAdd(bounds, pointSub(old, it.Scroll))
				if it.Scroll != old {
					invalidateFocus(it)
				}
			}
			return bounds, true
		}
	}

	return rect{}, false
}
