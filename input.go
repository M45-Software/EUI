package eui

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Update processes input and updates window state.
// Programs embedding the UI can call this from their Ebiten Update handler.
func (ctx *Context) Update() (err error) {
	if ctx == nil || ctx.disposed {
		return ErrContextDisposed
	}
	ctx.mustAlive()
	for _, win := range ctx.windows {
		if win != nil {
			ctx.requireWindow(win)
			ctx.requireItem(win.DefaultButton)
		}
	}

	if ctx.updateDepth > 0 || ctx.drawDepth > 0 {
		return ErrContextReentry
	}
	ctx.updateDepth++
	defer func() {
		ctx.updateDepth--
		if ctx.disposePending {
			ctx.disposeNow()
			if err == nil {
				err = ErrContextDisposed
			}
		}
	}()

	if err := ctx.sampleInput(); err != nil {
		return err
	}
	ctx.updateNow = ctx.sampledInput.now
	for _, it := range []*itemData{ctx.focusedItem, ctx.activeItem, ctx.hoveredItem, ctx.dragFlow} {
		if it != nil && (it.isInvisible() || it.ParentWindow != nil && !it.ParentWindow.Open) {
			it.clearHiddenState()
		}
	}
	ctx.checkThemeStyleMods()
	ctx.pointerPressHandled = false

	shiftPressed := ctx.keyPressed(ebiten.KeyShift)
	ctx.ShiftPressed = shiftPressed
	ctrlPressed := ctx.keyPressed(ebiten.KeyControl)
	ctx.CtrlPressed = ctrlPressed
	mods := ctx.inputModifiers()
	if ctx.activeSearch != nil && !ctx.activeSearch.Open {
		ctx.activeSearch = nil
	}

	prevHovered := ctx.hoveredItem
	ctx.hoveredItem = nil

	mx, my := ctx.sampledInput.x, ctx.sampledInput.y
	mpos := point{X: float32(mx), Y: float32(my)}
	if mpos != ctx.mposOld {
		for _, menu := range ctx.contextMenus {
			menu.keyboardMenu = false
		}
	}
	ctx.dismissContextMenusOutside(mpos)
	if prevHovered != nil && prevHovered.ItemType == ITEM_DROPDOWN {
		r, _ := ctx.dropdownOpenRect(prevHovered, point{X: prevHovered.DrawRect.X0, Y: prevHovered.DrawRect.Y0})
		if !prevHovered.keyboardMenu && (!prevHovered.Open || !r.containsPoint(mpos)) {
			prevHovered.clearDropdownHover()
		}
	}
	orderedWindows := ctx.windowsFrontToBack()
	ctx.updateCloseHover(orderedWindows, mpos)

	click := ctx.pointerJustPressed()
	midClick := ctx.middleClickMove && ctx.mouseJustPressed(ebiten.MouseButtonMiddle)
	if mx != int(ctx.mposOld.X) || my != int(ctx.mposOld.Y) || click {
		if ctx.focusedItem != nil {
			ctx.focusedItem.keyboardMenu = false
		}
	}
	if click {
		ctx.activeSearch = nil
		scrollbarClick := ctx.selectedTextItem != nil && ctx.selectedTextItem.editScrollbarAt(mpos) != PART_NONE
		if !scrollbarClick && (!shiftPressed || ctx.selectedTextItem == nil || !ctx.selectedTextItem.DrawRect.containsPoint(mpos)) {
			ctx.clearTextSelection()
		}
		if !ctx.dropdownOpenContainsAnywhere(mpos) {
			ctx.closeAllDropdowns()
		}
		// Close context menus when clicking anywhere outside them.
		if !ctx.contextMenuContainsAnywhere(mpos) {
			ctx.CloseContextMenus()
		}
		ctx.ClearFocus(ctx.focusedItem)
	}
	clickTime := ctx.pointerPressDuration()
	clickDrag := clickTime > 1
	midClickTime := 0
	midPressed := false
	if ctx.middleClickMove {
		midClickTime = ctx.mousePressDuration(ebiten.MouseButtonMiddle)
		midPressed = ctx.mousePressed(ebiten.MouseButtonMiddle)
	}
	midClickDrag := midClickTime > 1

	if (!ctx.pointerPressed() || ctx.sampledInput.primaryEnded) && !midPressed {
		if ctx.activeItem != nil {
			ctx.activeItem.emitSliderReleased()
			if ctx.activeItem.textEdit != nil {
				ctx.activeItem.textEdit.scrollDrag = PART_NONE
			}
		}
		if ctx.selectedTextItem != nil && ctx.selectedTextItem.selecting {
			ctx.selectedTextItem.selecting = false
			ctx.selectedTextItem.markDirty()
		}
		ctx.dragPart = PART_NONE
		ctx.dragWin = nil
		ctx.dragFlow = nil
		ctx.activeItem = nil
		ctx.downWin = nil
	}

	if click || midClick {
		ctx.downWin = windowAtPointer(orderedWindows, mpos)
	}
	wx, wy := ctx.pointerWheel()
	wheelDelta := point{X: float32(wx), Y: float32(wy)}

	delta := pointSub(mpos, ctx.mposOld)
	c := ebiten.CursorShapeDefault
	if dividerHandled, dividerCursor := ctx.updateTileDividerInput(mpos, click); dividerCursor != ebiten.CursorShapeDefault {
		c = dividerCursor
		if dividerHandled {
			ctx.pointerPressHandled = true
			click = false
			midClick = false
			ctx.downWin = nil
		}
	}

	var chars []rune
	if !ctx.keyboardInputCaptured && !mods.SuppressText() {
		chars = []rune(ctx.sampledInput.text)
	}

	// First, give active context menus a chance to handle the click/hover.
	if ctx.handleContextMenus(mpos, click) {
		ctx.pointerPressHandled = click
		// If a context menu handled the interaction, avoid passing the click to windows.
		// Still continue with cursor updates and general housekeeping below.
		click = false
	}

	//Check all windows
	for _, win := range orderedWindows {
		if !win.Open {
			continue
		}

		s := win.scale()
		posCh := point{X: delta.X / s, Y: delta.Y / s}

		var part dragType
		handled := false
		if ctx.dragPart != PART_NONE && ctx.dragWin == win {
			part = ctx.dragPart
		} else {
			localPos := point{X: mpos.X / s, Y: mpos.Y / s}
			part = win.getWindowPart(localPos, click)
			if part == PART_NONE && midClick && win.Movable && win.getWinRect().containsPoint(mpos) {
				part = PART_BAR
			}
		}

		if part != PART_NONE {
			// Resize handles own the same grab area for hover and clicks,
			// including the tolerance outside the visible border.
			handled = part >= PART_TOP && part <= PART_TOP_LEFT

			if ctx.dragPart == PART_NONE && c == ebiten.CursorShapeDefault {
				switch part {
				case PART_BAR:
					c = ebiten.CursorShapeMove
				case PART_LEFT, PART_RIGHT:
					c = ebiten.CursorShapeEWResize
				case PART_TOP, PART_BOTTOM:
					c = ebiten.CursorShapeNSResize
				case PART_TOP_LEFT, PART_BOTTOM_RIGHT:
					c = ebiten.CursorShapeNWSEResize
				case PART_TOP_RIGHT, PART_BOTTOM_LEFT:
					c = ebiten.CursorShapeNESWResize
				case PART_SCROLL_V, PART_SCROLL_H, PART_SEARCH, PART_CLOSE:
					c = ebiten.CursorShapePointer
				}
			}

			if click && ctx.dragPart == PART_NONE && ctx.downWin == win {
				ctx.pointerPressHandled = true
				if part == PART_CLOSE {
					win.Close()
					break
				}
				if part == PART_MAXIMIZE {
					if win.OnMaximize != nil {
						win.OnMaximize()
					} else {
						win.Maximize()
					}
					break
				}
				if part == PART_SEARCH {
					if win.searchOpen {
						win.closeSearch()
					} else {
						win.SearchText = ""
						win.OpenSearch()
						if win.OnSearch != nil {
							win.OnSearch("")
						}
						win.markDirty()
					}
					break
				}
				ctx.dragPart = part
				ctx.dragWin = win
			} else if midClick && ctx.dragPart == PART_NONE && part == PART_BAR && ctx.downWin == win {
				ctx.dragPart = part
				ctx.dragWin = win
			} else if (clickDrag || midClickDrag) && ctx.dragPart != PART_NONE && ctx.dragWin == win {
				geometryDragBlocked := win.Docked || (ctx.dragPart == PART_BAR && !win.Movable) ||
					(ctx.dragPart >= PART_TOP && ctx.dragPart <= PART_TOP_LEFT && !win.Resizable)
				if geometryDragBlocked && ctx.dragPart != PART_SCROLL_V && ctx.dragPart != PART_SCROLL_H {
					ctx.dragPart = PART_NONE
					ctx.dragWin = nil
					break
				}
				switch ctx.dragPart {
				case PART_BAR:
					ctx.dragWindowMove(win, posCh)
				case PART_TOP, PART_BOTTOM, PART_LEFT, PART_RIGHT,
					PART_TOP_LEFT, PART_TOP_RIGHT, PART_BOTTOM_LEFT, PART_BOTTOM_RIGHT:
					ctx.dragWindowResize(win, ctx.dragPart, posCh)
				case PART_SCROLL_V:
					if ctx.dragFlow != nil {
						ctx.dragFlowScroll(ctx.dragFlow, mpos, true)
					} else {
						ctx.dragWindowScroll(win, mpos, true)
					}
				case PART_SCROLL_H:
					if ctx.dragFlow != nil {
						ctx.dragFlowScroll(ctx.dragFlow, mpos, false)
					} else {
						ctx.dragWindowScroll(win, mpos, false)
					}
				}
				if ctx.dragPart != PART_BAR && ctx.dragPart != PART_SCROLL_V && ctx.dragPart != PART_SCROLL_H {
					if ctx.windowSnapping {
						ctx.snapResize(win, ctx.dragPart)
					}
				}
				win.clampToScreen()
				if ctx.dragPart == PART_BAR {
					ctx.snapMovedWindow(win)
				}
				break
			}
		} else if win.searchOpen {
			if ctx.dragPart == PART_NONE && c == ebiten.CursorShapeDefault {
				if win.searchBoxRect().containsPoint(mpos) {
					c = ebiten.CursorShapeText
				} else if win.searchCloseRect().containsPoint(mpos) {
					c = ebiten.CursorShapePointer
				}
			}
			if click && ctx.dragPart == PART_NONE && ctx.downWin == win {
				if win.searchCloseRect().containsPoint(mpos) {
					win.closeSearch()
					handled = true
				} else if win.searchBoxRect().containsPoint(mpos) {
					win.OpenSearch()
					handled = true
				}
			}
		}

		// Window items
		prevWinHovered := win.Hovered
		prevActiveWindow := ctx.activeWindow
		win.Hovered = false
		if !handled {
			handled = win.clickWindowItems(mpos, click)
		}
		if click && handled {
			ctx.pointerPressHandled = true
		}
		if win.Hovered != prevWinHovered {
			win.markDirty()
		}

		// Bring window forward on click if the cursor is over it or an
		// expanded dropdown. Break so windows behind don't receive the
		// event. The check includes clicks on dropdown menus which may
		// have closed during handling. Also consider context menus so a
		// right-click menu doesn't cause window activation behind it.
		if handled || win.containsPointer(mpos) || ctx.dropdownOpenContains(win.Contents, mpos) || ctx.contextMenuContainsAnywhere(mpos) {
			if click || midClick {
				if ctx.activeWindow == prevActiveWindow {
					if ctx.activeWindow != win || ctx.windows[len(ctx.windows)-1] != win {
						win.BringForward()
					}
				}
			}
			break
		}
	}

	// A slider owns its press until release, including outside its hit rectangle.
	// Process it once per tick, including the initial press, after hit testing has
	// established capture. This also makes short track clicks change the value.
	if item := ctx.activeItem; item != nil && item.ItemType == ITEM_SLIDER && ctx.pointerPressed() && ctx.downWin == item.ParentWindow && focusAvailable(item) {
		scale := float32(1)
		if ctx.CtrlPressed {
			scale = 1.0 / 8.0
		}
		item.setSliderValueWithModifiers(mpos, ctx.ShiftPressed, scale)
	}

	if ctx.activeItem != nil && ctx.activeItem.textEdit != nil && ctx.pointerPressed() && !click {
		ctx.activeItem.dragEditScrollbar(mpos)
	}
	if ctx.selectedTextItem != nil && ctx.selectedTextItem.selecting && ctx.pointerPressed() && !click {
		if itemHandlesTextEditing(ctx.selectedTextItem) {
			ctx.selectedTextItem.dragEditableText(mpos)
		} else {
			idx := ctx.selectedTextItem.cursorIndexAt(mpos)
			if idx != ctx.selectedTextItem.SelectEnd {
				ctx.selectedTextItem.SelectEnd = idx
				if itemAcceptsTextEditing(ctx.selectedTextItem) {
					ctx.selectedTextItem.CursorPos = idx
				}
				ctx.selectedTextItem.markDirty()
			}
		}
	}

	if ctx.hoveredItem != nil && ctx.hoveredItem.ItemType == ITEM_TEXT && ctx.hoveredItem.OnURLClick != nil && c == ebiten.CursorShapeDefault {
		if ctx.hoveredItem.urlAtChar(ctx.hoveredItem.cursorIndexAt(mpos)) != "" {
			c = ebiten.CursorShapePointer
		}
	}

	if ctx.hoveredItem != nil && !ctx.hoveredItem.isDisabled() && ctx.hoveredItem.editScrollbarAt(mpos) == PART_NONE && (itemAcceptsTextEditing(ctx.hoveredItem) || ctx.hoveredItem.SelectableText) && c == ebiten.CursorShapeDefault {
		c = ebiten.CursorShapeText
		if _, ok := ctx.hoveredItem.textSwatchAt(mpos); ok && !shiftPressed {
			c = ebiten.CursorShapePointer
		}
	}
	// The OS cursor is shared by Ebitengine's window. A disabled context
	// records its requested shape without overwriting the active interface.
	if ctx.inputEnabled && ebiten.CursorShape() != c {
		ebiten.SetCursorShape(c)
	}
	ctx.cursorShape = c

	if ctx.focusedItem != nil && !focusAvailable(ctx.focusedItem) {
		ctx.ClearFocus(ctx.focusedItem)
	}

	imeHandled, suppressText, err := ctx.pollTextInput()
	if err != nil {
		return err
	}
	if suppressText {
		chars = nil
	}
	keyboardAllowed := !ctx.keyboardInputCaptured && !ctx.sampledInput.unfocused && !imeHandled
	if keyboardAllowed && ctx.keyJustPressed(ebiten.KeyCapsLock) {
		if ctx.CapsLockToggleHandler != nil {
			ctx.CapsLockToggleHandler()
		}
	}

	if keyboardAllowed && ctx.activeWindow != nil && ctx.activeWindow.Open {
		if mods.Shortcut() && ctx.keyJustPressed(ebiten.KeyF) {
			ctx.activeWindow.OpenSearch()
		}
		if ctx.keyJustPressed(ebiten.KeyF3) && ctx.activeWindow.OnSearchNext != nil {
			ctx.activeWindow.OnSearchNext(shiftPressed)
		}
	}

	if keyboardAllowed && ctx.keyJustPressed(ebiten.KeyF6) {
		ctx.focusNextWindow(shiftPressed)
	}

	if keyboardAllowed && ctx.keyJustPressed(ebiten.KeyGraveAccent) && shiftPressed {
		_ = ctx.DumpTree()
	}
	if keyboardAllowed && mods.Shortcut() && ctx.keyJustPressed(ebiten.KeyC) && ctx.selectedTextItem != nil {
		if selected := ctx.selectedTextItem.SelectedText(); selected != "" {
			_ = ctx.writeClipboard(selected)
		}
	}

	textKeyboardHandled := imeHandled
	if keyboardAllowed && ctx.focusedItem != nil && ctx.activeSearch == nil && ctx.activeContextMenu() == nil && itemHandlesTextEditing(ctx.focusedItem) {
		textKeyboardHandled = ctx.focusedItem.updateTextEditing(chars, mods, shiftPressed)
	}
	if target := ctx.textInputTarget(); target != nil {
		target.updateCaretBlink(ctx.updateNow)
	}

	if !keyboardAllowed {
		// IME-owned keys and host capture must not activate UI shortcuts.
	} else if ctx.activeContextMenu() != nil {
		ctx.pollControlKeyboard(shiftPressed, mods)
		if ctx.keyJustPressed(ebiten.KeyTab) {
			ctx.CloseContextMenus()
			ctx.tabFocus(shiftPressed)
		}
	} else if ctx.activeSearch != nil {
		win := ctx.activeSearch
		target := win.ensureSearchInput()
		target.updateTextEditing(chars, mods, shiftPressed)
		if ctx.keyJustPressed(ebiten.KeyEscape) {
			win.closeSearch()
		} else if (ctx.keyJustPressed(ebiten.KeyEnter) || ctx.keyJustPressed(ebiten.KeyKPEnter)) && win.OnSearchNext != nil {
			win.OnSearchNext(shiftPressed)
		}
	} else {
		controlHandled := false
		if !textKeyboardHandled {
			controlHandled = ctx.pollControlKeyboard(shiftPressed, mods)
		}
		if !textKeyboardHandled && !controlHandled && (ctx.keyJustPressed(ebiten.KeyEnter) || ctx.keyJustPressed(ebiten.KeyKPEnter)) {
			if ctx.activeWindow != nil && ctx.activeWindow.Open {
				ctx.activateControl(ctx.activeWindow.DefaultButton)
			}
		}
		if !textKeyboardHandled && ctx.keyJustPressed(ebiten.KeyTab) {
			ctx.tabFocus(shiftPressed)
		}
	}
	if keyboardAllowed && ctx.keyJustPressed(ebiten.KeyEscape) {
		ctx.closeAllDropdowns()
	}
	if ctx.keyboardInputCaptured {
		ctx.finishKeyboardSlider()
	}

	ctx.mposOld = mpos

	if wheelDelta.X != 0 || wheelDelta.Y != 0 {
		for _, win := range orderedWindows {
			if !win.Open {
				continue
			}
			// Give context menus first chance at scroll.
			if ctx.scrollContextMenus(mpos, wheelDelta) {
				break
			}
			if win.getMainRect().containsPoint(mpos) || ctx.dropdownOpenContains(win.Contents, mpos) {
				if ctx.scrollDropdown(win.Contents, mpos, wheelDelta) {
					break
				}
				if ctx.scrollEditable(win.Contents, mpos, wheelDelta, mods) {
					break
				}
				if ctx.scrollFlow(win.Contents, mpos, wheelDelta) {
					break
				}
				if ctx.scrollWindow(win, wheelDelta) {
					break
				}
			}
		}
	}

	if ctx.hoveredItem != prevHovered {
		if prevHovered != nil {
			prevHovered.Hovered = false
			prevHovered.markDirty()
		}
	}

	// Refresh flow layouts only when needed. Constantly recalculating
	// layouts is expensive and can noticeably slow down the WebAssembly
	// build, especially on HiDPI screens. Windows and overlays handle their
	// own layout updates whenever sizes change, so avoid doing it every
	// frame here.

	for _, win := range ctx.windows {
		if win.Open {
			clearExpiredClicksAt(win.Contents, ctx.updateNow)
		}
	}

	return nil
}

