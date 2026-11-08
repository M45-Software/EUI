package eui

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	padConfirm = ebiten.StandardGamepadButtonRightBottom
	padBack    = ebiten.StandardGamepadButtonRightRight
	padLeft    = ebiten.StandardGamepadButtonLeftLeft
	padRight   = ebiten.StandardGamepadButtonLeftRight
	padUp      = ebiten.StandardGamepadButtonLeftTop
	padDown    = ebiten.StandardGamepadButtonLeftBottom
	padNext    = ebiten.StandardGamepadButtonFrontTopRight
	padPrev    = ebiten.StandardGamepadButtonFrontTopLeft
	padWindow  = ebiten.StandardGamepadButtonCenterRight
)

func (d *interactionDriver) pad(buttons ...ebiten.StandardGamepadButton) {
	d.t.Helper()
	d.frame.Keys = nil
	d.frame.MouseButtons = nil
	d.frame.Gamepads = []GamepadInput{{ID: 0, Buttons: buttons}}
	d.step()
}
func (d *interactionDriver) padTap(button ebiten.StandardGamepadButton) {
	d.t.Helper()
	d.pad(button)
	d.pad()
}

func TestGamepadSpatialFocusConfirmAndPrompts(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	clicks := 0
	a := ctx.NewActionButton("A", func() { clicks++ })
	b := ctx.NewActionButton("B", func() { clicks++ })
	c := ctx.NewActionButton("C", nil)
	e := ctx.NewActionButton("D", nil)
	win.AddItem(ctx.NewColumn(ctx.NewRow(a, b), ctx.NewRow(c, e)))
	ctx.SetGamepadNavigation(true)
	d.pad()
	ctx.Focus(a)
	d.padTap(padRight)
	if ctx.FocusedItem() != b {
		t.Fatal("right did not use row geometry")
	}
	d.padTap(padDown)
	if ctx.FocusedItem() != e {
		t.Fatal("down did not use column geometry")
	}
	d.padTap(padLeft)
	if ctx.FocusedItem() != c {
		t.Fatal("left did not use row geometry")
	}
	d.padTap(padUp)
	if ctx.FocusedItem() != a {
		t.Fatal("up did not use column geometry")
	}
	d.pad(padConfirm)
	for i := 0; i < 70; i++ {
		d.step()
	}
	if clicks != 1 || !ctx.GamepadInputHandled() {
		t.Fatal("held confirm repeated or was not claimed")
	}
	d.pad()
	prompts := ctx.NavigationPrompts()
	if prompts[0].Label != "D-pad / left stick" {
		t.Fatal("gamepad prompts not selected")
	}
	prompts[0].Label = "corrupt"
	if ctx.NavigationPrompts()[0].Label == "corrupt" {
		t.Fatal("prompts were aliased")
	}
	d.keys(ebiten.KeyTab)
	d.keys()
	if ctx.NavigationPrompts()[0].Label == "D-pad / left stick" {
		t.Fatal("keyboard did not restore prompts")
	}
	b.SetDisabled(true)
	ctx.Focus(a)
	d.padTap(padRight)
	if ctx.FocusedItem() == b {
		t.Fatal("disabled control received spatial focus")
	}
}

func TestGamepadStableOwnershipAndHotPlugNeutrality(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	n := 0
	button := ctx.NewActionButton("OK", func() { n++ })
	win.AddItem(button)
	ctx.Focus(button)
	ctx.SetGamepadNavigation(true)
	d.pad(padConfirm)
	if n != 0 {
		t.Fatal("hot-plug held confirm activated")
	}
	d.pad()
	d.padTap(padConfirm)
	if n != 1 {
		t.Fatal("fresh confirm was lost")
	}
	d.frame.Gamepads = []GamepadInput{{ID: 0}, {ID: 1, Buttons: []ebiten.StandardGamepadButton{padConfirm}}}
	d.step()
	if id, ok := ctx.ActiveGamepad(); !ok || id != 0 || n != 1 {
		t.Fatal("other controller stole ownership")
	}
	d.frame.Gamepads = d.frame.Gamepads[1:]
	d.step()
	if n != 1 {
		t.Fatal("held input activated on owner disconnect")
	}
	d.frame.Gamepads = []GamepadInput{{ID: 1}}
	d.step()
	d.frame.Gamepads[0].Buttons = []ebiten.StandardGamepadButton{padConfirm}
	d.step()
	if id, ok := ctx.ActiveGamepad(); !ok || id != 1 || n != 2 {
		t.Fatal("new controller did not recover after neutral")
	}
	ctx.SetInputSource(ctx.inputSource)
	d.ticks = d.reads // Source resetting does not sample.
	d.step()
	if n != 2 {
		t.Fatal("source handoff replayed held confirm")
	}
	d.frame.Gamepads = nil
	d.step()
	if _, ok := ctx.ActiveGamepad(); ok {
		t.Fatal("disconnected owner was retained")
	}
	// Two controllers acquire simultaneously: choose the lowest ID, independent of order.
	d.frame.Gamepads = []GamepadInput{{ID: 8}, {ID: 2}}
	d.step()
	d.frame.Gamepads[0].Buttons = []ebiten.StandardGamepadButton{padNext}
	d.frame.Gamepads[1].Buttons = []ebiten.StandardGamepadButton{padConfirm}
	d.step()
	if id, _ := ctx.ActiveGamepad(); id != 2 || n != 3 {
		t.Fatal("simultaneous ownership was not deterministic")
	}
}

