package eui

import (
	"math"
	"sort"

	"github.com/Distortions81/EUI/internal/inputkeys"
	"github.com/hajimehoshi/ebiten/v2"
)

// FocusedItem returns the available control with keyboard focus, including a
// tab strip. Hidden, disabled, inactive-tab, and closed-window controls return nil.
func (ctx *Context) FocusedItem() *ItemData {
	ctx.mustAlive()

	if !focusAvailable(ctx.focusedItem) {
		return nil
	}
	return ctx.focusedItem
}

func controlFocusable(it *itemData) bool {
	if it == nil || it.Disabled || it.Invisible {
		return false
	}
	if itemAcceptsTextEditing(it) {
		return true
	}
	switch it.ItemType {
	case ITEM_BUTTON, ITEM_CHECKBOX, ITEM_RADIO, ITEM_SLIDER, ITEM_DROPDOWN, ITEM_COLORWHEEL:
		return true
	case ITEM_FLOW:
		for _, tab := range it.Tabs {
			if tab != nil && !tab.Disabled && !tab.Invisible {
				return true
			}
		}
	}
	return false
}

// Find the path through active pages rather than trusting a stale drawing rect.
// Tabs can be assembled directly, so their Parent links need not be populated.
func focusPath(items []*itemData, target *itemData, path []*itemData) ([]*itemData, bool) {
	for _, it := range items {
		if it == nil || it.Disabled || it.Invisible {
			continue
		}
		next := append(path, it)
		if it == target {
			return next, true
		}
		children := it.Contents
		if len(it.Tabs) > 0 {
			if it.ActiveTab < 0 || it.ActiveTab >= len(it.Tabs) {
				continue
			}
			tab := it.Tabs[it.ActiveTab]
			if tab == nil || tab.Disabled || tab.Invisible {
				continue
			}
			next = append(next, tab)
			if tab == target {
				return next, true
			}
			children = tab.Contents
		}
		if found, ok := focusPath(children, target, next); ok {
			return found, true
		}
	}
	return nil, false
}

func focusAvailable(it *itemData) bool {
	if !controlFocusable(it) {
		return false
	}
	if it.ctx != nil && (it.ctx.disposed || !it.ctx.modalItemAllowed(it)) {
		return false
	}

	for parent := it.Parent; parent != nil; parent = parent.Parent {
		if parent.Disabled || parent.Invisible {
			return false
		}
	}
	if win := it.ParentWindow; win != nil {
		if !win.Open {
			return false
		}
		_, ok := focusPath(win.Contents, it, nil)
		return ok
	}
	return true
}

func collectFocusControls(items []*itemData, result *[]*itemData) {
	for _, it := range items {
		if it == nil || it.Disabled || it.Invisible {
			continue
		}
		// Externally edited text reserves its keyboard policy for the host.
		if controlFocusable(it) && !it.ExternalTextEditing {
			*result = append(*result, it)
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= 0 && it.ActiveTab < len(it.Tabs) {
				tab := it.Tabs[it.ActiveTab]
				if tab != nil && !tab.Disabled && !tab.Invisible {
					collectFocusControls(tab.Contents, result)
				}
			}
		} else {
			collectFocusControls(it.Contents, result)
		}
	}
}

func invalidateFocus(it *itemData) {
	if it == nil {
		return
	}
	it.Dirty = true
	if it.ParentWindow != nil {
		it.ParentWindow.markDirty()
		// Interaction feedback must not wait for a background refresh interval.
		it.ParentWindow.Dirty = true
		it.ParentWindow.refreshPending = false
	}
}