// emitSliderReleased completes a slider interaction. Release events are only
// useful when the interaction changed the value, so clicks without movement
// do not notify consumers.
func (item *itemData) emitSliderReleased() {
	if item.ItemType != ITEM_SLIDER || !item.dragStartInit {
		return
	}
	item.dragStartInit = false
	if item.Value == item.dragStartValue {
		return
	}
	item.emitEvent(UIEvent{Type: EventSliderReleased, Value: item.Value})
}

func (ctx *Context) windowsFrontToBack() []*windowData {
	ctx.inputWindowOrder = ctx.inputWindowOrder[:0]
	for i := len(ctx.windows) - 1; i >= 0; i-- {
		if !ctx.windows[i].Docked {
			ctx.inputWindowOrder = append(ctx.inputWindowOrder, ctx.windows[i])
		}
	}
	for i := len(ctx.windows) - 1; i >= 0; i-- {
		if ctx.windows[i].Docked {
			ctx.inputWindowOrder = append(ctx.inputWindowOrder, ctx.windows[i])
		}
	}
	return ctx.inputWindowOrder
}

func (win *windowData) clickWindowItems(mpos point, click bool) bool {
	ctx := win.ui()

	// If the mouse isn't within the window or any open dropdown, just return
	if !win.getMainRect().containsPoint(mpos) && !ctx.dropdownOpenContains(win.Contents, mpos) {
		return false
	}
	if ctx.clickOpenDropdown(win.Contents, mpos, click) {
		return true
	}
	win.Hovered = true

	// Hit test in reverse draw order so later (visually-on-top) items
	// receive input before earlier ones when their rects overlap.
	for i := len(win.Contents) - 1; i >= 0; i-- {
		item := win.Contents[i]
		handled := false
		if item.ItemType == ITEM_FLOW {
			if part := item.getScrollbarPart(mpos); part != PART_NONE {
				if click && ctx.dragPart == PART_NONE && ctx.downWin == win {
					ctx.dragPart = part
					ctx.dragWin = win
					ctx.dragFlow = item
				}
				return true
			}
			handled = item.clickFlows(mpos, click)
		} else {
			handled = item.clickItem(mpos, click)
		}
		if handled {
			return true
		}
	}
	return false
}

