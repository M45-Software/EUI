package eui

import (
	"fmt"
	"testing"
	"time"

	"github.com/Distortions81/EUI/internal/inputkeys"
	"github.com/hajimehoshi/ebiten/v2"
)

func (ctx *Context) keyboardFixture(t testing.TB) *WindowData {
	t.Helper()
	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	oldWindows, oldActive, oldFocus := ctx.windows, ctx.activeWindow, ctx.focusedItem
	oldSelection, oldHovered, oldSearch := ctx.selectedTextItem, ctx.hoveredItem, ctx.activeSearch
	oldVisible, oldSlider, oldStart := ctx.focusVisible, ctx.keyboardSlider, ctx.keyboardSliderStart
	oldScale, oldWidth, oldHeight, oldNow := ctx.uiScale, ctx.screenWidth, ctx.screenHeight, ctx.updateNow
	oldItem := ctx.activeItem
	ctx.windows, ctx.focusedItem, ctx.selectedTextItem, ctx.hoveredItem, ctx.activeSearch = nil, nil, nil, nil, nil
	ctx.keyboardSlider = nil
	ctx.focusVisible = false
	ctx.uiScale = 1
	ctx.screenWidth, ctx.screenHeight = 1024, 900
	ctx.updateNow = time.Unix(100, 0)
	t.Cleanup(func() {
		for _, win := range ctx.windows {
			win.deallocate()
		}
		ctx.windows, ctx.activeWindow, ctx.focusedItem = oldWindows, oldActive, oldFocus
		ctx.selectedTextItem, ctx.hoveredItem, ctx.activeSearch = oldSelection, oldHovered, oldSearch
		ctx.focusVisible, ctx.keyboardSlider, ctx.keyboardSliderStart = oldVisible, oldSlider, oldStart
		ctx.uiScale, ctx.screenWidth, ctx.screenHeight, ctx.updateNow = oldScale, oldWidth, oldHeight, oldNow
		ctx.activeItem = oldItem
	})
	win := ctx.NewWindow()
	win.Size = Point{X: 500, Y: 400}
	win.MarkOpen()
	return win
}

func (ctx *Context) key(t *testing.T, it *ItemData, k ebiten.Key) {
	t.Helper()
	if !ctx.controlKey(it, k, false, inputkeys.Modifiers{}) {
		t.Fatalf("control %v did not handle %v", it.ItemType, k)
	}
}

func TestFocusTraversalAllControlsAndActiveTabs(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	button := ctx.NewActionButton("Button", nil)
	input, _ := ctx.NewInput()
	check, _ := ctx.NewCheckbox()
	slider, _ := ctx.NewSlider()
	dd, _ := ctx.NewDropdown()
	radio1, _ := ctx.NewRadio()
	radio2, _ := ctx.NewRadio()
	radio1.RadioGroup = "group"
	radio2.RadioGroup = "group"
	radio2.Checked = true
	hidden := ctx.NewColumn(ctx.NewActionButton("Hidden", nil))
	hidden.Invisible = true
	disabled := ctx.NewColumn(ctx.NewActionButton("Disabled", nil))
	disabled.Disabled = true
	firstChild := ctx.NewActionButton("First page", nil)
	secondChild := ctx.NewActionButton("Second page", nil)
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 250, Y: 100}
	tabs.Tabs = []*ItemData{ctx.NewColumn(firstChild), ctx.NewColumn(secondChild)}
	tabs.Tabs[0].Name = "First"
	tabs.Tabs[1].Name = "Second"
	root := ctx.NewColumn(button, input, check, radio1, radio2, slider, dd, hidden, disabled, tabs)
	win.AddItem(root)
	win.Refresh()
	expected := []*ItemData{button, input, check, radio2, slider, dd, tabs, firstChild, button}
	for i, want := range expected {
		if !ctx.focusNext(win, false) || ctx.FocusedItem() != want || !want.Focused || !ctx.focusVisible {
			t.Fatalf("Tab step %d: got %p want %p", i, ctx.FocusedItem(), want)
		}
	}
	if !ctx.focusNext(win, true) || ctx.FocusedItem() != firstChild {
		t.Fatal("Shift+Tab did not wrap backward")
	}
	ctx.Focus(tabs)
	ctx.key(t, tabs, ebiten.KeyArrowRight)
	if tabs.ActiveTab != 1 || ctx.FocusedItem() != tabs || firstChild.Focused {
		t.Fatal("tab arrow did not switch page")
	}
	ctx.focusNext(win, false)
	if ctx.FocusedItem() != secondChild {
		t.Fatal("Tab entered inactive page")
	}
	ctx.Focus(firstChild)
	if ctx.FocusedItem() != secondChild {
		t.Fatal("inactive page accepted focus")
	}
	disabled.Disabled = false
	hidden.Invisible = false
	ctx.Focus(hidden.Contents[0])
	if ctx.FocusedItem() != hidden.Contents[0] {
		t.Fatal("shown subtree did not accept focus")
	}
	disabled.Disabled = true
	ctx.Focus(disabled.Contents[0])
	if ctx.FocusedItem() == disabled.Contents[0] {
		t.Fatal("disabled ancestor accepted focus")
	}
}

