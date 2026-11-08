package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func (ctx *Context) testModal(t *testing.T, children ...*ItemData) *WindowData {
	t.Helper()
	win := ctx.NewWindow()
	win.Position = Point{X: 580, Y: 20}
	win.Size = Point{X: 380, Y: 280}
	win.Movable = false
	win.Resizable = false
	win.AddItem(ctx.NewColumn(children...))
	win.MarkOpenModal()
	return win
}
func TestModalBlocksBackgroundInputAndTrapsFocus(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	calls := 0
	button := ctx.NewActionButton("background", func() { calls++ })
	editor, _ := ctx.NewInput()
	editor.UpdateText("background")
	editor.Position = Point{}
	button.Position = Point{}
	rows := ctx.NewColumn(button, editor)
	rows.Scrollable, rows.Fixed = true, true
	rows.Size = Point{X: 350, Y: 120}
	for range 20 {
		rows.AddItem(ctx.NewLabel("scroll row"))
	}
	base.AddItem(rows)
	base.Movable = true
	d.step()
	ctx.Focus(editor)
	first, second := ctx.NewActionButton("first", nil), ctx.NewActionButton("second", nil)
	modal := ctx.testModal(t, first, second)
	d.keys() // The transition's neutral frame.
	if ctx.FocusedItem() != first || ctx.ModalWindow() != modal || !modal.IsModal() {
		t.Fatal("modal did not establish focus/scope")
	}
	pButton := itemCenter(button)
	d.move(pButton, true)
	if calls != 0 || ctx.FocusedItem() != first || ctx.activeWindow != modal || !ctx.PointerPressHandled() {
		t.Fatal("outside click reached background or removed modal focus")
	}
	d.move(pButton, false)
	p := itemCenter(editor)
	d.frame.PointerX, d.frame.PointerY = int(p.X), int(p.Y)
	d.frame.WheelY = -5
	d.step()
	if rows.Scroll.Y != 0 {
		t.Fatal("wheel reached background flow")
	}
	before := base.Position
	d.frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButtonMiddle}
	ctx.SetMiddleClickMove(true)
	d.step()
	d.frame.PointerX += 80
	d.step()
	d.frame.MouseButtons = nil
	d.step()
	if base.Position != before {
		t.Fatal("middle-button move reached background")
	}
	ctx.Focus(editor)
	if ctx.FocusedItem() != first {
		t.Fatal("programmatic focus escaped modal scope")
	}
	for _, want := range []*ItemData{second, first, second} {
		d.keys(ebiten.KeyTab)
		d.keys()
		if ctx.FocusedItem() != want {
			t.Fatal("Tab escaped modal controls")
		}
	}
	d.keys(ebiten.KeyF6)
	d.keys()
	if ctx.activeWindow != modal || ctx.FocusedItem().ParentWindow != modal {
		t.Fatal("window cycling escaped modal")
	}
	d.frame.Text = "blocked"
	d.step()
	if editor.Text != "background" {
		t.Fatal("text reached background editor")
	}
}
func TestModalRestoresEditorSelectionHistoryAndSearch(t *testing.T) {
	ctx := newTestContext(t)
	base, item, d := ctx.imeEditor(t, "original")
	item.editMove(8, false)
	item.editInsert("!", "")
	item.editSelect(2, 5)
	snapshot := item.editSnapshot()
	modal := ctx.testModal(t, ctx.NewActionButton("OK", nil))
	d.keys()
	if ctx.HasTextSelection() {
		t.Fatal("background selection remained active for copy")
	}
	modal.Close()
	d.keys()
	if ctx.FocusedItem() != item || item.editSnapshot() != snapshot || !item.CanUndo() || ctx.selectedTextItem != item {
		t.Fatal("modal close reset document/caret/selection/history")
	}
	base.Searchable = true
	base.OpenSearch()
	base.searchInput.UpdateText("find")
	base.SearchText = "find"
	search := base.searchInput
	search.editSelect(1, 3)
	modal = ctx.testModal(t, ctx.NewActionButton("OK", nil))
	d.keys()
	modal.RemoveWindow()
	d.keys()
	if ctx.activeSearch != base || base.SearchText != "find" || search.SelectStart != 1 || search.SelectEnd != 3 {
		t.Fatal("modal retirement did not restore search editing")
	}
}
func TestNestedModalsRestoreLiveParentsAndOutOfOrderClose(t *testing.T) {
	for _, order := range []string{"nested", "parent first", "remove parent", "closed field"} {
		t.Run(order, func(t *testing.T) {
			ctx := newTestContext(t)
			_, original, d := ctx.imeEditor(t, "base")
			aButton := ctx.NewActionButton("A", nil)
			a := ctx.testModal(t, aButton)
			d.keys()
			bButton := ctx.NewActionButton("B", nil)
			b := ctx.testModal(t, bButton)
			d.keys()
			a.MarkOpenModal()
			if ctx.ModalWindow() != b || ctx.FocusedItem() != bButton {
				t.Fatal("reopening parent reordered modal chain")
			}
			switch order {
			case "nested":
				b.Close()
				d.keys()
				if ctx.FocusedItem() != aButton || ctx.ModalWindow() != a {
					t.Fatal("nested return lost parent")
				}
				a.Close()
			case "parent first":
				a.Close()
				if ctx.ModalWindow() != b || ctx.FocusedItem() != bButton {
					t.Fatal("closing parent stole child focus")
				}
				b.Close()
			case "remove parent":
				a.RemoveWindow()
				b.RemoveWindow()
			case "closed field":
				a.Open = false
				d.keys()
				b.Open = false
				d.keys()
			}
			d.keys()
			if ctx.ModalWindow() != nil || ctx.FocusedItem() != original || len(ctx.modals) != 0 {
				t.Fatal("last modal did not restore original focus")
			}
		})
	}
}
func TestModalDefaultCancelAndCloseVeto(t *testing.T) {
	ctx := newTestContext(t)
	_, original, d := ctx.imeEditor(t, "original")
	accepted, cancelled := 0, 0
	modal := ctx.ShowModal("confirm", "choose", []PopupButton{{Text: "Cancel", Cancel: true, Action: func() { cancelled++ }}, {Text: "Apply", Default: true, Action: func() { accepted++ }}})
	d.keys()
	if ctx.FocusedItem() != modal.DefaultButton {
		t.Fatal("popup did not focus designated default")
	}
	d.keys(ebiten.KeyEnter)
	if accepted != 1 || cancelled != 0 || modal.Open || ctx.FocusedItem() != original {
		t.Fatal("Enter did not activate only default and restore focus")
	}
	d.keys()
	modal = ctx.ShowModal("cancel", "", []PopupButton{{Text: "Cancel", Cancel: true, Action: func() { cancelled++ }}})
	d.keys()
	modal.BeforeClose = func() bool { return false }
	d.keys(ebiten.KeyEscape)
	d.keys()
	if !modal.Open || ctx.ModalWindow() != modal || cancelled != 1 {
		t.Fatal("close veto discarded modal scope")
	}
	modal.BeforeClose = nil
	d.keys(ebiten.KeyEscape)
	d.keys()
	if modal.Open || cancelled != 2 || ctx.FocusedItem() != original {
		t.Fatal("Escape cancel did not close modal")
	}
	modal = ctx.ShowModal("protected", "", nil)
	modal.Closable = false
	d.keys()
	d.keys(ebiten.KeyEscape)
	d.keys()
	if !modal.Open {
		t.Fatal("nonclosable dialog ignored cancellation policy")
	}
	modal.Closable = true
	d.keys(ebiten.KeyEscape)
	d.keys()
	if modal.Open {
		t.Fatal("closable fallback Escape did not close")
	}
}
func TestModalIMEAndDropdownEscapeTakePriority(t *testing.T) {
	ctx := newTestContext(t)
	_, _, d := ctx.imeEditor(t, "")
	editor, _ := ctx.NewInput()
	dd, _ := ctx.NewDropdown()
	dd.Options = []string{"one", "two"}
	modal := ctx.testModal(t, editor, dd)
	d.keys()
	ctx.Focus(editor)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	d.frame.Keys = []ebiten.Key{ebiten.KeyEscape}
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCancel}}
	d.step()
	if !modal.Open || editor.IsComposing() || editor.Text != "" {
		t.Fatal("IME Escape reached dialog cancellation")
	}
	d.keys()
	ctx.Focus(dd)
	d.keys(ebiten.KeyEnter)
	d.keys()
	if !dd.Open {
		t.Fatal("dropdown did not open inside modal")
	}
	d.keys(ebiten.KeyEscape)
	d.keys()
	if dd.Open || !modal.Open {
		t.Fatal("dropdown Escape reached dialog cancellation")
	}
	d.keys(ebiten.KeyEscape)
	d.keys()
	if modal.Open {
		t.Fatal("second Escape did not dismiss modal")
	}
}
func TestModalTransitionsConsumeOpeningKeyAndClosingPress(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	activated := 0
	behind := ctx.NewActionButton("behind", func() { activated++ })
	behind.Position = Point{}
	var dialog *WindowData
	opener := ctx.NewActionButton("open", func() {
		dialog = ctx.ShowModal("next", "", []PopupButton{{Text: "OK", Action: func() { activated++ }}})
	})
	base.AddItem(ctx.NewColumn(opener, behind))
	d.step()
	ctx.Focus(opener)
	d.keys(ebiten.KeyEnter)
	if dialog == nil || !dialog.Open || activated != 0 {
		t.Fatal("opening Enter also activated new dialog")
	}
	for range 35 {
		d.step()
	}
	if !dialog.Open || activated != 0 {
		t.Fatal("held opening key reached modal")
	}
	d.keys()
	d.click(dialog.DefaultButton)
	if dialog.Open || activated != 1 {
		t.Fatal("modal button did not close once")
	}
	// Keep a press held over the newly exposed background after a modal closes.
	dialog = ctx.testModal(t, ctx.NewActionButton("close", nil))
	d.keys()
	p := itemCenter(behind)
	d.move(p, true)
	dialog.Close()
	for range 35 {
		d.move(p, true)
	}
	if activated != 1 {
		t.Fatal("held pointer activated exposed background")
	}
	d.move(p, false)
	d.click(behind)
	if activated != 2 {
		t.Fatal("fresh input did not resume after neutral")
	}
}
func TestModalCallbacksRespectReplacementAndUnavailableReturnTargets(t *testing.T) {
	ctx := newTestContext(t)
	base, item, d := ctx.imeEditor(t, "base")
	a := ctx.testModal(t, ctx.NewActionButton("A", nil))
	d.keys()
	var b *WindowData
	a.OnClose = func() { b = ctx.testModal(t, ctx.NewActionButton("B", nil)) }
	a.Close()
	d.keys()
	if b == nil || ctx.ModalWindow() != b || ctx.FocusedItem().ParentWindow != b {
		t.Fatal("OnClose replacement lost modal scope")
	}
	item.SetVisible(false)
	b.Close()
	d.keys()
	if ctx.FocusedItem() == item || ctx.ModalWindow() != nil {
		t.Fatal("restoration revived an unavailable control")
	}
	item.SetVisible(true)
	reentrant := ctx.NewWindow()
	reentrant.AddItem(ctx.NewActionButton("C", nil))
	reentrant.OnOpen = func() { reentrant.Close() }
	reentrant.MarkOpenModal()
	d.keys()
	if reentrant.Open || reentrant.IsModal() || ctx.ModalWindow() != nil {
		t.Fatal("OnOpen close was overwritten")
	}
	base.Close()
}
func TestModalityIsContextLocalAndPickerNests(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	_, _, da := a.imeEditor(t, "A")
	_, ib, db := b.imeEditor(t, "B")
	modal := a.testModal(t, a.NewActionButton("A", nil))
	da.keys()
	picker := a.ShowColorPicker("color", ColorRed, nil)
	da.keys()
	if a.ModalWindow() != picker || !picker.IsModal() {
		t.Fatal("color picker escaped modal scope")
	}
	picker.Close()
	da.keys()
	if a.ModalWindow() != modal {
		t.Fatal("picker close did not restore parent")
	}
	db.frame.Text = "!"
	db.step()
	if ib.Text != "B!" || b.ModalWindow() != nil {
		t.Fatal("modality crossed context boundary")
	}
}

