package eui

// Insets describes logical space inside a flow around its children.
type Insets struct{ Left, Top, Right, Bottom float32 }

// LayoutAlign positions children in unused flow space. Fill is configured on
// each child separately; alignment never stretches a child's preferred size.
type LayoutAlign uint8

const (
	LayoutStart LayoutAlign = iota
	LayoutCenter
	LayoutEnd
)

type itemLayoutKey struct {
	padding                                   Insets
	size, minimum, maximum                    Point
	scale, font, gap                          float32
	text, label                               string
	fixed, constrained, fillWidth, fillHeight bool
	flow                                      flowType
}

func (item *itemData) currentLayoutKey() itemLayoutKey {
	ctx := item.ui()

	return itemLayoutKey{size: item.Size, minimum: item.MinSize, maximum: item.MaxSize, scale: ctx.uiScale, font: item.FontSize, gap: item.Gap, text: item.Text, label: item.Label, fixed: item.Fixed, constrained: item.ConstrainToSize, fillWidth: item.FillWidth, fillHeight: item.FillHeight, flow: item.FlowType, padding: item.ContentPadding}
}

func (item *itemData) hasResolvedLayout() bool {
	return item.layoutValid && item.layoutKey == item.currentLayoutKey()
}

// GetSize returns the resolved pixel footprint, or the constrained preferred
// size before layout. Size, MinSize, and MaxSize remain in logical units.
func (item *itemData) GetSize() Point {
	if item == nil || item.Invisible {
		return Point{}
	}
	if item.hasResolvedLayout() {
		return item.layoutSize
	}
	item.layoutWrapWidth = 0
	return item.constrainSize(item.intrinsicSize())
}

func positiveLayoutValue(value float32) float32 {
	if !finiteValue(value) || value < 0 {
		return 0
	}
	return value
}

func (ctx *Context) constrainLayoutAxis(value, minimum, maximum float32) float32 {
	value = positiveLayoutValue(value)
	minimum = positiveLayoutValue(minimum) * ctx.uiScale
	maximum = positiveLayoutValue(maximum) * ctx.uiScale
	if maximum > 0 {
		value = min(value, max(maximum, minimum))
	}
	return max(value, minimum)
}

func (item *itemData) constrainSize(size Point) Point {
	ctx := item.ui()

	return Point{X: ctx.constrainLayoutAxis(size.X, item.MinSize.X, item.MaxSize.X), Y: ctx.constrainLayoutAxis(size.Y, item.MinSize.Y, item.MaxSize.Y)}
}

// SetSizeLimits updates logical minimum/maximum dimensions and reflows. Zero
// removes a bound. Non-finite or negative dimensions are ignored; if a minimum
// exceeds its maximum, the minimum takes precedence.
func (item *itemData) SetSizeLimits(minimum, maximum Point) {
	if item == nil || !finiteValue(minimum.X) || !finiteValue(minimum.Y) || !finiteValue(maximum.X) || !finiteValue(maximum.Y) || minimum.X < 0 || minimum.Y < 0 || maximum.X < 0 || maximum.Y < 0 {
		return
	}
	if item.MinSize == minimum && item.MaxSize == maximum {
		return
	}
	item.MinSize, item.MaxSize = minimum, maximum
	item.runtimeChanged(true)
}

// SetFill controls how this child consumes its parent's available space.
func (item *itemData) SetFill(width, height bool) {
	if item == nil || item.FillWidth == width && item.FillHeight == height {
		return
	}
	item.FillWidth, item.FillHeight = width, height
	item.runtimeChanged(true)
}

// SetFlowLayout updates a flow's gap and alignment. Gaps use logical units and
// appear only between visible children. Invalid values or non-flows are ignored.
func (item *itemData) SetFlowLayout(gap float32, main, cross LayoutAlign) {
	if item == nil || item.ItemType != ITEM_FLOW || !finiteValue(gap) || gap < 0 || main > LayoutEnd || cross > LayoutEnd {
		return
	}
	if item.Gap == gap && item.MainAlign == main && item.CrossAlign == cross {
		return
	}
	item.Gap, item.MainAlign, item.CrossAlign = gap, main, cross
	item.runtimeChanged(true)
}

