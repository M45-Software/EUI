package eui

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (item *itemData) themeStyle() *itemData {
	ctx := item.ui()

	if item == nil {
		return nil
	}

	th := item.Theme
	if th == nil {
		th = ctx.currentTheme
	}
	if th == nil {
		return nil
	}

	switch item.ItemType {
	case ITEM_BUTTON:
		if item.buttonColors != nil {
			// Override storage belongs to this button and is refreshed on every
			// lookup, including after direct edits to its theme's appearance.
			style := &item.buttonColors.style
			*style = th.Button
			style.Color = item.buttonColors.normal
			style.HoverColor, style.ClickColor = item.buttonColors.hover, item.buttonColors.hover
			return style
		}
		return &th.Button
	case ITEM_TEXT:
		return &th.Text
	case ITEM_CHECKBOX:
		return &th.Checkbox
	case ITEM_RADIO:
		return &th.Radio
	case ITEM_INPUT:
		return &th.Input
	case ITEM_SLIDER:
		return &th.Slider
	case ITEM_DROPDOWN:
		return &th.Dropdown
	case ITEM_PROGRESS:
		return &th.Progress
	case ITEM_FLOW:
		if len(item.Tabs) > 0 {
			return &th.Tab
		}
	}
	return nil
}

// disabledStyle suppresses hover and active colors, with an optional separate
// caption color so disabled controls stay readable.
func disabledStyle(style *itemData) *itemData {
	if style == nil {
		return nil
	}
	ds := *style
	ds.Color = style.DisabledColor
	ds.HoverColor = style.DisabledColor
	ds.ClickColor = style.DisabledColor
	ds.OutlineColor = style.DisabledColor
	ds.TextColor = style.disabledTextColor()
	ds.SelectedColor = style.DisabledColor
	return &ds
}

func (item *itemData) disabledTextColor() Color {
	if item.DisabledTextColor != (Color{}) {
		return item.DisabledTextColor
	}
	return item.DisabledColor
}

func (win *windowData) getWinRect() rect {
	pos := win.GetPos()
	size := win.GetSize()
	return rect{
		X0: pos.X,
		Y0: pos.Y,
		X1: pos.X + size.X,
		Y1: pos.Y + size.Y,
	}
}

func (parent *itemData) addItemTo(item *itemData) {
	if parent == nil || item == nil {
		return
	}
	parent.insertItem(len(parent.Contents), item)
}

func (parent *itemData) prependItemTo(item *itemData) { parent.insertItem(0, item) }

func (parent *windowData) addItemTo(item *itemData) {
	if parent == nil || item == nil {
		return
	}
	parent.insertItem(len(parent.Contents), item)
}

func (parent *windowData) prependItemTo(item *itemData) { parent.insertItem(0, item) }

func (item *itemData) detach() {
	if item == nil {
		return
	}
	item.clearHiddenState()
	item.deallocate()
	item.Parent = nil
	item.setParentWindow(nil)
}

func (parent *itemData) removeItem(child *itemData) {
	if parent == nil || child == nil {
		return
	}
	for i, it := range parent.Contents {
		if it == child {
			copy(parent.Contents[i:], parent.Contents[i+1:])
			parent.Contents[len(parent.Contents)-1] = nil
			parent.Contents = parent.Contents[:len(parent.Contents)-1]
			child.detach()
			parent.runtimeChanged(true)
			return
		}
	}
}

func (parent *itemData) setItems(children []*itemData) {
	if parent != nil {
		parent.ui().bindItems(children)
	}
	if parent == nil {
		return
	}
	next := make([]*itemData, 0, len(children))
	retained := make(map[*itemData]bool, len(children))
	for _, child := range children {
		if child != nil {
			next = append(next, child)
			retained[child] = true
		}
	}
	previous := parent.Contents
	unchanged := len(next) == len(previous)
	if unchanged {
		for i, child := range next {
			if child != previous[i] || child.Parent != parent || child.ParentWindow != parent.ParentWindow {
				unchanged = false
				break
			}
		}
	}
	if unchanged {
		parent.Contents = next
		return
	}
	// Commit the new list before dismissal callbacks can inspect or append to it.
	parent.Contents = next
	for _, child := range previous {
		if child != nil && child.Parent == parent && !retained[child] {
			child.detach()
		}
	}
	for _, child := range parent.Contents {
		child.Parent = parent
		if child.Theme == nil {
			child.Theme = parent.Theme
		}
		child.setParentWindow(parent.ParentWindow)
	}
	parent.runtimeChanged(true)
}

func (parent *windowData) contentsChanged() {
	if parent.Open {
		parent.RefreshWithReason("items")
		parent.Dirty = true
		parent.refreshPending = false
	} else {
		parent.resizeFlows()
		parent.markDirty()
	}
	parent.updateHasIndeterminate()
}

func (parent *windowData) removeItem(child *itemData) {
	if parent == nil || child == nil {
		return
	}
	for i, it := range parent.Contents {
		if it == child {
			copy(parent.Contents[i:], parent.Contents[i+1:])
			parent.Contents[len(parent.Contents)-1] = nil
			parent.Contents = parent.Contents[:len(parent.Contents)-1]
			child.detach()
			parent.contentsChanged()
			return
		}
	}
}

func (parent *itemData) insertItem(idx int, child *itemData) {
	if parent != nil {
		parent.ui().bindItems([]*itemData{child})
	}
	if parent == nil || child == nil {
		return
	}
	idx = max(0, min(idx, len(parent.Contents)))
	parent.Contents = append(parent.Contents, nil)
	copy(parent.Contents[idx+1:], parent.Contents[idx:])
	parent.Contents[idx] = child
	child.Parent = parent
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent.ParentWindow)
	parent.runtimeChanged(true)
}

func (parent *windowData) insertItem(idx int, child *itemData) {
	if parent != nil {
		parent.ui().bindItems([]*itemData{child})
	}
	if parent == nil || child == nil {
		return
	}
	idx = max(0, min(idx, len(parent.Contents)))
	parent.Contents = append(parent.Contents, nil)
	copy(parent.Contents[idx+1:], parent.Contents[idx:])
	parent.Contents[idx] = child
	child.Parent = nil
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent)
	parent.contentsChanged()
}