func (ctx *Context) focusControl(it *itemData, visible bool) bool {
	if !focusAvailable(it) {
		return false
	}
	it.normalizeTabs()
	if ctx.focusedItem != it {
		beforeFocus, beforeWindow := ctx.focusedItem, ctx.activeWindow
		ctx.finishKeyboardSlider()
		// Release callbacks may close a window or explicitly focus another
		// control. Honor that result instead of reviving an obsolete target.
		if !focusAvailable(it) || ctx.focusedItem != beforeFocus || ctx.activeWindow != beforeWindow {
			return false
		}
		if previous := ctx.focusedItem; previous != nil {
			previous.CancelComposition()
			previous.Focused = false
			if previous.ItemType == ITEM_DROPDOWN && previous.Open {
				previous.Open = false
				previous.keyboardMenu = false
				previous.clearDropdownHover()
			}
			invalidateFocus(previous)
		}
	}
	ctx.focusedItem = it
	ctx.focusVisible = visible
	ctx.activeSearch = nil
	it.Focused = true
	invalidateFocus(it)
	if visible {
		ctx.revealFocusedControl(it)
	}
	return true
}

func (ctx *Context) focusNext(win *windowData, backward bool) bool {
	if win == nil || !win.Open || !ctx.modalAllows(win) {
		return false
	}
	ctx.closeAllDropdowns()
	if !win.Open {
		return false
	}
	var controls []*itemData
	collectFocusControls(win.Contents, &controls)
	// A radio group contributes one Tab stop; arrows move inside the group.
	type radioKey struct {
		parent *itemData
		name   string
	}
	chosen := map[radioKey]*itemData{}
	for _, it := range controls {
		if it.ItemType != ITEM_RADIO || it.RadioGroup == "" {
			continue
		}
		key := radioKey{it.Parent, it.RadioGroup}
		current := chosen[key]
		if current == nil || it == ctx.focusedItem || current != ctx.focusedItem && !current.Checked && it.Checked {
			chosen[key] = it
		}
	}
	stops := controls[:0]
	for _, it := range controls {
		if it.ItemType != ITEM_RADIO || it.RadioGroup == "" || chosen[radioKey{it.Parent, it.RadioGroup}] == it {
			stops = append(stops, it)
		}
	}
	controls = stops
	if len(controls) == 0 {
		return false
	}
	index := -1
	for i, it := range controls {
		if it == ctx.focusedItem {
			index = i
			break
		}
	}
	next := 0
	if backward {
		if index < 0 {
			next = len(controls) - 1
		} else {
			next = (index - 1 + len(controls)) % len(controls)
		}
	} else {
		next = (index + 1) % len(controls)
	}
	ctx.Focus(controls[next])
	return true
}

// Pointer and keyboard activation share state changes and callback ordering.
func (ctx *Context) activateControl(it *itemData) bool {
	if !focusAvailable(it) {
		return false
	}
	event := UIEvent{Item: it}
	switch it.ItemType {
	case ITEM_BUTTON:
		event.Type = EventClick
	case ITEM_CHECKBOX:
		it.Checked = !it.Checked
		event.Type = EventCheckboxChanged
		event.Checked = it.Checked
	case ITEM_RADIO:
		if it.RadioGroup != "" {
			it.uncheckRadioPeers()
		}
		it.Checked = true
		event.Type = EventRadioSelected
		event.Checked = true
	default:
		return false
	}
	it.Clicked = ctx.updateNow
	if it.Clicked.IsZero() {
		it.Clicked = ctx.uiNow()
	}
	invalidateFocus(it)
	it.emitEvent(event)
	return true
}

func changeActiveTab(it *itemData, index int) bool {
	if !it.selectableTab(index) {
		return false
	}
	it.SetActiveTab(index)
	invalidateFocus(it)
	return true
}

func (ctx *Context) moveTab(it *itemData, key ebiten.Key) {
	direction := 1
	if key == ebiten.KeyArrowLeft || key == ebiten.KeyArrowUp || key == ebiten.KeyPageUp {
		direction = -1
	}
	start := it.ActiveTab
	if key == ebiten.KeyHome {
		start = -1
		direction = 1
	}
	if key == ebiten.KeyEnd {
		start = len(it.Tabs)
		direction = -1
	}
	for n := 1; n <= len(it.Tabs); n++ {
		index := ((start+n*direction)%len(it.Tabs) + len(it.Tabs)) % len(it.Tabs)
		if changeActiveTab(it, index) {
			ctx.revealFocusedControl(it)
			return
		}
	}
}