func TestModalBlocksTouchAndBackgroundShortcuts(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	actions, searches, drags := 0, 0, 0
	button := ctx.NewActionButton("base", func() { actions++ })
	button.Position = Point{}
	base.AddItem(button)
	base.Searchable = true
	base.OnSearchNext = func(bool) { searches++ }
	modalButton := ctx.NewActionButton("modal", nil)
	modal := ctx.testModal(t, modalButton)
	d.keys()
	p := itemCenter(button)
	d.frame.Touches = []TouchPoint{{ID: 7, X: int(p.X), Y: int(p.Y)}}
	d.step()
	if !ctx.PointerPressHandled() || actions != 0 || ctx.FocusedItem() != modalButton {
		t.Fatal("touch escaped modal scope")
	}
	d.frame.Touches = nil
	d.step()
	ctx.SetTileDividers([]TileDivider{{Orientation: TileDividerVertical, Position: 10, Start: 0, End: 600, HitSize: 20, OnDrag: func(float32) { drags++ }}})
	d.frame.PointerX, d.frame.PointerY = 10, 300
	d.frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButton0}
	d.step()
	d.frame.PointerX = 15
	d.step()
	d.frame.MouseButtons = nil
	d.step()
	if drags != 0 {
		t.Fatal("divider interaction escaped modal scope")
	}
	d.keys(ebiten.KeyControlLeft, ebiten.KeyF)
	d.keys()
	d.keys(ebiten.KeyF3)
	d.keys()
	if ctx.activeSearch != nil || searches != 0 || ctx.activeWindow != modal {
		t.Fatal("background search shortcuts escaped modal scope")
	}
}