func TestKeyboardAndPointerActivationUseSameEvents(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	for _, kind := range []int{ITEM_BUTTON, ITEM_CHECKBOX, ITEM_RADIO} {
		for _, pointer := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/pointer=%v", kind, pointer), func(t *testing.T) {
				var it *ItemData
				switch kind {
				case ITEM_BUTTON:
					it, _ = ctx.NewButton()
				case ITEM_CHECKBOX:
					it, _ = ctx.NewCheckbox()
				case ITEM_RADIO:
					it, _ = ctx.NewRadio()
				}
				win.AddItem(it)
				it.DrawRect = Rect{X1: 100, Y1: 40}
				events, actions := 0, 0
				it.Handler.Handle = func(ev UIEvent) {
					events++
					if ev.Item != it {
						t.Fatal("event lost target")
					}
					if kind == ITEM_CHECKBOX && !ev.Checked {
						t.Fatal("checkbox event preceded value update")
					}
				}
				it.Action = func() { actions++ }
				ctx.Focus(it)
				if pointer {
					ctx.activeItem = nil
					it.clickItem(Point{X: 10, Y: 10}, true)
				} else {
					ctx.key(t, it, ebiten.KeySpace)
				}
				if events != 1 || actions != 1 {
					t.Fatalf("events=%d actions=%d", events, actions)
				}
				if kind != ITEM_BUTTON && !it.Checked {
					t.Fatal("activation did not update checked state")
				}
				if it.Clicked.IsZero() {
					t.Fatal("activation omitted visual feedback")
				}
			})
		}
	}
	// A callback may retire its own window and open another one.
	button := ctx.NewActionButton("Next", func() { win.Close(); next := ctx.NewWindow(); next.MarkOpen() })
	win.AddItem(button)
	ctx.Focus(button)
	ctx.key(t, button, ebiten.KeyEnter)
	if win.Open || ctx.activeWindow == win || ctx.FocusedItem() != nil {
		t.Fatal("activation retained retired focus")
	}
}

