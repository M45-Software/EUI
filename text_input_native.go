package eui

import (
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
)

// Ebitengine exposes one native text-input session for its OS window. This
// lease coordinates that physical resource; documents, focus and composition
// state remain owned by their contexts. Hosts route native input explicitly.
var nativeIMELease *Context

type imeContext struct {
	snapshot   textEditSnapshot
	start, end int
}

func (ctx *Context) cancelNativeTextInput(item *itemData) {
	n := &ctx.nativeTextInput
	if n.owner == nil {
		if nativeIMELease == ctx {
			nativeIMELease = nil
		}
		return
	}
	if item != nil && n.owner != item {
		return
	}
	n.owner.clearComposition()
	if n.pumping {
		n.cancelPending = true
		return
	}
	n.composer.Cancel()
	if nativeIMELease == ctx {
		nativeIMELease = nil
	}
	n.owner = nil
	n.context = imeContext{}
	n.geometry = inputGeometry{}
	n.composer = textinput.Composer{}
	n.cancelPending = false
}
func (ctx *Context) pollNativeTextInput(item *itemData) (handled bool, err error) {
	if nativeIMELease != nil && nativeIMELease != ctx {
		return true, nil
	}
	n := &ctx.nativeTextInput
	if n.owner != nil && (n.owner != item || n.geometry != item.inputGeometry() || n.context.snapshot != item.editSnapshot()) {
		ctx.cancelNativeTextInput(nil)
	}
	if n.owner == nil {
		ctx.configureNativeTextInput(item)
	}
	nativeIMELease = ctx
	n.pumping = true
	handled, err = n.composer.Update()
	n.pumping = false
	if err != nil || n.cancelPending || !ctx.textInputTargetAvailable(item) {
		ctx.cancelNativeTextInput(nil)
		handled = true
	}
	return handled, err
}
func (ctx *Context) configureNativeTextInput(item *itemData) {
	n := &ctx.nativeTextInput
	n.owner = item
	n.composer = textinput.Composer{
		OnNewSession: func() *textinput.SessionOptions {
			if n.cancelPending || !ctx.textInputTargetAvailable(n.owner) {
				return nil
			}
			target := n.owner
			bounds := target.TextInputBounds()
			if bounds.X1 <= bounds.X0 || bounds.Y1 <= bounds.Y0 {
				return nil
			}
			n.geometry = target.inputGeometry()
			n.context = imeContext{snapshot: target.editSnapshot()}
			runes := []rune(target.editText())
			start, end := target.editSelection()
			if start == end {
				start, end = target.CursorPos, target.CursorPos
			}
			lineStart, lineEnd := start, end
			for lineStart > 0 && runes[lineStart-1] != '\n' {
				lineStart--
			}
			for lineEnd < len(runes) && runes[lineEnd] != '\n' {
				lineEnd++
			}
			n.context.start, n.context.end = lineStart, lineEnd
			opts := &textinput.SessionOptions{CaretBounds: imeImageBounds(bounds)}
			if !target.HideText {
				opts.TextBeforeCaret = string(runes[lineStart:start])
				opts.TextAfterCaret = string(runes[end:lineEnd])
			}
			return opts
		},
		OnComposition: func(c *textinput.Composition) {
			if n.cancelPending || !ctx.nativeIMEContextValid() {
				return
			}
			start, end := c.SelectionRangeInBytes()
			value := c.Text()
			n.owner.setComposition(value, runesBeforeByte(value, start), runesBeforeByte(value, end))
		},
		OnCommit: func(c *textinput.Commit) {
			before, after := c.IsSurroundingTextReplaced()
			prefix, suffix := c.SurroundingText()
			ctx.applyNativeTextCommit(c.Text(), prefix, suffix, before || after)
		},
		OnEndByUser: func() {
			if n.owner != nil {
				ctx.endTextInput(n.owner)
			}
		},
	}
}
func (ctx *Context) applyNativeTextCommit(value, prefix, suffix string, replaced bool) {
	n := &ctx.nativeTextInput
	if n.cancelPending || !ctx.nativeIMEContextValid() {
		n.cancelPending = true
		return
	}
	target := n.owner
	if replaced && !target.HideText {
		value = target.normalizeEditText(value)
		if target.textLimit > 0 {
			outside := utf8.RuneCountInString(target.editText()) - (n.context.end - n.context.start) + utf8.RuneCountInString(prefix+suffix)
			runes := []rune(value)
			value = string(runes[:min(len(runes), max(0, target.textLimit-outside))])
		}
		caret := n.context.start + utf8.RuneCountInString(prefix+value)
		target.commitIMEEdit(n.context.start, n.context.end, prefix+value+suffix, caret)
	} else {
		target.applyIMECommit(value, nil)
	}
}
func (ctx *Context) nativeIMEContextValid() bool {
	n := &ctx.nativeTextInput
	return ctx.textInputTargetAvailable(n.owner) && n.owner.editSnapshot() == n.context.snapshot
}
func runesBeforeByte(value string, offset int) int {
	offset = max(0, min(offset, len(value)))
	for offset > 0 && !utf8.ValidString(value[:offset]) {
		offset--
	}
	return utf8.RuneCountInString(value[:offset])
}
func (win *windowData) ensureSearchInput() *itemData {
	ctx := win.ui()

	if win.searchInput == nil {
		item, handler := ctx.NewInput()
		item.Position = Point{}
		item.ParentWindow = win
		item.searchOwner = win
		item.textLimit = 64
		handler.Handle = func(event UIEvent) {
			if event.Type != EventInputChanged {
				return
			}
			win.SearchText = event.Text
			win.markDirty()
			if win.OnSearch != nil {
				win.OnSearch(event.Text)
			}
		}
		win.searchInput = item
	}
	item := win.searchInput
	if item.Text != win.SearchText {
		item.UpdateText(win.SearchText)
		item.setEditSelection(len([]rune(item.Text)), len([]rune(item.Text)))
	}
	item.Focused = ctx.activeSearch == win
	item.FontSize = max(1, (win.GetTitleSize()/2-2)/ctx.uiScale)
	item.Padding, item.BorderPad = 0, 0
	item.Face = nil
	return item
}
func (win *windowData) searchDisplayText() string {
	if win.searchInput != nil {
		return win.searchInput.displayEditText()
	}
	return win.SearchText
}
func (win *windowData) searchInputRect() rect {
	bounds := win.searchBoxRect()
	bounds.X1 = min(bounds.X1, win.searchCloseRect().X0)
	return bounds
}
