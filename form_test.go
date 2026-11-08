package eui

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFormNumericValidators(t *testing.T) {
	for _, tc := range []struct {
		validate       TextValidator
		valid, invalid []string
	}{
		{Required, []string{"0", "日本"}, []string{"", " \t\n", "\u2003"}},
		{NumberRange(-1, 2), []string{"", "  ", "-1", "2", "1e0", "+.5"}, []string{"x", "NaN", "Inf", "-Inf", "1e999", "-1.01", "2.01", "1,5"}},
		{IntegerRange(-1, 2), []string{"", "-1", "2", " +1 "}, []string{"1.5", "1e0", "NaN", "3", "-2", "9223372036854775808"}},
		{IntegerRange(math.MaxInt64, math.MaxInt64), []string{"9223372036854775807"}, []string{"9223372036854775806"}},
	} {
		for _, value := range tc.valid {
			if got := tc.validate(value); got != "" {
				t.Errorf("%q rejected: %s", value, got)
			}
		}
		for _, value := range tc.invalid {
			if got := tc.validate(value); got == "" {
				t.Errorf("%q accepted", value)
			}
		}
	}
	for _, makeValidator := range []func(){func() { NumberRange(2, 1) }, func() { NumberRange(math.NaN(), 1) }, func() { NumberRange(0, math.Inf(1)) }, func() { IntegerRange(2, 1) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid bounds accepted")
				}
			}()
			makeValidator()
		}()
	}
}

func TestFormApplyIsAtomicAndCancelDiscardsDraftHistory(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	name := ctx.NewFormField("name", "Name", "Original", Required)
	count := ctx.NewFormField("count", "Count", "12", Required, IntegerRange(1, 72))
	form := ctx.NewForm(name, count)
	win.AddItem(form.View())
	d.step()
	ctx.Focus(name.Input())
	events := 0
	name.Input().Handler.Handle = func(UIEvent) { events++ }
	name.Input().ReplaceText("Changed")
	count.Input().ReplaceText("73")
	err := form.Apply()
	var invalid *FormValidationError
	if !errors.As(err, &invalid) || invalid.Fields["count"] == "" || len(form.Errors()) != 1 || ctx.FocusedItem() != count.Input() {
		t.Fatal("invalid Apply lost errors/focus", err)
	}
	if form.Values()["name"] != "Original" || !form.Dirty() {
		t.Fatal("invalid Apply partially committed")
	}
	count.Input().UpdateText("24")
	if count.Error() != "" || !count.ErrorLabel().Invisible {
		t.Fatal("programmatic correction left inline error")
	}
	if err = form.Apply(); err != nil || form.Dirty() || form.Values()["name"] != "Changed" {
		t.Fatal("valid Apply failed", err)
	}
	values, draft := form.Values(), form.Draft()
	values["name"], draft["name"] = "corrupt", "corrupt"
	if form.Values()["name"] != "Changed" || name.Input().Text != "Changed" {
		t.Fatal("snapshot maps were aliased")
	}
	name.Input().ReplaceText("discard")
	name.Input().Undo() // Equal to applied value, with a redo that must be discarded.
	before := events
	form.Cancel()
	if form.Dirty() || name.Input().CanUndo() || name.Input().CanRedo() || events != before || len(form.Errors()) != 0 {
		t.Fatal("Cancel retained draft history or emitted events")
	}
	name.Input().Redo()
	if name.Input().Text != "Changed" {
		t.Fatal("redo recovered cancelled draft")
	}
	count.Input().UpdateText("")
	if count.Error() != "" {
		t.Fatal("fresh draft showed errors before validation")
	}
}