func (ctx *Context) radioArrow(it *itemData, key ebiten.Key) {
	if it.RadioGroup == "" {
		return
	}
	var candidates []*itemData
	if it.ParentWindow != nil {
		collectFocusControls(it.ParentWindow.Contents, &candidates)
	} else if it.Parent != nil {
		collectFocusControls(it.Parent.Contents, &candidates)
	} else {
		candidates = []*itemData{it}
	}
	group := candidates[:0]
	for _, other := range candidates {
		if other.ItemType == ITEM_RADIO && other.Parent == it.Parent && other.RadioGroup == it.RadioGroup {
			group = append(group, other)
		}
	}
	if len(group) == 0 {
		return
	}
	index := 0
	for i, other := range group {
		if other == it {
			index = i
			break
		}
	}
	direction := 1
	if key == ebiten.KeyArrowLeft || key == ebiten.KeyArrowUp {
		direction = -1
	}
	target := group[(index+direction+len(group))%len(group)]
	ctx.focusControl(target, true)
	ctx.activateControl(target)
}

func (ctx *Context) finishKeyboardSlider() {
	it, start := ctx.keyboardSlider, ctx.keyboardSliderStart
	ctx.keyboardSlider = nil
	if it != nil && it.Value != start {
		it.emitEvent(UIEvent{Type: EventSliderReleased, Value: it.Value})
	}
}

func (ctx *Context) keyboardSliderValue(it *itemData, key ebiten.Key, shift, fine bool) {
	if it.MaxValue <= it.MinValue {
		return
	}
	step := (it.MaxValue - it.MinValue) / 100
	if it.IntOnly {
		step = max(1, float32(math.Round(float64(step))))
	}
	if shift {
		step *= 10
	} else if fine && !it.IntOnly {
		step /= 10
	}
	value := it.Value
	switch key {
	case ebiten.KeyArrowLeft, ebiten.KeyArrowDown:
		value -= step
	case ebiten.KeyArrowRight, ebiten.KeyArrowUp:
		value += step
	case ebiten.KeyPageDown:
		value -= step * 10
	case ebiten.KeyPageUp:
		value += step * 10
	case ebiten.KeyHome:
		value = it.MinValue
	case ebiten.KeyEnd:
		value = it.MaxValue
	}
	if it.IntOnly {
		value = float32(math.Round(float64(value)))
	}
	value = max(it.MinValue, min(value, it.MaxValue))
	if value == it.Value {
		return
	}
	if ctx.keyboardSlider != it {
		ctx.finishKeyboardSlider()
		ctx.keyboardSlider = it
		ctx.keyboardSliderStart = it.Value
	}
	it.Value = value
	invalidateFocus(it)
	it.emitEvent(UIEvent{Type: EventSliderChanged, Value: value})
}