func TestSliderReleaseCallbackCanOpenModal(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	slider, events := ctx.NewSlider()
	slider.Position = Point{}
	slider.Size = Point{X: 250, Y: 30}
	slider.MinValue, slider.MaxValue = 0, 100
	base.AddItem(slider)
	d.step()
	var modal *WindowData
	events.Handle = func(event UIEvent) {
		if event.Type == EventSliderReleased {
			modal = ctx.testModal(t, ctx.NewActionButton("continue", nil))
		}
	}
	p := itemCenter(slider)
	d.move(p, true)
	d.move(Point{X: p.X + 50, Y: p.Y}, true)
	d.move(p, false)
	if modal == nil || ctx.ModalWindow() != modal || ctx.FocusedItem().ParentWindow != modal {
		t.Fatal("slider release did not preserve callback-created modal")
	}
}

func TestModalReturnUsesTheClickedInactiveWindow(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	var modal *WindowData
	opener := ctx.NewActionButton("open", func() { modal = ctx.testModal(t, ctx.NewActionButton("OK", nil)) })
	opener.Position = Point{}
	base.AddItem(opener)
	other := ctx.NewWindow()
	other.Position = Point{X: 700, Y: 600}
	other.Size = Point{X: 200, Y: 150}
	other.AddItem(ctx.NewActionButton("other", nil))
	other.MarkOpen()
	other.BringForward()
	d.step()
	d.click(opener)
	if modal == nil {
		t.Fatal("inactive window did not open modal")
	}
	modal.Close()
	d.keys()
	if ctx.FocusedItem() != opener || ctx.activeWindow != base {
		t.Fatal("restored control and active window disagree")
	}
}
func TestModalEscapeKeepsEditorFocusWhenCloseIsVetoed(t *testing.T) {
	ctx := newTestContext(t)
	_, _, d := ctx.imeEditor(t, "")
	editor, _ := ctx.NewInput()
	editor.UpdateText("draft")
	modal := ctx.testModal(t, editor)
	modal.BeforeClose = func() bool { return false }
	d.keys()
	ctx.Focus(editor)
	d.keys(ebiten.KeyEscape)
	d.keys()
	if !modal.Open || ctx.FocusedItem() != editor || editor.Text != "draft" {
		t.Fatal("vetoed Escape discarded editor focus or text")
	}
}
func TestModalCloseIconUsesCancelRoleAndHonorsDisabled(t *testing.T) {
	ctx := newTestContext(t)
	_, _, d := ctx.imeEditor(t, "")
	calls := 0
	modal := ctx.ShowModal("cancel", "", []PopupButton{{Text: "Cancel", Cancel: true, Action: func() { calls++ }}})
	d.keys()
	modal.CancelButton.SetDisabled(true)
	pos, size := modal.getPosition(), modal.GetSize()
	p := Point{X: pos.X + size.X - modal.GetTitleSize()/2, Y: pos.Y + modal.GetTitleSize()/2}
	d.move(p, true)
	d.move(p, false)
	if !modal.Open || calls != 0 {
		t.Fatal("disabled cancel action was bypassed by close icon")
	}
	modal.CancelButton.SetDisabled(false)
	d.move(p, true)
	d.move(p, false)
	if modal.Open || calls != 1 {
		t.Fatal("close icon did not run cancel action once")
	}
}

