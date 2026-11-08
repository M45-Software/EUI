package eui

import (
	"image"
	"math"
	"strings"
	"unicode/utf8"
)

// TextInputKind describes an ordered IME notification in an InputFrame.
type TextInputKind uint8

const (
	TextInputCompose TextInputKind = iota
	TextInputCommit
	TextInputCancel
	TextInputEnd
)

// TextReplacement replaces a document rune range. Caret is a rune offset in
// the normalized replacement text, or -1 for its end. Native adapters use this
// for reconversion, autocorrection, and surrounding-text edits.
type TextReplacement struct{ Start, End, Caret int }

// TextInputEvent carries preedit or committed text. SelectionStart/End are rune
// offsets within composition Text. A nil Replacement commits at the selection.
// Target, when set, prevents late events from reaching another focused control.
// Passthrough permits the triggering key to continue after this notification.
// Preedit never changes Text, bindings, undo history, or InputChanged events.
type TextInputEvent struct {
	Kind                         TextInputKind
	Text                         string
	SelectionStart, SelectionEnd int
	Replacement                  *TextReplacement
	Target                       *ItemData
	Passthrough                  bool
}

type editComposition struct {
	display, displaySource       string
	displayReady, displayMasked  bool
	geometry                     inputGeometry
	text                         string
	selectionStart, selectionEnd int
	start, end                   int
	snapshot                     textEditSnapshot
}

func (ctx *Context) textInputTarget() *itemData {
	if ctx.keyboardInputCaptured || ctx.sampledInput.unfocused || ctx.activeContextMenu() != nil {
		return nil
	}
	if ctx.activeSearch != nil && ctx.activeSearch.Open && ctx.activeSearch.searchOpen {
		return ctx.activeSearch.ensureSearchInput()
	}
	if focusAvailable(ctx.focusedItem) && itemHandlesTextEditing(ctx.focusedItem) {
		return ctx.focusedItem
	}
	return nil
}
func (ctx *Context) textInputTargetAvailable(item *itemData) bool {
	return item != nil && ctx.textInputTarget() == item
}

// IsComposing reports whether the control has uncommitted inline preedit.
func (item *itemData) IsComposing() bool { return item.activeComposition() != nil }

// CancelComposition discards preedit without changing committed text or history.
// It also releases any native session, even when that session is currently idle.
func (item *itemData) CancelComposition() {
	ctx := item.ui()

	if item == nil {
		return
	}
	item.clearComposition()
	ctx.cancelNativeTextInput(item)
}
func (item *itemData) clearComposition() {
	ctx := item.ui()

	if item == nil {
		return
	}
	if ctx.composingItem == item {
		ctx.composingItem = nil
	}
	if item.textEdit == nil || item.textEdit.composition == nil {
		return
	}
	item.textEdit.composition = nil
	item.textEdit.followCaret = true
	invalidateFocus(item)
	item.validateFormEdit()
}
func (item *itemData) activeComposition() *editComposition {
	ctx := item.ui()

	if item == nil || item.textEdit == nil || item.textEdit.composition == nil {
		return nil
	}
	c := item.textEdit.composition
	if !ctx.textInputTargetAvailable(item) || c.snapshot.text != item.editText() || c.snapshot.cursor != item.CursorPos || c.snapshot.anchor != item.SelectStart || c.snapshot.end != item.SelectEnd {
		item.CancelComposition()
		return nil
	}
	return c
}
func (item *itemData) setComposition(value string, start, end int) {
	ctx := item.ui()

	if value == "" {
		item.clearComposition()
		return
	}
	state := item.editor()
	n := utf8.RuneCountInString(value)
	start, end = max(0, min(start, n)), max(0, min(end, n))
	old := item.activeComposition()
	if old != nil && old.text == value && old.selectionStart == start && old.selectionEnd == end {
		return
	}
	c := &editComposition{text: value, selectionStart: start, selectionEnd: end}
	if old != nil {
		c.start, c.end, c.snapshot, c.geometry = old.start, old.end, old.snapshot, old.geometry
	} else {
		c.start, c.end = item.editSelection()
		if c.start == c.end {
			c.start, c.end = item.CursorPos, item.CursorPos
		}
		c.snapshot = item.editSnapshot()
		c.geometry = item.inputGeometry()
	}
	state.composition = c
	state.followCaret = true
	state.caretOn = true
	ctx.composingItem = item
	invalidateFocus(item)
}
func (item *itemData) displayEditText() string {
	c := item.activeComposition()
	if c == nil {
		return item.Text
	}
	if c.displayReady && c.displaySource == item.Text && c.displayMasked == item.HideText {
		return c.display
	}
	before := []rune(item.Text)
	value := c.text
	if item.HideText {
		value = strings.Repeat("*", utf8.RuneCountInString(value))
	}
	c.display = string(before[:c.start]) + value + string(before[c.end:])
	c.displaySource, c.displayReady, c.displayMasked = item.Text, true, item.HideText
	return c.display
}
func (item *itemData) displayEditCaret() int {
	if c := item.activeComposition(); c != nil {
		return c.start + c.selectionEnd
	}
	return item.CursorPos
}
func (item *itemData) displayEditSelection() (int, int) {
	if c := item.activeComposition(); c != nil {
		return c.start + min(c.selectionStart, c.selectionEnd), c.start + max(c.selectionStart, c.selectionEnd)
	}
	return item.editSelection()
}
func (item *itemData) applyIMECommit(value string, replacement *TextReplacement) {
	value = item.normalizeEditText(value)
	start, end := item.editSelection()
	if start == end {
		start, end = item.CursorPos, item.CursorPos
	}
	if replacement != nil {
		n := utf8.RuneCountInString(item.editText())
		if replacement.Start < 0 || replacement.End < replacement.Start || replacement.End > n {
			item.CancelComposition()
			return
		}
		start, end = replacement.Start, replacement.End
	}
	if item.textLimit > 0 {
		runes := []rune(value)
		available := max(0, item.textLimit-utf8.RuneCountInString(item.editText())+end-start)
		value = string(runes[:min(len(runes), available)])
	}
	caret := start + utf8.RuneCountInString(value)
	if replacement != nil && replacement.Caret >= 0 {
		caret = start + min(replacement.Caret, utf8.RuneCountInString(value))
	}
	item.commitIMEEdit(start, end, value, caret)
}