func (item *itemData) clickFlows(mpos point, click bool) bool {
	ctx := item.ui()

	if item.isInvisible() {
		return false
	}
	if item.isDisabled() {
		return item.DrawRect.containsPoint(mpos)
	}
	if part := item.getScrollbarPart(mpos); part != PART_NONE {
		if click && ctx.dragPart == PART_NONE && ctx.downWin == item.ParentWindow {
			ctx.dragPart = part
			ctx.dragWin = item.ParentWindow
			ctx.dragFlow = item
		}
		return true
	}
	if len(item.Tabs) > 0 {
		item.normalizeTabs()
		for i, tab := range item.Tabs {
			tab.Hovered = false
			if tab.DrawRect.containsPoint(mpos) {
				tab.Hovered = true
				ctx.hoveredItem = tab
				if click && !tab.Disabled && !tab.Invisible {
					ctx.focusControl(item, false)
					ctx.activeItem = tab
					tab.Clicked = ctx.updateNow
					changeActiveTab(item, i)
				}
				return true
			}
		}
		contents := item.activeContents()
		for i := len(contents) - 1; i >= 0; i-- {
			subItem := contents[i]
			if subItem.ItemType == ITEM_FLOW {
				if subItem.clickFlows(mpos, click) {
					return true
				}
			} else {
				if subItem.clickItem(mpos, click) {
					return true
				}
			}
		}
	} else {
		for i := len(item.Contents) - 1; i >= 0; i-- {
			subItem := item.Contents[i]
			if subItem.ItemType == ITEM_FLOW {
				if subItem.clickFlows(mpos, click) {
					return true
				}
			} else {
				if subItem.clickItem(mpos, click) {
					return true
				}
			}
		}
	}
	return item.DrawRect.containsPoint(mpos)
}