func (parent *itemData) replaceItem(idx int, child *itemData) {
	if parent != nil {
		parent.ui().bindItems([]*itemData{child})
	}
	if parent == nil || child == nil || idx < 0 || idx >= len(parent.Contents) || parent.Contents[idx] == child {
		return
	}
	previous := parent.Contents[idx]
	parent.Contents[idx] = child
	if previous != nil {
		previous.detach()
	}
	child.Parent = parent
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent.ParentWindow)
	parent.runtimeChanged(true)
}

func (parent *windowData) replaceItem(idx int, child *itemData) {
	if parent != nil {
		parent.ui().bindItems([]*itemData{child})
	}
	if parent == nil || child == nil || idx < 0 || idx >= len(parent.Contents) || parent.Contents[idx] == child {
		return
	}
	previous := parent.Contents[idx]
	parent.Contents[idx] = child
	if previous != nil {
		previous.detach()
	}
	child.Parent = nil
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent)
	parent.contentsChanged()
}

func (item *itemData) updateText(value string) {
	if item == nil {
		return
	}
	if item.wrappedLabel != nil {
		item.SetWrappedText(value)
		return
	}
	editing := item.ItemType == ITEM_INPUT || item.ItemType == ITEM_TEXT && item.EditableText
	previous := item.Text
	if editing && item.HideText {
		previous = item.SecretText
	}
	if item.TextPtr != nil && (!item.HideText || item.TextPtr != &item.Text) {
		*item.TextPtr = value
	}
	if previous == value {
		return
	}
	if editing {
		item.CancelComposition()
		item.editor()
		item.setEditedText(value)
		// Programmatic replacement starts a fresh history, unlike ReplaceText.
		item.textEdit = &textEditState{known: value, runeCount: len([]rune(value)), initialized: true, caretOn: true, followCaret: true}
	} else {
		item.Text = value
	}
	n := len([]rune(value))
	item.CursorPos = max(0, min(item.CursorPos, n))
	item.SelectStart = max(0, min(item.SelectStart, n))
	item.SelectEnd = max(0, min(item.SelectEnd, n))
	item.selecting = false
	item.tooltipW, item.tooltipH = 0, 0
	item.runtimeChanged(!editing)
}

func (win *windowData) getMainRect() rect {
	pos := win.getPosition()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X,
		Y0: pos.Y + title,
		X1: pos.X + size.X,
		Y1: pos.Y + size.Y,
	}
}

func (win *windowData) getTitleRect() rect {
	if win.TitleHeight <= 0 {
		return rect{}
	}
	pos := win.GetPos()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X, Y0: pos.Y,
		X1: pos.X + size.X,
		Y1: pos.Y + title,
	}
}

func (win *windowData) xRect() rect {
	if win.TitleHeight <= 0 || !win.Closable {
		return rect{}
	}

	var xpad float32 = win.Border * win.scale()
	pos := win.GetPos()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X + size.X - title + xpad,
		Y0: pos.Y + xpad,

		X1: pos.X + size.X - xpad,
		Y1: pos.Y + title - xpad,
	}
}

func (win *windowData) maxRect() rect {
	if win.TitleHeight <= 0 || !win.Maximizable {
		return rect{}
	}
	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	pos := win.GetPos()
	x1 := pos.X + win.GetSize().X - xpad
	if win.Closable {
		x1 -= size
	}
	x0 := x1 - size
	return rect{
		X0: x0,
		Y0: pos.Y + xpad,
		X1: x1,
		Y1: pos.Y + size - xpad,
	}
}

func (win *windowData) searchRect() rect {
	if win.TitleHeight <= 0 || !win.Searchable {
		return rect{}
	}

	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	pos := win.GetPos()
	x1 := pos.X + win.GetSize().X - xpad
	if win.Closable {
		x1 -= size
	}
	if win.Maximizable {
		x1 -= size
	}
	x0 := x1 - size
	return rect{
		X0: x0,
		Y0: pos.Y + xpad,
		X1: x1,
		Y1: pos.Y + size - xpad,
	}
}