func (item *itemData) commitIMEEdit(start, end int, value string, caret int) {
	item.clearComposition()
	if !item.editReplaceAt(start, end, value, "", caret, true) {
		item.setEditSelection(caret, caret)
	}
}

func (ctx *Context) pollTextInput() (handled, suppressText bool, err error) {
	target := ctx.textInputTarget()
	if ctx.composingItem != nil && (ctx.composingItem != target || ctx.composingItem.textEdit.composition.geometry != ctx.composingItem.inputGeometry()) {
		ctx.composingItem.CancelComposition()
	}
	if target == nil {
		ctx.cancelNativeTextInput(nil)
		return false, false, nil
	}
	if ctx.inputSource == nil {
		handled, err = ctx.pollNativeTextInput(target)
		return handled, true, err
	}
	ctx.cancelNativeTextInput(nil)
	for _, event := range ctx.sampledInput.textInput {
		if !event.Passthrough {
			handled = true
		}
		if ctx.textInputTarget() != target {
			break
		}
		if event.Target != nil && event.Target != target {
			continue
		}
		switch event.Kind {
		case TextInputCompose:
			target.setComposition(event.Text, event.SelectionStart, event.SelectionEnd)
		case TextInputCommit:
			target.applyIMECommit(event.Text, event.Replacement)
		case TextInputCancel:
			target.CancelComposition()
		case TextInputEnd:
			ctx.endTextInput(target)
		}
	}
	composing := target.IsComposing()
	return handled || composing, len(ctx.sampledInput.textInput) > 0 || composing, nil
}
func (ctx *Context) endTextInput(item *itemData) {
	if item.searchOwner != nil {
		if ctx.activeSearch == item.searchOwner {
			ctx.activeSearch = nil
		}
		item.Focused = false
		item.CancelComposition()
		item.ParentWindow.markDirty()
	} else {
		ctx.ClearFocus(item)
	}
}