func TestGamepadStickDeadzoneHysteresisAndRepeat(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	slider, _ := ctx.NewSlider()
	slider.MinValue, slider.MaxValue = 0, 100
	slider.Value = 50
	win.AddItem(slider)
	ctx.Focus(slider)
	ctx.SetGamepadNavigation(true)
	d.pad()
	d.frame.Gamepads[0].LeftX = .44
	d.step()
	if slider.Value != 50 {
		t.Fatal("subthreshold stick adjusted a control")
	}
	if _, owned := ctx.ActiveGamepad(); owned {
		t.Fatal("subthreshold drift acquired a controller")
	}
	d.pad()
	d.padTap(padConfirm)
	if ctx.NavigationPrompts()[0].Action != "Adjust" {
		t.Fatal("adjustment prompt missing")
	}
	releases, changes := 0, 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			releases++
		}
		if ev.Type == EventSliderChanged {
			changes++
		}
	}
	d.pad(padRight)
	for i := 0; i < 23; i++ {
		d.step()
	}
	if slider.Value != 51 || releases != 0 {
		t.Fatal("repeat started too early or slider released during hold")
	}
	d.step()
	if slider.Value != 52 {
		t.Fatal("repeat did not begin after delay")
	}
	for i := 0; i < 6; i++ {
		d.step()
	}
	if slider.Value != 53 {
		t.Fatal("repeat interval incorrect")
	}
	d.pad()
	if releases != 1 || changes != 3 {
		t.Fatal("slider did not release exactly once")
	}
	d.frame.Gamepads[0].LeftX = .6
	d.step()
	if slider.Value != 54 {
		t.Fatal("stick did not engage")
	}
	d.frame.Gamepads[0].LeftX = .35
	d.step()
	if ctx.gamepad.direction != 2 {
		t.Fatal("stick hysteresis released early")
	}
	d.frame.Gamepads[0].LeftX = -.35
	d.step()
	if ctx.gamepad.direction != 0 {
		t.Fatal("opposite subthreshold stick engaged")
	}
	d.frame.Gamepads[0].LeftX = .3
	d.step()
	if ctx.gamepad.direction != 0 {
		t.Fatal("stick did not release at deadzone")
	}
	d.padTap(padBack)
	if ctx.gamepad.editing != nil || ctx.ModalWindow() != nil {
		t.Fatal("Back did not leave adjustment")
	}
	ctx.Focus(slider)
	d.padTap(padRight)
	if slider.Value != 54 {
		t.Fatal("navigation adjusted slider outside adjustment mode")
	}
}

func TestGamepadCaptureFocusAndEnableHandoffs(t *testing.T) {
	for _, mode := range []string{"capture", "unfocused", "disabled", "navigation off"} {
		t.Run(mode, func(t *testing.T) {
			ctx := newTestContext(t)
			win, d, _ := ctx.interactionFixture(t)
			n := 0
			b := ctx.NewActionButton("OK", func() { n++ })
			win.AddItem(b)
			ctx.Focus(b)
			ctx.SetGamepadNavigation(true)
			d.pad()
			d.padTap(padConfirm)
			switch mode {
			case "capture":
				ctx.SetKeyboardInputCaptured(true)
			case "unfocused":
				d.frame.Unfocused = true
			case "disabled":
				ctx.SetInputEnabled(false)
			case "navigation off":
				ctx.SetGamepadNavigation(false)
			}
			d.pad(padConfirm)
			if n != 1 || ctx.GamepadInputHandled() {
				t.Fatal("host-owned pad input reached UI")
			}
			switch mode {
			case "capture":
				ctx.SetKeyboardInputCaptured(false)
			case "unfocused":
				d.frame.Unfocused = false
			case "disabled":
				ctx.SetInputEnabled(true)
			case "navigation off":
				ctx.SetGamepadNavigation(true)
			}
			d.step()
			if n != 1 {
				t.Fatal("held confirm replayed on handoff")
			}
			d.pad()
			d.padTap(padConfirm)
			if n != 2 {
				t.Fatal("neutral handoff did not recover")
			}
		})
	}
}

