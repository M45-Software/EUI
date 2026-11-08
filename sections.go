package eui

import text "github.com/hajimehoshi/ebiten/v2/text/v2"

// ApplyBoldFace sets a bold text face for the given item based on its current
// FontSize and the active UI scale, so it renders as a bold section label.
func (ctx *Context) ApplyBoldFace(it *ItemData) {
	ctx.mustAlive()
	ctx.requireItem(it)

	if it == nil {
		return
	}
	sz := float64(it.FontSize*ctx.UIScale() + 2)
	if src := ctx.BoldFontSource(); src != nil {
		it.Face = &text.GoTextFace{Source: src, Size: sz}
	} else {
		it.Face = &text.GoTextFace{Size: sz}
	}
}

// NewSection keeps configuration windows visually consistent.
// Each section owns its controls, with enough space above the heading to make
// neighboring groups easy to scan without adding decorative UI machinery.
func (ctx *Context) NewSection(title string, width float32) *ItemData {
	ctx.mustAlive()

	section := ctx.NewColumn()
	section.Size = Point{X: width, Y: 10}

	spacer, _ := ctx.NewText()
	spacer.Size = Point{X: width, Y: 10}
	section.AddItem(spacer)

	heading, _ := ctx.NewText()
	heading.Text = title
	heading.FontSize = 15
	heading.Size = Point{X: width, Y: 30}
	ctx.ApplyBoldFace(heading)
	section.AddItem(heading)
	return section
}

// NewSubheading creates a smaller bold heading for a configuration section.
func (ctx *Context) NewSubheading(title string, width float32) *ItemData {
	ctx.mustAlive()

	heading, _ := ctx.NewText()
	heading.Text = title
	heading.FontSize = 12
	heading.Size = Point{X: width, Y: 24}
	ctx.ApplyBoldFace(heading)
	return heading
}