func (ctx *Context) openKeyboardDropdown(it *itemData, backward bool) {
	if len(it.Options) <= max(0, it.HeaderCount) {
		return
	}
	ctx.closeAllDropdowns()
	it.Open = true
	it.keyboardMenu = true
	it.dropdownLayout = nil
	// Freeze preview geometry before an OnHover callback can re-layout its window.
	ctx.dropdownMenuLayout(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
	index := it.Selected
	if index < max(0, it.HeaderCount) || index >= len(it.Options) {
		index = max(0, it.HeaderCount)
		if backward {
			index = len(it.Options) - 1
		}
	}
	ctx.setKeyboardOption(it, index)
}

func (ctx *Context) setKeyboardOption(it *itemData, index int) {
	if index < max(0, it.HeaderCount) || index >= len(it.Options) {
		it.HoverIndex = -1
		invalidateFocus(it)
		return
	}
	it.keyboardMenu = true
	changed := it.HoverIndex != index
	it.HoverIndex = index
	_, visible := ctx.dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
	row := ctx.dropdownOptionHeight(it)
	it.Scroll.Y = max(0, min(it.Scroll.Y, float32(len(it.Options)-visible)*row))
	if float32(index)*row < it.Scroll.Y {
		it.Scroll.Y = float32(index) * row
	}
	if float32(index+1)*row > it.Scroll.Y+float32(visible)*row {
		it.Scroll.Y = float32(index+1-visible) * row
	}
	invalidateFocus(it)
	if changed && it.OnHover != nil {
		it.OnHover(index)
	}
}

func (ctx *Context) selectDropdownOption(it *itemData, index int) bool {
	if index < max(0, it.HeaderCount) || index >= len(it.Options) {
		return false
	}
	it.Selected = index
	it.Open = false
	it.keyboardMenu = false
	invalidateFocus(it)
	if it.contextMenu {
		ctx.CloseContextMenus()
	} else {
		it.clearDropdownHover()
	}
	it.emitEvent(UIEvent{Type: EventDropdownSelected, Index: index})
	return true
}

func (ctx *Context) keyboardDropdown(it *itemData, key ebiten.Key) bool {
	switch key {
	case ebiten.KeyEscape:
		if !it.Open {
			return false
		}
		it.Open = false
		it.keyboardMenu = false
		it.clearDropdownHover()
		invalidateFocus(it)
		if it.contextMenu {
			ctx.CloseContextMenus()
		}
	case ebiten.KeyEnter, ebiten.KeyKPEnter, ebiten.KeySpace:
		if it.Open {
			if it.HoverIndex < max(0, it.HeaderCount) || it.HoverIndex >= len(it.Options) {
				ctx.setKeyboardOption(it, max(0, it.HeaderCount))
			}
			ctx.selectDropdownOption(it, it.HoverIndex)
		} else {
			ctx.openKeyboardDropdown(it, false)
		}
	case ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd:
		if !it.Open {
			ctx.openKeyboardDropdown(it, key == ebiten.KeyArrowUp)
			if it.Open && key == ebiten.KeyHome {
				ctx.setKeyboardOption(it, max(0, it.HeaderCount))
			}
			if it.Open && key == ebiten.KeyEnd {
				ctx.setKeyboardOption(it, len(it.Options)-1)
			}
		} else {
			count := len(it.Options) - max(0, it.HeaderCount)
			if count <= 0 {
				return true
			}
			first := max(0, it.HeaderCount)
			index := it.HoverIndex
			valid := index >= first && index < len(it.Options)
			if !valid {
				index = first
			}
			switch key {
			case ebiten.KeyHome:
				index = first
			case ebiten.KeyEnd:
				index = len(it.Options) - 1
			case ebiten.KeyArrowUp:
				if valid {
					index = first + (index-first-1+count)%count
				} else {
					index = len(it.Options) - 1
				}
			case ebiten.KeyArrowDown:
				if valid {
					index = first + (index-first+1)%count
				}
			}
			ctx.setKeyboardOption(it, index)
		}
	default:
		return false
	}
	return true
}

func adjustmentKey(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd, ebiten.KeyPageUp, ebiten.KeyPageDown:
		return true
	}
	return false
}

// Dispatch a decoded control key. Tests supply commands without polling hardware.
func (ctx *Context) controlKey(it *itemData, key ebiten.Key, shift bool, mods inputkeys.Modifiers) bool {
	if !focusAvailable(it) || itemAcceptsTextEditing(it) {
		return false
	}
	ctx.focusVisible = true
	invalidateFocus(it)
	switch it.ItemType {
	case ITEM_DROPDOWN:
		return ctx.keyboardDropdown(it, key)
	case ITEM_FLOW:
		if adjustmentKey(key) {
			ctx.moveTab(it, key)
			return true
		}
	case ITEM_SLIDER:
		if adjustmentKey(key) {
			ctx.keyboardSliderValue(it, key, shift, mods.Control || mods.Meta)
			return true
		}
	case ITEM_RADIO:
		if key == ebiten.KeyArrowLeft || key == ebiten.KeyArrowRight || key == ebiten.KeyArrowUp || key == ebiten.KeyArrowDown {
			ctx.radioArrow(it, key)
			return true
		}
	case ITEM_COLORWHEEL:
		if adjustmentKey(key) {
			h, s, v, a := it.WheelColor.HSVA()
			step := .01
			if shift {
				step = .1
			}
			switch key {
			case ebiten.KeyArrowLeft:
				h -= step * 360
			case ebiten.KeyArrowRight:
				h += step * 360
			case ebiten.KeyArrowDown:
				s -= step
			case ebiten.KeyArrowUp:
				s += step
			case ebiten.KeyPageDown:
				v -= step
			case ebiten.KeyPageUp:
				v += step
			case ebiten.KeyHome:
				s = 0
			case ebiten.KeyEnd:
				s = 1
			}
			it.setUserColor(NewColorHSV(math.Mod(h+360, 360), s, v, a))
			return true
		}
	}
	if key == ebiten.KeyEnter || key == ebiten.KeyKPEnter || key == ebiten.KeySpace {
		return ctx.activateControl(it)
	}
	return false
}