// SetContentPadding changes a flow's inner spacing and reflows its children.
func (item *itemData) SetContentPadding(padding Insets) {
	if item == nil || item.ItemType != ITEM_FLOW || item.ContentPadding == padding {
		return
	}
	for _, value := range []float32{padding.Left, padding.Top, padding.Right, padding.Bottom} {
		if !finiteValue(value) || value < 0 {
			return
		}
	}
	item.ContentPadding = padding
	item.runtimeChanged(true)
}

func (item *itemData) contentInsets() Insets {
	ctx := item.ui()

	p := item.ContentPadding
	return Insets{positiveLayoutValue(p.Left) * ctx.uiScale, positiveLayoutValue(p.Top) * ctx.uiScale, positiveLayoutValue(p.Right) * ctx.uiScale, positiveLayoutValue(p.Bottom) * ctx.uiScale}
}

func (item *itemData) layoutBody(size Point) (Point, Point) {
	ctx := item.ui()

	padding := item.contentInsets()
	tabHeight := float32(0)
	if len(item.Tabs) > 0 {
		tabHeight = ctx.tabStripHeight(item, item.themeStyle())
	}
	return Point{X: padding.Left, Y: padding.Top + tabHeight}, Point{X: max(0, size.X-padding.Left-padding.Right-item.layoutGutter.X), Y: max(0, size.Y-tabHeight-padding.Top-padding.Bottom-item.layoutGutter.Y)}
}

func (item *itemData) clearLayout() {
	if item == nil {
		return
	}
	item.layoutValid = false
	item.layoutWrapWidth = 0
	item.layoutGutter = Point{}
	for _, child := range item.Contents {
		child.clearLayout()
	}
	for _, page := range item.Tabs {
		page.clearLayout()
	}
}

func layoutAlignment(extra float32, align LayoutAlign) float32 {
	extra = max(0, extra)
	switch align {
	case LayoutCenter:
		return extra / 2
	case LayoutEnd:
		return extra
	}
	return 0
}

func (item *itemData) horizontalFlow() bool {
	return item.FlowType == FLOW_HORIZONTAL || item.FlowType == FLOW_HORIZONTAL_REV
}
func (item *itemData) verticalFlow() bool {
	return item.FlowType == FLOW_VERTICAL || item.FlowType == FLOW_VERTICAL_REV
}

func (item *itemData) linearFlow() bool {
	return item.ItemType == ITEM_FLOW && item.FlowType != FLOW_OVERLAY
}

