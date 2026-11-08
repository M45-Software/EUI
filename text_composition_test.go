package eui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

func (d *interactionDriver) ime(events ...TextInputEvent) {
	d.t.Helper()
	d.frame.TextInput = events
	d.frame.Keys = nil
	d.frame.MouseButtons = nil
	d.step()
}
func (ctx *Context) imeEditor(t *testing.T, value string) (*WindowData, *ItemData, *interactionDriver) {
	t.Helper()
	win, d, _ := ctx.interactionFixture(t)
	item, _ := ctx.NewTextArea()
	item.Position = Point{}
	item.Size = Point{X: 350, Y: 140}
	item.UpdateText(value)
	win.AddItem(item)
	d.step()
	ctx.Focus(item)
	return win, item, d
}
func TestIMEPreeditIsSeparateAndCommitIsOneUndoableChange(t *testing.T) {
	ctx := newTestContext(t)

	win, item, d := ctx.imeEditor(t, "AoldZ")
	item.editSelect(1, 4)
	binding := item.Text
	item.TextPtr = &binding
	var events []UIEvent
	item.Handler.Handle = func(event UIEvent) { events = append(events, event) }
	defaults := 0
	win.DefaultButton = ctx.NewActionButton("Default", func() { defaults++ })
	win.AddItem(win.DefaultButton)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "にほん", SelectionStart: 0, SelectionEnd: 2})
	if item.Text != "AoldZ" || binding != "AoldZ" || len(events) != 0 || item.CanUndo() || item.displayEditText() != "AにほんZ" || !item.IsComposing() {
		t.Fatal("preedit leaked into the committed document, binding, events, or history")
	}
	if item.CursorPos != 4 || item.SelectStart != 1 || item.SelectEnd != 4 {
		t.Fatal("preedit changed the document selection")
	}
	d.frame.Keys = []ebiten.Key{ebiten.KeyEnter, ebiten.KeyBackspace, ebiten.KeyTab, ebiten.KeyControlLeft, ebiten.KeyF}
	d.frame.Text = "duplicate"
	d.step()
	if item.Text != "AoldZ" || defaults != 0 || ctx.activeSearch != nil || ctx.FocusedItem() != item {
		t.Fatal("IME-owned keys reached ordinary UI actions")
	}
	d.frame.Keys = []ebiten.Key{ebiten.KeyEnter}
	d.frame.Text = "日本"
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCommit, Text: "日本"}}
	d.step()
	if item.Text != "A日本Z" || binding != item.Text || item.CursorPos != 3 || item.SelectStart != 3 || item.SelectEnd != 3 || item.IsComposing() || len(events) != 1 || defaults != 0 {
		t.Fatalf("commit failed: text=%q caret=%d events=%d defaults=%d", item.Text, item.CursorPos, len(events), defaults)
	}
	d.keys()
	d.keys(ebiten.KeyControlLeft, ebiten.KeyZ)
	d.keys()
	if item.Text != "AoldZ" || item.SelectStart != 1 || item.SelectEnd != 4 {
		t.Fatal("one undo did not restore pre-composition document/selection")
	}
	d.keys(ebiten.KeyControlLeft, ebiten.KeyShiftLeft, ebiten.KeyZ)
	d.keys()
	if item.Text != "A日本Z" {
		t.Fatal("redo lost committed text")
	}
}

func TestIMEReplacementCaretAndIdenticalCommit(t *testing.T) {
	ctx := newTestContext(t)

	_, item, d := ctx.imeEditor(t, "teh dog")
	item.editMove(3, false)
	events := 0
	item.Handler.Handle = func(UIEvent) { events++ }
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "the", Replacement: &TextReplacement{Start: 0, End: 3, Caret: 3}})
	if item.Text != "the dog" || item.CursorPos != 3 || events != 1 {
		t.Fatal("surrounding replacement misplaced text or caret")
	}
	item.Undo()
	if item.Text != "teh dog" {
		t.Fatal("replacement was not one edit")
	}
	item.UpdateText("same")
	item.editSelect(0, 4)
	events = 0
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "same", SelectionEnd: 4}, TextInputEvent{Kind: TextInputCommit, Text: "same"})
	if item.Text != "same" || item.CursorPos != 4 || item.SelectStart != 4 || item.CanUndo() || events != 0 {
		t.Fatal("identical commit added history/events or retained old selection")
	}
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "stale", Replacement: &TextReplacement{Start: 0, End: 100, Caret: -1}})
	if item.Text != "same" || events != 0 {
		t.Fatal("out-of-document replacement changed text")
	}
}

