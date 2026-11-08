package eui

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRuntimeSettersUpdateStateWithoutUserEvents(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	slider, _ := ctx.NewSlider()
	progress, _ := ctx.NewProgressBar()
	check, _ := ctx.NewCheckbox()
	dd, _ := ctx.NewDropdown()
	wheel, _ := ctx.NewColorWheel()
	input, _ := ctx.NewInput()
	items := []*ItemData{slider, progress, check, dd, wheel, input}
	win.AddItem(ctx.NewColumn(items...))
	events, actions, selections, colors := 0, 0, 0, 0
	for _, it := range items {
		it.Handler.Handle = func(UIEvent) { events++ }
		it.Handler.EnableQueue(16)
		it.Action = func() { actions++ }
	}
	dd.OnSelect = func(int) { selections++ }
	wheel.OnColorChange = func(Color) { colors++ }
	slider.SetRange(-5, 5)
	slider.SetValue(7)
	progress.SetRange(0, 10)
	progress.SetValue(4)
	check.SetChecked(true)
	dd.SetOptions([]string{"One", "Two"})
	dd.SetSelected(1)
	wheel.SetWheelColor(NewColorHSV(90, .5, .6, 1))
	input.UpdateText("Runtime update")
	check.SetDisabled(true)
	check.SetDisabled(false)
	check.SetVisible(false)
	check.SetVisible(true)
	if slider.Value != 5 || progress.Value != 4 || !check.Checked || dd.Selected != 1 || input.Text != "Runtime update" {
		t.Fatal("model changes were not applied")
	}
	if events != 0 || actions != 0 || selections != 0 || colors != 0 {
		t.Fatalf("program updates emitted callbacks: events=%d actions=%d selects=%d colors=%d", events, actions, selections, colors)
	}
	for _, it := range items {
		if len(it.Handler.Events()) != 0 {
			t.Fatal("program update queued a user event")
		}
	}
	if !win.Dirty {
		t.Fatal("program updates did not request repaint")
	}
	win.Dirty = false
	slider.SetValue(5)
	progress.SetValue(4)
	check.SetChecked(true)
	dd.SetSelected(1)
	dd.SetOptions([]string{"One", "Two"})
	input.UpdateText("Runtime update")
	check.SetDisabled(false)
	check.SetVisible(true)
	if win.Dirty {
		t.Fatal("unchanged updates scheduled unnecessary repaint")
	}
}

func TestRuntimeNumericValidationAndInteractionRebasing(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	slider, _ := ctx.NewSlider()
	slider.IntOnly = true
	win.AddItem(slider)
	slider.SetRange(.5, 9.5)
	slider.SetValue(50)
	if slider.Value != 9 {
		t.Fatal("integer normalization left a fractional upper bound")
	}
	slider.SetValue(-50)
	if slider.Value != 1 {
		t.Fatal("integer normalization missed first in-range integer")
	}
	slider.SetValue(float32(math.NaN()))
	slider.SetRange(8, 2)
	slider.SetRange(.1, .2)
	slider.SetRange(0, float32(math.Inf(1)))
	if slider.Value != 1 || slider.MinValue != .5 || slider.MaxValue != 9.5 {
		t.Fatal("invalid numeric update changed the model")
	}
	releases := 0
	slider.Handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	ctx.keyboardSlider = slider
	ctx.keyboardSliderStart = 1
	slider.dragStartInit = true
	slider.dragStartValue = 1
	slider.SetValue(5)
	ctx.finishKeyboardSlider()
	slider.emitSliderReleased()
	if releases != 0 {
		t.Fatal("programmatic value update manufactured a release event")
	}
	progress, _ := ctx.NewProgressBar()
	win.AddItem(progress)
	progress.SetRange(4, 4)
	progress.SetValue(100)
	if progress.Value != 4 {
		t.Fatal("constant range was not respected")
	}
}

func TestRuntimeRadioUpdatesStayInTheirContainer(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	a, _ := ctx.NewRadio()
	b, _ := ctx.NewRadio()
	other, _ := ctx.NewRadio()
	for _, it := range []*ItemData{a, b, other} {
		it.RadioGroup = "quality"
	}
	// The containing window need not have opened or registered yet.
	closed := ctx.NewWindow()
	closed.AddItem(a)
	closed.AddItem(b)
	another := ctx.NewWindow()
	another.AddItem(other)
	other.SetChecked(true)
	a.SetChecked(true)
	b.SetChecked(true)
	if a.Checked || !b.Checked || !other.Checked {
		t.Fatal("radio group escaped its window or failed before opening")
	}
	b.SetChecked(false)
	if b.Checked {
		t.Fatal("radio could not be cleared programmatically")
	}
}