func (item *itemData) clickItem(mpos point, click bool) bool {
	ctx := item.ui()

	if item.isInvisible() {
		return false
	}
	if item.isDisabled() {
		inside := item.DrawRect.containsPoint(mpos)
		if inside && item.Tooltip != "" {
			ctx.hoveredItem = item
		}
		return inside
	}
	if ctx.pointerPressed() && ctx.activeItem != nil && ctx.activeItem != item {
		return false
	}
	// For dropdowns check the expanded option area as well
	if !item.DrawRect.containsPoint(mpos) {
		if !(item.ItemType == ITEM_DROPDOWN && item.Open && func() bool {
			r, _ := ctx.dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
			return r.containsPoint(mpos)
		}()) {
			return false
		}
	}

	if click {
		ctx.focusControl(item, false)
		ctx.activeItem = item
		item.Clicked = ctx.updateNow
		if itemHandlesTextEditing(item) {
			if span, ok := item.textSwatchAt(mpos); ok && !ctx.ShiftPressed {
				item.textSwatchClick(span)
				return true
			}
			if part := item.editScrollbarAt(mpos); part != PART_NONE {
				item.pressEditScrollbar(mpos, part)
				item.Focused, ctx.focusedItem = true, item
				ctx.activeSearch = nil
				return true
			}
			item.clickEditableText(mpos, ctx.ShiftPressed)
			return true
		}
		if (item.ItemType == ITEM_TEXT || item.ItemType == ITEM_INPUT) && item.SelectableText {
			idx := item.cursorIndexAt(mpos)
			item.SelectStart = idx
			item.SelectEnd = idx
			item.selecting = true
			ctx.selectedTextItem = item
			if itemAcceptsTextEditing(item) {
				ctx.focusedItem = item
				item.CursorPos = idx
				item.Focused = true
			}
			item.markDirty()
			if item.OnURLClick != nil {
				if url := item.urlAtChar(idx); url != "" {
					item.OnURLClick(url)
				}
			}
			return true
		}
		if item.ItemType == ITEM_SLIDER {
			// Record drag start state for precision-drag (Ctrl) behavior
			item.dragStart = mpos
			item.dragStartValue = item.Value
			item.dragStartInit = true
		}
		if item.ItemType == ITEM_BUTTON || item.ItemType == ITEM_CHECKBOX || item.ItemType == ITEM_RADIO {
			ctx.activateControl(item)
			return true
		}
		item.markDirty()
		if item.ItemType == ITEM_COLORWHEEL {
			if col, ok := item.colorAt(mpos); ok {
				item.setUserColor(col)
			}
		}
		if itemAcceptsTextEditing(item) {
			ctx.focusedItem = item
			ctx.focusedItem.CursorPos = item.cursorIndexAt(mpos)
			item.Focused = true
			item.markDirty()
		} else if item.ItemType == ITEM_DROPDOWN {
			if item.Open {
				optionH := ctx.dropdownOptionHeight(item)
				r, _ := ctx.dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
				startY := r.Y0
				if r.containsPoint(mpos) {
					idx := int((mpos.Y - startY + item.Scroll.Y) / optionH)
					if idx >= 0 && idx < len(item.Options) {
						ctx.selectDropdownOption(item, idx)
					}
				} else {
					item.Open = false
					item.clearDropdownHover()
					item.markDirty()
				}
			} else {
				item.Open = true
				item.dropdownLayout = nil
				item.HoverIndex = -1
				item.markDirty()
			}
		}
	} else {
		if !item.Hovered {
			item.Hovered = true
			item.markDirty()
		}
		ctx.hoveredItem = item
		if item.ItemType == ITEM_COLORWHEEL && ctx.pointerPressed() && ctx.downWin == item.ParentWindow {
			if col, ok := item.colorAt(mpos); ok {
				item.setUserColor(col)
			}
		} else if item.ItemType == ITEM_DROPDOWN && item.Open && !item.keyboardMenu {
			optionH := ctx.dropdownOptionHeight(item)
			r, _ := ctx.dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
			startY := r.Y0
			if r.containsPoint(mpos) {
				idx := int((mpos.Y - startY + item.Scroll.Y) / optionH)
				if idx >= 0 && idx < len(item.Options) {
					if idx != item.HoverIndex {
						item.HoverIndex = idx
						item.markDirty()
						if item.OnHover != nil {
							item.OnHover(idx)
						}
					}
				}
			} else {
				item.clearDropdownHover()
			}
		}

	}
	return true
}

