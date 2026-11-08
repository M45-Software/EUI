package eui

import (
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// initFonts installs the embedded fonts when a context is constructed.
func (ctx *Context) initFonts() error {
	ctx.mustAlive()

	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		return err
	}
	return ctx.EnsureBoldFontSource(gobold.TTF)
}

// NewColumn arranges children vertically. Set Size and Fixed or Scrollable on
// the returned item when the container needs a fixed viewport instead of auto
// sizing. ConstrainToSize keeps a non-scrollable flow at its declared size.
func (ctx *Context) NewColumn(children ...*ItemData) *ItemData {
	ctx.mustAlive()

	item := &ItemData{ctx: ctx, ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	item.SetItems(children)
	return ctx.registerItem(item)
}

// NewRow arranges children horizontally.
func (ctx *Context) NewRow(children ...*ItemData) *ItemData {
	ctx.mustAlive()

	item := &ItemData{ctx: ctx, ItemType: ITEM_FLOW, FlowType: FLOW_HORIZONTAL}
	item.SetItems(children)
	return ctx.registerItem(item)
}

// NewOverlay stacks children in one coordinate space. Set each child's
// Position to place it within the container; later children draw and receive
// input above earlier children.
func (ctx *Context) NewOverlay(children ...*ItemData) *ItemData {
	ctx.mustAlive()

	item := &ItemData{ctx: ctx, ItemType: ITEM_FLOW, FlowType: FLOW_OVERLAY}
	item.SetItems(children)
	return ctx.registerItem(item)
}

// NewLabel creates a compact, automatically measured label using theme colors.
func (ctx *Context) NewLabel(label string) *ItemData {
	ctx.mustAlive()

	item, _ := ctx.NewText()
	item.Text = label
	item.FontSize = 12
	item.Face = nil
	item.Size = Point{}
	return ctx.registerItem(item)
}

// NewActionButton creates a themed button with an optional click callback.
// Use NewButton when access to the full event handler is needed.
func (ctx *Context) NewActionButton(label string, action func()) *ItemData {
	ctx.mustAlive()

	item, events := ctx.NewButton()
	item.Text = label
	item.Size = Point{X: max(128, ctx.NewLabel(label).GetSize().X/ctx.UIScale()+24), Y: 28}
	events.Handle = func(ev UIEvent) {
		if ev.Type == EventClick && action != nil {
			action()
		}
	}
	return ctx.registerItem(item)
}