func (win *windowData) searchBoxRect() rect {
	if win.TitleHeight <= 0 || !win.searchOpen {
		return rect{}
	}
	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	width := max(180*win.scale(), size*2)
	sr := win.searchRect()
	x1 := sr.X0 - xpad
	width = min(width, max(0, x1-win.getPosition().X-xpad))
	x0 := x1 - width
	y0 := win.getPosition().Y + xpad
	y1 := y0 + size - xpad*2
	return rect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

func (win *windowData) searchCloseRect() rect {
	sb := win.searchBoxRect()
	if sb.X0 == 0 && sb.X1 == 0 {
		return rect{}
	}
	h := sb.Y1 - sb.Y0
	return rect{X0: sb.X1 - h, Y0: sb.Y0, X1: sb.X1, Y1: sb.Y1}
}

// closeSearch resets the search state and clears highlights.
func (win *windowData) closeSearch() {
	ctx := win.ui()

	if win.searchInput != nil {
		win.searchInput.clearHiddenState()
		win.searchInput.UpdateText("")
	}
	win.searchOpen = false
	if ctx.activeSearch == win {
		ctx.activeSearch = nil
	}
	win.SearchText = ""
	if win.OnSearch != nil {
		win.OnSearch("")
	}
	win.markDirty()
}

func (win *windowData) dragbarRect() rect {
	if win.TitleHeight <= 0 && !win.Resizable {
		return rect{}
	}
	pos := win.GetPos()
	right := pos.X + win.GetSize().X
	for _, control := range []rect{win.xRect(), win.maxRect(), win.searchRect()} {
		if control.X0 > pos.X && control.X0 < right {
			right = control.X0
		}
	}
	return rect{
		X0: pos.X,
		Y0: pos.Y,
		X1: right,
		Y1: pos.Y + win.GetTitleSize(),
	}
}

func captureScroll(items []*itemData) map[*itemData]point {
	saved := make(map[*itemData]point)
	var walk func([]*itemData)
	walk = func(list []*itemData) {
		for _, it := range list {
			if it == nil {
				continue
			}
			if it.ItemType == ITEM_FLOW {
				saved[it] = it.Scroll
			}
			if len(it.Tabs) > 0 {
				for _, tab := range it.Tabs {
					walk(tab.Contents)
				}
			}
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(items)
	return saved
}

func restoreScroll(items []*itemData, saved map[*itemData]point) {
	var walk func([]*itemData)
	walk = func(list []*itemData) {
		for _, it := range list {
			if it == nil {
				continue
			}
			if it.ItemType == ITEM_FLOW {
				if sc, ok := saved[it]; ok {
					it.Scroll = sc
					clampFlowScroll(it)
				}
			}
			if len(it.Tabs) > 0 {
				for _, tab := range it.Tabs {
					walk(tab.Contents)
				}
			}
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(items)
}

func clampFlowScroll(item *itemData) {
	req := item.contentBounds()
	size := item.GetSize()
	if req.Y <= size.Y {
		item.Scroll.Y = 0
	} else {
		max := req.Y - size.Y
		if item.Scroll.Y > max {
			item.Scroll.Y = max
		}
	}
	if req.X <= size.X {
		item.Scroll.X = 0
	} else {
		max := req.X - size.X
		if item.Scroll.X > max {
			item.Scroll.X = max
		}
	}
}

const scrollBottomSlop float32 = 1

func scrolledToBottom(scrollY, contentH, viewH float32) bool {
	if contentH <= viewH {
		return true
	}
	max := contentH - viewH
	return scrollY >= max-scrollBottomSlop
}

func (item *itemData) ScrollAtBottom() bool {
	if item == nil {
		return true
	}
	req := item.contentBounds()
	size := item.GetSize()
	return scrolledToBottom(item.Scroll.Y, req.Y, size.Y)
}

func (win *windowData) ScrollAtBottom() bool {
	if win.NoScroll {
		return true
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	availY := win.GetSize().Y - win.GetTitleSize() - 2*pad
	return scrolledToBottom(win.Scroll.Y, req.Y, availY)
}

func (win *windowData) Refresh() {
	win.RefreshWithReason("layout")
}

// RefreshWithReason refreshes layout and labels the next cached repaint for
// diagnostics. Like Refresh, this is called on the UI thread.
func (win *windowData) RefreshWithReason(reason string) {
	if !win.IsOpen() {
		return
	}

	savedWinScroll := win.Scroll
	savedFlows := captureScroll(win.Contents)

	win.resizeFlows()
	if win.AutoSize {
		win.updateAutoSize()
		if win.zone != nil {
			// After autosizing, re-apply zone centering so initial
			// creation and subsequent Refresh keep the window centered
			// on its selected zone rather than appearing pinned by
			// the top-left corner.
			win.updateZonePosition()
		}
	}

	win.Scroll = savedWinScroll
	restoreScroll(win.Contents, savedFlows)
	win.adjustScrollForResize()
	for _, item := range win.Contents {
		markItemTreeDirty(item)
	}
	win.markDirty()
	win.repaintReason = reason
}

func (win *windowData) IsOpen() bool {
	return win.Open
}

func (win *windowData) setSize(size point) bool {
	if size.X < 1 || size.Y < 1 {
		return false
	}
	// Docked geometry belongs to the workspace layout. Expanding a tile here
	// would cover its neighbours when the screen cannot fit all preferred minima.
	if !win.Docked {
		if size.X < MinWindowSize {
			size.X = MinWindowSize
		}
		if size.Y < MinWindowSize {
			size.Y = MinWindowSize
		}
	}

	old := win.Size
	win.Size = size
	if old != size {
		win.markDirty()
	}

	win.BringForward()
	win.resizeFlows()
	if old != size {
		for _, item := range win.Contents {
			markItemTreeDirty(item)
		}
	}
	win.adjustScrollForResize()
	if win.zone != nil {
		win.updateZonePosition()
	}
	win.clampToScreen()

	if old != size && win.OnResize != nil {
		win.OnResize()
	}

	return true
}

func (win *windowData) adjustScrollForResize() {
	if win.NoScroll {
		return
	}

	old := win.Scroll
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if req.Y <= avail.Y {
		win.Scroll.Y = 0
	} else {
		max := req.Y - avail.Y
		if win.Scroll.Y > max {
			win.Scroll.Y = max
		}
	}
	if req.X <= avail.X {
		win.Scroll.X = 0
	} else {
		max := req.X - avail.X
		if win.Scroll.X > max {
			win.Scroll.X = max
		}
	}
	if win.Scroll != old {
		win.markDirty()
	}
}

func (win *windowData) clampToScreen() {
	ctx := win.ui()

	pos := win.getPosition()
	size := win.GetSize()
	old := win.Position
	s := win.scale()

	// Ensure window size never exceeds the screen bounds. If it does,
	// shrink the window so it can be fully clamped on-screen.
	sizeChanged := false
	if size.X > float32(ctx.screenWidth) {
		win.Size.X = float32(ctx.screenWidth) / s
		size.X = float32(ctx.screenWidth)
		sizeChanged = true
	}
	if size.Y > float32(ctx.screenHeight) {
		win.Size.Y = float32(ctx.screenHeight) / s
		size.Y = float32(ctx.screenHeight)
		sizeChanged = true
	}

	if pos.X < 0 {
		win.Position.X -= pos.X / s
		pos.X = 0
	}
	if pos.Y < 0 {
		win.Position.Y -= pos.Y / s
		pos.Y = 0
	}

	overX := pos.X + size.X - float32(ctx.screenWidth)
	if overX > 0 {
		win.Position.X -= overX / s
	}
	overY := pos.Y + size.Y - float32(ctx.screenHeight)
	if overY > 0 {
		win.Position.Y -= overY / s
	}
	if sizeChanged {
		win.resizeFlows()
		win.adjustScrollForResize()
		if win.OnResize != nil {
			win.OnResize()
		}
	}
	if win.Position != old {
		//win.markDirty()
	}
}

// containsPointer includes the same outside-border grab area that chooses the
// resize cursor. mpos is in screen pixels.
func (win *windowData) containsPointer(mpos point) bool {
	if win.getWinRect().containsPoint(mpos) {
		return true
	}
	return !win.Docked && win.getResizePart(mpos) != PART_NONE
}

func windowAtPointer(orderedWindows []*windowData, mpos point) *windowData {
	for _, win := range orderedWindows {
		if win.Open && win.containsPointer(mpos) {
			return win
		}
	}
	return nil
}

func (win *windowData) getWindowPart(mpos point, click bool) dragType {
	s := win.scale()
	mpos = point{X: mpos.X * s, Y: mpos.Y * s}
	if win.Docked {
		// Docked panes retain title-bar search and content scrollbars, but the
		// workspace owns their move, resize, close, and maximize interactions.
		if win.TitleHeight > 0 && win.getTitleRect().containsPoint(mpos) && win.Searchable && win.searchRect().containsPoint(mpos) {
			win.HoverSearch = true
			return PART_SEARCH
		}
		return win.getScrollbarPart(mpos)
	}
	if part := win.getTitlebarPart(mpos); part != PART_NONE {
		return part
	}
	if part := win.getResizePart(mpos); part != PART_NONE {
		return part
	}
	if !win.Resizable && win.Movable {
		ct := cornerTolerance * s
		winRect := win.getWinRect()
		inCorner := func(x0, y0 float32) bool {
			return mpos.X >= x0-ct && mpos.X <= x0+ct && mpos.Y >= y0-ct && mpos.Y <= y0+ct
		}
		if inCorner(winRect.X0, winRect.Y0) || inCorner(winRect.X1, winRect.Y0) || inCorner(winRect.X0, winRect.Y1) || inCorner(winRect.X1, winRect.Y1) {
			return PART_BAR
		}
	}
	return win.getScrollbarPart(mpos)
}

// SetDocked switches a window between standalone and workspace-owned input
// and rendering behavior.
func (win *windowData) SetDocked(docked bool) {
	if win.Docked == docked {
		return
	}
	win.Docked = docked
	win.Dirty = true
}

// Update cached title bars on both hover entry and exit, including windows
// covered by another window or an open menu. Drawing never consumes this state.
func (ctx *Context) updateCloseHover(orderedWindows []*windowData, mpos point) {
	blocked := ctx.contextMenuContainsAnywhere(mpos) || ctx.dropdownOpenContainsAnywhere(mpos)
	for _, win := range orderedWindows {
		hover := false
		if !blocked && win.Open && win.getWinRect().containsPoint(mpos) {
			hover = !win.Docked && win.Closable && win.TitleHeight > 0 && win.xRect().containsPoint(mpos)
			blocked = true
		}
		if win.HoverClose != hover {
			win.HoverClose = hover
			win.markDirty()
		}
	}
}

func (win *windowData) getTitlebarPart(mpos point) dragType {
	if win.TitleHeight <= 0 {
		return PART_NONE
	}
	if win.getTitleRect().containsPoint(mpos) {
		if win.searchOpen {
			if win.searchCloseRect().containsPoint(mpos) || win.searchBoxRect().containsPoint(mpos) {
				return PART_NONE
			}
		}
		if win.Closable && win.xRect().containsPoint(mpos) {
			return PART_CLOSE
		}
		if win.Maximizable && win.maxRect().containsPoint(mpos) {
			win.HoverMax = true
			return PART_MAXIMIZE
		}
		if win.Searchable && win.searchRect().containsPoint(mpos) {
			win.HoverSearch = true
			return PART_SEARCH
		}
		if win.Movable && win.dragbarRect().containsPoint(mpos) {
			win.HoverDragbar = true
			return PART_BAR
		}
	}
	return PART_NONE
}

func (win *windowData) getResizePart(mpos point) dragType {
	if !win.Resizable {
		return PART_NONE
	}

	s := win.scale()
	t := scrollTolerance * s
	ct := cornerTolerance * s
	winRect := win.getWinRect()
	// Check enlarged corner areas first
	if mpos.X >= winRect.X0-ct && mpos.X <= winRect.X0+ct && mpos.Y >= winRect.Y0-ct && mpos.Y <= winRect.Y0+ct {
		return PART_TOP_LEFT
	}
	if mpos.X >= winRect.X1-ct && mpos.X <= winRect.X1+ct && mpos.Y >= winRect.Y0-ct && mpos.Y <= winRect.Y0+ct {
		return PART_TOP_RIGHT
	}
	if mpos.X >= winRect.X0-ct && mpos.X <= winRect.X0+ct && mpos.Y >= winRect.Y1-ct && mpos.Y <= winRect.Y1+ct {
		return PART_BOTTOM_LEFT
	}
	if mpos.X >= winRect.X1-ct && mpos.X <= winRect.X1+ct && mpos.Y >= winRect.Y1-ct && mpos.Y <= winRect.Y1+ct {
		return PART_BOTTOM_RIGHT
	}
	outRect := winRect
	outRect.X0 -= t
	outRect.X1 += t
	outRect.Y0 -= t
	outRect.Y1 += t

	inRect := winRect
	inRect.X0 += t
	inRect.X1 -= t
	inRect.Y0 += t
	inRect.Y1 -= t

	if outRect.containsPoint(mpos) && !inRect.containsPoint(mpos) {
		top := mpos.Y < inRect.Y0
		bottom := mpos.Y > inRect.Y1
		left := mpos.X < inRect.X0
		right := mpos.X > inRect.X1

		switch {
		case top && left:
			return PART_TOP_LEFT
		case top && right:
			return PART_TOP_RIGHT
		case bottom && left:
			return PART_BOTTOM_LEFT
		case bottom && right:
			return PART_BOTTOM_RIGHT
		case top:
			return PART_TOP
		case bottom:
			return PART_BOTTOM
		case left:
			return PART_LEFT
		case right:
			return PART_RIGHT
		}
	}
	return PART_NONE
}

func (win *windowData) getScrollbarPart(mpos point) dragType {
	ctx := win.ui()

	if win.NoScroll {
		return PART_NONE
	}
	// Measuring nested content (including every tab label) is expensive.
	// Only resolve thumb positions when the pointer could hit a scrollbar.
	position, size := win.getPosition(), win.GetSize()
	sbW := ctx.currentStyle.BorderPad.Slider * 2
	right, bottom := position.X+size.X-win.BorderPad, position.Y+size.Y-win.BorderPad
	vertical := rect{X0: right - sbW, Y0: position.Y, X1: right, Y1: position.Y + size.Y}
	horizontal := rect{X0: position.X, Y0: bottom - sbW, X1: position.X + size.X, Y1: bottom}
	if !vertical.containsPoint(mpos) && !horizontal.containsPoint(mpos) {
		return PART_NONE
	}

	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if req.Y > avail.Y {
		barH := scrollbarThumbLength(avail.Y, req.Y, win.scale())
		maxScroll := req.Y - avail.Y
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.Y / maxScroll) * (avail.Y - barH)
		}
		sbW := ctx.currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: win.getPosition().X + win.GetSize().X - win.BorderPad - sbW,
			Y0: win.getPosition().Y + win.GetTitleSize() + win.BorderPad + pos,
			X1: win.getPosition().X + win.GetSize().X - win.BorderPad,
			Y1: win.getPosition().Y + win.GetTitleSize() + win.BorderPad + pos + barH,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_V
		}
	}
	if req.X > avail.X {
		barW := scrollbarThumbLength(avail.X, req.X, win.scale())
		maxScroll := req.X - avail.X
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.X / maxScroll) * (avail.X - barW)
		}
		sbW := ctx.currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: win.getPosition().X + win.BorderPad + pos,
			Y0: win.getPosition().Y + win.GetSize().Y - win.BorderPad - sbW,
			X1: win.getPosition().X + win.BorderPad + pos + barW,
			Y1: win.getPosition().Y + win.GetSize().Y - win.BorderPad,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_H
		}
	}
	return PART_NONE
}