func TestKeyboardRadioGroupAndSliderTransactions(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	first, _ := ctx.NewRadio()
	blocked, _ := ctx.NewRadio()
	last, _ := ctx.NewRadio()
	first.RadioGroup = "quality"
	blocked.RadioGroup = "quality"
	last.RadioGroup = "quality"
	first.Checked = true
	blocked.Disabled = true
	group := ctx.NewRow(first, blocked, last)
	slider, _ := ctx.NewSlider()
	slider.MinValue = -10
	slider.MaxValue = 10
	slider.Value = 0
	win.AddItem(ctx.NewColumn(group, slider))
	ctx.Focus(first)
	ctx.key(t, first, ebiten.KeyArrowRight)
	if !last.Checked || first.Checked || ctx.FocusedItem() != last {
		t.Fatal("radio arrows did not skip disabled choice")
	}
	ctx.key(t, last, ebiten.KeyArrowRight)
	if !first.Checked || last.Checked || ctx.FocusedItem() != first {
		t.Fatal("radio arrows did not wrap")
	}
	changes, releases, actions := 0, 0, 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderChanged {
			changes++
		}
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	slider.Action = func() { actions++ }
	ctx.Focus(slider)
	ctx.key(t, slider, ebiten.KeyArrowRight)
	ctx.key(t, slider, ebiten.KeyArrowRight)
	if slider.Value < .39 || slider.Value > .41 || changes != 2 || releases != 0 || actions != 2 {
		t.Fatalf("bad slider repeat: value=%v changes=%d releases=%d actions=%d", slider.Value, changes, releases, actions)
	}
	ctx.finishKeyboardSlider()
	ctx.finishKeyboardSlider()
	if releases != 1 {
		t.Fatal("slider release was duplicated or omitted")
	}
	ctx.key(t, slider, ebiten.KeyEnd)
	ctx.key(t, slider, ebiten.KeyArrowRight)
	ctx.Focus(first)
	if slider.Value != 10 || releases != 2 {
		t.Fatal("slider did not clamp or finish on blur")
	}
	slider.IntOnly = true
	ctx.Focus(slider)
	ctx.key(t, slider, ebiten.KeyArrowLeft)
	if slider.Value != 9 {
		t.Fatal("integer slider did not take integer step")
	}
	ctx.finishKeyboardSlider()
	slider.Disabled = true
	if ctx.controlKey(slider, ebiten.KeyHome, false, inputkeys.Modifiers{}) || slider.Value != 9 {
		t.Fatal("disabled slider accepted keyboard")
	}
}

func TestKeyboardDropdownPreviewCommitCancelAndHeaders(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	dd, _ := ctx.NewDropdown()
	dd.Options = []string{"Header", "One", "Two", "Three", "Four", "Five"}
	dd.HeaderCount = 1
	dd.Selected = 2
	dd.MaxVisible = 2
	win.AddItem(dd)
	dd.DrawRect = Rect{X0: 10, Y0: 20, X1: 200, Y1: 44}
	ctx.Focus(dd)
	var preview []int
	dd.OnHover = func(index int) { preview = append(preview, index) }
	selected, events := 0, 0
	dd.OnSelect = func(index int) { selected = index }
	dd.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventDropdownSelected {
			events++
		}
	}
	ctx.key(t, dd, ebiten.KeySpace)
	if !dd.Open || dd.HoverIndex != 2 || !dd.keyboardMenu {
		t.Fatal("dropdown did not open on selection")
	}
	ctx.key(t, dd, ebiten.KeyEnd)
	if dd.HoverIndex != 5 || dd.Scroll.Y <= 0 || dd.Selected != 2 || selected != 0 {
		t.Fatal("preview committed early or failed to scroll")
	}
	ctx.key(t, dd, ebiten.KeyEscape)
	if dd.Open || dd.Selected != 2 || dd.HoverIndex != -1 || preview[len(preview)-1] != 2 {
		t.Fatal("Escape failed to restore preview")
	}
	ctx.key(t, dd, ebiten.KeyHome)
	if dd.HoverIndex != 1 {
		t.Fatal("Home did not skip header")
	}
	ctx.key(t, dd, ebiten.KeyArrowUp)
	if dd.HoverIndex != 5 {
		t.Fatal("option navigation did not wrap")
	}
	ctx.key(t, dd, ebiten.KeyEnter)
	if dd.Open || dd.Selected != 5 || selected != 5 || events != 1 {
		t.Fatal("Enter did not commit once")
	}
	if ctx.selectDropdownOption(dd, 0) {
		t.Fatal("header was selectable")
	}
	ctx.key(t, dd, ebiten.KeySpace)
	ctx.focusNext(win, false)
	// A one-control window retains focus, but Tab must still dismiss its menu.
	if dd.Open {
		t.Fatal("Tab did not dismiss dropdown")
	}
}

