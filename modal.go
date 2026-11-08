package eui

import "github.com/hajimehoshi/ebiten/v2"

type modalFocus struct {
	window                      *windowData
	search                      *windowData
	item, selection             *itemData
	itemWindow, selectionWindow *windowData
	visible                     bool
}
type modalEntry struct {
	window  *windowData
	restore modalFocus
	opening bool
}

// ModalWindow returns this context's top modal, or nil. Hosts can use it to
// reserve game input; modality does not change another context's input routing.
func (ctx *Context) ModalWindow() *WindowData { ctx.mustAlive(); return ctx.modalWindow() }
func (ctx *Context) modalWindow() *windowData {
	for i := len(ctx.modals) - 1; i >= 0; i-- {
		entry := ctx.modals[i]
		if entry.window.Open || entry.opening {
			return entry.window
		}
	}
	return nil
}
func (ctx *Context) modalAllows(win *windowData) bool {
	modal := ctx.modalWindow()
	return modal == nil || modal == win
}

// IsModal reports whether this window is currently in its context's modal stack.
func (win *windowData) IsModal() bool {
	ctx := win.ui()
	if win == nil {
		return false
	}
	for _, entry := range ctx.modals {
		if entry.window == win {
			return win.Open || entry.opening
		}
	}
	return false
}

// MarkOpenModal opens a modal window, suspends other windows' input, and focuses
// its first available control unless OnOpen already chose one. Modals nest;
// closing one restores the previous available focus without resetting editors.
// Calling this on an existing modal does not reorder its nested children.
func (win *windowData) MarkOpenModal() {
	ctx := win.ui()
	if win == nil {
		return
	}
	ctx.syncModals()
	if ctx.disposed || ctx.disposePending {
		return
	}

	for _, entry := range ctx.modals {
		if entry.window == win {
			if ctx.modalWindow() == win {
				win.BringForward()
			}
			return
		}
	}
	restore := modalFocus{window: ctx.activeWindow, search: ctx.activeSearch, item: ctx.focusedItem, selection: ctx.selectedTextItem, visible: ctx.focusVisible}
	if restore.item != nil && restore.item.ParentWindow != nil {
		restore.itemWindow = restore.item.ParentWindow
		restore.window = restore.item.ParentWindow
	}
	if restore.search != nil {
		restore.window = restore.search
	}
	if ctx.downWin != nil && ctx.sampledInput.primaryDown {
		restore.window = ctx.downWin
	}
	if restore.selection != nil {
		restore.selectionWindow = restore.selection.ParentWindow
	}
	ctx.modals = append(ctx.modals, modalEntry{window: win, restore: restore, opening: true})
	ctx.modalChanged()
	ctx.activeWindow = win
	win.MarkOpen()
	for i := range ctx.modals {
		if ctx.modals[i].window == win {
			ctx.modals[i].opening = false
		}
	}
	if !win.Open {
		ctx.retireModal(win)
		return
	}
	if ctx.modalWindow() != win {
		return
	}
	// Dismiss previews only after the new scope is installed. Preview callbacks
	// can open/close modals; the newest scope wins rather than being overwritten.
	ctx.CloseContextMenus()
	ctx.closeAllDropdowns()
	if ctx.modalWindow() != win {
		return
	}
	ctx.activeWindow = win
	if !focusAvailable(ctx.focusedItem) || ctx.focusedItem.ParentWindow != win {
		ctx.focusNext(win, false)
	}
}

