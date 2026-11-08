package eui

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Distortions81/EUI/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type editTextLine struct {
	start, length int // source rune indices, excluding the newline
	raw           string
	display       string
	bytes         []int // source rune boundary -> display byte boundary (tabs expand)
	stops         []int // grapheme boundaries in the source line
	width         float32
	softBreak     bool // the next visual row continues this source line
}

type editTextLayout struct {
	value             string
	face              text.Face
	lines             []editTextLine
	logical           []editTextLine // unwrapped rows, shared across width-only reflows
	lineHeight, width float32
	wrapWidth         float32
	wraps             []editLineWrapCache
}

func (line editTextLine) advance(pos int, face text.Face) float32 {
	pos = max(0, min(pos, line.length))
	return float32(text.AdvanceAt(line.display, line.bytes[pos], face))
}

func (line editTextLine) nearest(x float32, face text.Face) int {
	best, distance := 0, float32(math.Inf(1))
	for _, pos := range line.stops {
		d := float32(math.Abs(float64(x - line.advance(pos, face))))
		if d < distance {
			best, distance = pos, d
		}
	}
	return line.start + best
}

func (item *itemData) editLayout() *editTextLayout {
	ctx := item.ui()

	s := item.editor()
	face := ctx.itemFace(item, item.FontSize*ctx.uiScale+2)
	metrics := face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	wrapWidth := float32(0)
	if item.WordWrap && item.Multiline {
		offset, size := item.editDrawGeometry()
		viewport, gutter := item.editBaseViewport(offset, size, lineHeight)
		wrapWidth = max(1, viewport.X1-viewport.X0-gutter-2)
	}
	value := item.displayEditText()
	if s.layout != nil && s.layout.value == value && s.layout.face == face && s.layout.wrapWidth == wrapWidth {
		return s.layout
	}
	if s.layout != nil {
		s.followCaret, s.hasPreferredX = true, false
	}
	layout := &editTextLayout{value: value, face: face, lineHeight: lineHeight, wrapWidth: wrapWidth}
	if s.layout != nil && s.layout.value == value && s.layout.face == face {
		layout.logical = s.layout.logical
		layout.wraps = s.layout.wraps
	} else {
		lineCount := strings.Count(value, "\n") + 1
		layout.logical = make([]editTextLine, 0, lineCount)
		previous := s.layout
		reusable := previous != nil && previous.face == face && !item.HideText
		if wrapWidth > 0 || reusable && len(previous.wraps) > 0 {
			layout.wraps = make([]editLineWrapCache, lineCount)
		}
		start := 0
		var oldByRaw map[string]int
		misses := 0
		for raw := range strings.SplitSeq(value, "\n") {
			index := len(layout.logical)
			oldIndex := index
			if reusable {
				matches := func(i int) bool { return i >= 0 && i < len(previous.logical) && previous.logical[i].raw == raw }
				if !matches(oldIndex) {
					oldIndex = index + len(previous.logical) - lineCount
				}
				if !matches(oldIndex) {
					misses++
					// Multiple edits can shift an unchanged middle block by a
					// different amount than either the prefix or suffix. Build
					// a fallback index only when the cheap matches are insufficient.
					if misses > 1 && oldByRaw == nil {
						oldByRaw = make(map[string]int, len(previous.logical))
						for i, line := range previous.logical {
							oldByRaw[line.raw] = i
						}
					}
					if i, found := oldByRaw[raw]; found {
						oldIndex = i
					}
				}
				if matches(oldIndex) {
					line := previous.logical[oldIndex]
					// Rebind source slices to the current snapshot. Otherwise editing
					// different lines could retain one whole old document per line.
					line.raw = raw
					line.start = start
					layout.logical = append(layout.logical, line)
					if len(layout.wraps) > 0 && oldIndex < len(previous.wraps) {
						layout.wraps[index] = previous.wraps[oldIndex]
					}
					start += line.length + 1
					continue
				}
			}
			line := editTextLine{raw: raw, start: start, length: utf8.RuneCountInString(raw), stops: graphemeStops(raw)}
			line.bytes = make([]int, 1, line.length+1)
			var display strings.Builder
			display.Grow(len(raw) + strings.Count(raw, "\t")*3)
			column := 0
			for _, r := range raw {
				if r == '\t' {
					spaces := 4 - column%4
					display.WriteString(strings.Repeat(" ", spaces))
					column += spaces
				} else {
					display.WriteRune(r)
					column++
				}
				line.bytes = append(line.bytes, display.Len())
			}
			if item.HideText {
				// Mask glyphs still represent source rune offsets; pointer placement
				// must not split a composed character in the underlying password.
				source := item.SecretText
				if c := item.activeComposition(); c != nil {
					runes := []rune(source)
					source = string(runes[:c.start]) + c.text + string(runes[c.end:])
				}
				line.stops = graphemeStops(source)
			}
			line.display = display.String()
			line.width = float32(text.AdvanceAt(line.display, len(line.display), face))
			layout.logical = append(layout.logical, line)
			start += line.length + 1
		}
	}
	if wrapWidth == 0 {
		layout.lines = layout.logical
	} else {
		capacity := len(layout.logical)
		if previous := s.layout; previous != nil && previous.wrapWidth > 0 {
			estimate := len(previous.lines)
			if previous.wrapWidth != wrapWidth {
				estimate = int(float64(estimate)*float64(previous.wrapWidth)/float64(wrapWidth)) + len(layout.logical)
			}
			last := layout.logical[len(layout.logical)-1]
			capacity = max(capacity, min(estimate, last.start+last.length+1))
		}
		layout.lines = make([]editTextLine, 0, capacity)
		if len(layout.wraps) != len(layout.logical) {
			layout.wraps = make([]editLineWrapCache, len(layout.logical))
		}
		for i, line := range layout.logical {
			layout.lines = layout.wraps[i].appendRows(layout.lines, line, face, wrapWidth)
		}
	}
	for _, line := range layout.lines {
		layout.width = max(layout.width, line.width)
	}
	s.layout = layout
	return layout
}

