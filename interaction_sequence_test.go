package eui

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type memoryClipboard struct {
	text              string
	readErr, writeErr error
	reads, writes     int
}

func (m *memoryClipboard) ReadText() (string, error) { m.reads++; return m.text, m.readErr }
func (m *memoryClipboard) WriteText(value string) error {
	m.writes++
	if m.writeErr == nil {
		m.text = value
	}
	return m.writeErr
}

type interactionDriver struct {
	ctx *Context

	t            *testing.T
	frame        InputFrame
	reads, ticks int
	screen       *ebiten.Image
}

func (ctx *Context) interactionFixture(t *testing.T) (*WindowData, *interactionDriver, *memoryClipboard) {
	t.Helper()
	win := ctx.keyboardFixture(t)
	ctx.activeWindow = nil
	previousCapture := ctx.keyboardInputCaptured
	ctx.keyboardInputCaptured = false
	win.Position = Point{X: 20, Y: 20}
	win.Movable = false
	win.Resizable = false
	d := &interactionDriver{ctx: ctx, t: t, frame: InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}, screen: ebiten.NewImage(1024, 900)}
	previous := ctx.SetInputSource(func() InputFrame { d.reads++; return d.frame })
	clip := &memoryClipboard{}
	previousClip := ctx.SetClipboard(clip)
	t.Cleanup(func() {
		if !ctx.IsDisposed() {
			ctx.SetInputSource(previous)
			ctx.keyboardInputCaptured = previousCapture
			ctx.SetClipboard(previousClip)
		}
		d.screen.Deallocate()
	})
	return win, d, clip
}
func (d *interactionDriver) step() {
	ctx := d.ctx

	d.t.Helper()
	d.frame.Time = d.frame.Time.Add(time.Second / 60)
	if err := ctx.Update(); err != nil {
		d.t.Fatal(err)
	}
	d.ticks++
	if d.reads != d.ticks {
		d.t.Fatal("source not sampled once per Update")
	}
	d.screen.Clear()
	ctx.Draw(d.screen)
	if d.reads != d.ticks {
		d.t.Fatal("Draw sampled another input frame")
	}
	d.frame.Text = ""
	d.frame.TextInput = nil
	d.frame.WheelX, d.frame.WheelY = 0, 0
}
func (d *interactionDriver) keys(keys ...ebiten.Key) {
	d.t.Helper()
	d.frame.Keys = keys
	d.frame.MouseButtons = nil
	d.step()
}
func (d *interactionDriver) move(p Point, held bool) {
	d.t.Helper()
	d.frame.PointerX, d.frame.PointerY = int(p.X), int(p.Y)
	d.frame.Keys = nil
	d.frame.MouseButtons = nil
	if held {
		d.frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButton0}
	}
	d.step()
}
func itemCenter(item *ItemData) Point {
	return Point{X: (item.DrawRect.X0 + item.DrawRect.X1) / 2, Y: (item.DrawRect.Y0 + item.DrawRect.Y1) / 2}
}
func (d *interactionDriver) click(item *ItemData) {
	d.t.Helper()
	p := itemCenter(item)
	d.move(p, true)
	d.move(p, false)
}

