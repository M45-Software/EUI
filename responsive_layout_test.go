package eui

import (
	"math"
	"strings"
	"testing"
)

func (ctx *Context) layoutBox(w, h float32) *ItemData {
	item, _ := ctx.NewButton()
	item.Text = ""
	item.Position = Point{}
	item.Size = Point{X: w, Y: h}
	return item
}
func nearLayout(t *testing.T, got, want Point) {
	t.Helper()
	if math.Abs(float64(got.X-want.X)) > .01 || math.Abs(float64(got.Y-want.Y)) > .01 {
		t.Fatalf("layout size/position=%v, want %v", got, want)
	}
}

func TestResponsiveFillSharesRespectMinMaxAndKeepPreferences(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	for _, scale := range []float32{1, 1.5, 2} {
		ctx.uiScale = scale
		a, b, c := ctx.layoutBox(40, 20), ctx.layoutBox(40, 20), ctx.layoutBox(40, 20)
		a.FillWidth, b.FillWidth, c.FillWidth = true, true, true
		a.MinSize.X = 180
		b.MaxSize.X = 60
		row := ctx.NewRow(a, b, c)
		row.Fixed = true
		row.Size = Point{X: 400, Y: 60}
		row.Gap = 10
		row.resizeFlow(Point{})
		nearLayout(t, a.GetSize(), Point{X: 180 * scale, Y: 20 * scale})
		nearLayout(t, b.GetSize(), Point{X: 60 * scale, Y: 20 * scale})
		nearLayout(t, c.GetSize(), Point{X: 140 * scale, Y: 20 * scale})
		if a.Size.X != 40 || row.Size != (Point{X: 400, Y: 60}) {
			t.Fatal("layout overwrote preferred size")
		}
		row.Size.X = 240
		row.resizeFlow(Point{})
		nearLayout(t, a.GetSize(), Point{X: 180 * scale, Y: 20 * scale})
		nearLayout(t, b.GetSize(), Point{X: 20 * scale, Y: 20 * scale})
		nearLayout(t, c.GetSize(), Point{X: 20 * scale, Y: 20 * scale})
	}
}

func TestResponsiveGapPaddingAlignmentAndHiddenChildren(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	a, b, hidden := ctx.layoutBox(40, 20), ctx.layoutBox(60, 30), ctx.layoutBox(100, 100)
	hidden.Invisible = true
	row := ctx.NewRow(a, hidden, b)
	row.Fixed = true
	row.Size = Point{X: 300, Y: 100}
	row.SetContentPadding(Insets{Left: 10, Top: 10, Right: 20, Bottom: 20})
	row.SetFlowLayout(10, LayoutCenter, LayoutEnd)
	row.resizeFlow(Point{})
	positions := map[*ItemData]Point{}
	for child, pos := range row.layoutChildren(row.GetSize()) {
		positions[child] = pos
	}
	nearLayout(t, positions[a], Point{X: 90, Y: 60})
	nearLayout(t, positions[b], Point{X: 140, Y: 50})
	if len(positions) != 2 {
		t.Fatal("hidden child reserved a gap")
	}
	// A content-sized flow has inner padding and gaps, with no trailing gap.
	row.Fixed = false
	row.Size = Point{}
	row.SetFlowLayout(10, LayoutStart, LayoutStart)
	row.resizeFlow(Point{})
	nearLayout(t, row.GetSize(), Point{X: 140, Y: 60})
}

func TestResponsiveConstraintsIncludeLabelsAndOverflowMinima(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	item := ctx.layoutBox(100, 20)
	item.Label = "Label"
	item.SetSizeLimits(Point{X: 80, Y: 10}, Point{X: 90, Y: 30})
	nearLayout(t, item.GetSize(), Point{X: 90, Y: 30})
	item.SetSizeLimits(Point{X: 110, Y: 40}, Point{X: 90, Y: 30})
	nearLayout(t, item.GetSize(), Point{X: 110, Y: 40})
	item.FillWidth = true
	peer := ctx.layoutBox(60, 20)
	row := ctx.NewRow(item, peer)
	row.Fixed = true
	row.Size = Point{X: 100, Y: 50}
	row.Gap = 10
	row.resizeFlow(Point{})
	if row.contentBounds().X != 180 || item.GetSize().X != 110 {
		t.Fatal("insufficient space silently violated a minimum")
	}
	item.SetSizeLimits(Point{X: float32(math.NaN())}, Point{})
	if item.MinSize.X != 110 {
		t.Fatal("invalid setter replaced valid limits")
	}
}