func (layout *editTextLayout) caret(pos int) (int, float32) {
	return layout.caretAt(pos, false)
}

func (item *itemData) editCaret() (int, float32) {
	return item.editLayout().caretAt(item.displayEditCaret(), item.editor().caretUpstream && !item.IsComposing())
}

func (layout *editTextLayout) caretAt(pos int, upstream bool) (int, float32) {
	i := sort.Search(len(layout.lines), func(i int) bool {
		line := layout.lines[i]
		end := line.start + line.length
		return pos < end || pos == end && (!line.softBreak || upstream)
	})
	if i == len(layout.lines) {
		i--
	}
	if i < 0 {
		return 0, 0
	}
	line := layout.lines[i]
	return i, line.advance(pos-line.start, layout.face)
}

// Only inspect rows intersecting the viewport. Document length must not add
// work to a scroll repaint; row height is uniform even with soft wrapping.
func (layout *editTextLayout) visibleLines(clip rect, originY float32) (int, int) {
	n := len(layout.lines)
	first := max(0, min(n, int(math.Floor(float64((clip.Y0-originY)/layout.lineHeight)))))
	last := max(first, min(n, int(math.Ceil(float64((clip.Y1-originY)/layout.lineHeight)))))
	return first, last
}

func (item *itemData) editViewport(offset, size point) rect {
	viewport, _, _ := item.editScrollGeometry(offset, size)
	return viewport
}

func (item *itemData) editBaseViewport(offset, size point, lineHeight float32) (rect, float32) {
	ctx := item.ui()

	pad := (item.BorderPad + item.Padding + ctx.currentStyle.TextPadding) * ctx.uiScale
	// Small inputs must retain a full line even when a theme has generous
	// horizontal padding. Center that line in the available control height.
	padY := min(pad, max(0, (size.Y-lineHeight)/2))
	viewport := rect{X0: offset.X + pad, Y0: offset.Y + padY,
		X1: max(offset.X+pad+1, offset.X+size.X-pad),
		Y1: max(offset.Y+padY+1, offset.Y+size.Y-padY)}
	if item.ItemType == ITEM_INPUT && item.Label == "" {
		if info := item.tooltipIndicatorRect(offset, size); info.X1 > info.X0 {
			viewport.X1 = max(viewport.X0+1, min(viewport.X1, info.X0-3*ctx.uiScale))
		}
	}
	width := min(max(10*ctx.uiScale, ctx.ScrollbarWidth()), (viewport.X1-viewport.X0)/2, (viewport.Y1-viewport.Y0)/2)
	return viewport, width
}