var urlRe = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// urlRanges returns rune-indexed HTTP(S) URL spans in the item text.
func (item *itemData) urlRanges() []TextSpan {
	var spans []TextSpan
	for _, match := range urlRe.FindAllStringIndex(item.Text, -1) {
		start := len([]rune(item.Text[:match[0]]))
		end := start + len([]rune(item.Text[match[0]:match[1]]))
		spans = append(spans, TextSpan{Start: start, End: end})
	}
	return spans
}

func (item *itemData) urlAtChar(index int) string {
	for _, span := range item.urlRanges() {
		if index >= span.Start && index < span.End {
			return string([]rune(item.Text)[span.Start:span.End])
		}
	}
	return ""
}

// SelectedText returns the currently selected text range for this item.
func (item *itemData) SelectedText() string {
	if item == nil || item.HideText || item.SelectStart == item.SelectEnd {
		return ""
	}
	runes := []rune(item.Text)
	start, end := item.SelectStart, item.SelectEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

func (ctx *Context) clearTextSelection() {
	if ctx.selectedTextItem == nil {
		return
	}
	ctx.selectedTextItem.SelectStart = 0
	ctx.selectedTextItem.SelectEnd = 0
	ctx.selectedTextItem.selecting = false
	ctx.selectedTextItem.markDirty()
	ctx.selectedTextItem = nil
}

func (item *itemData) cursorIndexAt(mpos point) int {
	ctx := item.ui()

	if item.ItemType == ITEM_INPUT || itemHandlesTextEditing(item) {
		return item.editCursorAt(mpos)
	}
	face := ctx.itemFace(item, item.FontSize*ctx.uiScale+2)
	lines := strings.Split(item.Text, "\n")
	lineHeight := (item.FontSize*ctx.uiScale + 2) * 1.2
	origin := item.textDrawOrigin
	if item.textDrawSize.X <= 0 {
		origin = point{X: item.DrawRect.X0, Y: item.DrawRect.Y0}
	}
	line := max(0, min(int(math.Floor(float64((mpos.Y-origin.Y)/lineHeight))), len(lines)-1))
	pos := 0
	for i := 0; i < line; i++ {
		pos += len([]rune(lines[i])) + 1
	}
	runes := []rune(lines[line])
	best, distance := 0, math.Inf(1)
	for _, stop := range graphemeStops(lines[line]) {
		x := text.AdvanceAt(lines[line], len(string(runes[:stop])), face)
		d := math.Abs(float64(mpos.X-origin.X) - x)
		if d < distance {
			best, distance = stop, d
		}
	}
	return pos + best
}

func subUncheckRadio(list []*itemData, group string, except *itemData) {
	for _, it := range list {
		if it.ItemType == ITEM_RADIO && it.RadioGroup == group && it != except {
			if it.Checked {
				it.Checked = false
				it.markDirty()
			}
		}
		if len(it.Tabs) > 0 {
			for _, tab := range it.Tabs {
				subUncheckRadio(tab.Contents, group, except)
			}
		}
		subUncheckRadio(it.Contents, group, except)
	}
}

func clearExpiredClicksAt(list []*itemData, now time.Time) {
	for _, it := range list {
		if !it.Clicked.IsZero() && now.Sub(it.Clicked) >= clickFlash {
			it.Clicked = time.Time{}
			it.markDirty()
		}
		for index, tab := range it.Tabs {
			if !tab.Clicked.IsZero() && now.Sub(tab.Clicked) >= clickFlash {
				tab.Clicked = time.Time{}
				tab.markDirty()
			}
			// Hidden pages cannot display a click flash. Clear their old
			// timestamps when selected, before their next paint.
			if index == it.ActiveTab {
				clearExpiredClicksAt(tab.Contents, now)
			}
		}
		clearExpiredClicksAt(it.Contents, now)
	}
}

// setSliderValueWithModifiers adjusts slider value honoring Shift/Ctrl behavior.
// shiftSnap: when true, snap to nearest integer.
// moveScale: scales movement toward the pointer target (e.g., 1/8 for fine adjust).
func (item *itemData) setSliderValueWithModifiers(mpos point, shiftSnap bool, moveScale float32) {
	ctx := item.ui()

	// Compute target from pointer using the base maths.
	old := item.Value
	// Recompute the target locally so we can adjust it before emitting events.
	if item.Vertical {
		knobH := item.AuxSize.Y * ctx.uiScale
		height := item.DrawRect.Y1 - item.DrawRect.Y0 - knobH
		if height <= 0 {
			return
		}
		start := item.DrawRect.Y0 + knobH/2
		end := start + height
		val := end - mpos.Y
		if val < 0 {
			val = 0
		}
		if val > height {
			val = height
		}
		ratio := val / height
		target := item.MinValue + ratio*(item.MaxValue-item.MinValue)
		// Fine adjust by moving fraction toward target
		if moveScale != 1 {
			target = old + (target-old)*moveScale
		}
		if shiftSnap {
			target = float32(int(target + 0.5))
		} else if item.IntOnly {
			target = float32(int(target + 0.5))
		}
		if target != item.Value {
			item.Value = target
			item.markDirty()
			item.emitEvent(UIEvent{Type: EventSliderChanged, Value: item.Value})
		}
		return
	}

	// Horizontal
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * ctx.uiScale) + 2
	face := ctx.textFace(textSize)
	maxW, _ := text.Measure(maxLabel, face, 0)
	knobW := item.AuxSize.X * ctx.uiScale
	gap := ctx.currentStyle.SliderValueGap
	if item.HideValue {
		gap = 0
		maxW = 0
	}
	width := item.DrawRect.X1 - item.DrawRect.X0 - knobW - gap - float32(maxW)
	if width < knobW {
		width = item.DrawRect.X1 - item.DrawRect.X0 - knobW
		if width < 0 {
			width = 0
		}
	}
	if width <= 0 {
		return
	}
	start := item.DrawRect.X0 + knobW/2
	val := (mpos.X - start)
	if val < 0 {
		val = 0
	}
	if val > width {
		val = width
	}
	ratio := val / width
	target := item.MinValue + ratio*(item.MaxValue-item.MinValue)
	if moveScale != 1 {
		target = old + (target-old)*moveScale
	}
	if shiftSnap {
		target = float32(int(target + 0.5))
	} else if item.IntOnly {
		target = float32(int(target + 0.5))
	}
	if target != item.Value {
		item.Value = target
		item.markDirty()
		item.emitEvent(UIEvent{Type: EventSliderChanged, Value: item.Value})
	}
}