// layoutChildren is the shared placement source for measurement, drawing and
// keyboard reveal. Position is an additional logical inset; its main-axis
// component is included in the space reserved for that child.
func (item *itemData) layoutChildren(size Point) func(func(*itemData, Point) bool) {
	ctx := item.ui()

	return func(yield func(*itemData, Point) bool) {
		horizontal, linear := item.horizontalFlow(), item.linearFlow()
		origin, body := item.layoutBody(size)
		total, count := float32(0), 0
		reverse := item.FlowType == FLOW_HORIZONTAL_REV || item.FlowType == FLOW_VERTICAL_REV
		gap := positiveLayoutValue(item.Gap) * ctx.uiScale
		// Start-aligned forward flows need no total-size prepass.
		if linear && (reverse || item.MainAlign != LayoutStart) {
			for _, child := range item.activeContents() {
				if child == nil || child.Invisible {
					continue
				}
				sz, pos := child.GetSize(), child.getPosition(item.ParentWindow)
				if horizontal {
					total += sz.X + pos.X
				} else {
					total += sz.Y + pos.Y
				}
				count++
			}
			if count > 1 {
				total += float32(count-1) * gap
			}
		}
		mainSize := body.Y
		if horizontal {
			mainSize = body.X
		}
		cursor := layoutAlignment(mainSize-total, item.MainAlign)
		for _, child := range item.activeContents() {
			if child == nil || child.Invisible {
				continue
			}
			sz, pos := child.GetSize(), child.getPosition(item.ParentWindow)
			inset := pos
			if linear {
				if horizontal {
					if reverse {
						pos.X = max(body.X, total) - cursor - sz.X - pos.X
					} else {
						pos.X += cursor
					}
					pos.Y += layoutAlignment(body.Y-sz.Y-pos.Y, item.CrossAlign)
					cursor += sz.X + inset.X + gap
				} else {
					if reverse {
						pos.Y = max(body.Y, total) - cursor - sz.Y - pos.Y
					} else {
						pos.Y += cursor
					}
					pos.X += layoutAlignment(body.X-sz.X-pos.X, item.CrossAlign)
					cursor += sz.Y + inset.Y + gap
				}
			} else if item.ItemType == ITEM_FLOW {
				pos.X += layoutAlignment(body.X-sz.X-pos.X, item.MainAlign)
				pos.Y += layoutAlignment(body.Y-sz.Y-pos.Y, item.CrossAlign)
			}
			pos = pointAdd(pos, origin)
			if !yield(child, pos) {
				return
			}
		}
	}
}

func (item *itemData) storeLayout(size Point) {
	if item.layoutSize != size {
		item.Dirty = true
	}
	item.layoutSize = size
	item.layoutKey = item.currentLayoutKey()
	item.layoutValid = true
}

// distributeFill allocates equal shares, pinning bounded children and then
// redistributing their excess. Minima may overflow a too-small viewport; layout
// does not silently violate minimum sizes or resize fixed siblings.
func (ctx *Context) distributeFill(children []*itemData, space float32, horizontal bool) map[*itemData]float32 {
	if len(children) == 0 {
		return nil
	}
	result := make(map[*itemData]float32, len(children))
	pending := append([]*itemData(nil), children...)
	for len(pending) > 0 {
		share := max(0, space) / float32(len(pending))
		pinned := -1
		value := share
		// Resolve minima first: reserving a large minimum can reduce the share
		// below another child's maximum, so maxima must wait for that reservation.
		for i, child := range pending {
			minimum := child.MinSize.Y
			if horizontal {
				minimum = child.MinSize.X
			}
			minimum = positiveLayoutValue(minimum) * ctx.uiScale
			if minimum > share {
				pinned = i
				value = minimum
				break
			}
		}
		if pinned < 0 {
			for i, child := range pending {
				minimum, maximum := child.MinSize.Y, child.MaxSize.Y
				if horizontal {
					minimum, maximum = child.MinSize.X, child.MaxSize.X
				}
				bounded := ctx.constrainLayoutAxis(share, minimum, maximum)
				if bounded < share {
					pinned = i
					value = bounded
					break
				}
			}
		}
		if pinned < 0 {
			for _, child := range pending {
				result[child] = share
			}
			break
		}
		result[pending[pinned]] = value
		space -= value
		pending = append(pending[:pinned], pending[pinned+1:]...)
	}

	return result
}

func (item *itemData) measureLayout() {
	if item == nil {
		return
	}
	item.normalizeTabs()
	if item.Invisible {
		item.clearHiddenState()
		return
	}
	for _, child := range item.activeContents() {
		child.measureLayout()
	}
	item.layoutPreferred = item.constrainSize(item.intrinsicSize())
	item.storeLayout(item.layoutPreferred)
}

func (item *itemData) scrollbarGutter(size Point) Point {
	ctx := item.ui()

	if !item.Scrollable {
		return Point{}
	}
	req := item.contentBounds()
	width := ctx.currentStyle.BorderPad.Slider * 2
	if item.verticalFlow() && req.Y > size.Y {
		return Point{X: width}
	}
	if item.horizontalFlow() && req.X > size.X {
		return Point{Y: width}
	}
	return Point{}
}