func TestInteractionPointerActivationAndSliderRelease(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	check, _ := ctx.NewCheckbox()
	check.Text = "Enabled"
	check.Position = Point{}
	slider, _ := ctx.NewSlider()
	slider.Position = Point{}
	slider.Size = Point{X: 300, Y: 30}
	slider.HideValue = true
	slider.SetRange(0, 100)
	win.AddItem(ctx.NewColumn(check, slider))
	var events []UIEvent
	check.Handler.Handle = func(ev UIEvent) { events = append(events, ev) }
	slider.Handler.Handle = func(ev UIEvent) { events = append(events, ev) }
	d.step()
	d.click(check)
	if !check.Checked || len(events) != 1 || events[0].Type != EventCheckboxChanged {
		t.Fatal("press/release did not toggle checkbox exactly once")
	}
	if ctx.PointerPressWindow() != nil || ctx.PointerPressHandled() {
		t.Fatal("release retained press ownership")
	}
	start := Point{X: slider.DrawRect.X0 + 10, Y: slider.DrawRect.Y0 + 15}
	d.move(start, true)
	if ctx.PointerPressWindow() != win || !ctx.PointerPressHandled() {
		t.Fatal("slider press did not claim its window")
	}
	d.move(Point{X: slider.DrawRect.X0 + 230, Y: start.Y}, true)
	if slider.Value < 70 {
		t.Fatal("held pointer did not change slider")
	}
	d.move(Point{X: 900, Y: 800}, true)
	if slider.Value != 100 {
		t.Fatal("captured slider stopped updating outside its window")
	}
	d.move(Point{X: 900, Y: 800}, false)
	if len(events) < 3 || events[len(events)-1].Type != EventSliderReleased {
		t.Fatalf("missing completed slider release: %v", events)
	}
	n := len(events)
	d.step()
	if len(events) != n || ctx.activeItem != nil || ctx.downWin != nil {
		t.Fatal("release repeated or capture remained active")
	}
}

func TestInteractionKeyboardTraversalRepeatAndCapture(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	check, _ := ctx.NewCheckbox()
	check.Position = Point{}
	slider, _ := ctx.NewSlider()
	slider.Position = Point{}
	slider.SetRange(0, 100)
	slider.SetValue(0)
	blocked := ctx.NewActionButton("Unavailable", nil)
	blocked.Disabled = true
	dd, _ := ctx.NewDropdown()
	dd.SetOptions([]string{"One", "Two", "Three"})
	dd.Position = Point{}
	win.AddItem(ctx.NewColumn(check, blocked, slider, dd))
	d.step()
	d.keys(ebiten.KeyTab)
	if ctx.FocusedItem() != check {
		t.Fatalf("Tab did not focus first control: focus=%p expected=%p activeWin=%p win=%p captured=%v tabTicks=%d context=%d search=%p", ctx.FocusedItem(), check, ctx.activeWindow, win, ctx.keyboardInputCaptured, ctx.keyPressDuration(ebiten.KeyTab), len(ctx.contextMenus), ctx.activeSearch)
	}
	d.keys()
	d.keys(ebiten.KeySpace)
	d.keys()
	if !check.Checked {
		t.Fatal("Space did not activate checkbox")
	}
	d.keys(ebiten.KeyTab)
	d.keys()
	if ctx.FocusedItem() != slider {
		t.Fatal("Tab did not skip disabled control")
	}
	changes, releases := 0, 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderChanged {
			changes++
		}
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	for range 30 {
		d.keys(ebiten.KeyArrowRight)
	}
	if changes != 3 || slider.Value != 3 || releases != 0 {
		t.Fatalf("held key repeat: value=%g changes=%d releases=%d", slider.Value, changes, releases)
	}
	d.keys()
	if releases != 1 {
		t.Fatal("key release did not finish slider transaction once")
	}
	d.keys(ebiten.KeyTab)
	d.keys()
	if ctx.FocusedItem() != dd {
		t.Fatal("Tab did not reach dropdown")
	}
	d.keys(ebiten.KeyEnter)
	d.keys()
	d.keys(ebiten.KeyArrowDown)
	d.keys()
	d.keys(ebiten.KeyEnter)
	d.keys()
	if dd.Selected != 1 || dd.Open {
		t.Fatal("keyboard dropdown sequence did not commit")
	}
	ctx.SetKeyboardInputCaptured(true)
	t.Cleanup(func() { ctx.SetKeyboardInputCaptured(false) })
	d.keys(ebiten.KeyTab, ebiten.KeyEnter)
	if ctx.FocusedItem() != dd || dd.Open {
		t.Fatal("captured host keyboard affected UI")
	}
	d.keys()
	d.click(check)
	if check.Checked {
		t.Fatal("host keyboard capture disabled pointer controls")
	}
}