func (item *itemData) colorAt(mpos point) (Color, bool) {
	ctx := item.ui()

	size := point{X: item.Size.X * ctx.uiScale, Y: item.Size.Y * ctx.uiScale}
	offsetY := float32(0)
	if item.Label != "" {
		offsetY = (item.FontSize*ctx.uiScale + 2) + ctx.currentStyle.TextPadding*ctx.uiScale
	}
	wheelSize := size.Y
	if wheelSize > size.X {
		wheelSize = size.X
	}
	radius := wheelSize / 2
	cx := item.DrawRect.X0 + radius
	cy := item.DrawRect.Y0 + offsetY + radius
	dx := float64(mpos.X - cx)
	dy := float64(mpos.Y - cy)
	r := float64(radius)
	dist := math.Hypot(dx, dy)

	if !item.DrawRect.containsPoint(mpos) {
		return Color{}, false
	}
	if dist > r {
		dist = r
	}

	ang := math.Atan2(dy, dx) * 180 / math.Pi
	if ang < 0 {
		ang += 360
	}
	v := dist / r
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	col := hsvaToRGBA(ang, 1, v, 1)
	return Color(col), true
}

func (ctx *Context) scrollFlow(items []*itemData, mpos point, delta point) bool {
	for _, it := range items {
		if it.Invisible {
			continue
		}
		if it.Disabled {
			continue
		}
		if it.ItemType == ITEM_FLOW {
			if it.DrawRect.containsPoint(mpos) {
				req := it.contentBounds()
				size := it.GetSize()
				old := it.Scroll
				if it.Scrollable {
					if it.verticalFlow() && req.Y > size.Y {
						it.Scroll.Y -= delta.Y * 16
						if it.Scroll.Y < 0 {
							it.Scroll.Y = 0
						}
						slack := 4 * ctx.UIScale()
						max := req.Y - size.Y + slack
						if it.Scroll.Y > max {
							it.Scroll.Y = max
						}
						if it.Scroll != old && it.ParentWindow != nil {
							it.ParentWindow.markDirty()
						}
						return true
					} else if it.horizontalFlow() && req.X > size.X {
						it.Scroll.X -= delta.X * 16
						if it.Scroll.X < 0 {
							it.Scroll.X = 0
						}
						slack := 4 * ctx.UIScale()
						max := req.X - size.X + slack
						if it.Scroll.X > max {
							it.Scroll.X = max
						}
						if it.Scroll != old && it.ParentWindow != nil {
							it.ParentWindow.markDirty()
						}
						return true
					}
				} else {
					if req.Y <= size.Y {
						it.Scroll.Y = 0
					}
					if req.X <= size.X {
						it.Scroll.X = 0
					}
				}
			}
			sub := it.activeContents()
			if ctx.scrollFlow(sub, mpos, delta) {
				return true
			}
		}
	}
	return false
}

func (ctx *Context) scrollDropdown(items []*itemData, mpos point, delta point) bool {
	for _, it := range items {
		if it.Invisible {
			continue
		}
		if it.Disabled {
			continue
		}
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			optionH := ctx.dropdownOptionHeight(it)
			r, _ := ctx.dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			openH := r.Y1 - r.Y0
			if r.containsPoint(mpos) {
				maxScroll := optionH*float32(len(it.Options)) - openH
				if maxScroll < 0 {
					maxScroll = 0
				}
				// Use the same scaling as window scrolling for a
				// consistent feel across widgets.
				old := it.Scroll
				it.Scroll.Y -= delta.Y * 16
				if it.Scroll.Y < 0 {
					it.Scroll.Y = 0
				}
				if it.Scroll.Y > maxScroll {
					it.Scroll.Y = maxScroll
				}
				if it.Scroll != old && it.ParentWindow != nil {
					it.ParentWindow.markDirty()
				}
				return true
			}
		}
		if ctx.scrollDropdown(it.activeContents(), mpos, delta) {
			return true
		}
	}
	return false
}