func TestUnavailableSubtreeCancelsInteractionAndRestoresPreviewOnce(t *testing.T) {
	ctx := newTestContext(t)

	for _, hide := range []bool{false, true} {
		win := ctx.keyboardFixture(t)
		input, _ := ctx.NewInput()
		input.Text = "Selected draft"
		slider, _ := ctx.NewSlider()
		dd, _ := ctx.NewDropdown()
		dd.Options = []string{"Selected", "Preview"}
		root := ctx.NewColumn(input, slider, dd)
		win.AddItem(root)
		win.Refresh()
		ctx.Focus(input)
		input.editSelect(0, 4)
		ctx.hoveredItem = input
		ctx.activeItem = slider
		ctx.keyboardSlider = slider
		ctx.keyboardSliderStart = slider.Value
		slider.dragStartInit = true
		input.editor().scrollDrag = PART_SCROLL_V
		dd.Open = true
		dd.HoverIndex = 1
		callbacks := 0
		dd.OnHover = func(index int) {
			callbacks++
			if index != dd.Selected || dd.Open || dd.HoverIndex != -1 {
				t.Fatal("preview callback saw incomplete dismissal")
			}
		}
		if hide {
			root.SetVisible(false)
		} else {
			root.SetDisabled(true)
		}
		if ctx.focusedItem != nil || ctx.selectedTextItem != nil || ctx.hoveredItem != nil || ctx.activeItem != nil || ctx.keyboardSlider != nil || slider.dragStartInit || input.editor().scrollDrag != PART_NONE || dd.Open || input.SelectedText() != "" {
			t.Fatal("unavailable subtree retained interaction state")
		}
		if callbacks != 1 {
			t.Fatal("preview did not finish once")
		}
		if hide {
			root.SetVisible(false)
			root.SetVisible(true)
		} else {
			root.SetDisabled(true)
			root.SetDisabled(false)
		}
		if callbacks != 1 || ctx.FocusedItem() != nil {
			t.Fatal("showing/re-enabling resumed old interaction")
		}
		// Parent disabling must not rewrite the child's own disabled preference.
		input.SetDisabled(true)
		root.SetDisabled(true)
		root.SetDisabled(false)
		if !input.Disabled || !input.isDisabled() || slider.isDisabled() {
			t.Fatal("parent disabling overwrote child flags")
		}
	}
}

func TestOptionReplacementCopiesNormalizesAndEndsPreview(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	dd, _ := ctx.NewDropdown()
	dd.HeaderCount = 1
	values := []string{"Header", "One", "Two"}
	dd.SetOptions(values)
	values[1] = "External mutation"
	if dd.Options[1] != "One" || dd.Selected != 1 {
		t.Fatal("options were aliased or selected a header")
	}
	win.AddItem(dd)
	dd.SetSelected(2)
	dd.Open = true
	dd.HoverIndex = 1
	dd.Scroll = Point{Y: 100}
	calls := 0
	dd.OnHover = func(index int) {
		calls++
		if index != 2 || dd.Options[2] != "Two" {
			t.Fatal("preview was not ended against the old options")
		}
	}
	dd.SetOptions([]string{"Header", "Replacement"})
	if calls != 1 || dd.Open || dd.Selected != 1 || dd.HoverIndex != -1 || dd.Scroll != (Point{}) {
		t.Fatal("replacing options left invalid selection or geometry")
	}
	dd.OnHover = nil
	dd.SetSelected(0)
	if dd.Selected != 1 {
		t.Fatal("header was programmatically selected")
	}
	dd.SetSelected(-1)
	dd.SetOptions(slices.Clone(dd.Options))
	if dd.Selected != -1 {
		t.Fatal("identical list lost explicit clear selection")
	}
	dd.SetOptions(nil)
	if dd.Selected != -1 || len(dd.Options) != 0 {
		t.Fatal("empty list retained selection")
	}
}

func TestRuntimeUpdatesReflowWrappedLabelsSizeAndVisibility(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.AutoSize = true
	label := ctx.NewWrappedLabel("A short initial label.", 180)
	second := ctx.NewLabel("Below")
	root := ctx.NewColumn(label, second)
	win.AddItem(root)
	win.Refresh()
	initial := win.GetSize().Y
	label.SetWrappedText(strings.Repeat("A longer phrase that should wrap. ", 6))
	if win.GetSize().Y <= initial || !win.Dirty {
		t.Fatal("wrapped text did not reflow its window")
	}
	label.UpdateText("Short again.")
	if label.wrappedLabel.raw != "Short again." || label.Text != "Short again." {
		t.Fatal("UpdateText lost wrapped label source")
	}
	before := root.contentBounds().Y
	label.SetVisible(false)
	if root.contentBounds().Y >= before {
		t.Fatal("hidden label retained layout space")
	}
	label.SetVisible(true)
	label.SetSize(Point{X: 80, Y: 0})
	if label.GetSize().Y <= 14 {
		t.Fatal("size setter did not rewrap label")
	}
	button := ctx.NewActionButton("Action", nil)
	root.AddItem(button)
	win.Refresh()
	before = win.GetSize().Y
	button.SetLabel("Field label")
	if win.GetSize().Y <= before {
		t.Fatal("field label setter did not reflow")
	}
}