func TestOpeningModalReportsTheTriggeringPointerAsHandled(t *testing.T) {
	ctx := newTestContext(t)
	base, d, _ := ctx.interactionFixture(t)
	opener := ctx.NewActionButton("open", func() { ctx.ShowModal("dialog", "", nil) })
	opener.Position = Point{}
	base.AddItem(opener)
	d.step()
	d.move(itemCenter(opener), true)
	if ctx.ModalWindow() == nil || !ctx.PointerPressHandled() {
		t.Fatal("opening modal leaked its triggering press to the host")
	}
}
func TestModalCancelsPreeditAndCapturedSliderWithoutSyntheticEvents(t *testing.T) {
	ctx := newTestContext(t)
	base, item, d := ctx.imeEditor(t, "committed")
	events := 0
	item.Handler.Handle = func(UIEvent) { events++ }
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "uncommitted", SelectionEnd: 11})
	modal := ctx.testModal(t, ctx.NewActionButton("OK", nil))
	d.keys()
	if item.IsComposing() || item.Text != "committed" || events != 0 {
		t.Fatal("opening modal committed background preedit")
	}
	modal.Close()
	d.keys()
	slider, handler := ctx.NewSlider()
	slider.Position = Point{}
	slider.Size = Point{X: 250, Y: 30}
	slider.MinValue, slider.MaxValue = 0, 100
	base.AddItem(slider)
	d.step()
	released := 0
	handler.Handle = func(event UIEvent) {
		if event.Type == EventSliderReleased {
			released++
		}
	}
	p := itemCenter(slider)
	d.move(p, true)
	d.move(Point{X: p.X + 30, Y: p.Y}, true)
	modal = ctx.testModal(t, ctx.NewActionButton("OK", nil))
	d.move(p, false)
	if released != 0 || ctx.activeItem != nil || ctx.dragWin != nil {
		t.Fatal("modal transition retained capture or synthesized release")
	}
}
func TestIMECommitCallbackCannotAcceptItsNewModal(t *testing.T) {
	ctx := newTestContext(t)
	_, item, d := ctx.imeEditor(t, "")
	var modal *WindowData
	accepted := 0
	item.Handler.Handle = func(UIEvent) {
		modal = ctx.ShowModal("next", "", []PopupButton{{Text: "OK", Action: func() { accepted++ }}})
	}
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	d.frame.Keys = []ebiten.Key{ebiten.KeyEnter}
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCommit, Text: "日本"}}
	d.step()
	if modal == nil || !modal.Open || accepted != 0 || item.Text != "日本" {
		t.Fatal("IME commit activated a callback-created modal")
	}
}