func (ctx *Context) scrollWindow(win *windowData, delta point) bool {
	if win.NoScroll {
		return false
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	old := win.Scroll
	handled := false
	if req.Y > avail.Y {
		win.Scroll.Y -= delta.Y * 16
		if win.Scroll.Y < 0 {
			win.Scroll.Y = 0
		}
		slack := 4 * ctx.UIScale()
		max := req.Y - avail.Y + slack
		if win.Scroll.Y > max {
			win.Scroll.Y = max
		}
		handled = true
	} else {
		win.Scroll.Y = 0
	}
	if req.X > avail.X {
		win.Scroll.X -= delta.X * 16
		if win.Scroll.X < 0 {
			win.Scroll.X = 0
		}
		slack := 4 * ctx.UIScale()
		max := req.X - avail.X + slack
		if win.Scroll.X > max {
			win.Scroll.X = max
		}
		handled = true
	} else {
		win.Scroll.X = 0
	}
	if handled || win.Scroll != old {
		win.markDirty()
	}
	return handled
}

func (ctx *Context) dragWindowMove(win *windowData, delta point) {
	// Re-check ownership at execution time. A workspace can dock a window while
	// a pointer drag is already active, and that stale drag must not move it.
	if win == nil || win.Docked || !win.Movable {
		return
	}
	if win.zone != nil && win.Movable {
		win.ClearZone()
	}
	win.resizeSnapAnchorX = false
	win.resizeSnapAnchorY = false
	if win.zone == nil {
		if ctx.windowSnapping && win.snapAnchorActive {
			win.snapDragPosition = pointAdd(win.snapDragPosition, delta)
			if win.snapAnchorX && math.Abs(float64(win.snapDragPosition.X-win.snapAnchor.X)) > float64(UnsnapThreshold) {
				win.snapAnchorX = false
			}
			if win.snapAnchorY && math.Abs(float64(win.snapDragPosition.Y-win.snapAnchor.Y)) > float64(UnsnapThreshold) {
				win.snapAnchorY = false
			}
			if win.snapAnchorX {
				win.Position.X = win.snapAnchor.X
			} else {
				win.Position.X = win.snapDragPosition.X
			}
			if win.snapAnchorY {
				win.Position.Y = win.snapAnchor.Y
			} else {
				win.Position.Y = win.snapDragPosition.Y
			}
			win.snapAnchorActive = win.snapAnchorX || win.snapAnchorY
		} else {
			win.Position = pointAdd(win.Position, delta)
		}
	}
}

// dragWindowResize constrains the dragged edges before setSize can reposition
// the window to fit the screen. The opposite edges stay fixed, including when
// a corner reaches the minimum size on only one axis. Delta uses window units.
func (ctx *Context) dragWindowResize(win *windowData, part dragType, delta point) bool {
	if win == nil || win.Docked || !win.Resizable {
		return false
	}
	if win.resizeSnapAnchorPart != part {
		win.resizeSnapAnchorX = false
		win.resizeSnapAnchorY = false
	}
	left, top := win.Position.X, win.Position.Y
	right, bottom := left+win.Size.X, top+win.Size.Y
	sw, sh := float32(ctx.screenWidth)/win.scale(), float32(ctx.screenHeight)/win.scale()
	switch part {
	case PART_LEFT, PART_TOP_LEFT, PART_BOTTOM_LEFT:
		left = max(0, min(left+delta.X, right-MinWindowSize))
	case PART_RIGHT, PART_TOP_RIGHT, PART_BOTTOM_RIGHT:
		right = min(sw, max(right+delta.X, left+MinWindowSize))
	}
	switch part {
	case PART_TOP, PART_TOP_LEFT, PART_TOP_RIGHT:
		top = max(0, min(top+delta.Y, bottom-MinWindowSize))
	case PART_BOTTOM, PART_BOTTOM_LEFT, PART_BOTTOM_RIGHT:
		bottom = min(sh, max(bottom+delta.Y, top+MinWindowSize))
	}
	pos, size := point{X: left, Y: top}, point{X: right - left, Y: bottom - top}
	if pos == win.Position && size == win.Size {
		return false
	}
	// Explicit resizing takes ownership of placement, just like dragging.
	win.ClearZone()
	win.snapAnchorActive = false
	win.snapAnchorX = false
	win.snapAnchorY = false
	win.Position = pos
	changed := win.setSize(size)
	if win.resizeSnapAnchorX {
		current, anchor := win.Position.X, win.resizeSnapAnchorPosition.X
		switch part {
		case PART_RIGHT, PART_TOP_RIGHT, PART_BOTTOM_RIGHT:
			current = win.Position.X + win.Size.X
			anchor = win.resizeSnapAnchorPosition.X + win.resizeSnapAnchorSize.X
		}
		if math.Abs(float64(current-anchor)) > float64(resizeUnsnapThreshold) {
			win.resizeSnapAnchorX = false
		}
	}
	if win.resizeSnapAnchorY {
		current, anchor := win.Position.Y, win.resizeSnapAnchorPosition.Y
		switch part {
		case PART_BOTTOM, PART_BOTTOM_LEFT, PART_BOTTOM_RIGHT:
			current = win.Position.Y + win.Size.Y
			anchor = win.resizeSnapAnchorPosition.Y + win.resizeSnapAnchorSize.Y
		}
		if math.Abs(float64(current-anchor)) > float64(resizeUnsnapThreshold) {
			win.resizeSnapAnchorY = false
		}
	}
	return changed
}

func (ctx *Context) dragWindowScroll(win *windowData, mpos point, vert bool) {
	if win.NoScroll {
		return
	}
	old := win.Scroll
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if vert && req.Y > avail.Y {
		barH := scrollbarThumbLength(avail.Y, req.Y, win.scale())
		slack := 4 * ctx.UIScale()
		maxScroll := req.Y - avail.Y + slack
		track := win.getPosition().Y + win.GetTitleSize() + win.BorderPad*win.scale()
		pos := mpos.Y - (track + barH/2)
		if pos < 0 {
			pos = 0
		}
		if pos > avail.Y-barH {
			pos = avail.Y - barH
		}
		if avail.Y-barH > 0 {
			win.Scroll.Y = (pos / (avail.Y - barH)) * maxScroll
		} else {
			win.Scroll.Y = 0
		}
	} else if vert {
		win.Scroll.Y = 0
	}
	if !vert && req.X > avail.X {
		barW := scrollbarThumbLength(avail.X, req.X, win.scale())
		slack := 4 * ctx.UIScale()
		maxScroll := req.X - avail.X + slack
		track := win.getPosition().X + win.BorderPad*win.scale()
		pos := mpos.X - (track + barW/2)
		if pos < 0 {
			pos = 0
		}
		if pos > avail.X-barW {
			pos = avail.X - barW
		}
		if avail.X-barW > 0 {
			win.Scroll.X = (pos / (avail.X - barW)) * maxScroll
		} else {
			win.Scroll.X = 0
		}
	} else if !vert {
		win.Scroll.X = 0
	}
	if win.Scroll != old {
		win.markDirty()
	}
}

func (ctx *Context) dragFlowScroll(flow *itemData, mpos point, vert bool) {
	if !flow.Scrollable {
		return
	}
	old := flow.Scroll
	req := flow.contentBounds()
	size := flow.GetSize()
	if vert && flow.verticalFlow() && req.Y > size.Y {
		barH := scrollbarThumbLength(size.Y, req.Y, ctx.UIScale())
		slack := 4 * ctx.UIScale()
		maxScroll := req.Y - size.Y + slack
		track := flow.DrawRect.Y0
		pos := mpos.Y - (track + barH/2)
		if pos < 0 {
			pos = 0
		}
		if pos > size.Y-barH {
			pos = size.Y - barH
		}
		if size.Y-barH > 0 {
			flow.Scroll.Y = (pos / (size.Y - barH)) * maxScroll
		} else {
			flow.Scroll.Y = 0
		}
	} else if vert {
		flow.Scroll.Y = 0
	}
	if !vert && flow.horizontalFlow() && req.X > size.X {
		barW := scrollbarThumbLength(size.X, req.X, ctx.UIScale())
		slack := 4 * ctx.UIScale()
		maxScroll := req.X - size.X + slack
		track := flow.DrawRect.X0
		pos := mpos.X - (track + barW/2)
		if pos < 0 {
			pos = 0
		}
		if pos > size.X-barW {
			pos = size.X - barW
		}
		if size.X-barW > 0 {
			flow.Scroll.X = (pos / (size.X - barW)) * maxScroll
		} else {
			flow.Scroll.X = 0
		}
	} else if !vert {
		flow.Scroll.X = 0
	}
	if flow.Scroll != old && flow.ParentWindow != nil {
		flow.ParentWindow.markDirty()
	}
}
func (ctx *Context) dropdownOpenContains(items []*itemData, mpos point) bool {
	for _, it := range items {
		if it.Invisible {
			continue
		}
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			r, _ := ctx.dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			if r.containsPoint(mpos) {
				return true
			}
		}
		if ctx.dropdownOpenContains(it.activeContents(), mpos) {
			return true
		}
	}
	return false
}