func TestProgrammaticTextReplacesBindingAndHistorySilently(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	input, _ := ctx.NewInput()
	win.AddItem(input)
	ctx.Focus(input)
	input.ReplaceText("User edit")
	if !input.CanUndo() {
		t.Fatal("editing fixture lacks undo state")
	}
	value := ""
	input.TextPtr = &value
	events := 0
	input.Handler.Handle = func(UIEvent) { events++ }
	input.SelectStart = 0
	input.SelectEnd = 99
	input.CursorPos = 99
	input.UpdateText("New")
	if input.Text != "New" || value != "New" || input.CanUndo() || input.CanRedo() || input.CursorPos != 3 || input.SelectEnd != 3 || events != 0 {
		t.Fatal("programmatic text did not synchronize state silently")
	}
	input.SetDisabled(true)
	input.UpdateText("Disabled draft")
	if value != "Disabled draft" || input.Text != value {
		t.Fatal("disabled control rejected a model update")
	}
	input.HideText = true
	input.UpdateText("secret")
	if input.SecretText != "secret" || input.Text != "******" || value != "secret" || events != 0 {
		t.Fatal("masked model update exposed or lost its logical value")
	}
}

func TestRuntimeTabAvailabilityAndProgressTracking(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	progress, _ := ctx.NewProgressBar()
	SetProgressIndeterminate(progress, true)
	firstButton := ctx.NewActionButton("First", nil)
	first := ctx.NewColumn(firstButton)
	first.Name = "First"
	second := ctx.NewColumn(progress)
	second.Name = "Second"
	tabs := ctx.NewColumn()
	tabs.Fixed = true
	tabs.Size = Point{X: 300, Y: 180}
	tabs.Tabs = []*ItemData{first, second}
	win.AddItem(tabs)
	win.Refresh()
	ctx.Focus(firstButton)
	if first.Parent != tabs {
		t.Fatal("tab ownership was not linked")
	}
	first.SetDisabled(true)
	if tabs.ActiveTab != 1 || !win.HasIndeterminate || ctx.FocusedItem() != nil {
		t.Fatal("disabling active page did not select available page and clear old focus")
	}
	second.SetVisible(false)
	if tabs.ActiveTab != -1 || win.HasIndeterminate || len(tabs.activeContents()) != 0 {
		t.Fatal("all unavailable pages retained content or animation tracking")
	}
	// All existing input/layout walks must tolerate an empty active page.
	win.Refresh()
	ctx.scrollDropdown(win.Contents, Point{}, Point{})
	ctx.dropdownOpenContains(win.Contents, Point{})
	ctx.clickOpenDropdown(win.Contents, Point{}, false)
	first.SetDisabled(false)
	if tabs.ActiveTab != 0 {
		t.Fatal("re-enabled page was not restored")
	}
	second.SetVisible(true)
	tabs.SetActiveTab(1)
	if tabs.ActiveTab != 1 || !win.HasIndeterminate {
		t.Fatal("active tab setter did not update progress tracking")
	}
	progress.SetVisible(false)
	if win.HasIndeterminate {
		t.Fatal("hidden progress kept animation tracking")
	}
}

func TestRuntimeNilAndWrongControlUpdatesAreHarmless(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	var item *ItemData
	item.SetValue(1)
	item.SetRange(0, 1)
	item.SetChecked(true)
	item.SetDisabled(true)
	item.SetVisible(false)
	item.SetSelected(1)
	item.SetOptions(nil)
	item.SetActiveTab(0)
	item.SetLabel("Label")
	item.SetSize(Point{X: 2, Y: 2})
	item.SetWheelColor(ColorWhite)
	item.UpdateText("Text")
	SetColorSwatch(item, ColorWhite)
	label := ctx.NewLabel("Label")
	label.SetValue(10)
	label.SetChecked(true)
	label.SetOptions([]string{"Wrong type"})
	if label.Value != 0 || label.Checked || len(label.Options) != 0 {
		t.Fatal("type-specific setters mutated unrelated controls")
	}
	image := ebiten.NewImage(30, 60)
	defer image.Deallocate()
	item = ctx.NewImageReferenceItem(1, 1)
	item.UpdateImage(image)
	if item.Size != (Point{X: 30, Y: 60}) {
		t.Fatal("image update did not measure new image")
	}
}