func (item *itemData) editScrollGeometry(offset, size point) (rect, rect, rect) {
	layout := item.editLayout()
	viewport, width := item.editBaseViewport(offset, size, layout.lineHeight)
	var vertical, horizontal rect
	if item.Multiline {
		if item.WordWrap {
			// Reserve a stable gutter so a vertical scrollbar never changes wrapping.
			viewport.X1 -= width
			if float32(len(layout.lines))*layout.lineHeight > viewport.Y1-viewport.Y0 {
				vertical = rect{X0: viewport.X1, Y0: viewport.Y0, X1: viewport.X1 + width, Y1: viewport.Y1}
			}
			return viewport, vertical, horizontal
		}
		needV, needH := false, false
		// One scrollbar can make the other axis overflow.
		for range 2 {
			if !needV && float32(len(layout.lines))*layout.lineHeight > viewport.Y1-viewport.Y0 {
				needV = true
				viewport.X1 -= width
			}
			if !needH && layout.width+2 > viewport.X1-viewport.X0 {
				needH = true
				viewport.Y1 -= width
			}
		}
		if needV {
			vertical = rect{X0: viewport.X1, Y0: viewport.Y0, X1: viewport.X1 + width, Y1: viewport.Y1}
		}
		if needH {
			horizontal = rect{X0: viewport.X0, Y0: viewport.Y1, X1: viewport.X1, Y1: viewport.Y1 + width}
		}
	}
	return viewport, vertical, horizontal
}

func (item *itemData) editDrawGeometry() (point, point) {
	offset, size := item.textDrawOrigin, item.textDrawSize
	if size.X <= 0 || size.Y <= 0 {
		offset = point{X: item.DrawRect.X0, Y: item.DrawRect.Y0}
		size = point{X: item.DrawRect.X1 - item.DrawRect.X0, Y: item.DrawRect.Y1 - item.DrawRect.Y0}
	}
	return offset, size
}

func (item *itemData) editGeometry() (rect, point) {
	offset, size := item.editDrawGeometry()
	viewport := item.editViewport(offset, size)
	origin := point{X: viewport.X0 - item.editor().scroll.X, Y: viewport.Y0 - item.editor().scroll.Y}
	if !item.Multiline {
		origin.Y += max(0, (viewport.Y1-viewport.Y0-item.editLayout().lineHeight)/2)
	}
	return viewport, origin
}

func (item *itemData) editCursorAt(mpos point) int {
	pos, _ := item.editHit(mpos)
	return pos
}

func (item *itemData) editHit(mpos point) (int, bool) {
	layout := item.editLayout()
	_, origin := item.editGeometry()
	line := max(0, min(int(math.Floor(float64((mpos.Y-origin.Y)/layout.lineHeight))), len(layout.lines)-1))
	row := layout.lines[line]
	pos := row.nearest(mpos.X-origin.X, layout.face)
	return pos, row.softBreak && pos == row.start+row.length
}

func (item *itemData) editVertical(direction int, page, extend bool) {
	layout, state := item.editLayout(), item.editor()
	line, x := item.editCaret()
	if !state.hasPreferredX {
		state.preferredX, state.hasPreferredX = x, true
	}
	step := 1
	if page {
		viewport, _ := item.editGeometry()
		step = max(1, int((viewport.Y1-viewport.Y0)/layout.lineHeight)-1)
	}
	line = max(0, min(line+direction*step, len(layout.lines)-1))
	row := layout.lines[line]
	pos := row.nearest(state.preferredX, layout.face)
	item.editMove(pos, extend)
	state.caretUpstream = row.softBreak && pos == row.start+row.length
}

func (item *itemData) clampEditScroll(viewport rect) {
	layout, state := item.editLayout(), item.editor()
	state.scroll.X = max(0, min(state.scroll.X, layout.width+2-(viewport.X1-viewport.X0)))
	state.scroll.Y = max(0, min(state.scroll.Y, float32(len(layout.lines))*layout.lineHeight-(viewport.Y1-viewport.Y0)))
	if item.Multiline && item.WordWrap {
		state.scroll.X = 0
	}
	if !item.Multiline {
		state.scroll.Y = 0
	}
}