func (item *itemData) getScrollbarPart(mpos point) dragType {
	ctx := item.ui()

	if !item.Scrollable {
		return PART_NONE
	}
	sbW := ctx.currentStyle.BorderPad.Slider * 2
	track := item.DrawRect
	switch item.FlowType {
	case FLOW_VERTICAL, FLOW_VERTICAL_REV:
		track.X0 = track.X1 - sbW
	case FLOW_HORIZONTAL, FLOW_HORIZONTAL_REV:
		track.Y0 = track.Y1 - sbW
	default:
		return PART_NONE
	}
	if !track.containsPoint(mpos) {
		return PART_NONE
	}

	req := item.contentBounds()
	size := item.GetSize()
	if item.verticalFlow() && req.Y > size.Y {
		barH := scrollbarThumbLength(size.Y, req.Y, ctx.UIScale())
		maxScroll := req.Y - size.Y
		pos := float32(0)
		if maxScroll > 0 {
			pos = (item.Scroll.Y / maxScroll) * (size.Y - barH)
		}
		sbW := ctx.currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: item.DrawRect.X1 - sbW,
			Y0: item.DrawRect.Y0 + pos,
			X1: item.DrawRect.X1,
			Y1: item.DrawRect.Y0 + pos + barH,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_V
		}
	}
	if item.horizontalFlow() && req.X > size.X {
		barW := scrollbarThumbLength(size.X, req.X, ctx.UIScale())
		maxScroll := req.X - size.X
		pos := float32(0)
		if maxScroll > 0 {
			pos = (item.Scroll.X / maxScroll) * (size.X - barW)
		}
		sbW := ctx.currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: item.DrawRect.X0 + pos,
			Y0: item.DrawRect.Y1 - sbW,
			X1: item.DrawRect.X0 + pos + barW,
			Y1: item.DrawRect.Y1,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_H
		}
	}
	return PART_NONE
}