func TestInteractionEditorTypingSelectionClipboardUndo(t *testing.T) {
	ctx := newTestContext(t)

	for _, mac := range []bool{false, true} {
		t.Run(map[bool]string{false: "Control", true: "Command"}[mac], func(t *testing.T) {
			win, d, clip := ctx.interactionFixture(t)
			d.frame.Mac = mac
			editor, _ := ctx.NewTextArea()
			editor.Position = Point{}
			editor.Size = Point{X: 350, Y: 150}
			editor.AcceptTab = true
			win.AddItem(editor)
			d.step()
			d.click(editor)
			d.frame.Text = "Hello 世界"
			d.step()
			if editor.Text != "Hello 世界" {
				t.Fatal("typed Unicode text not inserted through Update")
			}
			shortcut := ebiten.KeyControlLeft
			if mac {
				shortcut = ebiten.KeyMetaRight
			}
			d.keys(shortcut, ebiten.KeyA)
			d.keys()
			if editor.SelectedText() != editor.Text {
				t.Fatal("shortcut did not select all")
			}
			d.keys(shortcut, ebiten.KeyC)
			d.keys()
			if clip.text != "Hello 世界" || clip.writes != 1 {
				t.Fatal("copy did not use injected clipboard")
			}
			clip.writeErr = errors.New("clipboard unavailable")
			d.keys(shortcut, ebiten.KeyX)
			d.keys()
			if editor.Text != "Hello 世界" {
				t.Fatal("failed cut deleted text")
			}
			clip.writeErr = nil
			d.keys(shortcut, ebiten.KeyX)
			d.keys()
			if editor.Text != "" || clip.text != "Hello 世界" {
				t.Fatal("cut did not commit clipboard before deleting")
			}
			clip.text = "Replacement\nline"
			d.keys(shortcut, ebiten.KeyV)
			d.keys()
			if editor.Text != clip.text {
				t.Fatal("paste did not use injected clipboard")
			}
			d.keys(shortcut, ebiten.KeyZ)
			d.keys()
			if editor.Text != "" {
				t.Fatal("undo did not restore cut state")
			}
			d.keys(shortcut, ebiten.KeyShiftLeft, ebiten.KeyZ)
			d.keys()
			if editor.Text != "Replacement\nline" {
				t.Fatal("redo did not restore paste")
			}
			d.keys(ebiten.KeyArrowLeft, ebiten.KeyShiftRight)
			d.keys()
			if editor.SelectedText() != "e" {
				t.Fatal("shift navigation did not select grapheme")
			}
			d.frame.Text = "!"
			d.step()
			if editor.Text != "Replacement\nlin!" {
				t.Fatal("typing did not replace selection")
			}
			d.keys(ebiten.KeyTab)
			d.keys()
			if !strings.ContainsRune(editor.Text, '\t') || ctx.FocusedItem() != editor {
				t.Fatal("editor Tab escaped despite AcceptTab")
			}
		})
	}
}

func TestInteractionDoubleClickTimingCaretAndClickExpiry(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	editor, _ := ctx.NewInput()
	editor.Position = Point{}
	editor.Size = Point{X: 300, Y: 32}
	editor.UpdateText("alpha beta")
	button := ctx.NewActionButton("Click", nil)
	button.Position = Point{}
	win.AddItem(ctx.NewColumn(editor, button))
	d.step()
	p := Point{X: editor.DrawRect.X0 + 12, Y: editor.DrawRect.Y0 + 15}
	d.move(p, true)
	d.move(p, false)
	d.move(p, true)
	d.move(p, false)
	if editor.SelectedText() != "alpha" {
		t.Fatalf("double click did not select word: %q", editor.SelectedText())
	}
	d.frame.Time = d.frame.Time.Add(time.Second)
	d.move(p, true)
	d.move(p, false)
	if editor.SelectStart != editor.SelectEnd || editor.textEdit.clickCount != 1 {
		t.Fatal("late click incorrectly joined double-click sequence")
	}
	d.frame.Text = "x"
	d.step()
	if !editor.textEdit.caretOn {
		t.Fatal("typing did not reset caret")
	}
	d.frame.Time = d.frame.Time.Add(600 * time.Millisecond)
	d.step()
	if editor.textEdit.caretOn {
		t.Fatal("caret did not follow injected clock")
	}
	d.click(button)
	if button.Clicked.IsZero() {
		t.Fatal("press omitted click flash")
	}
	d.frame.Time = d.frame.Time.Add(clickFlash + time.Millisecond)
	d.step()
	if !button.Clicked.IsZero() {
		t.Fatal("click flash did not expire on injected clock")
	}
}