func TestModalDoesNotRestoreDetachedMovedOrInactiveTargets(t *testing.T) {
	for _, change := range []string{"detached", "moved", "disabled", "inactive tab", "search unavailable"} {
		t.Run(change, func(t *testing.T) {
			ctx := newTestContext(t)
			base, item, d := ctx.imeEditor(t, "draft")
			item.editSelect(1, 3)
			if change == "search unavailable" {
				base.Searchable = true
				base.OpenSearch()
			}
			modal := ctx.testModal(t, ctx.NewActionButton("OK", nil))
			d.keys()
			switch change {
			case "detached":
				base.RemoveItem(item)
			case "moved":
				base.RemoveItem(item)
				other := ctx.NewWindow()
				other.AddItem(item)
				other.MarkOpen()
			case "disabled":
				item.SetDisabled(true)
			case "inactive tab":
				base.RemoveItem(item)
				tabs := ctx.NewColumn()
				tabs.Tabs = []*ItemData{ctx.NewColumn(item), ctx.NewColumn(ctx.NewLabel("other"))}
				base.AddItem(tabs)
				tabs.SetActiveTab(1)
			case "search unavailable":
				base.Searchable = false
				base.Refresh()
			}
			modal.Close()
			d.keys()
			if ctx.activeSearch != nil || (change != "search unavailable" && (ctx.FocusedItem() == item || ctx.selectedTextItem == item)) {
				t.Fatal("restoration revived unavailable or moved input")
			}
		})
	}
}
