package eui

import (
	"errors"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/gobold"
)

func TestContextsOwnFocusInputThemesFontsAndCallbacks(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	wa, ia, da := a.imeEditor(t, "A")
	wb, ib, db := b.imeEditor(t, "B")
	a.SetUIScale(1.5)
	b.SetUIScale(.75)
	a.SetScreenSize(1400, 900)
	b.SetScreenSize(600, 500)
	if wa.Context() != a || wb.Context() != b || ia.Context() != a || ib.Context() != b {
		t.Fatal("incorrect ownership")
	}
	if err := b.LoadTheme("Paper"); err != nil {
		t.Fatal(err)
	}
	if a.currentTheme == b.currentTheme || a.currentStyle == b.currentStyle || a.currentThemeName == b.currentThemeName {
		t.Fatal("shared appearance state")
	}
	if err := b.EnsureFontSource(gobold.TTF); err != nil {
		t.Fatal(err)
	}
	// Explicitly choose a different source; Ensure preserves an installed font.
	source := a.FontSource()
	b.SetFontSource(a.BoldFontSource())
	if a.FontSource() == b.FontSource() || source != a.FontSource() {
		t.Fatal("font source crossed contexts")
	}
	if a.textFace(12) == b.textFace(12) || a.faceCache[12] == b.faceCache[12] {
		t.Fatal("shared font face cache")
	}
	callsA, callsB := 0, 0
	ia.Handler.Handle = func(UIEvent) { callsA++; b.SetAccentColor(ColorRed) }
	ib.Handler.Handle = func(UIEvent) { callsB++ }
	da.ime(TextInputEvent{Kind: TextInputCompose, Text: "一", SelectionEnd: 1})
	db.ime(TextInputEvent{Kind: TextInputCompose, Text: "二", SelectionEnd: 1})
	if !ia.IsComposing() || !ib.IsComposing() || a.composingItem != ia || b.composingItem != ib {
		t.Fatal("preedit ownership crossed contexts")
	}
	da.ime(TextInputEvent{Kind: TextInputCommit, Text: "一"})
	db.ime(TextInputEvent{Kind: TextInputCommit, Text: "二"})
	if ia.Text != "A一" || ib.Text != "B二" || callsA != 1 || callsB != 1 || a.FocusedItem() != ia || b.FocusedItem() != ib {
		t.Fatal("interleaved input/callback state crossed contexts")
	}
	if a.AccentColor() == b.AccentColor() {
		t.Fatal("callback changed another context's accent")
	}
	a.ShowContextMenu([]string{"A menu"}, 1, 2, nil)
	if !a.ContextMenusOpen() || b.ContextMenusOpen() {
		t.Fatal("shared menu state")
	}
	a.CloseContextMenus()
	a.SetClipboard(&memoryClipboard{text: "A clipboard"})
	b.SetClipboard(&memoryClipboard{text: "B clipboard"})
	ca, _ := a.readClipboard()
	cb, _ := b.readClipboard()
	if ca == cb {
		t.Fatal("shared clipboard adapter")
	}
	a.SetInputEnabled(false)
	da.frame.Text = "discarded"
	da.step()
	db.frame.Text = "!"
	db.step()
	if ia.Text != "A一" || ib.Text != "B二!" || a.FocusedItem() != ia {
		t.Fatal("disabled input leaked or lost retained focus")
	}
}
func TestContextRejectsForeignControlsBeforeChangingTrees(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	wa, wb := a.NewWindow(), b.NewWindow()
	own, foreign := a.NewLabel("own"), b.NewLabel("foreign")
	wa.AddItem(own)
	wb.AddItem(foreign)
	wa.MarkOpen()
	wb.MarkOpen()
	snapshot := a.Windows()
	snapshot[0] = wb
	if a.Windows()[0] != wa {
		t.Fatal("window list exposed mutable internal storage")
	}
	rejected := func(action func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("foreign context operation was accepted")
			}
		}()
		action()
	}
	rejected(func() { wa.AddItem(foreign) })
	rejected(func() { own.SetItems([]*ItemData{foreign}) })
	rejected(func() { a.Focus(foreign) })
	wa.DefaultButton = foreign
	rejected(func() { _ = a.Update() })
	wa.DefaultButton = nil
	rejected(func() { a.LayoutWindowBody(wb, foreign, foreign) })
	if len(wa.Contents) != 1 || wa.Contents[0] != own || foreign.ParentWindow != wb || len(own.Contents) != 0 {
		t.Fatal("rejected operation partially changed trees")
	}
	raw := &ItemData{ItemType: ITEM_FLOW, Contents: []*ItemData{{ItemType: ITEM_TEXT, Text: "raw child"}}}
	wa.AddItem(raw)
	if raw.Context() != a || raw.Contents[0].Context() != a {
		t.Fatal("unowned configuration was not adopted recursively")
	}
}
func TestContextDisposeRetiresOwnedDetachedHandlesAndQueues(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	wa := a.NewWindow()
	ia, handler := a.NewInput()
	wa.AddItem(ia)
	wa.MarkOpen()
	queue := handler.EnableQueue(2)
	handler.Emit(UIEvent{Type: EventInputChanged, Text: "buffered"})
	wa.RemoveItem(ia)
	bwin, bi, driver := b.imeEditor(t, "live")
	a.Dispose()
	a.Dispose()
	if !a.IsDisposed() || wa.Open || ia.Render != nil || a.whiteImage != nil {
		t.Fatal("dispose did not retire resources")
	}
	if event, ok := <-queue; !ok || event.Text != "buffered" {
		t.Fatal("dispose discarded a buffered event")
	}
	if _, ok := <-queue; ok {
		t.Fatal("dispose did not close detached control's queue")
	}
	callbacks := 0
	handler.Handle = func(UIEvent) { callbacks++ }
	handler.Emit(UIEvent{})
	if callbacks != 0 {
		t.Fatal("retired handler emitted a callback")
	}
	if err := a.Update(); !errors.Is(err, ErrContextDisposed) {
		t.Fatal("disposed Update did not return its lifecycle error")
	}
	driver.frame.Text = "!"
	driver.step()
	if bi.Text != "live!" || !bwin.Open || b.IsDisposed() {
		t.Fatal("disposing one context affected another")
	}
}
func TestContextDisposeDuringUpdateAndCrossContextCallback(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	_, item, d := a.imeEditor(t, "")
	reentered := error(nil)
	item.Handler.Handle = func(UIEvent) { reentered = a.Update(); b.Dispose(); a.Dispose() }
	d.frame.Text = "done"
	d.frame.Time = d.frame.Time.Add(time.Second / 60)
	err := a.Update()
	if !errors.Is(reentered, ErrContextReentry) || !errors.Is(err, ErrContextDisposed) || !a.IsDisposed() || !b.IsDisposed() {
		t.Fatalf("reentrant lifecycle failed: %v / %v", reentered, err)
	}
}
func TestNativeInputLeaseIsReleasedOnlyByItsOwner(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	_, item, _ := a.imeEditor(t, "")
	a.configureNativeTextInput(item)
	nativeIMELease = a
	b.cancelNativeTextInput(nil)
	if nativeIMELease != a {
		t.Fatal("another context released the OS session")
	}
	if handled, err := b.pollNativeTextInput(nil); !handled || err != nil {
		t.Fatal("competing native session was not suppressed")
	}
	a.SetInputEnabled(false)
	if nativeIMELease != nil {
		t.Fatal("disabling native owner did not release its session")
	}
}