func TestInteractionDropdownPreviewCancelAndContextFollowUp(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	dd, _ := ctx.NewDropdown()
	dd.Position = Point{}
	dd.Size = Point{X: 240, Y: 32}
	dd.SetOptions([]string{"One", "Two", "Three"})
	win.AddItem(dd)
	d.step()
	var previews []int
	selected := 0
	dd.OnHover = func(index int) { previews = append(previews, index) }
	dd.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventDropdownSelected {
			selected++
		}
	}
	d.click(dd)
	if !dd.Open {
		t.Fatal("click did not open dropdown")
	}
	r, _ := ctx.dropdownOpenRect(dd, Point{X: dd.DrawRect.X0, Y: dd.DrawRect.Y0})
	d.move(Point{X: r.X0 + 10, Y: r.Y0 + 1.5*ctx.dropdownOptionHeight(dd)}, false)
	if dd.HoverIndex != 1 {
		t.Fatal("pointer preview did not reach second row")
	}
	d.keys(ebiten.KeyEscape)
	d.keys()
	if dd.Open || dd.Selected != 0 || selected != 0 || len(previews) < 2 || previews[len(previews)-1] != 0 {
		t.Fatal("cancel committed selection or omitted preview restoration")
	}
	menu := ctx.ShowContextMenu([]string{"Next"}, 100, 120, nil)
	menu.OnSelect = func(int) { ctx.ShowContextMenu([]string{"Follow-up"}, 100, 120, nil) }
	d.step()
	p := Point{X: 110, Y: 120 + ctx.dropdownOptionHeight(menu)/2}
	d.move(p, true)
	d.move(p, false)
	if len(ctx.contextMenus) != 1 || ctx.contextMenus[0] == menu || !ctx.contextMenus[0].Open {
		t.Fatal("selection callback lost its follow-up menu")
	}
}

func TestInteractionWheelScrollingAndFocusReveal(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	var buttons []*ItemData
	flow := ctx.NewColumn()
	flow.Fixed = true
	flow.Scrollable = true
	flow.Size = Point{X: 300, Y: 110}
	flow.Gap = 6
	for range 10 {
		b := ctx.NewActionButton("Row", nil)
		b.Position = Point{}
		b.Size = Point{X: 200, Y: 30}
		buttons = append(buttons, b)
		flow.AddItem(b)
	}
	win.AddItem(flow)
	d.step()
	d.frame.PointerX, d.frame.PointerY = int(flow.DrawRect.X0+20), int(flow.DrawRect.Y0+20)
	d.frame.WheelY = -3
	d.step()
	if flow.Scroll.Y <= 0 {
		t.Fatal("wheel did not scroll flow")
	}
	for range 10 {
		d.keys(ebiten.KeyTab)
		d.keys()
	}
	if ctx.FocusedItem() != buttons[9] || flow.Scroll.Y <= 100 || buttons[9].DrawRect.Y1 <= buttons[9].DrawRect.Y0 {
		t.Fatal("keyboard navigation did not reveal offscreen control")
	}
}