func TestClosedDropdownUpdatesDoNotInventPreviewCallbacks(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	dd, _ := ctx.NewDropdown()
	if dd.HoverIndex != -1 {
		t.Fatal("new dropdown has a hover option before opening")
	}
	previews := 0
	dd.OnHover = func(int) { previews++ }
	dd.SetOptions([]string{"One", "Two"})
	dd.SetSelected(1)
	dd.SetDisabled(true)
	dd.SetVisible(false)
	if previews != 0 {
		t.Fatal("closed dropdown updates invented a preview dismissal")
	}
}

func TestLayoutAndAvailabilityBypassRefreshCoalescing(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	root := ctx.NewColumn(ctx.NewLabel("Initial"))
	win.AddItem(root)
	win.reusableRenderTarget(500, 400)
	win.SetRefreshInterval(time.Hour)
	for _, update := range []func(){func() { root.SetDisabled(true) }, func() { root.SetDisabled(false) }, func() { root.SetVisible(false) }, func() { root.SetVisible(true) }, func() { root.Contents[0].UpdateText("A longer label that changes the layout") }} {
		win.Dirty = false
		win.refreshPending = false
		win.lastRefresh = time.Now()
		update()
		if !win.Dirty || win.refreshPending {
			t.Fatal("layout/availability waited for background cache interval")
		}
	}
}

func TestSelectedIndexIsRevalidatedAfterPreviewCallback(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	dd, _ := ctx.NewDropdown()
	dd.SetOptions([]string{"One", "Two"})
	win.AddItem(dd)
	dd.Open = true
	dd.HoverIndex = 1
	dd.OnHover = func(int) { dd.SetOptions(nil) }
	dd.SetSelected(1)
	if dd.Selected != -1 || len(dd.Options) != 0 {
		t.Fatal("preview callback left selection outside replacement options")
	}
}

func TestTooltipIndicatorChangesReflowTheirWindow(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.AutoSize = true
	win.ShowTooltipIndicators = true
	label := ctx.NewLabel("Label")
	win.AddItem(ctx.NewColumn(label))
	win.Refresh()
	before := label.GetSize().X
	label.SetTooltip("Help for the label")
	if label.GetSize().X <= before || !win.Dirty {
		t.Fatal("new help indicator did not invalidate layout")
	}
	label.SetTooltip("")
	if label.GetSize().X != before {
		t.Fatal("removing help did not restore label width")
	}
}

func TestRuntimeContextMenuDismissalPreservesOtherMenus(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	previous := ctx.contextMenus
	ctx.contextMenus = nil
	defer func() { ctx.contextMenus = previous }()
	first := ctx.ShowContextMenu([]string{"One", "Two"}, 10, 20, nil)
	second := ctx.ShowContextMenu([]string{"Another"}, 200, 20, nil)
	first.HoverIndex = 1
	previews := 0
	first.OnHover = func(index int) {
		previews++
		if index != first.Selected {
			t.Fatal("wrong preview restoration")
		}
		ctx.ShowContextMenu([]string{"Follow up"}, 40, 50, nil)
	}
	first.SetDisabled(true)
	if previews != 1 || len(ctx.contextMenus) != 2 || ctx.contextMenus[0] != second || first.Open {
		t.Fatal("disabling menu discarded other menus or failed to close its overlay")
	}
}

func TestManualRefreshInvalidatesItemCaches(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	label := ctx.NewLabel("Cached label")
	win.AddItem(ctx.NewColumn(label))
	label.Render = ctx.uiRenderTargets.Acquire(100, 30, false)
	label.Dirty = false
	win.Dirty = false
	label.FontSize = 20
	win.Refresh()
	if !label.Dirty || !win.Dirty {
		t.Fatal("manual refresh did not invalidate nested item cache")
	}
}

func TestChildUpdatesReflowOpenAutosizedWindows(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.AutoSize = true
	root := ctx.NewColumn(ctx.NewLabel("First"))
	win.AddItem(root)
	win.Refresh()
	initial := win.GetSize().Y
	added := ctx.NewLabel("Added\nAnother line")
	root.AddItem(added)
	if win.GetSize().Y <= initial {
		t.Fatal("adding a child did not reflow the live window")
	}
	grown := win.GetSize().Y
	root.RemoveItem(added)
	if win.GetSize().Y >= grown || added.Parent != nil || added.ParentWindow != nil {
		t.Fatal("removing a child did not reflow/detach")
	}
	root.SetItems([]*ItemData{nil, ctx.NewLabel("New\nBatched\nContent")})
	if len(root.Contents) != 1 || win.GetSize().Y <= initial {
		t.Fatal("batch replacement did not normalize/reflow")
	}
	old := root.Contents[0]
	root.ReplaceItem(0, ctx.NewLabel("Short"))
	if old.ParentWindow != nil || win.GetSize().Y >= grown {
		t.Fatal("replacement did not detach/reflow")
	}
	win.AddItem(ctx.NewLabel("Root-level addition"))
	if !win.Dirty {
		t.Fatal("root-level update did not repaint")
	}
}