func TestResponsiveWrappedLabelsReflowAndRecoverPreferredWidth(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	text := "A responsive label wraps at its allocated width and pushes the following control down."
	label := ctx.NewWrappedLabel(text, 420)
	label.Position = Point{}
	label.FillWidth = true
	next := ctx.layoutBox(60, 24)
	col := ctx.NewColumn(label, next)
	col.FillWidth = true
	col.Gap = 8
	col.resizeFlow(Point{X: 180, Y: 800})
	narrow := label.GetSize().Y
	if label.GetSize().X != 180 || strings.Count(label.Text, "\n") < 2 {
		t.Fatal("filled label did not wrap to allocated width")
	}
	for child, pos := range col.layoutChildren(col.GetSize()) {
		if child == next && pos.Y != narrow+8 {
			t.Fatal("following control used stale wrapped height")
		}
	}
	before := col.GetSize()
	col.resizeFlow(Point{X: 180, Y: 800})
	nearLayout(t, col.GetSize(), before)
	col.resizeFlow(Point{X: 600, Y: 800})
	if label.Size.X != 420 || label.GetSize().X != 600 || label.GetSize().Y >= narrow {
		t.Fatal("widening lost preferred width or kept narrow wrapping")
	}
}

func TestResponsiveCrossFillAndReverseFlow(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	a, b := ctx.layoutBox(40, 20), ctx.layoutBox(60, 30)
	a.FillHeight = true
	row := ctx.NewRow(a, b)
	row.Fixed = true
	row.Size = Point{X: 200, Y: 80}
	row.Gap = 10
	row.FlowType = FLOW_HORIZONTAL_REV
	row.resizeFlow(Point{})
	nearLayout(t, a.GetSize(), Point{X: 40, Y: 80})
	positions := map[*ItemData]Point{}
	for child, pos := range row.layoutChildren(row.GetSize()) {
		positions[child] = pos
	}
	nearLayout(t, positions[a], Point{X: 160})
	nearLayout(t, positions[b], Point{X: 90})
	overlay := ctx.NewOverlay(a, b)
	overlay.Fixed = true
	overlay.Size = Point{X: 200, Y: 100}
	overlay.SetFlowLayout(0, LayoutEnd, LayoutCenter)
	a.FillHeight = false
	overlay.resizeFlow(Point{})
	for child, pos := range overlay.layoutChildren(overlay.GetSize()) {
		if child == a {
			nearLayout(t, pos, Point{X: 160, Y: 40})
		}
		if child == b {
			nearLayout(t, pos, Point{X: 140, Y: 35})
		}
	}
}

func TestResponsiveWindowFillAndRuntimeSetters(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.Size = Point{X: 500, Y: 300}
	win.Padding = 10
	win.BorderPad = 0
	first, second := ctx.layoutBox(40, 20), ctx.layoutBox(40, 20)
	first.FillWidth, second.FillWidth = true, true
	row := ctx.NewRow(first, second)
	row.FillWidth = true
	win.AddItem(row)
	row.SetFlowLayout(12, LayoutStart, LayoutStart)
	nearLayout(t, row.GetSize(), Point{X: 480, Y: 20})
	nearLayout(t, first.GetSize(), Point{X: 234, Y: 20})
	first.SetSizeLimits(Point{}, Point{X: 100})
	nearLayout(t, first.GetSize(), Point{X: 100, Y: 20})
	nearLayout(t, second.GetSize(), Point{X: 368, Y: 20})
	win.Size.X = 300
	win.Refresh()
	nearLayout(t, second.GetSize(), Point{X: 168, Y: 20})
	second.SetVisible(false)
	nearLayout(t, first.GetSize(), Point{X: 100, Y: 20})
	second.SetVisible(true)
	nearLayout(t, second.GetSize(), Point{X: 168, Y: 20})
}

func TestResponsiveScrollableFlowReservesGutterAndRevealsWithoutSelectionLoss(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	input, _ := ctx.NewInput()
	input.Position = Point{}
	input.Size = Point{X: 100, Y: 30}
	input.FillWidth = true
	input.UpdateText("selected text")
	top := ctx.layoutBox(100, 150)
	flow := ctx.NewColumn(top, input)
	flow.Fixed = true
	flow.Scrollable = true
	flow.Size = Point{X: 220, Y: 80}
	flow.ContentPadding = Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}
	flow.Gap = 12
	win.AddItem(flow)
	gutter := ctx.currentStyle.BorderPad.Slider * 2
	nearLayout(t, input.GetSize(), Point{X: 200 - gutter, Y: 30})
	ctx.Focus(input)
	input.SelectStart, input.SelectEnd, input.CursorPos = 1, 4, 4
	flow.Scroll = Point{}
	input.ScrollIntoView()
	if flow.Scroll.Y <= 0 || ctx.FocusedItem() != input || input.SelectStart != 1 || input.SelectEnd != 4 || input.CursorPos != 4 {
		t.Fatal("reveal failed or changed editor state")
	}
	hidden := ctx.layoutBox(10, 100)
	hidden.Invisible = true
	flow.AddItem(hidden)
	before := flow.Scroll
	hidden.ScrollIntoView()
	if flow.Scroll != before {
		t.Fatal("hidden control changed scrolling")
	}
}