func TestInputFrameValidationAndSourceSwitchCancelGesture(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	slider, _ := ctx.NewSlider()
	slider.Position = Point{}
	slider.HideValue = true
	slider.Size = Point{X: 300, Y: 30}
	slider.SetRange(0, 100)
	win.AddItem(slider)
	d.step()
	start := itemCenter(slider)
	releases := 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	d.move(start, true)
	d.move(Point{X: start.X + 50, Y: start.Y}, true)
	value := slider.Value
	saved := d.frame.Time
	d.frame.Time = time.Time{}
	if err := ctx.Update(); err == nil {
		t.Fatal("zero frame clock accepted")
	}
	if slider.Value != value || !ctx.pointerPressed() {
		t.Fatal("invalid frame changed interaction state")
	}
	d.frame.Time = saved.Add(-time.Hour)
	if err := ctx.Update(); err == nil {
		t.Fatal("backward frame clock accepted")
	}
	source := ctx.inputSource
	ctx.SetInputSource(source)
	d.reads, d.ticks = 0, 0
	d.frame.Time = time.Unix(10, 0)
	d.frame.MouseButtons = nil
	d.step()
	if releases != 0 || ctx.activeItem != nil || ctx.downWin != nil || slider.dragStartInit || math.Abs(float64(slider.Value-value)) > .01 {
		t.Fatal("source switch committed or retained old gesture")
	}
}

func TestInteractionTouchCaptureEndsOnceAndDoesNotClickAfterPinch(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	slider, _ := ctx.NewSlider()
	slider.Position = Point{}
	slider.HideValue = true
	slider.Size = Point{X: 300, Y: 30}
	slider.SetRange(0, 100)
	win.AddItem(slider)
	d.step()
	p := itemCenter(slider)
	releases := 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	touch := TouchPoint{ID: 7, X: int(p.X), Y: int(p.Y)}
	d.frame.Touches = []TouchPoint{touch}
	d.step()
	if slider.Value < 40 || ctx.downWin != win {
		t.Fatal("single touch did not capture slider")
	}
	touch.X += 40
	d.frame.Touches = []TouchPoint{touch}
	d.step()
	d.frame.Touches = []TouchPoint{touch, {ID: 9, X: touch.X + 40, Y: touch.Y + 40}}
	d.step()
	if releases != 1 || ctx.activeItem != nil || ctx.downWin != nil {
		t.Fatal("second finger did not finish slider transaction once")
	}
	value := slider.Value
	touch.X += 50
	d.frame.Touches = []TouchPoint{touch}
	d.step()
	if slider.Value != value || ctx.downWin != nil || releases != 1 {
		t.Fatal("pinch-to-single transition resumed capture without a new press")
	}
	d.frame.Touches = nil
	d.step()
	// Replacing a touch contact completes its old capture before the new one.
	d.frame.Touches = []TouchPoint{{ID: 12, X: int(p.X), Y: int(p.Y)}}
	d.step()
	d.frame.Touches = []TouchPoint{{ID: 14, X: int(p.X + 70), Y: int(p.Y)}}
	d.step()
	if releases != 2 || ctx.downWin != win {
		t.Fatal("contact replacement lost old release or new press ownership")
	}
	d.frame.Touches = nil
	d.step()
	if releases != 3 {
		t.Fatal("new contact release did not finish its own transaction")
	}
}