func (item *itemData) followEditCaret(viewport rect) {
	layout, state := item.editLayout(), item.editor()
	if state.followCaret {
		line, x := item.editCaret()
		y := float32(line) * layout.lineHeight
		w, h := viewport.X1-viewport.X0, viewport.Y1-viewport.Y0
		if x < state.scroll.X {
			state.scroll.X = x
		}
		if x+2 > state.scroll.X+w {
			state.scroll.X = x + 2 - w
		}
		// Keep a selected match fully visible when it fits on one line.
		start, end := item.displayEditSelection()
		anchorLine, anchorX := layout.caret(start)
		if start != end && anchorLine == line && x >= anchorX && x-anchorX+2 <= w && anchorX < state.scroll.X {
			state.scroll.X = anchorX
		}
		if y < state.scroll.Y {
			state.scroll.Y = y
		}
		if y+layout.lineHeight > state.scroll.Y+h {
			state.scroll.Y = y + layout.lineHeight - h
		}
		state.followCaret = false
	}
	item.clampEditScroll(viewport)
}

func (item *itemData) drawEditableText(dst *ebiten.Image, offset, size point, clip rect, caption, selection Color) {
	ctx := item.ui()

	layout := item.editLayout()
	viewport := item.editViewport(offset, size)
	item.followEditCaret(viewport)
	item.drawEditScrollbars(dst, offset, size, clip)
	clip = intersectRect(clip, viewport)
	if clip.X1 <= clip.X0 || clip.Y1 <= clip.Y0 {
		return
	}
	target := dst.RecyclableSubImage(clip.getRectangle())
	defer target.Recycle()
	state := item.editor()
	origin := point{X: viewport.X0 - state.scroll.X, Y: viewport.Y0 - state.scroll.Y}
	if !item.Multiline {
		origin.Y += max(0, (viewport.Y1-viewport.Y0-layout.lineHeight)/2)
	}
	start, end := item.displayEditSelection()
	composition := item.activeComposition()
	var highlights, swatches []TextColorSpan
	if composition == nil {
		highlights = item.textHighlights()
		swatches = item.textSwatches()
	}
	first, last := layout.visibleLines(clip, origin.Y)
	for i := first; i < last; i++ {
		line := layout.lines[i]
		y := origin.Y + float32(i)*layout.lineHeight
		op := &text.DrawOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Translate(float64(origin.X), float64(y))
		op.ColorScale.ScaleWithColor(caption)
		if len(highlights) == 0 && len(swatches) == 0 {
			text.Draw(target, line.display, layout.face, op)
		} else {
			item.drawHighlightedEditLine(target, line, layout.face, point{X: origin.X, Y: y}, highlights, caption)
		}
		if composition == nil {
			item.drawTextSwatches(target, line, layout, origin, y)
		}
		a, b := max(start, line.start), min(end, line.start+line.length)
		if start != end && (a < b || start <= line.start+line.length && end > line.start+line.length && !line.softBreak) {
			x0 := line.advance(a-line.start, layout.face)
			x1 := line.advance(b-line.start, layout.face)
			if end > line.start+line.length && !line.softBreak {
				x1 += max(3, layout.lineHeight/3)
			}
			r := intersectRect(rect{X0: origin.X + min(x0, x1), Y0: y, X1: origin.X + max(x0, x1), Y1: y + layout.lineHeight}, clip)
			if r.X1 > r.X0 && r.Y1 > r.Y0 {
				drawFilledRect(target, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, selection, false)
				selected := target.RecyclableSubImage(r.getRectangle())
				op.ColorScale.Reset()
				op.ColorScale.ScaleWithColor(item.surfaceTextColor(caption, selection, true))
				text.Draw(selected, line.display, layout.face, op)
				selected.Recycle()
			}
		}
		if composition != nil {
			a, b := max(composition.start, line.start), min(composition.start+utf8.RuneCountInString(composition.text), line.start+line.length)
			if a < b {
				x0, x1 := line.advance(a-line.start, layout.face), line.advance(b-line.start, layout.face)
				ctx.strokeLine(target, origin.X+x0, y+layout.lineHeight-2, origin.X+x1, y+layout.lineHeight-2, max(1, ctx.uiScale), caption, false)
			}
		}

	}
	if suffix := item.textCompletion(); suffix != "" && composition == nil {
		line, x := item.editCaret()
		op := &text.DrawOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Translate(float64(origin.X+x), float64(origin.Y+float32(line)*layout.lineHeight))
		ghost := caption
		ghost.A /= 2
		op.ColorScale.ScaleWithColor(ghost)
		text.Draw(target, suffix, layout.face, op)
	}
	if item.Focused && (composition != nil || state.caretOn && start == end) {
		line, x := item.editCaret()
		y := origin.Y + float32(line)*layout.lineHeight
		ctx.strokeLine(target, origin.X+x, y, origin.X+x, y+layout.lineHeight-2, max(1, ctx.uiScale), caption, false)
	}
}