func TestFormLiveValidationIncludesUndoPasteAndComposition(t *testing.T) {
	ctx := newTestContext(t)
	win, d, _ := ctx.interactionFixture(t)
	field := ctx.NewFormField("name", "Name", "", Required)
	form := ctx.NewForm(field)
	win.AddItem(form.View())
	d.step()
	if field.Error() != "" || !field.ErrorLabel().Invisible {
		t.Fatal("untouched field showed error")
	}
	if form.Validate() || field.ErrorLabel().Invisible || ctx.FocusedItem() != field.Input() {
		t.Fatal("validation did not reveal error and focus")
	}
	d.frame.Text = "A"
	d.step()
	if field.Error() != "" {
		t.Fatal("typing retained error")
	}
	d.frame.Text = ""
	d.keys(ebiten.KeyControlLeft, ebiten.KeyZ)
	d.keys()
	if field.Error() == "" {
		t.Fatal("undo did not revalidate")
	}
	ctx.SetClipboard(&memoryClipboard{text: "pasted"})
	d.keys(ebiten.KeyControlLeft, ebiten.KeyV)
	d.keys()
	if field.Error() != "" || field.Input().Text != "pasted" {
		t.Fatal("paste did not correct validation")
	}
	field.Input().editSelect(0, 6)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	if form.Apply() == nil || !field.Input().IsComposing() || form.Values()["name"] != "" || form.Draft()["name"] != "pasted" {
		t.Fatal("Apply consumed composition or committed stale text")
	}
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "日本"})
	if field.Error() != "" || form.Apply() != nil || form.Values()["name"] != "日本" {
		t.Fatal("committed IME text failed validation")
	}
	field.Input().editSelect(0, 2)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	if form.Apply() == nil {
		t.Fatal("identical preedit was applied early")
	}
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "日本"})
	if field.Error() != "" {
		t.Fatal("identical IME commit retained pending-composition error")
	}
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "discard", SelectionEnd: 7})
	if form.Apply() == nil {
		t.Fatal("composition was applied early")
	}
	d.ime(TextInputEvent{Kind: TextInputCancel})
	if field.Error() != "" {
		t.Fatal("composition cancellation retained pending error")
	}
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "discard", SelectionEnd: 7})
	form.Cancel()
	if field.Input().IsComposing() || field.Input().Text != "日本" || form.Dirty() {
		t.Fatal("Cancel did not discard preedit")
	}
}

func TestFormOwnershipCustomRulesAndUnavailableInvalidFields(t *testing.T) {
	ctx := newTestContext(t)
	other := newTestContext(t)
	field := ctx.NewFormField("name", "Name", "ok", Required, func(value string) string {
		if strings.Contains(value, "/") {
			return "Use a plain name."
		}
		return ""
	})
	duplicate := ctx.NewFormField("name", "Other", "", Required)
	foreign := other.NewFormField("other", "Other", "", Required)
	for _, fields := range [][]*FormField{{field, duplicate}, {field, foreign}, {field, nil}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid fields accepted")
				}
			}()
			ctx.NewForm(fields...)
		}()
		if field.form != nil || field.View().Parent != nil {
			t.Fatal("rejected form partially attached fields")
		}
	}
	form := ctx.NewForm(field)
	field.Input().UpdateText("bad/name")
	field.Input().SetVisible(false)
	if form.Apply() == nil || !reflect.DeepEqual(form.Values(), map[string]string{"name": "ok"}) {
		t.Fatal("hidden invalid field silently committed")
	}
	copy := form.Fields()
	copy[0] = nil
	if form.Fields()[0] != field {
		t.Fatal("Fields aliased internal slice")
	}
	form.Cancel()
	if field.Error() != "" || field.Input().Text != "ok" {
		t.Fatal("Cancel did not restore hidden field")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("reused field accepted")
			}
		}()
		ctx.NewForm(field)
	}()
}

func TestModalFormApplyCancelAndClose(t *testing.T) {
	ctx := newTestContext(t)
	_, d, _ := ctx.interactionFixture(t)
	field := ctx.NewFormField("name", "Name", "original", Required)
	form := ctx.NewForm(field)
	win := ctx.NewWindow()
	win.AutoSize, win.Closable = true, true
	apply := ctx.NewActionButton("Apply", func() {
		if form.Apply() == nil {
			win.Close()
		}
	})
	cancel := ctx.NewActionButton("Cancel", win.Close)
	win.DefaultButton, win.CancelButton = apply, cancel
	win.OnClose = form.Cancel
	win.AddItem(ctx.NewColumn(form.View(), ctx.NewRow(apply, cancel)))
	win.MarkOpenModal()
	d.keys()
	field.Input().UpdateText("")
	d.keys(ebiten.KeyEnter)
	d.keys()
	if !win.Open || field.Error() == "" {
		t.Fatal("invalid Enter closed form")
	}
	field.Input().UpdateText("accepted")
	d.keys(ebiten.KeyEnter)
	d.keys()
	if win.Open || form.Values()["name"] != "accepted" {
		t.Fatal("valid Enter did not apply/close")
	}
	win.MarkOpenModal()
	d.keys()
	field.Input().UpdateText("discarded")
	d.keys(ebiten.KeyEscape)
	d.keys()
	if win.Open || field.Input().Text != "accepted" || form.Dirty() {
		t.Fatal("Escape did not cancel draft")
	}
}
