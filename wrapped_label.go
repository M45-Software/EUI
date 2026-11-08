package eui

import "strings"

type wrappedLabelLayout struct {
	raw    string
	config textWindowWrapConfig
}

// NewWrappedLabel creates a label that wraps to width (in logical pixels) and
// grows vertically. It reflows when the width, font, or UI scale changes.
// Use SetWrappedText to change its content; Text contains the rendered lines.
func (ctx *Context) NewWrappedLabel(value string, width float32) *ItemData {
	ctx.mustAlive()

	item := ctx.NewLabel("")
	item.Size.X = width
	item.SetWrappedText(value)
	return item
}

// SetWrappedText updates a wrapped label, preserving the original paragraphs
// so widening the label can join previously wrapped lines again.
func (item *itemData) SetWrappedText(value string) {
	if item == nil {
		return
	}
	if item.wrappedLabel != nil && item.wrappedLabel.raw == value {
		return
	}
	item.wrappedLabel = &wrappedLabelLayout{raw: value}
	item.layoutWrappedLabel()
	item.runtimeChanged(true)
}

func (item *itemData) layoutWrappedLabel() {
	ctx := item.ui()

	if item.wrappedLabel == nil || item.ItemType != ITEM_TEXT || item.Size.X <= 0 {
		return
	}
	fontSize := item.FontSize
	if fontSize <= 0 {
		fontSize = 12
	}
	width := item.constrainSize(Point{X: item.Size.X * ctx.UIScale()}).X
	if item.layoutWrapWidth > 0 {
		width = item.layoutWrapWidth
	}
	config := textWindowWrapConfig{
		width: float64(width), faceSize: float64(fontSize*ctx.UIScale() + 2), source: ctx.FontSource(),
	}
	if item.wrappedLabel.config == config {
		return
	}
	_, lines := WrapText(item.wrappedLabel.raw, ctx.textFace(float32(config.faceSize)), config.width)
	item.Text = strings.Join(lines, "\n")
	item.wrappedLabel.config = config
	item.Dirty = true
}