func (item *itemData) editZoomWheel(delta point, mods inputkeys.Modifiers) bool {
	state := item.editor()
	if item.OnTextZoom == nil || !mods.Shortcut() {
		state.zoomWheel = 0
		return false
	}
	if delta.Y == 0 {
		return false
	}
	if state.zoomWheel*delta.Y < 0 {
		state.zoomWheel = 0
	}
	state.zoomWheel += delta.Y
	steps := int(state.zoomWheel)
	if steps != 0 {
		state.zoomWheel -= float32(steps)
		item.OnTextZoom(steps)
	}
	return true
}

func (ctx *Context) scrollEditable(items []*itemData, mpos, delta point, mods inputkeys.Modifiers) bool {
	for _, item := range items {
		if item.isInvisible() || !item.DrawRect.containsPoint(mpos) {
			continue
		}
		if itemHandlesTextEditing(item) {
			item.CancelComposition()
			if !ctx.keyboardInputCaptured && item.editZoomWheel(delta, mods) {
				return true
			}
			state := item.editor()
			viewport, _ := item.editGeometry()
			if item.Multiline && !ctx.ShiftPressed {
				state.scroll.Y -= delta.Y * item.editLayout().lineHeight * 3
			} else {
				state.scroll.X -= delta.Y * item.editLayout().lineHeight * 3
			}
			state.scroll.X -= delta.X * item.editLayout().lineHeight * 3
			state.followCaret = false
			item.clampEditScroll(viewport)
			item.markDirty()
			return true
		}
		if ctx.scrollEditable(item.activeContents(), mpos, delta, mods) {
			return true
		}
	}
	return false
}

// SetWordWrap changes only presentation, retaining the draft, selection and undo.
func (item *itemData) SetWordWrap(enabled bool) {
	if item == nil || item.ItemType != ITEM_INPUT && !(item.ItemType == ITEM_TEXT && item.EditableText) || item.WordWrap == enabled {
		return
	}
	item.CancelComposition()
	item.WordWrap = enabled
	state := item.editor()
	state.scroll.X = 0
	state.caretUpstream, state.hasPreferredX = false, false
	item.resetCaret()
}

// Keep source offsets and tab expansion while breaking at whitespace when
// possible. Long tokens break only between graphemes, never inside one.
func wrapEditLine(line editTextLine, raw string, face text.Face, width float32) []editTextLine {
	return appendWrappedEditLine(nil, line, raw, face, width)
}
func appendWrappedEditLine(rows []editTextLine, line editTextLine, raw string, face text.Face, width float32) []editTextLine {
	if line.width <= width || line.length == 0 {
		return append(rows, line)
	}
	runes := []rune(raw)
	for first := 0; first < len(line.stops)-1; {
		start := line.stops[first]
		measure := func(last int) float32 {
			value := line.display[line.bytes[start]:line.bytes[line.stops[last]]]
			return float32(text.AdvanceAt(value, len(value), face))
		}
		// Exponential probing keeps work bounded by a visual row even for very
		// long, unspaced songs. Always allow at least one complete grapheme.
		low, high := first+1, first+2
		for high < len(line.stops) && measure(high) <= width {
			low, high = high, first+2*(high-first)
		}
		high = min(high, len(line.stops)-1)
		for low < high {
			middle := (low + high + 1) / 2
			if measure(middle) <= width {
				low = middle
			} else {
				high = middle - 1
			}
		}
		last := low
		if last < len(line.stops)-1 {
			for i := last; i > first; i-- {
				if unicode.IsSpace(runes[line.stops[i]-1]) {
					last = i
					break
				}
			}
		}
		end := line.stops[last]
		row := editTextLine{start: line.start + start, length: end - start,
			bytes: make([]int, 0, end-start+1), stops: make([]int, 0, last-first+1),
			display: line.display[line.bytes[start]:line.bytes[end]], softBreak: end < line.length}
		for _, b := range line.bytes[start : end+1] {
			row.bytes = append(row.bytes, b-line.bytes[start])
		}
		for _, stop := range line.stops[first : last+1] {
			row.stops = append(row.stops, stop-start)
		}
		row.width = float32(text.AdvanceAt(row.display, len(row.display), face))
		rows = append(rows, row)
		first = last
	}
	return rows
}