func TestContextsKeepNamedColorsAndEditableRootsIndependent(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	a.SetUserDataRoot(t.TempDir())
	b.SetUserDataRoot(t.TempDir())
	a.SetAccentColor(ColorRed)
	b.SetAccentColor(ColorBlue)
	red, err := a.ParseColor([]byte(`"accent"`))
	if err != nil {
		t.Fatal(err)
	}
	blue, err := b.ParseColor([]byte(`"accent"`))
	if err != nil {
		t.Fatal(err)
	}
	if red == blue || red != a.AccentColor() || blue != b.AccentColor() {
		t.Fatal("named colors crossed contexts")
	}
	var literal Color
	if literal.UnmarshalJSON([]byte(`"accent"`)) == nil {
		t.Fatal("literal decoder consulted a live context")
	}
	fileA, err := a.EditableThemeFile("AccentDark", false)
	if err != nil {
		t.Fatal(err)
	}
	fileB, err := b.EditableThemeFile("AccentDark", false)
	if err != nil {
		t.Fatal(err)
	}
	if fileA == fileB || a.themeDirectory == b.themeDirectory || a.themePath == b.themePath {
		t.Fatal("editable roots crossed contexts")
	}
	a.AutoReload = true
	if b.AutoReload {
		t.Fatal("reload configuration crossed contexts")
	}
	before := b.AccentColor()
	if err := a.ValidateThemeSource([]byte(`{"Colors":{"accent":"#00ff00"},"Button":{"Color":"accent"},"Window":{"Title":"accent"}}`)); err != nil {
		t.Fatal(err)
	}
	if a.AccentColor() != red || b.AccentColor() != before {
		t.Fatal("validation changed an instance's live colors")
	}
}

func TestDisabledContextDoesNotOverwriteActiveOSCursor(t *testing.T) {
	a, b := newTestContext(t), newTestContext(t)
	_, _, da := a.imeEditor(t, "")
	_, _, db := b.imeEditor(t, "")
	previous := ebiten.CursorShape()
	defer ebiten.SetCursorShape(previous)
	b.cursorShape = ebiten.CursorShapeText
	b.SetInputEnabled(false)
	ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
	db.step()
	if ebiten.CursorShape() != ebiten.CursorShapeCrosshair {
		t.Fatal("disabled context changed the physical cursor")
	}
	da.frame.PointerX, da.frame.PointerY = -100, -100
	da.step()
	if ebiten.CursorShape() != ebiten.CursorShapeDefault || a.CursorShape() != ebiten.CursorShapeDefault {
		t.Fatal("active context trusted a stale per-instance OS cursor cache")
	}
}