func scrollbarThumbLength(track, content, scale float32) float32 {
	if track <= 0 {
		return 0
	}
	if content <= track {
		return track
	}
	length := track * track / content
	minimum := minScrollbarThumbSize * scale
	if minimum > track {
		minimum = track
	}
	if length < minimum {
		length = minimum
	}
	return length
}

func (win *windowData) SetTitleSize(size float32) {
	win.TitleHeight = size / win.scale()
}

func (ctx *Context) SetUIScale(scale float32) {
	ctx.mustAlive()

	// Clamp to a sane, supported range so very small or large values don't
	// break hit testing or rendering. The effective limit is higher than the
	// user preference limit so a 4x preference can still work on a 2x Retina
	// display.
	if scale < 0.5 {
		scale = 0.5
	} else if scale > 8.0 {
		scale = 8.0
	}
	if ctx.uiScale == scale {
		return
	}
	ctx.uiScale = scale
	for _, win := range ctx.windows {
		if win.AutoSize {
			win.updateAutoSize()
		} else {
			win.resizeFlows()
		}
		if win.zone != nil {
			win.updateZonePosition()
		}
		win.clampToScreen()
	}
	ctx.updateAllTooltipBounds()
	ctx.markAllDirty()
}

// SetUserUIScale sets the user's preferred UI scale. The effective scale also
// includes the active display's device scale factor, keeping controls and text
// physically readable on Retina and other HiDPI displays.
func (ctx *Context) SetUserUIScale(scale float32) {
	ctx.mustAlive()

	if scale < 0.5 {
		scale = 0.5
	} else if scale > 4.0 {
		scale = 4.0
	}
	ctx.userUIScale = scale
	deviceScale := float32(1)
	if ctx.AutoHiDPI {
		deviceScale = float32(ctx.lastDeviceScale)
	}
	ctx.SetUIScale(ctx.userUIScale * deviceScale)
}

// UserUIScale reports the cross-display UI preference before automatic HiDPI
// scaling is applied.
func (ctx *Context) UserUIScale() float32 {
	ctx.mustAlive()
	return ctx.userUIScale
}

func (ctx *Context) UIScale() float32 {
	ctx.mustAlive()
	return ctx.uiScale
}

func (win *windowData) scale() float32 {
	ctx := win.ui()

	if win.NoScale {
		return 1
	}
	return ctx.uiScale
}

func (win *windowData) GetRawTitleSize() float32 { return win.TitleHeight }

func (win *windowData) GetTitleSize() float32 {
	return win.TitleHeight * win.scale()
}

func (win *windowData) GetSize() Point {
	s := win.scale()
	return Point{X: win.Size.X * s, Y: win.Size.Y * s}
}