func TestFocusRejectsHiddenDisabledAndClosedTabState(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	child := ctx.NewActionButton("Page", nil)
	tabs := ctx.NewColumn()
	tabs.Tabs = []*ItemData{ctx.NewColumn(child), ctx.NewColumn()}
	tabs.Tabs[0].Disabled = true
	win.AddItem(tabs)
	ctx.Focus(tabs)
	if tabs.ActiveTab != 1 {
		t.Fatal("focus did not normalize disabled active tab")
	}
	ctx.Focus(child)
	if ctx.FocusedItem() != tabs {
		t.Fatal("disabled page accepted focus")
	}
	tabs.Tabs[1].Invisible = true
	if ctx.FocusedItem() != nil {
		t.Fatal("unavailable tab strip retained focus")
	}
	win.Close()
	ctx.Focus(tabs)
	if ctx.FocusedItem() != nil {
		t.Fatal("closed window accepted focus")
	}
}

func TestFocusRevealsOffscreenControlsBeforeDrawing(t *testing.T) {
	ctx := newTestContext(t)

	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			win := ctx.keyboardFixture(t)
			ctx.uiScale = scale
			win.Size = Point{X: 240, Y: 180}
			win.Padding = 0
			win.BorderPad = 0
			win.TitleHeight = 0
			root := ctx.NewColumn()
			root.Fixed = true
			root.Scrollable = true
			root.Size = Point{X: 200, Y: 80}
			var first, last *ItemData
			for i := 0; i < 12; i++ {
				b := ctx.NewActionButton(fmt.Sprint(i), nil)
				b.Position = Point{}
				b.Size = Point{X: 180, Y: 25}
				root.AddItem(b)
				if i == 0 {
					first = b
				}
				last = b
			}
			spacer := ctx.NewLabel("")
			spacer.Size = Point{X: 1, Y: 150}
			spacer.Position = Point{}
			win.AddItem(ctx.NewColumn(spacer, root))
			win.Refresh()
			ctx.Focus(last)
			if root.Scroll.Y <= 0 || win.Scroll.Y <= 0 || ctx.FocusedItem() != last {
				t.Fatalf("scale %v: nested focus not revealed: flow=%v win=%v", scale, root.Scroll, win.Scroll)
			}
			ctx.Focus(first)
			if root.Scroll.Y != 0 {
				t.Fatalf("scale %v: reverse traversal did not reveal first item: %v", scale, root.Scroll)
			}
		})
	}
}

func TestKeyboardContextMenuKeepsHostFocusAndFollowUpMenu(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	oldMenus := ctx.contextMenus
	ctx.contextMenus = nil
	defer func() { ctx.contextMenus = oldMenus }()
	input, _ := ctx.NewInput()
	win.AddItem(input)
	ctx.Focus(input)
	calls := 0
	menu := ctx.ShowContextMenu([]string{"Heading", "Open", "Cancel"}, 20, 30, func(index int) {
		calls++
		if index != 1 {
			t.Fatal("wrong menu option")
		}
		ctx.ShowContextMenu([]string{"Follow up"}, 40, 50, nil)
	})
	menu.HeaderCount = 1
	ctx.key(t, menu, ebiten.KeyArrowDown)
	if menu.HoverIndex != 1 || !menu.keyboardMenu || ctx.FocusedItem() != input {
		t.Fatal("menu navigation lost focus or selected header")
	}
	// Stationary pointer hover must not replace keyboard selection.
	ctx.handleContextMenus(Point{X: menu.DrawRect.X0 + 2, Y: menu.DrawRect.Y0 + 2}, false)
	if menu.HoverIndex != 1 {
		t.Fatal("stationary pointer replaced keyboard option")
	}
	ctx.key(t, menu, ebiten.KeyEnter)
	if calls != 1 || len(ctx.contextMenus) != 1 || ctx.activeContextMenu() == menu || ctx.FocusedItem() != input {
		t.Fatal("follow-up menu was discarded or host focus changed")
	}
	ctx.key(t, ctx.activeContextMenu(), ebiten.KeyEscape)
	if ctx.ContextMenusOpen() {
		t.Fatal("Escape left a context menu open")
	}
}