func (item *itemData) resolveLayout(available, forced Point) {
	ctx := item.ui()

	if item == nil {
		return
	}
	item.normalizeTabs()
	if item.Invisible {
		item.clearHiddenState()
		return
	}
	size := item.layoutPreferred
	explicitX, explicitY := forced.X >= 0, forced.Y >= 0
	if !explicitX && item.FillWidth && available.X >= 0 {
		forced.X = available.X
		explicitX = true
	}
	if !explicitY && item.FillHeight && available.Y >= 0 {
		forced.Y = available.Y
		explicitY = true
	}
	if explicitX {
		size.X = ctx.constrainLayoutAxis(forced.X, item.MinSize.X, item.MaxSize.X)
	}
	if explicitY {
		size.Y = ctx.constrainLayoutAxis(forced.Y, item.MinSize.Y, item.MaxSize.Y)
	}
	if item.wrappedLabel != nil {
		item.layoutWrapWidth = size.X
		natural := item.intrinsicSize()
		if !explicitY {
			size.Y = ctx.constrainLayoutAxis(natural.Y, item.MinSize.Y, item.MaxSize.Y)
		}
	}
	item.storeLayout(size)
	if item.ItemType == ITEM_FLOW {
		for pass := 0; pass < 2; pass++ {
			item.layoutGutter = item.scrollbarGutter(size)
			_, body := item.layoutBody(size)
			horizontal, linear := item.horizontalFlow(), item.linearFlow()
			budget := body.Y
			if horizontal {
				budget = body.X
			}
			var fill []*itemData
			count := 0
			for _, child := range item.activeContents() {
				if child == nil || child.Invisible {
					continue
				}
				count++
				sz, pos := child.GetSize(), child.getPosition(item.ParentWindow)
				main := sz.Y + pos.Y
				fills := child.FillHeight
				if horizontal {
					main = sz.X + pos.X
					fills = child.FillWidth
				}
				if linear && fills {
					fill = append(fill, child)
					if horizontal {
						budget -= pos.X
					} else {
						budget -= pos.Y
					}
				} else {
					budget -= main
				}
			}
			if linear && count > 1 {
				budget -= float32(count-1) * positiveLayoutValue(item.Gap) * ctx.uiScale
			}
			shares := ctx.distributeFill(fill, budget, horizontal)
			for _, child := range item.activeContents() {
				if child == nil || child.Invisible {
					continue
				}
				childForced := Point{X: -1, Y: -1}
				if share, ok := shares[child]; ok {
					if horizontal {
						childForced.X = share
					} else {
						childForced.Y = share
					}
				}
				pos := child.getPosition(item.ParentWindow)
				child.resolveLayout(Point{X: max(0, body.X-pos.X), Y: max(0, body.Y-pos.Y)}, childForced)
			}
			if item.layoutGutter == item.scrollbarGutter(size) {
				break
			}
		}

		// A filled wrapped label can change a natural column's height. Re-measure
		// after width allocation, while retaining explicitly assigned dimensions.
		if !item.Fixed && !item.ConstrainToSize {
			req := item.contentBounds()
			if !explicitX {
				size.X = ctx.constrainLayoutAxis(max(item.Size.X*ctx.uiScale, req.X), item.MinSize.X, item.MaxSize.X)
			}
			if !explicitY {
				size.Y = ctx.constrainLayoutAxis(max(item.Size.Y*ctx.uiScale, req.Y), item.MinSize.Y, item.MaxSize.Y)
			}
		}
		item.storeLayout(size)
		req := item.contentBounds()
		item.Scroll.X = max(0, min(item.Scroll.X, max(0, req.X-size.X)))
		item.Scroll.Y = max(0, min(item.Scroll.Y, max(0, req.Y-size.Y)))
	}
}