func (ctx *Context) modalChanged() {
	ctx.modalEpoch++
	ctx.cancelGamepadAdjustment()
	ctx.gamepad.awaitNeutral = true
	ctx.pointerPressHandled = ctx.pointerPressHandled || ctx.modalPointerHeld()
	ctx.modalAwaitNeutral = true
	if ctx.focusedItem != nil {
		ctx.focusedItem.CancelComposition()
		if ctx.focusedItem.textEdit != nil {
			ctx.focusedItem.textEdit.group = ""
		}
		ctx.focusedItem.Focused = false
		invalidateFocus(ctx.focusedItem)
	}
	ctx.cancelNativeTextInput(nil)
	if ctx.selectedTextItem != nil {
		ctx.selectedTextItem.selecting = false
	}
	if ctx.activeItem != nil {
		ctx.activeItem.selecting = false
		ctx.activeItem.dragStartInit = false
	}
	if ctx.keyboardSlider != nil {
		ctx.keyboardSlider.dragStartInit = false
	}
	if ctx.hoveredItem != nil {
		ctx.hoveredItem.Hovered = false
		ctx.hoveredItem.markDirty()
	}
	ctx.focusedItem, ctx.selectedTextItem, ctx.activeItem, ctx.hoveredItem = nil, nil, nil, nil
	ctx.keyboardSlider = nil
	ctx.dragWin, ctx.dragFlow, ctx.downWin = nil, nil, nil
	ctx.dragPart = PART_NONE
	ctx.activeTileDivider, ctx.hoveredTileDivider = -1, -1
	ctx.activeSearch = nil
	for _, win := range ctx.windows {
		if win.searchInput != nil {
			win.searchInput.Focused = false
		}
		win.Hovered, win.HoverClose, win.HoverDragbar, win.HoverMax, win.HoverSearch = false, false, false, false, false
		win.Dirty = true
		win.refreshPending = false
	}
}
func (ctx *Context) retireModal(win *windowData) {
	index := -1
	for i, entry := range ctx.modals {
		if entry.window == win {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	entry := ctx.modals[index]
	top := index == len(ctx.modals)-1
	// Removing a parent out of order must not leave a child restoring a dead
	// parent. Carry the parent's return path through its immediate dependents.
	for i := index + 1; i < len(ctx.modals); i++ {
		saved := &ctx.modals[i].restore
		if saved.window == win || saved.itemWindow == win {
			*saved = entry.restore
		}
	}
	copy(ctx.modals[index:], ctx.modals[index+1:])
	ctx.modals[len(ctx.modals)-1] = modalEntry{}
	ctx.modals = ctx.modals[:len(ctx.modals)-1]
	if !top {
		return
	}
	ctx.modalChanged()
	ctx.restoreModalFocus(entry.restore)
	ctx.CloseContextMenus()
	ctx.closeAllDropdowns()
}
func (ctx *Context) restoreModalFocus(saved modalFocus) {
	modal := ctx.modalWindow()
	if modal != nil {
		ctx.activeWindow = modal
	} else if saved.window != nil && saved.window.Open {
		ctx.activeWindow = saved.window
		saved.window.BringForward()
	} else {
		ctx.activeWindow = nil
	}
	if saved.search != nil && saved.search.Open && saved.search.searchOpen && saved.search.Searchable && ctx.modalAllows(saved.search) {
		ctx.activeSearch = saved.search
		saved.search.ensureSearchInput()
		ctx.activeWindow = saved.search
		ctx.restoreModalSelection(saved)
		saved.search.markDirty()
		return
	}

	if saved.item != nil && saved.itemWindow != nil && saved.item.ParentWindow == saved.itemWindow && focusAvailable(saved.item) && ctx.modalAllows(saved.item.ParentWindow) {
		ctx.focusControl(saved.item, saved.visible)
	} else if modal != nil {
		ctx.focusNext(modal, false)
	}
	ctx.restoreModalSelection(saved)
}
func (ctx *Context) restoreModalSelection(saved modalFocus) {
	if saved.selection != nil && saved.selection.ParentWindow == saved.selectionWindow && !saved.selection.isInvisible() && !saved.selection.isDisabled() && ctx.modalAllows(saved.selection.ParentWindow) {
		if win := saved.selection.ParentWindow; win != nil && win.Open {
			if _, ok := focusPath(win.Contents, saved.selection, nil); ok {
				ctx.selectedTextItem = saved.selection
			}
		}
	}
}
func (ctx *Context) syncModals() {
	for i := 0; i < len(ctx.modals); {
		entry := ctx.modals[i]
		if !entry.opening && !entry.window.Open {
			ctx.retireModal(entry.window)
		} else {
			i++
		}
	}
}
func (ctx *Context) modalInputNeutral() bool {
	if !ctx.gamepadNeutral() {
		return false
	}
	for _, ticks := range ctx.sampledInput.keys {
		if ticks > 0 {
			return false
		}
	}
	for _, ticks := range ctx.sampledInput.buttons {
		if ticks > 0 {
			return false
		}
	}
	return len(ctx.sampledInput.touches) == 0
}
func (ctx *Context) modalFrameChanged(epoch uint64) bool {
	return epoch != ctx.modalEpoch || ctx.disposePending
}
func (ctx *Context) cancelModal() {
	win := ctx.modalWindow()
	if win == nil {
		return
	}
	if win.CancelButton != nil {
		ctx.activateControl(win.CancelButton)
	} else if win.Closable {
		win.Close()
	}
}
func (ctx *Context) hasOpenDropdown() bool {
	var contains func([]*itemData) bool
	contains = func(items []*itemData) bool {
		for _, item := range items {
			if item == nil || item.Invisible {
				continue
			}
			if item.ItemType == ITEM_DROPDOWN && item.Open {
				return true
			}
			if contains(item.activeContents()) {
				return true
			}
		}
		return false
	}
	for _, win := range ctx.windows {
		if win.Open && ctx.modalAllows(win) && contains(win.Contents) {
			return true
		}
	}
	return false
}
func (ctx *Context) drawModalBackdrop(screen *ebiten.Image) {
	bounds := screen.Bounds()
	if ctx.ModalBackdrop.A != 0 {
		drawFilledRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Dx()), float32(bounds.Dy()), ctx.ModalBackdrop.ToRGBA(), false)
	}
}

func (ctx *Context) modalItemAllowed(item *itemData) bool {
	if item.contextMenu {
		return item.modalOwner == ctx.modalWindow()
	}
	return ctx.modalAllows(item.ParentWindow)
}

func (ctx *Context) modalPointerHeld() bool {
	if ctx.sampledInput.primaryDown || len(ctx.sampledInput.touches) > 0 {
		return true
	}
	for _, ticks := range ctx.sampledInput.buttons {
		if ticks > 0 {
			return true
		}
	}
	return false
}