func (ctx *Context) clickOpenDropdown(items []*itemData, mpos point, click bool) bool {
	for _, it := range items {
		if it.Invisible {
			continue
		}
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			r, _ := ctx.dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			if r.containsPoint(mpos) {
				it.clickItem(mpos, click)
				return true
			}
		}
		if ctx.clickOpenDropdown(it.activeContents(), mpos, click) {
			return true
		}
	}
	return false
}

func (ctx *Context) dropdownOpenContainsAnywhere(mpos point) bool {
	for _, win := range ctx.windows {
		if win.Open && ctx.dropdownOpenContains(win.Contents, mpos) {
			return true
		}
	}
	return false
}

// Notify preview consumers when the pointer leaves or the menu is dismissed.
func (item *itemData) clearDropdownHover() {
	if !item.Open {
		item.dropdownLayout = nil
	}
	if item.HoverIndex == -1 {
		return
	}
	item.HoverIndex = -1
	item.markDirty()
	if item.OnHover != nil {
		item.OnHover(item.Selected)
	}
}

func closeDropdowns(items []*itemData) {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN {
			if it.Open {
				it.Open = false
				it.keyboardMenu = false
				it.clearDropdownHover()
				it.markDirty()
			}
		}
		for _, tab := range it.Tabs {
			closeDropdowns(tab.Contents)
		}
		closeDropdowns(it.Contents)
	}
}

func (ctx *Context) closeAllDropdowns() {
	for _, win := range ctx.windows {
		if win.Open {
			closeDropdowns(win.Contents)
		}
	}
}

// ---- Context menu helpers (overlay, screen-space) ----

// handleContextMenus processes hover and clicks for active context menus. Returns true if handled.
func (ctx *Context) handleContextMenus(mpos point, click bool) bool {
	handled := false
	for _, cm := range ctx.contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		if cm.keyboardMenu && !click {
			handled = true
			continue
		}
		// Compute the open rectangle at the stored origin.
		r, _ := ctx.dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		if r.containsPoint(mpos) {
			handled = true
			optionH := ctx.dropdownOptionHeight(cm)
			startY := r.Y0
			idx := int((mpos.Y - startY + cm.Scroll.Y) / optionH)
			if idx >= 0 && idx < len(cm.Options) {
				// Treat header rows as non-selectable: ignore hover highlight
				// and do not close or invoke selection callbacks.
				if idx < cm.HeaderCount {
					if !click {
						if cm.HoverIndex != -1 {
							cm.HoverIndex = -1
							cm.markDirty()
						}
					}
					// Do not close menus; simply consume the hover/click area.
					return true
				}
				if click {
					ctx.selectDropdownOption(cm, idx)
					return true
				} else {
					if idx != cm.HoverIndex {
						cm.HoverIndex = idx
						// markDirty not required for overlays, but keeps parity.
						cm.markDirty()
						if cm.OnHover != nil {
							cm.OnHover(idx)
						}
					}
				}
			}
		} else if !click {
			if cm.HoverIndex != -1 {
				cm.HoverIndex = -1
				cm.markDirty()
				if cm.OnHover != nil {
					cm.OnHover(cm.Selected)
				}
			}
		}
	}
	return handled
}

func (ctx *Context) contextMenuContainsAnywhere(mpos point) bool {
	for _, cm := range ctx.contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		r, _ := ctx.dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		if r.containsPoint(mpos) {
			return true
		}
	}
	return false
}

func (ctx *Context) scrollContextMenus(mpos point, delta point) bool {
	for _, cm := range ctx.contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		optionH := ctx.dropdownOptionHeight(cm)
		r, _ := ctx.dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		openH := r.Y1 - r.Y0
		if r.containsPoint(mpos) {
			maxScroll := optionH*float32(len(cm.Options)) - openH
			if maxScroll < 0 {
				maxScroll = 0
			}
			old := cm.Scroll
			cm.Scroll.Y -= delta.Y * 16
			if cm.Scroll.Y < 0 {
				cm.Scroll.Y = 0
			}
			if cm.Scroll.Y > maxScroll {
				cm.Scroll.Y = maxScroll
			}
			if cm.Scroll != old {
				cm.markDirty()
			}
			return true
		}
	}
	return false
}

// SearchActive reports whether a window search box currently has focus.
func (ctx *Context) SearchActive() bool {
	ctx.mustAlive()

	return ctx.activeSearch != nil
}