func TestIMEPassthroughAndMultipleCommits(t *testing.T) {
	ctx := newTestContext(t)

	_, item, d := ctx.imeEditor(t, "")
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCommit, Text: "語", Passthrough: true}}
	d.frame.Keys = []ebiten.Key{ebiten.KeyEnter}
	d.step()
	if item.Text != "語\n" {
		t.Fatal("passthrough commit swallowed or duplicated Enter")
	}
	item.UpdateText("")
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "first", SelectionEnd: 5}, TextInputEvent{Kind: TextInputCommit, Text: "一"}, TextInputEvent{Kind: TextInputCommit, Text: "二"})
	if item.Text != "一二" || len(item.editor().undo) != 2 {
		t.Fatal("ordered commits were lost or coalesced")
	}
	item.Handler.Handle = func(UIEvent) { ctx.ClearFocus(item) }
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "三"}, TextInputEvent{Kind: TextInputCommit, Text: "must not move"})
	if item.Text != "一二三" || ctx.FocusedItem() != nil {
		t.Fatal("commit continued applying after its callback retired focus")
	}
}

func TestIMECancellationPathsDoNotCommitPreedit(t *testing.T) {
	ctx := newTestContext(t)

	for _, action := range []string{"escape", "focus", "hidden", "disabled", "closed", "replace", "capture", "unfocused", "source", "caret", "geometry"} {
		t.Run(action, func(t *testing.T) {
			win, item, d := ctx.imeEditor(t, "original")
			events := 0
			item.Handler.Handle = func(UIEvent) { events++ }
			d.ime(TextInputEvent{Kind: TextInputCompose, Text: "pending", SelectionEnd: 7})
			switch action {
			case "escape":
				d.frame.Keys = []ebiten.Key{ebiten.KeyEscape}
				d.frame.TextInput = []TextInputEvent{{Kind: TextInputCancel}}
				d.step()
			case "focus":
				other, _ := ctx.NewInput()
				win.AddItem(other)
				ctx.Focus(other)
				d.ime(TextInputEvent{Kind: TextInputCommit, Text: "late", Target: item})
				if other.Text != "" {
					t.Fatal("late event reached another field")
				}
			case "hidden":
				item.SetVisible(false)
			case "disabled":
				item.SetDisabled(true)
			case "closed":
				win.Close()
			case "replace":
				item.UpdateText("replacement")
			case "capture":
				ctx.SetKeyboardInputCaptured(true)
			case "unfocused":
				d.frame.Unfocused = true
				d.step()
			case "source":
				ctx.SetInputSource(ctx.inputSource)
			case "caret":
				item.editMove(0, false)
			case "geometry":
				win.Position.X += 20
				d.step()
			}
			want := "original"
			if action == "replace" {
				want = "replacement"
			}
			if item.IsComposing() || item.Text != want || events != 0 || item.CanUndo() {
				t.Fatal("cancellation committed provisional text or changed history")
			}
		})
	}
}

func TestIMEPasswordMaskingAndNativePrivacy(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	item, _ := ctx.NewInput()
	item.HideText = true
	item.Position = Point{}
	item.UpdateText("secret")
	win.AddItem(item)
	d.step()
	ctx.Focus(item)
	item.editSelect(2, 4)
	binding := item.SecretText
	item.TextPtr = &binding
	var payloads []string
	item.Handler.Handle = func(event UIEvent) { payloads = append(payloads, event.Text) }
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "秘密", SelectionEnd: 2})
	if item.Text != "******" || item.SecretText != "secret" || binding != "secret" || item.displayEditText() != "******" || len(payloads) != 0 {
		t.Fatal("password preedit exposed text or changed secret")
	}
	ctx.configureNativeTextInput(item)
	opts := ctx.nativeTextInput.composer.OnNewSession()
	if opts == nil || opts.TextBeforeCaret != "" || opts.TextAfterCaret != "" {
		t.Fatal("password context exposed to platform prediction")
	}
	ctx.cancelNativeTextInput(nil)
	if ctx.nativeTextInput.context.snapshot.text != "" {
		t.Fatal("closed native session retained a password snapshot")
	}
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "秘密"})
	if item.SecretText != "se秘密et" || binding != item.SecretText || item.Text != "******" || len(payloads) != 1 || payloads[0] != "******" || item.CanUndo() {
		t.Fatal("password commit violated masking/history policy")
	}
}