func TestLayoutWindowBodyReservesFlowPaddingAndGaps(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	win.Size = Point{X: 400, Y: 300}
	win.Padding = 0
	win.BorderPad = 0
	head, footer := ctx.layoutBox(100, 30), ctx.layoutBox(100, 25)
	body := ctx.NewColumn(ctx.layoutBox(100, 500))
	root := ctx.NewColumn(head, body, footer)
	root.ContentPadding = Insets{Left: 10, Top: 12, Right: 15, Bottom: 14}
	root.Gap = 8
	win.AddItem(root)
	ctx.LayoutWindowBody(win, root, body)
	win.Refresh()
	nearLayout(t, body.GetSize(), Point{X: 375, Y: 300 - win.GetTitleSize() - 12 - 14 - 30 - 25 - 16})
	for child, pos := range root.layoutChildren(root.GetSize()) {
		if child == footer && pos.Y+footer.GetSize().Y > root.GetSize().Y-14+.01 {
			t.Fatal("body helper ignored padding or gap reservation")
		}
	}
}

func TestResponsiveColumnFillAndZeroAvailableSpace(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	a, b, c := ctx.layoutBox(20, 40), ctx.layoutBox(20, 40), ctx.layoutBox(20, 40)
	a.FillHeight, b.FillHeight, c.FillHeight = true, true, true
	a.MinSize.Y = 180
	b.MaxSize.Y = 60
	col := ctx.NewColumn(a, b, c)
	col.Fixed = true
	col.Size = Point{X: 100, Y: 400}
	col.Gap = 10
	col.resizeFlow(Point{})
	nearLayout(t, a.GetSize(), Point{X: 20, Y: 180})
	nearLayout(t, b.GetSize(), Point{X: 20, Y: 60})
	nearLayout(t, c.GetSize(), Point{X: 20, Y: 140})
	// Zero usable space is an actual allocation, not an unspecified size.
	a.MinSize = Point{}
	a.FillWidth = true
	col.Size.X = 20
	col.ContentPadding = Insets{Left: 10, Right: 10}
	col.resizeFlow(Point{})
	nearLayout(t, a.GetSize(), Point{X: 0, Y: 160})
}

func TestResponsiveReverseOverflowRemainsReachable(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	first, last := ctx.layoutBox(150, 20), ctx.layoutBox(150, 20)
	flow := ctx.NewRow(first, last)
	flow.FlowType = FLOW_HORIZONTAL_REV
	flow.Fixed = true
	flow.Scrollable = true
	flow.Size = Point{X: 200, Y: 60}
	win.AddItem(flow)
	if flow.contentBounds().X != 300 {
		t.Fatal("reverse overflow disappeared from content bounds")
	}
	first.ScrollIntoView()
	if flow.Scroll.X != 100 {
		t.Fatal("reverse overflow could not reveal first child")
	}
	flow.DrawRect = Rect{X1: 200, Y1: 60}
	if flow.getScrollbarPart(Point{X: 100, Y: 59}) != PART_SCROLL_H {
		t.Fatal("reverse flow scrollbar lacks hit target")
	}
	first.PinTo = PIN_BOTTOM_RIGHT
	first.Position.X = 900
	first.SetVisible(false)
	before := win.Scroll
	first.ScrollIntoView()
	if win.Scroll != before {
		t.Fatal("hidden pinned control changed scrolling")
	}
}

func TestResponsiveStyleChangesReflowTabGeometry(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	win := ctx.keyboardFixture(t)
	win.Size = Point{X: 320, Y: 400}
	win.Padding = 0
	win.BorderPad = 0
	tabs := ctx.NewColumn()
	tabs.FillWidth = true
	for _, name := range []string{"One", "Two", "Three", "Four"} {
		page := ctx.NewColumn(ctx.layoutBox(80, 20))
		page.Name = name
		tabs.Tabs = append(tabs.Tabs, page)
	}
	root := ctx.NewColumn(tabs)
	root.FillWidth = true
	win.AddItem(root)
	before := root.GetSize().Y
	ctx.currentTheme.Tab.BorderPad += 80
	ctx.applyStyleToItems(ctx.currentTheme)
	if root.GetSize().Y <= before || !win.Dirty {
		t.Fatal("style changed tab wrapping without reflowing ancestors")
	}
	size := root.GetSize()
	win.Refresh()
	nearLayout(t, root.GetSize(), size)
}