func TestGamepadModalMenusEditorAndNestedOpening(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	ctx.SetGamepadNavigation(true)
	d.pad()
	accepted, cancelled := 0, 0
	input, _ := ctx.NewInput()
	input.UpdateText("draft")
	dd, _ := ctx.NewDropdown()
	dd.SetOptions([]string{"one", "two"})
	win := ctx.NewWindow()
	win.AutoSize, win.Closable = true, true
	accept := ctx.NewActionButton("Apply", func() { accepted++; win.Close() })
	cancel := ctx.NewActionButton("Cancel", func() { cancelled++; win.Close() })
	win.DefaultButton, win.CancelButton = accept, cancel
	win.AddItem(ctx.NewColumn(input, dd, ctx.NewRow(accept, cancel)))
	win.MarkOpenModal()
	d.pad()
	ctx.Focus(input)
	d.padTap(padDown)
	if input.Text != "draft" || ctx.FocusedItem() != dd {
		t.Fatal("gamepad mutated editor instead of moving focus")
	}
	d.padTap(padConfirm)
	if !dd.Open {
		t.Fatal("confirm did not open dropdown")
	}
	d.padTap(padDown)
	d.padTap(padConfirm)
	if dd.Selected != 1 || dd.Open || accepted != 0 {
		t.Fatal("dropdown selection accepted modal")
	}
	d.padTap(padConfirm)
	d.padTap(padBack)
	if dd.Open || !win.Open || cancelled != 0 {
		t.Fatal("Back skipped open dropdown")
	}
	ctx.Focus(input)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	d.padTap(padBack)
	if input.IsComposing() || input.Text != "draft" || !win.Open {
		t.Fatal("Back did not cancel only composition")
	}
	d.padTap(padBack)
	if win.Open || cancelled != 1 || accepted != 0 {
		t.Fatal("Back did not call modal Cancel")
	}
	base.Searchable = true
	base.OpenSearch()
	d.padTap(padBack)
	if ctx.activeSearch != nil {
		t.Fatal("Back did not dismiss search")
	}
	// An opening confirm must not accept the child, even across held-repeat ticks.
	var child *WindowData
	opener := ctx.NewActionButton("Open", func() {
		child = ctx.ShowModal("Child", "", []PopupButton{{Text: "Accept", Action: func() { accepted++ }}})
	})
	base.AddItem(opener)
	ctx.Focus(opener)
	d.pad(padConfirm)
	for i := 0; i < 40; i++ {
		d.step()
	}
	if child == nil || !child.Open || accepted != 0 {
		t.Fatal("held opening confirm reached child")
	}
	d.pad()
	d.padTap(padConfirm)
	if child.Open || accepted != 1 {
		t.Fatal("fresh child confirm failed")
	}
}

func TestGamepadReplayValidationAndContextIsolation(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	n := 0
	b := ctx.NewActionButton("OK", func() { n++ })
	win.AddItem(b)
	ctx.Focus(b)
	ctx.SetGamepadNavigation(true)
	d.pad()
	prior := ctx.sampledInput.now
	frame := d.frame
	frame.Gamepads = []GamepadInput{{ID: 0, LeftX: math.NaN()}}
	ctx.inputSource = func() InputFrame { return frame }
	if err := ctx.Update(); err == nil || ctx.sampledInput.now != prior || n != 0 {
		t.Fatal("invalid axis partially mutated input")
	}
	ctx.inputSource = func() InputFrame { d.reads++; return d.frame }
	d.frame.Gamepads = []GamepadInput{{ID: -1, Buttons: []ebiten.StandardGamepadButton{padConfirm}}, {ID: 0}, {ID: 0, Buttons: []ebiten.StandardGamepadButton{padConfirm}}}
	d.step()
	if n != 0 {
		t.Fatal("invalid/duplicate IDs activated")
	}
	other := newTestContext(t)
	otherWin, otherDriver, _ := other.interactionFixture(t)
	m := 0
	otherButton := other.NewActionButton("Other", func() { m++ })
	otherWin.AddItem(otherButton)
	other.Focus(otherButton)
	other.SetGamepadNavigation(true)
	otherDriver.pad()
	other.SetInputEnabled(false)
	d.padTap(padConfirm)
	otherDriver.pad(padConfirm)
	if n != 1 || m != 0 {
		t.Fatal("pad input leaked across contexts")
	}
}