func TestF6CyclesInteractiveWindowsWithoutZOrderStarvation(t *testing.T) {
	ctx := newTestContext(t)

	first := ctx.keyboardFixture(t)
	first.AddItem(ctx.NewActionButton("First", nil))
	second := ctx.NewWindow()
	second.AddItem(ctx.NewActionButton("Second", nil))
	second.MarkOpen()
	third := ctx.NewWindow()
	third.AddItem(ctx.NewActionButton("Third", nil))
	third.MarkOpen()
	empty := ctx.NewWindow()
	empty.MarkOpen()
	closed := ctx.NewWindow()
	closed.AddItem(ctx.NewActionButton("Closed", nil))
	closed.MarkOpen()
	closed.Close()
	third.BringForward()
	for i, want := range []*WindowData{first, second, third, first, second, third} {
		if !ctx.focusNextWindow(false) || ctx.activeWindow != want || ctx.FocusedItem() == nil || ctx.FocusedItem().ParentWindow != want {
			t.Fatalf("F6 step %d failed", i)
		}
	}
	for i, want := range []*WindowData{second, first, third} {
		if !ctx.focusNextWindow(true) || ctx.activeWindow != want {
			t.Fatalf("Shift+F6 step %d failed", i)
		}
	}
}

func TestFocusRevealsHorizontalAndTabbedScrollers(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.Size = Point{X: 450, Y: 260}
	row := ctx.NewRow()
	row.Fixed = true
	row.Scrollable = true
	row.Size = Point{X: 220, Y: 60}
	var last *ItemData
	for i := 0; i < 8; i++ {
		last = ctx.NewActionButton(fmt.Sprint(i), nil)
		last.Position = Point{}
		row.AddItem(last)
	}
	page := ctx.NewColumn(row)
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 300, Y: 150}
	tabs.Tabs = []*ItemData{page}
	win.AddItem(tabs)
	win.Refresh()
	ctx.Focus(last)
	if row.Scroll.X <= 0 {
		t.Fatal("horizontal scroller did not reveal offscreen control")
	}
	before := row.Scroll.X
	ctx.Focus(tabs)
	if row.Scroll.X != before || tabs.Scroll.Y != 0 {
		t.Fatal("focusing tab strip disturbed content scroll")
	}
}

func TestSliderReleaseCallbackCanRetireFocusTarget(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	slider, _ := ctx.NewSlider()
	button := ctx.NewActionButton("Next", nil)
	win.AddItem(ctx.NewColumn(slider, button))
	var newInput *ItemData
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			win.Close()
			next := ctx.NewWindow()
			newInput, _ = ctx.NewInput()
			next.AddItem(newInput)
			next.MarkOpen()
			ctx.Focus(newInput)
		}
	}
	ctx.Focus(slider)
	ctx.key(t, slider, ebiten.KeyArrowRight)
	ctx.Focus(button)
	if win.Open || ctx.FocusedItem() != newInput || ctx.activeWindow == win {
		t.Fatal("focus transition revived target retired by release callback")
	}
}

func TestKeyboardColorWheelAndTextShortcutBoundary(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	wheel, _ := ctx.NewColorWheel()
	wheel.WheelColor = NewColorHSV(120, .5, .5, 1)
	input, _ := ctx.NewInput()
	win.AddItem(ctx.NewColumn(wheel, input))
	events, callbacks := 0, 0
	wheel.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventColorChanged {
			events++
		}
	}
	wheel.OnColorChange = func(Color) { callbacks++ }
	ctx.Focus(wheel)
	ctx.key(t, wheel, ebiten.KeyArrowRight)
	ctx.key(t, wheel, ebiten.KeyArrowUp)
	h, s, _, _ := wheel.WheelColor.HSVA()
	if h <= 120 || s <= .5 || events != 2 || callbacks != 2 {
		t.Fatal("color wheel keys did not update state and callbacks")
	}
	ctx.Focus(input)
	if ctx.FocusedTextInput() != input || ctx.controlKey(input, ebiten.KeyArrowRight, false, inputkeys.Modifiers{}) {
		t.Fatal("general navigation intercepted editor arrows")
	}
}

