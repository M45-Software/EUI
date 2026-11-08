package eui

import (
	"math"
	"slices"
)

// Runtime setters update the model without emitting UIEvent, Action, OnSelect,
// or OnColorChange. Preview dismissal can call OnHover to restore the selected
// appearance. User interactions and editor commands retain their event behavior.

func finiteValue(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (item *itemData) runtimeChanged(layout bool) {
	ctx := item.ui()

	if item == nil {
		return
	}
	item.markDirty()
	if win := item.ParentWindow; win != nil {
		if layout {
			win.RefreshWithReason("state")
			win.updateHasIndeterminate()
		}
		// Geometry and active interaction cannot display an old cache while
		// hit testing has already adopted the new model.
		if layout || ctx.focusedItem == item || ctx.activeItem == item {
			item.repaintStateNow()
		}
	} else if layout {
		root := item
		for root.Parent != nil {
			root = root.Parent
		}
		root.resizeFlow(root.GetSize())
	}
}

func normalizedControlValue(value, minimum, maximum float32, integer bool) (float32, bool) {
	if !finiteValue(value) || !finiteValue(minimum) || !finiteValue(maximum) || minimum > maximum {
		return 0, false
	}
	if integer {
		minimum = float32(math.Ceil(float64(minimum)))
		maximum = float32(math.Floor(float64(maximum)))
		if minimum > maximum {
			return 0, false
		}
		value = float32(math.Round(float64(value)))
	}
	return max(minimum, min(value, maximum)), true
}

// SetValue silently updates a slider or progress bar. Finite values are clamped
// to its range; IntOnly sliders snap to an in-range integer. Invalid ranges and
// non-finite values are ignored. An ongoing slider interaction is rebased so a
// programmatic update alone cannot produce a release event.
func (item *itemData) SetValue(value float32) {
	if item == nil || item.ItemType != ITEM_SLIDER && item.ItemType != ITEM_PROGRESS {
		return
	}
	value, ok := normalizedControlValue(value, item.MinValue, item.MaxValue, item.ItemType == ITEM_SLIDER && item.IntOnly)
	if !ok || item.Value == value {
		return
	}
	item.Value = value
	item.rebaseSliderInteraction()
	item.runtimeChanged(false)
}

func (item *itemData) rebaseSliderInteraction() {
	ctx := item.ui()

	if item.ItemType != ITEM_SLIDER {
		return
	}
	if ctx.keyboardSlider == item {
		ctx.keyboardSliderStart = item.Value
	}
	if item.dragStartInit {
		item.dragStartValue = item.Value
		item.dragStart = ctx.mposOld
	}
}

// SetRange changes a slider/progress range and clamps its current value. Bounds
// must be finite and ordered. An IntOnly slider's range must contain an integer.
// Equal bounds are permitted. A non-finite current value is reset to the minimum.
func (item *itemData) SetRange(minimum, maximum float32) {
	if item == nil || item.ItemType != ITEM_SLIDER && item.ItemType != ITEM_PROGRESS {
		return
	}
	value := item.Value
	if !finiteValue(value) {
		value = minimum
	}
	value, ok := normalizedControlValue(value, minimum, maximum, item.ItemType == ITEM_SLIDER && item.IntOnly)
	if !ok || item.MinValue == minimum && item.MaxValue == maximum && item.Value == value {
		return
	}
	item.MinValue, item.MaxValue, item.Value = minimum, maximum, value
	item.rebaseSliderInteraction()
	item.runtimeChanged(false)
}

// SetChecked silently changes a checkbox or radio. Checking a grouped radio
// unchecks its peers in the containing item/window, including inactive pages.
func (item *itemData) SetChecked(checked bool) {
	if item == nil || item.ItemType != ITEM_CHECKBOX && item.ItemType != ITEM_RADIO {
		return
	}
	if item.ItemType == ITEM_RADIO && checked {
		item.uncheckRadioPeers()
	}
	if item.Checked == checked {
		return
	}
	item.Checked = checked
	item.runtimeChanged(false)
}

func (item *itemData) uncheckRadioPeers() {
	if item.RadioGroup == "" {
		return
	}
	if item.Parent != nil {
		subUncheckRadio(item.Parent.Contents, item.RadioGroup, item)
	} else if item.ParentWindow != nil {
		subUncheckRadio(item.ParentWindow.Contents, item.RadioGroup, item)
	}
}

// SetDisabled retains the control's layout while disabling its entire subtree.
// Disabling immediately cancels its focus, selection, drag, and open dropdowns.
func (item *itemData) SetDisabled(disabled bool) {
	if item == nil || item.Disabled == disabled {
		return
	}
	item.Disabled = disabled
	if disabled {
		item.clearHiddenState()
	}
	item.normalizeOwningTabs()
	markItemTreeDirty(item)
	item.runtimeChanged(false)
	item.repaintStateNow()
}

// SetVisible hides or shows an item and its descendants. Hidden items contribute
// no layout space. Hiding immediately cancels their interaction state; showing
// does not take focus. Images and requested sizes remain available for reuse.
func (item *itemData) SetVisible(visible bool) {
	if item == nil || item.Invisible == !visible {
		return
	}
	item.Invisible = !visible
	if !visible {
		item.clearHiddenState()
	}
	item.normalizeOwningTabs()
	item.runtimeChanged(true)
}

// SetSelected silently selects a dropdown row, or clears selection with -1.
// Header rows and other out-of-range indices are ignored. A change dismisses
// an open menu, ending its hover preview before the selected index is replaced.
func (item *itemData) SetSelected(index int) {
	if item == nil || item.ItemType != ITEM_DROPDOWN {
		return
	}
	if index != -1 && (index < max(0, item.HeaderCount) || index >= len(item.Options)) {
		return
	}
	if item.Selected == index {
		return
	}
	item.dismissRuntimeMenu()
	// Ending a preview can replace the options through an application callback.
	if index != -1 && (index < max(0, item.HeaderCount) || index >= len(item.Options)) {
		item.runtimeChanged(false)
		return
	}
	item.Selected = index
	item.runtimeChanged(false)
}

// SetOptions copies the dropdown's options and dismisses an open menu when its
// contents change. Selection is clamped to a selectable row, or -1 if none exist.
// Existing option images retain their row indices and are truncated if needed.
// Identical options preserve an explicitly cleared (-1) selection.
func (item *itemData) SetOptions(options []string) {
	if item == nil || item.ItemType != ITEM_DROPDOWN {
		return
	}
	changed := !slices.Equal(item.Options, options)
	index := item.Selected
	first := max(0, item.HeaderCount)
	if first >= len(options) {
		index = -1
	} else if changed || index != -1 {
		index = max(first, min(index, len(options)-1))
	}
	// Clone even an identical input so caller-owned slices cannot mutate the model.
	next := slices.Clone(options)
	if !changed && index == item.Selected {
		item.Options = next
		return
	}
	item.dismissRuntimeMenu()
	item.Options = next
	// A preview callback may also update header configuration.
	first = max(0, item.HeaderCount)
	if first >= len(next) {
		index = -1
	} else if index != -1 {
		index = max(first, min(index, len(next)-1))
	}
	item.Selected = index
	if len(item.OptionImages) > len(next) {
		item.OptionImages = slices.Clone(item.OptionImages[:len(next)])
	}
	item.Scroll = point{}
	item.dropdownLayout = nil
	item.runtimeChanged(false)
}

func (item *itemData) dismissRuntimeMenu() {
	ctx := item.ui()

	preview := item.Open && item.HoverIndex >= 0
	item.Open = false
	item.keyboardMenu = false
	if item.contextMenu {
		for i, menu := range ctx.contextMenus {
			if menu == item {
				copy(ctx.contextMenus[i:], ctx.contextMenus[i+1:])
				ctx.contextMenus[len(ctx.contextMenus)-1] = nil
				ctx.contextMenus = ctx.contextMenus[:len(ctx.contextMenus)-1]
				break
			}
		}
	}
	if preview {
		item.clearDropdownHover()
	} else {
		item.HoverIndex = -1
		item.dropdownLayout = nil
	}
}

// SetActiveTab selects an enabled, visible page without emitting user events.
// Invalid indices are ignored. Page changes preserve the new page's own state,
// clear the old page's input state, and recalculate layout and progress tracking.
func (item *itemData) SetActiveTab(index int) {
	if item == nil || item.ItemType != ITEM_FLOW || !item.selectableTab(index) || item.ActiveTab == index {
		return
	}
	item.setActivePage(index)
	item.runtimeChanged(true)
}

func (item *itemData) selectableTab(index int) bool {
	return index >= 0 && index < len(item.Tabs) && item.Tabs[index] != nil && !item.Tabs[index].Invisible && !item.Tabs[index].Disabled
}

func (item *itemData) activePage() *itemData {
	if item == nil || !item.selectableTab(item.ActiveTab) {
		return nil
	}
	return item.Tabs[item.ActiveTab]
}

func (item *itemData) activeContents() []*itemData {
	if len(item.Tabs) == 0 {
		return item.Contents
	}
	if page := item.activePage(); page != nil {
		return page.Contents
	}
	return nil
}

func (item *itemData) setActivePage(index int) {
	previous := item.ActiveTab
	item.ActiveTab = index
	item.Scroll = point{}
	if previous != index && previous >= 0 && previous < len(item.Tabs) {
		item.Tabs[previous].clearHiddenState()
	}
}

// Empty/all-unavailable page sets use -1, never a hidden page's children.
func (item *itemData) normalizeTabs() {
	if len(item.Tabs) == 0 || item.selectableTab(item.ActiveTab) {
		return
	}
	next := -1
	for i := range item.Tabs {
		if item.selectableTab(i) {
			next = i
			break
		}
	}
	if item.ActiveTab != next {
		item.setActivePage(next)
	}
}

func (item *itemData) normalizeOwningTabs() {
	if owner := item.Parent; owner != nil && len(owner.Tabs) > 0 {
		for _, page := range owner.Tabs {
			if page == item {
				before := owner.ActiveTab
				owner.normalizeTabs()
				if before != owner.ActiveTab {
					owner.runtimeChanged(true)
				}
				break
			}
		}
	}
}

// SetLabel updates a control's field label and reflows its containing layout.
func (item *itemData) SetLabel(label string) {
	if item == nil || item.Label == label {
		return
	}
	item.Label = label
	item.runtimeChanged(true)
}

// SetSize sets logical item dimensions and reflows layout. Zero dimensions use
// the control's automatic sizing rules; negative/non-finite sizes are ignored.
func (item *itemData) SetSize(size Point) {
	if item == nil || !finiteValue(size.X) || !finiteValue(size.Y) || size.X < 0 || size.Y < 0 || item.Size == size {
		return
	}
	item.Size = size
	item.runtimeChanged(true)
}

// SetWheelColor silently updates a color wheel or color-swatch button.
func (item *itemData) SetWheelColor(value Color) {
	if item == nil || item.ItemType != ITEM_COLORWHEEL && !item.ColorSwatch || item.WheelColor == value {
		return
	}
	item.WheelColor = value
	item.runtimeChanged(false)
}

func (item *itemData) repaintStateNow() {
	if win := item.ParentWindow; win != nil {
		win.Dirty = true
		win.refreshPending = false
	}
}