func TestGamepadTabsRadioMenusAndCancelCallback(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	tabs := ctx.NewColumn()
	tabs.Tabs = []*ItemData{ctx.NewColumn(ctx.NewLabel("First")), ctx.NewColumn(ctx.NewLabel("Second"))}
	tabs.Tabs[0].Name, tabs.Tabs[1].Name = "First", "Second"
	radio, _ := ctx.NewRadio()
	radio.Text, radio.RadioGroup = "One", "group"
	other, _ := ctx.NewRadio()
	other.Text, other.RadioGroup = "Two", "group"
	win.AddItem(ctx.NewColumn(tabs, ctx.NewRow(radio, other)))
	ctx.SetGamepadNavigation(true)
	d.pad()
	ctx.Focus(tabs)
	d.padTap(padConfirm)
	d.padTap(padRight)
	if tabs.ActiveTab != 1 {
		t.Fatal("adjustment did not switch tab")
	}
	d.padTap(padBack)
	if ctx.gamepad.editing != nil {
		t.Fatal("Back did not finish tab adjustment")
	}
	ctx.Focus(radio)
	d.padTap(padConfirm)
	d.padTap(padRight)
	if ctx.FocusedItem() != other || !radio.Checked || other.Checked {
		t.Fatal("radio navigation changed value before confirm")
	}
	d.padTap(padConfirm)
	if radio.Checked || !other.Checked {
		t.Fatal("radio confirm did not select peer")
	}
	selected := -1
	menu := ctx.ShowContextMenu([]string{"First", "Second"}, 100, 100, func(index int) { selected = index })
	d.padTap(padDown)
	d.padTap(padDown)
	d.padTap(padConfirm)
	if selected != 1 || menu.Open {
		t.Fatal("context menu did not navigate/confirm")
	}
	back := 0
	ctx.GamepadCancelHandler = func() { back++ }
	d.pad(padBack)
	for i := 0; i < 40; i++ {
		d.step()
	}
	d.pad()
	if back != 1 {
		t.Fatal("unclaimed Back callback repeated")
	}
}

func TestGamepadReleaseCallbackCannotNavigateItsNewModal(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	slider, _ := ctx.NewSlider()
	slider.MinValue, slider.MaxValue = 0, 100
	win.AddItem(ctx.NewColumn(slider, ctx.NewActionButton("Other", nil)))
	ctx.SetGamepadNavigation(true)
	d.pad()
	ctx.Focus(slider)
	d.padTap(padConfirm)
	var modal *WindowData
	first := ctx.NewActionButton("First", nil)
	second := ctx.NewActionButton("Second", nil)
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			modal = ctx.testModal(t, ctx.NewColumn(first, second))
		}
	}
	d.pad(padRight)
	d.pad(padNext) // Release adjustment and request traversal in the same tick.
	if modal == nil || ctx.FocusedItem() != first {
		t.Fatal("release/traversal packet moved focus inside new modal")
	}
	d.pad()
	d.padTap(padWindow)
	if ctx.FocusedItem() != second || ctx.activeWindow != modal {
		t.Fatal("window cycle escaped modal")
	}
}

func TestGamepadSpatialNavigationRevealsUndrawnControls(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	var buttons []*ItemData
	for i := 0; i < 20; i++ {
		buttons = append(buttons, ctx.NewActionButton("Row", nil))
		buttons[i].Size = Point{X: 140, Y: 30}
	}
	flow := ctx.NewColumn(buttons...)
	flow.Fixed, flow.Scrollable = true, true
	flow.Size = Point{X: 180, Y: 100}
	win.AddItem(flow)
	ctx.SetGamepadNavigation(true)
	d.pad()
	ctx.Focus(buttons[0])
	for i := 1; i < len(buttons); i++ {
		d.padTap(padDown)
		if ctx.FocusedItem() != buttons[i] {
			t.Fatalf("offscreen navigation stopped at %d", i)
		}
	}
	if flow.Scroll.Y <= 0 || buttons[19].DrawRect.Y1 > flow.DrawRect.Y1+1 {
		t.Fatal("spatial focus did not reveal clipped control")
	}
}