func (ctx *Context) pollControlKeyboard(shift bool, mods inputkeys.Modifiers) bool {
	held := false
	for _, key := range []ebiten.Key{ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd, ebiten.KeyPageUp, ebiten.KeyPageDown} {
		held = held || ctx.keyPressed(key)
	}
	if !held && !ctx.gamepadSliderHeld() {
		ctx.finishKeyboardSlider()
	}
	handled := false
	for _, key := range []ebiten.Key{ebiten.KeyEnter, ebiten.KeyKPEnter, ebiten.KeySpace, ebiten.KeyEscape, ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd, ebiten.KeyPageUp, ebiten.KeyPageDown} {
		pressed := ctx.keyJustPressed(key)
		if adjustmentKey(key) {
			pressed = ctx.textKeyRepeats(key)
		}
		if pressed {
			target := ctx.FocusedItem()
			if menu := ctx.activeContextMenu(); menu != nil {
				target = menu
			}
			handled = ctx.controlKey(target, key, shift, mods) || handled
		}
	}
	return handled
}

func (ctx *Context) activeContextMenu() *itemData {
	for i := len(ctx.contextMenus) - 1; i >= 0; i-- {
		if menu := ctx.contextMenus[i]; menu != nil && menu.Open && menu.modalOwner == ctx.modalWindow() {
			return menu
		}
	}
	return nil
}

// F6 visits every interactive window in registration order, independently of
// z-order changes caused by focusing it. Empty/background windows are skipped.
func (ctx *Context) focusNextWindow(backward bool) bool {
	if modal := ctx.modalWindow(); modal != nil {
		return ctx.focusNext(modal, backward)
	}

	ctx.CloseContextMenus()
	ctx.closeAllDropdowns()
	var available []*windowData
	for _, win := range ctx.windows {
		if !win.Open {
			continue
		}
		var controls []*itemData
		collectFocusControls(win.Contents, &controls)
		if len(controls) > 0 {
			available = append(available, win)
		}
	}
	if len(available) == 0 {
		return false
	}
	sort.Slice(available, func(i, j int) bool { return available[i].keyboardOrder < available[j].keyboardOrder })
	current := -1
	for i, win := range available {
		if win == ctx.activeWindow {
			current = i
			break
		}
	}
	next := (current + 1) % len(available)
	if backward {
		if current < 0 {
			next = len(available) - 1
		} else {
			next = (current - 1 + len(available)) % len(available)
		}
	}
	target := available[next]
	target.BringForward()
	return ctx.focusNext(target, backward)
}

// ScrollIntoView reveals this visible control through its scrolling ancestors
// without changing keyboard focus, the text caret, or selection. Detached,
// hidden, disabled, and inactive-page controls do not change scrolling.
func (item *itemData) ScrollIntoView() {
	ctx := item.ui()

	if item == nil || item.ParentWindow == nil {
		return
	}
	if _, visible := focusPath(item.ParentWindow.Contents, item, nil); visible {
		ctx.revealFocusedControl(item)
	}
}

// Tab can enter the UI before any pointer click has activated a window.
func (ctx *Context) tabFocus(backward bool) {
	if !ctx.focusNext(ctx.activeWindow, backward) {
		ctx.focusNextWindow(backward)
	}
}