func TestInteractionWindowDragResizeDividerAndReentrantClose(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	win.Movable, win.Resizable = true, true
	button := ctx.NewActionButton("Next", nil)
	button.Position = Point{}
	win.AddItem(button)
	d.step()
	before := win.GetPos()
	bar := Point{X: before.X + 150, Y: before.Y + win.GetTitleSize()/2}
	d.move(bar, true)
	d.move(pointAdd(bar, Point{X: 50, Y: 30}), true)
	d.move(pointAdd(bar, Point{X: 50, Y: 30}), false)
	if win.GetPos() != pointAdd(before, Point{X: 50, Y: 30}) {
		t.Fatal("window drag did not follow full pointer sequence")
	}
	size := win.Size
	corner := pointAdd(win.GetPos(), win.GetSize())
	corner.X -= 1
	corner.Y -= 1
	d.move(corner, true)
	d.move(pointAdd(corner, Point{X: 40, Y: 25}), true)
	d.move(pointAdd(corner, Point{X: 40, Y: 25}), false)
	if win.Size.X <= size.X || win.Size.Y <= size.Y {
		t.Fatal("resize drag did not enlarge window")
	}
	dividerCalls := 0
	ctx.SetTileDividers([]TileDivider{{Orientation: TileDividerVertical, Position: 800, Start: 0, End: 800, OnDrag: func(float32) { dividerCalls++ }}})
	t.Cleanup(func() { ctx.SetTileDividers(nil) })
	d.move(Point{X: 800, Y: 100}, true)
	d.move(Point{X: 850, Y: 100}, true)
	if dividerCalls != 2 || ctx.activeTileDivider < 0 {
		t.Fatal("divider gesture was not captured")
	}
	d.move(Point{X: 850, Y: 100}, false)
	if ctx.activeTileDivider != -1 {
		t.Fatal("divider remained captured after release")
	}
	calls := 0
	button.Handler.Handle = func(UIEvent) {
		calls++
		win.Close()
		next := ctx.NewWindow()
		next.AddItem(ctx.NewActionButton("New window", nil))
		next.MarkOpen()
	}
	d.click(button)
	d.step()
	if calls != 1 || win.Open || ctx.PointerPressWindow() != nil || ctx.FocusedItem() != nil {
		t.Fatal("closing window inside activation retained stale interaction")
	}
}

func TestInteractionSearchWindowCycleAndRadioTabs(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	first, last := ctx.NewActionButton("First", nil), ctx.NewActionButton("Last", nil)
	radioA, _ := ctx.NewRadio()
	radioB, _ := ctx.NewRadio()
	radioA.RadioGroup = "choice"
	radioB.RadioGroup = "choice"
	radioA.Checked = true
	group := ctx.NewRow(radioA, radioB)
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 300, Y: 100}
	pageA, pageB := ctx.NewColumn(first), ctx.NewColumn(last)
	pageA.Name, pageB.Name = "A", "B"
	tabs.Tabs = []*ItemData{pageA, pageB}
	win.AddItem(ctx.NewColumn(group, tabs))
	other := ctx.NewWindow()
	other.Position = Point{X: 600, Y: 30}
	other.AddItem(ctx.NewActionButton("Other", nil))
	other.MarkOpen()
	win.BringForward()
	d.step()
	d.keys(ebiten.KeyTab)
	d.keys()
	if ctx.FocusedItem() != radioA {
		t.Fatal("radio group missing initial Tab stop")
	}
	d.keys(ebiten.KeyArrowRight)
	d.keys()
	if !radioB.Checked || radioA.Checked || ctx.FocusedItem() != radioB {
		t.Fatal("radio arrow sequence failed")
	}
	d.keys(ebiten.KeyTab)
	d.keys()
	d.keys(ebiten.KeyArrowRight)
	d.keys()
	d.keys(ebiten.KeyTab)
	d.keys()
	if tabs.ActiveTab != 1 || ctx.FocusedItem() != last {
		t.Fatal("tab strip did not switch page and traverse active controls")
	}
	d.keys(ebiten.KeyF6)
	d.keys()
	if ctx.activeWindow != other {
		t.Fatal("F6 did not cycle to next window")
	}
	d.keys(ebiten.KeyShiftLeft, ebiten.KeyF6)
	d.keys()
	if ctx.activeWindow != win {
		t.Fatal("Shift-F6 did not return to previous window")
	}
	win.Searchable = true
	var searches []string
	advances := 0
	win.OnSearch = func(value string) { searches = append(searches, value) }
	win.OnSearchNext = func(bool) { advances++ }
	d.keys(ebiten.KeyControlRight, ebiten.KeyF)
	d.keys()
	d.frame.Text = "needle"
	d.step()
	d.keys(ebiten.KeyEnter)
	d.keys()
	d.keys(ebiten.KeyEscape)
	d.keys()
	if len(searches) < 2 || searches[len(searches)-2] != "needle" || searches[len(searches)-1] != "" || advances != 1 || ctx.activeSearch != nil {
		t.Fatalf("search sequence failed: searches=%v advances=%d active=%p open=%v", searches, advances, ctx.activeSearch, win.searchOpen)
	}
}