func TestIMESearchCompositionAndLimit(t *testing.T) {
	ctx := newTestContext(t)

	win, d, _ := ctx.interactionFixture(t)
	win.Searchable = true
	win.BringForward()
	d.step()
	win.OpenSearch()
	var queries []string
	next := 0
	win.OnSearch = func(value string) { queries = append(queries, value) }
	win.OnSearchNext = func(bool) { next++ }
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "にほん", SelectionEnd: 3})
	if win.SearchText != "" || len(queries) != 0 || !win.searchInput.IsComposing() {
		t.Fatal("preedit reached search model/callback")
	}
	box := win.searchBoxRect()
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: strings.Repeat("に", 40), SelectionEnd: 40})
	if win.searchBoxRect() != box {
		t.Fatal("preedit resized native session's search viewport")
	}
	d.frame.Keys = []ebiten.Key{ebiten.KeyEnter}
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCommit, Text: "日本"}}
	d.step()
	if win.SearchText != "日本" || len(queries) != 1 || next != 0 {
		t.Fatal("commit duplicated query or advanced search")
	}
	d.keys()
	d.keys(ebiten.KeyEnter)
	d.keys()
	if next != 1 {
		t.Fatal("subsequent Enter did not advance")
	}
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: strings.Repeat("x", 100)})
	if utf8.RuneCountInString(win.SearchText) != 64 {
		t.Fatal("IME bypassed search query limit")
	}
	d.keys(ebiten.KeyControlLeft, ebiten.KeyA)
	d.keys()
	d.keys(ebiten.KeyEscape)
	d.keys()
	if win.SearchText != "" || ctx.activeSearch != nil || ctx.selectedTextItem == win.searchInput || win.searchInput.IsComposing() {
		t.Fatal("closed search retained editing state")
	}
}

func TestIMENativeContextReplacementAndDeferredCancellation(t *testing.T) {
	ctx := newTestContext(t)

	_, item, d := ctx.imeEditor(t, "prev\nAoldZ\nnext")
	item.editSelect(6, 9)
	ctx.configureNativeTextInput(item)
	opts := ctx.nativeTextInput.composer.OnNewSession()
	if opts == nil || opts.TextBeforeCaret != "A" || opts.TextAfterCaret != "Z" {
		t.Fatalf("incorrect surrounding context: %+v", opts)
	}
	events := 0
	item.Handler.Handle = func(UIEvent) {
		events++
		if item.CursorPos != 7 {
			t.Fatal("callback observed caret after unchanged suffix")
		}
		item.UpdateText("callback replacement")
	}
	ctx.nativeTextInput.pumping = true
	ctx.applyNativeTextCommit("新", "A", "Z", true)
	if events != 1 || item.Text != "callback replacement" || !ctx.nativeTextInput.cancelPending || ctx.nativeTextInput.owner != item {
		t.Fatal("native callback cancellation was not deferred until session pump ended")
	}
	ctx.nativeTextInput.pumping = false
	ctx.cancelNativeTextInput(nil)
	if ctx.nativeTextInput.owner != nil {
		t.Fatal("deferred cancellation retained session owner")
	}
	item.Handler.Handle = nil
	item.UpdateText("new context")
	ctx.Focus(item)
	ctx.configureNativeTextInput(item)
	ctx.nativeTextInput.composer.OnNewSession()
	item.Text = "external assignment"
	ctx.applyNativeTextCommit("late", "", "", false)
	if item.Text != "external assignment" || !ctx.nativeTextInput.cancelPending {
		t.Fatal("stale native commit overwrote external state")
	}
	ctx.cancelNativeTextInput(nil)
	d.step()
}

func TestIMECandidateBoundsFollowRenderedGeometry(t *testing.T) {
	ctx := newTestContext(t)

	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			win, item, d := ctx.imeEditor(t, "line one\nline two\n"+strings.Repeat("long text ", 20))
			ctx.SetUIScale(scale)
			win.Size = Point{X: 450, Y: 320}
			item.Size = Point{X: 300, Y: 100}
			item.Label = "Document"
			item.SetWordWrap(true)
			root := win.Contents[0]
			root.Position = Point{X: 12, Y: 8}
			win.Refresh()
			ctx.Focus(item)
			d.step()
			viewport, origin := item.editGeometry()
			line, x := item.editCaret()
			layout := item.editLayout()
			expectedX, expectedY := origin.X+x, origin.Y+float32(line)*layout.lineHeight
			bounds := item.TextInputBounds()
			if math.Abs(float64(bounds.X0-expectedX)) > .01 || math.Abs(float64(bounds.Y0-expectedY)) > .01 || bounds.X0 < viewport.X0 || bounds.X1 > viewport.X1+.01 || bounds.Y1 > viewport.Y1+.01 {
				t.Fatalf("candidate anchor differs from rendered caret: %v expected (%g,%g)", bounds, expectedX, expectedY)
			}
			old := bounds
			win.Position.X += 20 / win.scale()
			bounds = item.TextInputBounds()
			if math.Abs(float64(bounds.X0-old.X0-20)) > .01 {
				t.Fatal("candidate geometry used stale DrawRect after window move")
			}
			if item.CursorPos != len([]rune(item.Text)) {
				t.Fatal("candidate query moved document caret")
			}
		})
	}
}