// RequiresScroll reports whether the window content exceeds its visible area.
// It is useful to callers that need to guarantee a fixed settings layout fits
// without introducing a scrollbar.
func (win *windowData) RequiresScroll() (horizontal, vertical bool) {
	if win.NoScroll {
		return false, false
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	return req.X > avail.X, req.Y > avail.Y
}

func (win *windowData) GetPos() Point {
	s := win.scale()
	return Point{X: win.Position.X * s, Y: win.Position.Y * s}
}

func (win *windowData) SetPos(pos Point) bool {
	if win.zone != nil {
		return false
	}
	s := win.scale()
	win.Position = point{X: pos.X / s, Y: pos.Y / s}
	win.clampToScreen()
	return true
}

func (win *windowData) SetSize(size Point) bool {
	if !win.Resizable {
		return false
	}
	s := win.scale()
	return win.setSize(point{X: size.X / s, Y: size.Y / s})
}

func (win *windowData) GetRawSize() Point { return win.Size }

func (win *windowData) GetRawPos() Point { return win.Position }

func (item *itemData) intrinsicSize() Point {
	ctx := item.ui()

	if item.Invisible {
		return Point{}
	}
	item.layoutWrappedLabel()
	// Start with the explicitly set size (scaled to pixels).
	sz := Point{X: item.Size.X * ctx.uiScale, Y: item.Size.Y * ctx.uiScale}

	// Auto-size when a dimension is missing or zero.
	if sz.X <= 0 || sz.Y <= 0 {
		// 1) If this is an image and no size is provided, use the image bounds.
		if item.Image != nil {
			b := item.Image.Bounds()
			w, h := b.Dx(), b.Dy()
			if sz.X <= 0 {
				sz.X = float32(w)
			}
			if sz.Y <= 0 {
				sz.Y = float32(h)
			}
		}

		// 2) If this is a text item, derive size from its text content and font metrics.
		if item.ItemType == ITEM_TEXT && (sz.X <= 0 || sz.Y <= 0) {
			// Choose an effective font size.
			effFont := item.FontSize
			if effFont <= 0 {
				if st := item.themeStyle(); st != nil && st.FontSize > 0 {
					effFont = st.FontSize
				} else {
					effFont = 12
				}
			}
			textSize := (effFont * ctx.uiScale) + 2
			face := ctx.textFace(textSize)
			metrics := item.textContentSize(face)
			if sz.X <= 0 {
				if metrics.width <= 0 {
					sz.X = textSize // minimal width
				} else {
					sz.X = float32(metrics.width)
				}
			}
			if sz.Y <= 0 {
				sz.Y = metrics.height
			}
		}

		// 3) For flows with unspecified size, adopt their content bounds for missing dimensions.
		if item.ItemType == ITEM_FLOW && (sz.X <= 0 || sz.Y <= 0) {
			cb := item.contentBounds()
			if sz.X <= 0 {
				sz.X = cb.X
			}
			if sz.Y <= 0 {
				sz.Y = cb.Y
			}
		}

		// 4) Fall back to theme defaults for remaining unspecified dimensions.
		if (sz.X <= 0 || sz.Y <= 0) && ctx.currentTheme != nil {
			if st := item.themeStyle(); st != nil {
				def := st.GetSize()
				if sz.X <= 0 {
					sz.X = def.X
				}
				if sz.Y <= 0 {
					sz.Y = def.Y
				}
			}
		}
		// Ensure size is at least 1px to participate in layout.
		if sz.X <= 0 {
			sz.X = 1
		}
		if sz.Y <= 0 {
			sz.Y = 1
		}
	}

	if item.ItemType == ITEM_FLOW && !item.Fixed && !item.ConstrainToSize {
		content := item.contentBounds()
		sz.X, sz.Y = max(sz.X, content.X), max(sz.Y, content.Y)
	}

	if item.Size.X <= 0 && item.ItemType == ITEM_TEXT && item.Tooltip != "" && item.ParentWindow != nil && item.ParentWindow.ShowTooltipIndicators {
		sz.X += 18 * ctx.uiScale
	}

	// Text commonly changes after a window has been created (validation and
	// status messages in particular). Unless the caller explicitly requests a
	// constrained box, let the item grow beyond its theme default so the new
	// content is not clipped by a stale fixed width.
	if item.ItemType == ITEM_TEXT && !item.ConstrainToSize {
		effFont := item.FontSize
		if effFont <= 0 {
			if st := item.themeStyle(); st != nil && st.FontSize > 0 {
				effFont = st.FontSize
			} else {
				effFont = 12
			}
		}
		textSize := effFont*ctx.uiScale + 2
		face := ctx.itemFace(item, textSize)
		var metrics textContentMetrics
		if defaultFace := ctx.textFace(textSize); face == defaultFace {
			metrics = item.textContentSize(defaultFace)
		} else {
			metrics = measureTextContent(item.Text, face)
		}
		sz.X = max(sz.X, float32(math.Ceil(metrics.width)))
	}

	if !item.ConstrainToSize || item.Size.X <= 0 {
		sz.X = max(sz.X, item.buttonContentWidth())
	}

	// Account for label text below an item if set.
	if item.Label != "" {
		textSize := (item.FontSize * ctx.uiScale) + 2
		sz.Y += textSize + ctx.currentStyle.TextPadding*ctx.uiScale
	}
	return sz
}

func (item *itemData) GetPos() Point {
	ctx := item.ui()

	return Point{X: item.Position.X * ctx.uiScale, Y: item.Position.Y * ctx.uiScale}
}

func (item *itemData) GetTextPtr() *string {
	return &item.Text
}

func itemAcceptsTextEditing(item *itemData) bool {
	return item != nil && !item.isDisabled() && (item.ItemType == ITEM_INPUT || (item.ItemType == ITEM_TEXT && item.EditableText))
}

func itemHandlesTextEditing(item *itemData) bool {
	return itemAcceptsTextEditing(item) && !item.ExternalTextEditing
}

const (
	tooltipMaxRunes = 256
	tooltipMaxWidth = 340
	tooltipPadding  = 4
)

// SetTooltip assigns compact, wrapped tooltip text and caches its measured size.
func (item *itemData) SetTooltip(tip string) {
	if item == nil {
		return
	}
	raw := truncateTooltip(tip)
	if item.tooltipRaw == raw {
		return
	}
	had := item.Tooltip != ""
	item.tooltipRaw = raw
	item.updateTooltipBounds()
	layout := item.ParentWindow != nil && item.ParentWindow.ShowTooltipIndicators && had != (item.Tooltip != "")
	item.runtimeChanged(layout)
}

func (item *itemData) updateTooltipBounds() {
	ctx := item.ui()

	if item == nil {
		return
	}
	if item.tooltipRaw == "" {
		item.Tooltip = ""
		item.tooltipW, item.tooltipH = 0, 0
		return
	}
	faceSize := float32(12) * ctx.uiScale
	face := ctx.textFace(faceSize)
	item.Tooltip = wrapTooltip(item.tooltipRaw, face, ctx.tooltipWrapWidth())
	w, h := text.Measure(item.Tooltip, face, ctx.tooltipLineSpacing(face))
	item.tooltipW = float32(w)
	item.tooltipH = float32(h)
}

func (ctx *Context) tooltipWrapWidth() float64 {
	maxWidth := tooltipMaxWidth * float64(ctx.uiScale)
	available := float64(ctx.screenWidth) - 2*tooltipPadding*float64(ctx.uiScale)
	if available < maxWidth {
		return math.Max(float64(ctx.uiScale), available)
	}
	return maxWidth
}

func (ctx *Context) tooltipLineSpacing(face text.Face) float64 {
	metrics := face.Metrics()
	return math.Ceil(metrics.HAscent + metrics.HDescent + 2*float64(ctx.uiScale))
}

func truncateTooltip(tip string) string {
	tip = strings.Join(strings.Fields(tip), " ")
	runes := []rune(tip)
	if len(runes) <= tooltipMaxRunes {
		return tip
	}
	const suffix = " ..."
	limit := tooltipMaxRunes - len([]rune(suffix))
	short := strings.TrimSpace(string(runes[:limit]))
	if split := strings.LastIndexByte(short, ' '); split >= limit/2 {
		short = strings.TrimSpace(short[:split])
	}
	return short + suffix
}

func wrapTooltip(tip string, face text.Face, maxWidth float64) string {
	words := strings.Fields(tip)
	if len(words) == 0 || maxWidth <= 0 {
		return ""
	}
	lines := make([]string, 0, 3)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if width, _ := text.Measure(candidate, face, 0); width <= maxWidth {
			line = candidate
			continue
		}
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
		for len(word) > 0 {
			chunk, rest := tooltipWordChunk(word, face, maxWidth)
			if rest == "" {
				line = chunk
				break
			}
			lines = append(lines, chunk)
			word = rest
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func tooltipWordChunk(word string, face text.Face, maxWidth float64) (chunk, rest string) {
	runes := []rune(word)
	for end := len(runes); end > 0; end-- {
		candidate := string(runes[:end])
		if width, _ := text.Measure(candidate, face, 0); width <= maxWidth || end == 1 {
			return candidate, string(runes[end:])
		}
	}
	return word, ""
}

func (win *windowData) markDirty() {
	ctx := win.ui()

	if win == nil {
		return
	}
	if win.repaintRequested.IsZero() {
		win.repaintRequested = ctx.uiNow()
	}
	if win.refreshInterval > 0 && win.Render != nil && !win.lastRefresh.IsZero() && ctx.uiNow().Sub(win.lastRefresh) < win.refreshInterval {
		win.refreshPending = true
		return
	}
	win.Dirty = true
}

func (item *itemData) markDirty() {
	if item != nil {
		if item.ItemType != ITEM_FLOW {
			item.Dirty = true
		}
		if item.ParentWindow != nil {
			item.ParentWindow.markDirty()
			if item.ParentWindow.repaintReason == "" {
				item.ParentWindow.repaintReason = "item"
			}
		}
	}
}

// UpdateImage replaces the item's image and adjusts size if needed.
func (item *itemData) UpdateImage(img *ebiten.Image) {
	if item == nil {
		return
	}
	item.Image = img
	if img != nil {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		if item.Size.X != float32(w) || item.Size.Y != float32(h) {
			item.Size = point{X: float32(w), Y: float32(h)}
		}
	}
	item.runtimeChanged(true)
}

func (item *itemData) setParentWindow(win *windowData) {
	item.ParentWindow = win
	for _, child := range item.Contents {
		if child != nil {
			child.Parent = item
			child.setParentWindow(win)
		}
	}
	for _, tab := range item.Tabs {
		if tab != nil {
			tab.Parent = item
			tab.setParentWindow(win)
		}
	}
	item.normalizeTabs()
}

// itemsHaveIndeterminate reports whether any of the provided items or their
// children contain an indeterminate progress bar.
func itemsHaveIndeterminate(items []*itemData) bool {
	for _, it := range items {
		if it == nil || it.Invisible {
			continue
		}
		if it.ItemType == ITEM_PROGRESS && it.Indeterminate {
			return true
		}
		if itemsHaveIndeterminate(it.activeContents()) {
			return true
		}
	}
	return false
}

// updateHasIndeterminate refreshes the window's HasIndeterminate flag based on
// its current contents.
func (win *windowData) updateHasIndeterminate() {
	if win == nil {
		return
	}
	win.HasIndeterminate = itemsHaveIndeterminate(win.Contents)
}

func markItemTreeDirty(it *itemData) {
	if it == nil {
		return
	}
	it.markDirty()
	for _, child := range it.Contents {
		markItemTreeDirty(child)
	}
	for _, tab := range it.Tabs {
		markItemTreeDirty(tab)
	}
}

// updateItemTooltipTree walks items and updates cached tooltip bounds.
func updateItemTooltipTree(it *itemData) {
	if it == nil {
		return
	}
	it.updateTooltipBounds()
	for _, child := range it.Contents {
		updateItemTooltipTree(child)
	}
	for _, tab := range it.Tabs {
		updateItemTooltipTree(tab)
	}
}

func (ctx *Context) updateAllTooltipBounds() {
	for _, win := range ctx.windows {
		for _, it := range win.Contents {
			updateItemTooltipTree(it)
		}
	}
}

func (ctx *Context) markAllDirty() {
	for _, win := range ctx.windows {
		win.markDirty()
		for _, it := range win.Contents {
			markItemTreeDirty(it)
		}
	}
}

func (item *itemData) bounds(offset point) rect {
	if item == nil || item.Invisible {
		return rect{}
	}
	size := item.GetSize()
	r := rect{X0: offset.X, Y0: offset.Y, X1: offset.X + size.X, Y1: offset.Y + size.Y}
	if item.ItemType == ITEM_FLOW {
		return r
	}
	for _, sub := range item.Contents {
		if sub != nil && !sub.Invisible {
			r = unionRect(r, sub.bounds(pointAdd(offset, sub.getPosition(item.ParentWindow))))
		}
	}
	return r
}

func (win *windowData) contentBounds() point {
	if len(win.Contents) == 0 {
		return point{}
	}

	base := point{X: 0, Y: win.GetTitleSize()}
	first := true
	var b rect

	for _, item := range win.Contents {
		if item.Invisible {
			continue
		}
		// Use the generic bounds calculation for all items, including flows.
		// This ensures fixed flows contribute their set size to autosize,
		// while unfixed flows contribute based on their content.
		r := item.bounds(pointAdd(base, item.getPosition(win)))
		if first {
			b = r
			first = false
		} else {
			b = unionRect(b, r)
		}
	}

	if first {
		return point{}
	}
	return point{X: b.X1 - base.X, Y: b.Y1 - base.Y}
}

func (win *windowData) updateAutoSize() {
	ctx := win.ui()

	// Resolve nested flows before measuring them. In particular, a UI-scale
	// change can leave a flow with its previous dimensions until it is laid out
	// again; measuring that stale size made an auto-sized window one layout pass
	// too short and caused an unnecessary scrollbar.
	win.resizeFlows()
	req := win.contentBounds()
	pad := (win.Padding + win.BorderPad) * win.scale()

	size := win.GetSize()
	needX := float32(math.Ceil(float64(req.X + 2*pad)))
	if needX > size.X {
		size.X = needX
	}

	// Always include the titlebar height in the calculated size
	// Round outward to whole pixels so fractional UI scales cannot leave the
	// viewport slightly smaller than its content after converting to UI units.
	size.Y = float32(math.Ceil(float64(req.Y + win.GetTitleSize() + 2*pad)))
	if size.X > float32(ctx.screenWidth) {
		size.X = float32(ctx.screenWidth)
	}
	if size.Y > float32(ctx.screenHeight) {
		size.Y = float32(ctx.screenHeight)
	}
	s := win.scale()
	win.Size = point{X: size.X / s, Y: size.Y / s}
	win.resizeFlows()

	// A parent flow can grow after it receives its final width. Re-measure once
	// so the window tracks that resolved content rather than leaving a stray
	// scroll range after increasing UI scale.
	resolved := win.contentBounds()
	resolvedY := float32(math.Ceil(float64(resolved.Y + win.GetTitleSize() + 2*pad)))
	if resolvedY > size.Y && resolvedY <= float32(ctx.screenHeight) {
		win.Size.Y = resolvedY / s
		win.resizeFlows()
	}
	win.clampToScreen()
}

func (item *itemData) contentBounds() point {
	ctx := item.ui()

	if item == nil || item.Invisible {
		return point{}
	}
	size := point{X: item.Size.X * ctx.uiScale, Y: item.Size.Y * ctx.uiScale}
	if item.hasResolvedLayout() {
		size = item.layoutSize
	}
	padding := item.contentInsets()
	result := point{X: max(ctx.tabStripWidth(item), padding.Left+padding.Right), Y: ctx.tabStripHeight(item, item.themeStyle()) + padding.Top + padding.Bottom}
	for child, pos := range item.layoutChildren(size) {
		r := child.bounds(pos)
		result.X = max(result.X, r.X1+padding.Right)
		result.Y = max(result.Y, r.Y1+padding.Bottom)
	}
	return result
}

func (item *itemData) resizeFlow(parentSize point) {
	item.clearLayout()
	item.measureLayout()
	item.resolveLayout(parentSize, point{X: -1, Y: -1})
}

func (win *windowData) resizeFlows() {
	ctx := win.ui()
	ctx.bindItems(win.Contents)

	pad := (win.Padding + win.BorderPad) * win.scale()
	available := point{X: max(1, win.GetSize().X-2*pad), Y: max(1, win.GetSize().Y-win.GetTitleSize()-2*pad)}
	layout := func(space Point) {
		for _, item := range win.Contents {
			position := item.getPosition(win)
			item.resizeFlow(point{X: max(0, space.X-position.X), Y: max(0, space.Y-position.Y)})
		}
	}
	layout(available)
	if !win.NoScroll && win.contentBounds().Y > available.Y {
		available.X = max(0, available.X-ctx.currentStyle.BorderPad.Slider*2)
		layout(available)
	}

}

func pixelOffset(width float32) float32 {
	if int(math.Round(float64(width)))%2 == 0 {
		return 0
	}
	return 0.5
}

func (ctx *Context) strokeLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, col color.Color, aa bool) {
	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)
	x0 = float32(math.Round(float64(x0))) + off
	y0 = float32(math.Round(float64(y0))) + off
	x1 = float32(math.Round(float64(x1))) + off
	y1 = float32(math.Round(float64(y1))) + off
	if !ctx.drawCachedLine(dst, x0, y0, x1, y1, width, col, aa) {
		ctx.strokeLineFn(dst, x0, y0, x1, y1, width, col, aa)
	}
}

func (ctx *Context) strokeRect(dst *ebiten.Image, x, y, w, h, width float32, col color.Color, aa bool) {
	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)
	x = float32(math.Round(float64(x))) + off
	y = float32(math.Round(float64(y))) + off
	w = float32(math.Round(float64(w)))
	h = float32(math.Round(float64(h)))
	if !drawPixelFrame(dst, x, y, w, h, width, col) {
		ctx.strokeRectFn(dst, x, y, w, h, width, col, aa)
	}
}

func drawFilledRect(dst *ebiten.Image, x, y, w, h float32, col color.Color, _ bool) {
	x = float32(math.Round(float64(x)))
	y = float32(math.Round(float64(y)))
	w = float32(math.Round(float64(w)))
	h = float32(math.Round(float64(h)))
	// All edges are on pixel boundaries, so antialiasing adds no coverage.
	// The non-AA path batches an image quad instead of building a stencil
	// and filling it, which is especially costly during full-pane repaints.
	vector.FillRect(dst, x, y, w, h, col, false)
}