func TestInteractionTextProducingModifiersAndPasswordClipboard(t *testing.T) {
	ctx := newTestContext(t)

	for _, tc := range []struct {
		name       string
		mac        bool
		keys       []ebiten.Key
		text, want string
	}{
		{"Mac Option", true, []ebiten.Key{ebiten.KeyAltRight}, "é", "é"},
		{"AltGr", false, []ebiten.Key{ebiten.KeyControlRight, ebiten.KeyAltRight}, "€", "€"},
		{"Shortcut suppresses text", true, []ebiten.Key{ebiten.KeyMetaLeft}, "x", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			win, d, _ := ctx.interactionFixture(t)
			input, _ := ctx.NewInput()
			input.Position = Point{}
			win.AddItem(input)
			d.step()
			d.click(input)
			d.frame.Mac = tc.mac
			d.frame.Keys = tc.keys
			d.frame.Text = tc.text
			d.step()
			if input.Text != tc.want {
				t.Fatalf("text=%q want %q", input.Text, tc.want)
			}
		})
	}
	win, d, clip := ctx.interactionFixture(t)
	password, _ := ctx.NewInput()
	password.HideText = true
	password.Position = Point{}
	win.AddItem(password)
	d.step()
	d.click(password)
	d.frame.Text = "secret"
	d.step()
	d.keys(ebiten.KeyControlLeft, ebiten.KeyA)
	d.keys()
	d.keys(ebiten.KeyControlLeft, ebiten.KeyC)
	d.keys()
	d.keys(ebiten.KeyControlLeft, ebiten.KeyX)
	d.keys()
	if clip.writes != 0 || password.SecretText != "secret" {
		t.Fatal("password copy/cut exposed or removed secret")
	}
	clip.text = "public"
	clip.readErr = errors.New("read failed")
	d.keys(ebiten.KeyControlLeft, ebiten.KeyV)
	d.keys()
	if password.SecretText != "secret" {
		t.Fatal("failed paste changed password")
	}
}

func TestInteractionSourceSwitchPreservesSelectionAndRestartsCaret(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	input, _ := ctx.NewInput()
	input.Position = Point{}
	input.UpdateText("retained text")
	win.AddItem(input)
	d.step()
	ctx.Focus(input)
	input.SelectStart, input.SelectEnd, input.CursorPos = 1, 4, 4
	source := ctx.inputSource
	ctx.SetInputSource(source)
	d.reads, d.ticks = 0, 0
	d.frame.Time = time.Unix(10, 0)
	d.step()
	if ctx.FocusedItem() != input || input.SelectStart != 1 || input.SelectEnd != 4 || input.CursorPos != 4 || input.Text != "retained text" || !input.textEdit.caretOn {
		t.Fatal("source switch changed editing state")
	}
	d.frame.Time = d.frame.Time.Add(600 * time.Millisecond)
	d.step()
	if input.textEdit.caretOn {
		t.Fatal("caret baseline did not restart in the new input stream")
	}
}

func TestInteractionMiddleButtonWindowGesture(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	old := ctx.middleClickMove
	ctx.middleClickMove = true
	t.Cleanup(func() { ctx.middleClickMove = old })
	win.Movable = true
	clicks := 0
	button := ctx.NewActionButton("Primary only", func() { clicks++ })
	button.Position = Point{}
	win.AddItem(button)
	d.step()
	before := win.Position
	p := itemCenter(button)
	d.frame.PointerX, d.frame.PointerY = int(p.X), int(p.Y)
	d.frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButtonMiddle}
	d.step()
	d.frame.PointerX += 40
	d.frame.PointerY += 25
	d.step()
	d.frame.MouseButtons = nil
	d.step()
	if win.Position != pointAdd(before, Point{X: 40, Y: 25}) || clicks != 0 || ctx.dragWin != nil || ctx.downWin != nil {
		t.Fatal("middle-button gesture moved or activated the wrong target, or retained capture")
	}
}