// TextInputBounds returns the visible caret rectangle in Ebitengine logical
// screen pixels, including composition, wrapping, scrolling, and labels. Empty
// means the control is unavailable, uses external text editing, or has no
// visible text viewport. A selected range anchors at its start.
func (item *itemData) TextInputBounds() Rect {
	ctx := item.ui()

	if item == nil || !itemHandlesTextEditing(item) || item.ParentWindow == nil || !item.ParentWindow.Open {
		return Rect{}
	}
	geometry := item.inputGeometry()
	if geometry.clip.X1 <= geometry.clip.X0 || geometry.clip.Y1 <= geometry.clip.Y0 {
		return Rect{}
	}
	item.setInputDrawGeometry(geometry.bounds)
	viewport, origin := item.editGeometry()
	item.followEditCaret(viewport)
	viewport, origin = item.editGeometry()
	layout := item.editLayout()
	caret := item.displayEditCaret()
	if !item.IsComposing() {
		if start, end := item.editSelection(); start != end {
			caret = start
		}
	}
	line, x := layout.caretAt(caret, false)
	y := origin.Y + float32(line)*layout.lineHeight
	visible := intersectRect(viewport, geometry.clip)
	if visible.X1 <= visible.X0 || visible.Y1 <= visible.Y0 {
		return Rect{}
	}
	left := max(visible.X0, min(origin.X+x, visible.X1-1))
	top := max(visible.Y0, min(y, visible.Y1-1))
	return Rect{X0: left, Y0: top, X1: min(visible.X1, left+max(1, ctx.uiScale)), Y1: min(visible.Y1, top+layout.lineHeight)}
}
func (item *itemData) setInputDrawGeometry(bounds rect) {
	ctx := item.ui()

	item.textDrawOrigin = Point{X: bounds.X0, Y: bounds.Y0}
	item.textDrawSize = Point{X: bounds.X1 - bounds.X0, Y: bounds.Y1 - bounds.Y0}
	if item.Label != "" {
		height := item.FontSize*ctx.uiScale + 2 + ctx.currentStyle.TextPadding*ctx.uiScale
		item.textDrawOrigin.Y += height
		item.textDrawSize.Y = max(0, item.textDrawSize.Y-height)
	}
}

type inputGeometry struct {
	bounds, clip rect
	scale, font  float32
}

func (item *itemData) inputGeometry() inputGeometry {
	ctx := item.ui()

	win := item.ParentWindow
	if win == nil {
		return inputGeometry{}
	}
	if item.searchOwner != nil {
		bounds := win.searchInputRect()
		return inputGeometry{bounds: bounds, clip: bounds, scale: ctx.uiScale, font: item.FontSize}
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	base := pointAdd(win.getPosition(), Point{X: pad, Y: win.GetTitleSize() + pad})
	base = pointSub(base, win.Scroll)
	var walk func(*itemData, Point, rect) (inputGeometry, bool)
	walk = func(node *itemData, pos Point, clip rect) (inputGeometry, bool) {
		if node == nil || node.Invisible || node.Disabled {
			return inputGeometry{}, false
		}
		size := node.GetSize()
		bounds := rect{X0: pos.X, Y0: pos.Y, X1: pos.X + size.X, Y1: pos.Y + size.Y}
		if node == item {
			return inputGeometry{bounds: bounds, clip: intersectRect(bounds, clip), scale: ctx.uiScale, font: item.FontSize}, true
		}
		if node.ItemType == ITEM_FLOW {
			origin, area := node.layoutBody(size)
			clip = intersectRect(clip, rect{X0: pos.X + origin.X, Y0: pos.Y + origin.Y, X1: pos.X + origin.X + area.X, Y1: pos.Y + origin.Y + area.Y})
			if node.Outlined {
				border := node.Border * ctx.uiScale
				if border <= 0 {
					border = ctx.uiScale
				}
				border = min(border, min(size.X, size.Y)/2)
				clip = intersectRect(clip, rect{X0: bounds.X0 + border, Y0: bounds.Y0 + border, X1: bounds.X1 - border, Y1: bounds.Y1 - border})
			}
		}
		for child, relative := range node.layoutChildren(size) {
			childPos := pointAdd(pointSub(pos, node.Scroll), relative)
			if child.ItemType != ITEM_FLOW && child.PinTo != PIN_TOP_LEFT {
				childPos = pointAdd(base, child.getPosition(win))
			}
			if geometry, found := walk(child, childPos, clip); found {
				return geometry, true
			}
		}
		return inputGeometry{}, false
	}
	for _, root := range win.Contents {
		if geometry, found := walk(root, pointAdd(base, root.getPosition(win)), win.getMainRect()); found {
			return geometry
		}
	}
	return inputGeometry{}
}
func imeImageBounds(bounds Rect) image.Rectangle {
	return image.Rect(int(math.Floor(float64(bounds.X0))), int(math.Floor(float64(bounds.Y0))), int(math.Ceil(float64(bounds.X1))), int(math.Ceil(float64(bounds.Y1))))
}