func TestIMEByteOffsetsAndFrameOwnership(t *testing.T) {
	ctx := newTestContext(t)

	for _, tc := range []struct{ bytes, want int }{{-1, 0}, {0, 0}, {2, 0}, {3, 1}, {5, 1}, {7, 2}, {8, 3}, {100, 3}} {
		if got := runesBeforeByte("あ🙂a", tc.bytes); got != tc.want {
			t.Fatalf("byte %d => %d runes, want %d", tc.bytes, got, tc.want)
		}
	}
	_, item, d := ctx.imeEditor(t, "abc")
	replacement := &TextReplacement{Start: 0, End: 3, Caret: -1}
	events := []TextInputEvent{{Kind: TextInputCommit, Text: "new", Replacement: replacement}}
	frame := d.frame
	frame.TextInput = events
	var state inputState
	if err := state.sample(frame); err != nil {
		t.Fatal(err)
	}
	replacement.Start = 2
	events[0].Text = "mutated"
	if state.textInput[0].Replacement.Start != 0 || state.textInput[0].Text != "new" {
		t.Fatal("input snapshot retained borrowed IME event storage")
	}
	d.frame.TextInput = []TextInputEvent{{Kind: TextInputCompose, Text: string([]byte{0xff})}}
	before := item.Text
	if err := ctx.Update(); err == nil || item.Text != before {
		t.Fatal("invalid IME UTF-8 partially changed UI state")
	}
}

func TestIMECandidateThroughTabsScrollingAndNoScaleWindows(t *testing.T) {
	ctx := newTestContext(t)

	for _, noScale := range []bool{false, true} {
		t.Run(fmt.Sprintf("NoScale=%v", noScale), func(t *testing.T) {
			win, d, _ := ctx.interactionFixture(t)
			win.NoScale = noScale
			win.Size = Point{X: 600, Y: 400}
			ctx.SetUIScale(2)
			item, _ := ctx.NewTextArea()
			item.Position = Point{}
			item.Size = Point{X: 180, Y: 70}
			item.Label = "Edit"
			item.UpdateText("first\nsecond\nlast")
			spacer := ctx.layoutBox(180, 120)
			body := ctx.NewColumn(spacer, item)
			body.Fixed = true
			body.Scrollable = true
			body.Size = Point{X: 200, Y: 110}
			body.ContentPadding = Insets{Left: 6, Top: 6, Right: 6, Bottom: 6}
			body.Gap = 10
			page := ctx.NewColumn(body)
			page.Name = "Active"
			inactive, _ := ctx.NewInput()
			inactivePage := ctx.NewColumn(inactive)
			inactivePage.Name = "Inactive"
			tabs := ctx.NewColumn()
			tabs.Fixed = true
			tabs.Size = Point{X: 240, Y: 190}
			tabs.Tabs = []*ItemData{page, inactivePage}
			win.AddItem(tabs)
			d.step()
			ctx.Focus(item)
			d.step()
			bounds := item.TextInputBounds()
			clip := intersectRect(item.DrawRect, win.getMainRect())
			if bounds.X1 <= bounds.X0 || bounds.Y1 <= bounds.Y0 || bounds.X0 < clip.X0 || bounds.X1 > clip.X1+.01 || bounds.Y0 < clip.Y0 || bounds.Y1 > clip.Y1+.01 || body.Scroll.Y <= 0 {
				t.Fatalf("caret anchor escaped nested viewport: %v in %v scroll=%v", bounds, clip, body.Scroll)
			}
			if inactive.TextInputBounds() != (Rect{}) {
				t.Fatal("inactive tab exposed an IME anchor")
			}
			d.ime(TextInputEvent{Kind: TextInputCompose, Text: "e\u0301🙂", SelectionStart: 2, SelectionEnd: 3})
			if !item.IsComposing() || item.displayEditCaret() != len([]rune(item.Text))+3 {
				t.Fatal("composition caret used bytes instead of rune offsets")
			}
			bounds = item.TextInputBounds()
			if bounds.X0 < clip.X0 || bounds.X1 > clip.X1+.01 {
				t.Fatal("composed caret escaped clipped viewport")
			}
			item.SetWordWrap(true)
			if item.IsComposing() {
				t.Fatal("external wrap change retained native preedit")
			}
		})
	}
}