func TestFocusFeedbackBypassesBackgroundRefreshCoalescing(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	button := ctx.NewActionButton("Focus", nil)
	win.AddItem(button)
	win.reusableRenderTarget(500, 400)
	win.lastRefresh = time.Now()
	win.SetRefreshInterval(time.Hour)
	win.Dirty = false
	ctx.Focus(button)
	if !win.Dirty || win.refreshPending {
		t.Fatal("focus indicator waited for background refresh interval")
	}
}

func TestPinnedFocusDoesNotScrollItsFlow(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	flow := ctx.NewColumn()
	flow.Fixed = true
	flow.Scrollable = true
	flow.Size = Point{X: 300, Y: 80}
	for i := 0; i < 10; i++ {
		flow.AddItem(ctx.NewActionButton(fmt.Sprint(i), nil))
	}
	pinned := ctx.NewActionButton("Pinned", nil)
	pinned.PinTo = PIN_BOTTOM_LEFT
	pinned.Position = Point{X: 10, Y: 30}
	flow.AddItem(pinned)
	win.AddItem(flow)
	win.Refresh()
	flow.Scroll.Y = 30
	ctx.Focus(pinned)
	if flow.Scroll.Y != 30 || win.Scroll != (Point{}) {
		t.Fatal("pinned control incorrectly moved ancestor scrolling")
	}
}

func TestKeyboardSliderPrecisionAndTabEndpoints(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	slider, _ := ctx.NewSlider()
	slider.MinValue = 0
	slider.MaxValue = 100
	slider.Value = 50
	tabs := ctx.NewColumn()
	tabs.Tabs = []*ItemData{ctx.NewColumn(), ctx.NewColumn(), ctx.NewColumn(), ctx.NewColumn()}
	for i, tab := range tabs.Tabs {
		tab.Name = fmt.Sprint(i)
	}
	tabs.Tabs[1].Disabled = true
	tabs.Tabs[2].Invisible = true
	win.AddItem(ctx.NewColumn(slider, tabs))
	ctx.Focus(slider)
	ctx.controlKey(slider, ebiten.KeyArrowRight, true, inputkeys.Modifiers{})
	if slider.Value != 60 {
		t.Fatal("Shift did not take a larger step")
	}
	ctx.controlKey(slider, ebiten.KeyArrowLeft, false, inputkeys.Modifiers{Control: true})
	if slider.Value < 59.89 || slider.Value > 59.91 {
		t.Fatal("Control did not take a fine step")
	}
	ctx.key(t, slider, ebiten.KeyHome)
	if slider.Value != 0 {
		t.Fatal("Home missed lower endpoint")
	}
	ctx.key(t, slider, ebiten.KeyEnd)
	if slider.Value != 100 {
		t.Fatal("End missed upper endpoint")
	}
	ctx.Focus(tabs)
	ctx.key(t, tabs, ebiten.KeyArrowRight)
	if tabs.ActiveTab != 3 {
		t.Fatal("tabs did not skip disabled and hidden pages")
	}
	ctx.key(t, tabs, ebiten.KeyArrowRight)
	if tabs.ActiveTab != 0 {
		t.Fatal("tab arrows did not wrap")
	}
	ctx.key(t, tabs, ebiten.KeyEnd)
	if tabs.ActiveTab != 3 {
		t.Fatal("End missed last enabled tab")
	}
	ctx.key(t, tabs, ebiten.KeyHome)
	if tabs.ActiveTab != 0 {
		t.Fatal("Home missed first enabled tab")
	}
}
