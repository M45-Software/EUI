package eui

import (
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Add window to window list
func (target *windowData) AddWindow(toBack bool) {
	ctx := target.ui()

	for _, win := range ctx.windows {
		if win == target {
			log.Println("Window already exists")
			return
		}
	}

	if target.keyboardOrder == 0 {
		ctx.windowFocusSerial++
		target.keyboardOrder = ctx.windowFocusSerial
	}

	if target.AutoSize {
		target.updateAutoSize()
		if target.zone != nil {
			// Re-center to the chosen zone after size changes
			target.updateZonePosition()
		}
	}

	// Closed windows shouldn't steal focus, so add them to the back by
	// default and don't update the active window.
	if !target.Open {
		toBack = true
	}

	if target.AlwaysDrawFirst {
		ctx.windows = append([]*windowData{target}, ctx.windows...)
		return
	}

	if !toBack {
		ctx.windows = append(ctx.windows, target)
	} else {
		idx := 0
		for idx < len(ctx.windows) && ctx.windows[idx].AlwaysDrawFirst {
			idx++
		}
		ctx.windows = append(ctx.windows[:idx], append([]*windowData{target}, ctx.windows[idx:]...)...)
	}
}

// deallocate releases cached render images for the window and its items.
func (target *windowData) deallocate() {
	ctx := target.ui()

	target.releaseOwnImages()
	if target.searchInput != nil {
		target.searchInput.deallocate()
	}
	for _, item := range target.Contents {
		if item == nil || item.ctx != ctx {
			continue
		}
		item.deallocate()
	}
}

func (target *windowData) releaseOwnImages() {
	ctx := target.ui()
	if target.Render != nil {
		ctx.releaseUIRenderTarget(target.Render)
		target.Render = nil
	}
	if target.titleRender != nil {
		ctx.releaseUIRenderTarget(target.titleRender)
		target.titleRender = nil
		target.titleRenderValid = false
	}
}

// deallocate releases cached render images for the item and its children.
func (item *itemData) deallocate() {
	ctx := item.ui()

	if item == nil {
		return
	}
	item.releaseOwnImages()
	for _, child := range item.Contents {
		if child == nil || child.ctx != ctx {
			continue
		}
		child.deallocate()
	}
	for _, tab := range item.Tabs {
		if tab == nil || tab.ctx != ctx {
			continue
		}
		tab.deallocate()
	}
}

func (item *itemData) releaseOwnImages() {
	ctx := item.ui()
	if item.ItemType == ITEM_COLORWHEEL && item.Image != nil {
		ctx.releaseUIRenderTarget(item.Image)
		item.Image = nil
	}
	if item.Render != nil {
		ctx.releaseUIRenderTarget(item.Render)
		item.Render = nil
	}
}

// clearInteraction retires input state before callbacks can open other windows.
func (target *windowData) clearInteraction() {
	ctx := target.ui()

	if target.searchInput != nil {
		target.searchInput.clearHiddenState()
	}
	for _, item := range target.Contents {
		item.clearHiddenState()
	}
	if ctx.activeSearch == target {
		ctx.activeSearch = nil
	}
	target.searchOpen = false
	if ctx.dragWin == target {
		ctx.dragWin, ctx.dragFlow, ctx.dragPart = nil, nil, PART_NONE
	}
	if ctx.activeWindow == target {
		ctx.activeWindow = nil
		for i := len(ctx.windows) - 1; i >= 0; i-- {
			if ctx.windows[i] != target && ctx.windows[i].Open {
				ctx.activeWindow = ctx.windows[i]
				break
			}
		}
	}
}

// RemoveWindow removes a window from the active list. Any cached images
// belonging to the window are disposed and pointers cleared.
func (target *windowData) RemoveWindow() {
	ctx := target.ui()

	for i, win := range ctx.windows {
		if win == target { // Compare pointers
			win.clearInteraction()
			ctx.windows = append(ctx.windows[:i], ctx.windows[i+1:]...)
			win.deallocate()
			win.Open = false
			return
		}
	}

	log.Println("Window not found")
}

// Create a new window from the default theme
func (ctx *Context) NewWindow() *windowData {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newWindow := ctx.currentTheme.Window
	newWindow.ctx = ctx
	// Window colors are theme values unless the caller explicitly overrides
	// them. Leaving the copied values here freezes the palette that was active
	// when the window was created.
	newWindow.BGColor = Color{}
	newWindow.TitleBGColor = Color{}
	newWindow.BorderColor = Color{}
	// Default: windows can be maximized if desired by the app
	newWindow.Maximizable = false
	newWindow.Theme = ctx.currentTheme
	return ctx.registerWindow(&newWindow)
}

func (win *windowData) backgroundColor() Color {
	ctx := win.ui()

	if win == nil {
		return Color{}
	}
	if win.BGColor != (Color{}) {
		return win.BGColor
	}
	if win.Theme != nil {
		return win.Theme.Window.BGColor
	}
	if ctx.currentTheme != nil {
		return ctx.currentTheme.Window.BGColor
	}
	return baseTheme.Window.BGColor
}

// BackgroundColor returns the window's resolved background color, including
// its active theme when the window does not define an override.
func (win *windowData) BackgroundColor() Color {
	return win.backgroundColor()
}

func (win *windowData) titleBackgroundColor() Color {
	ctx := win.ui()

	if win == nil {
		return Color{}
	}
	if win.TitleBGColor != (Color{}) {
		return win.TitleBGColor
	}
	if win.Theme != nil {
		return win.Theme.Window.TitleBGColor
	}
	if ctx.currentTheme != nil {
		return ctx.currentTheme.Window.TitleBGColor
	}
	return baseTheme.Window.TitleBGColor
}

func (win *windowData) backgroundTint(c Color) Color {
	opacity := 1 - max(float32(0), min(float32(1), win.BackgroundTransparency))
	// Color uses premultiplied RGBA, so fade all four channels together.
	return Color{
		R: uint8(float32(c.R) * opacity), G: uint8(float32(c.G) * opacity),
		B: uint8(float32(c.B) * opacity), A: uint8(float32(c.A) * opacity),
	}
}

// Maximize resizes this window to cover the full screen area and moves it to (0,0).
func (win *windowData) Maximize() {
	ctx := win.ui()

	if win == nil {
		return
	}
	win.ClearZone()
	_ = win.SetPos(Point{X: 0, Y: 0})
	_ = win.SetSize(Point{X: float32(ctx.screenWidth), Y: float32(ctx.screenHeight)})
}

// Create a new button from the default theme
func (ctx *Context) NewButton() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Button
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new button from the default theme
func (ctx *Context) NewCheckbox() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Checkbox
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new radio button from the default theme
func (ctx *Context) NewRadio() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Radio
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new input box from the default theme
func (ctx *Context) NewInput() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Input
	newItem.ctx = ctx
	newItem.SelectableText = true
	if newItem.TextPtr == nil {
		newItem.TextPtr = &newItem.Text
	} else {
		*newItem.TextPtr = newItem.Text
	}
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// NewTextArea creates a bounded, multiline text input with horizontal and
// vertical scrolling. Text is not soft-wrapped. Set AcceptTab for code editing.
func (ctx *Context) NewTextArea() (*itemData, *EventHandler) {
	ctx.mustAlive()

	item, events := ctx.NewInput()
	item.Multiline = true
	item.Size = Point{X: 360, Y: 180}
	return item, events
}

// Create a new slider from the default theme
func (ctx *Context) NewSlider() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Slider
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new dropdown from the default theme
func (ctx *Context) NewDropdown() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Dropdown
	newItem.ctx = ctx
	newItem.HoverIndex = -1
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new color wheel from the default theme
func (ctx *Context) NewColorWheel() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := baseColorWheel
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new image item with a new image buffer
func (ctx *Context) NewImageItem(w, h int) (*itemData, *ebiten.Image) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := itemData{ctx: ctx,
		ItemType: ITEM_IMAGE,
		Size:     point{X: float32(w), Y: float32(h)},
		Theme:    ctx.currentTheme,
	}
	newItem.Image = ctx.newManagedImage(w, h)
	return ctx.registerItem(&newItem), newItem.Image
}

// NewImageReferenceItem creates an image item without allocating a backing
// render target. Use it when the image will be supplied later or borrowed from
// a stable cache.
func (ctx *Context) NewImageReferenceItem(w, h int) *itemData {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	return ctx.registerItem(&itemData{ctx: ctx,
		ItemType: ITEM_IMAGE,
		Size:     point{X: float32(w), Y: float32(h)},
		Theme:    ctx.currentTheme,
	})
}

// Create a new image item with a new image buffer
func (ctx *Context) NewImageFastItem(w, h int) (*itemData, *ebiten.Image) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := itemData{ctx: ctx,
		ItemType: ITEM_IMAGE_FAST,
		Size:     point{X: float32(w), Y: float32(h)},
		Theme:    ctx.currentTheme,
	}
	newItem.Image = ctx.newUnmanagedImage(w, h)
	return ctx.registerItem(&newItem), newItem.Image
}

// Create a new textbox from the default theme
func (ctx *Context) NewText() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Text
	newItem.ctx = ctx
	// Ensure a default text face is available immediately for code that
	// queries metrics without drawing first.
	if newItem.Face == nil {
		// Face is public and callers may mutate it. Keep EUI's shared faces
		// private so their measured text can be cached safely.
		face := *ctx.textFace(newItem.FontSize)
		newItem.Face = &face
	}
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Create a new progress bar from the default theme
func (ctx *Context) NewProgressBar() (*itemData, *EventHandler) {
	ctx.mustAlive()

	if ctx.currentTheme == nil {
		ctx.currentTheme = ctx.baseThemeCopy()
	}
	newItem := ctx.currentTheme.Progress
	newItem.ctx = ctx
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = ctx.currentTheme
	return ctx.registerItem(&newItem), h
}

// Bring a window to the front
func (target *windowData) BringForward() {
	ctx := target.ui()

	if target.AlwaysDrawFirst {
		// Background windows stay behind ordinary UI, but multiple background
		// windows still need normal front-to-back behavior among themselves.
		for index, win := range ctx.windows {
			if win != target {
				continue
			}
			front := index
			for front+1 < len(ctx.windows) && ctx.windows[front+1].AlwaysDrawFirst {
				front++
			}
			if front != index {
				copy(ctx.windows[index:front], ctx.windows[index+1:front+1])
				ctx.windows[front] = target
			}
			ctx.activeWindow = target
			return
		}
		return
	}
	for w, win := range ctx.windows {
		if win == target {
			ctx.windows = append(ctx.windows[:w], ctx.windows[w+1:]...)
			ctx.windows = append(ctx.windows, target)
			ctx.activeWindow = target
		}
	}
}

// MarkOpen sets the window to open and brings it forward if necessary.
func (target *windowData) MarkOpen() {
	ctx := target.ui()

	wasOpen := target.Open
	target.Open = true
	if !wasOpen && target.OnOpen != nil {
		target.OnOpen()
	}
	found := false
	for _, win := range ctx.windows {
		if win == target {
			found = true
			break
		}
	}
	if !found {
		target.AddWindow(false)
	} else {
		target.BringForward()
	}
	target.Refresh()
	if ctx.WindowStateChanged != nil {
		ctx.WindowStateChanged()
	}

}

// SetRefreshInterval coalesces repeated redraw requests for this window.
// A zero interval keeps the default immediate refresh behavior.
func (target *windowData) SetRefreshInterval(interval time.Duration) {
	if interval < 0 {
		interval = 0
	}
	target.refreshInterval = interval
	if interval == 0 && target.refreshPending {
		target.refreshPending = false
		target.Dirty = true
	}
}

// RefreshInterval reports the window's redraw-coalescing interval.
func (target *windowData) RefreshInterval() time.Duration {
	return target.refreshInterval
}

// Refresh marks the window and its children dirty so they are redrawn and
// layout is recalculated on the next frame.

// MarkOpen sets the window to open and brings it forward if necessary.
func (target *windowData) Toggle() {
	if target.Open {
		target.Close()
	} else {
		target.MarkOpen()
	}
}

func (target *windowData) Close() {
	ctx := target.ui()

	if target.BeforeClose != nil && !target.BeforeClose() {
		return
	}
	closeDropdowns(target.Contents)
	target.Open = false
	target.clearInteraction()
	if target.OnClose != nil {
		target.OnClose()
	}
	target.deallocate()
	if ctx.WindowStateChanged != nil {
		ctx.WindowStateChanged()
	}
}

// MarkOpenNear opens the window and attempts to place it near the given
// anchor item (typically the button that triggered the open). The window is
// positioned adjacent to the anchor while trying to avoid overlapping the
// anchor's parent window when possible and clamping to screen bounds.
func (target *windowData) MarkOpenNear(anchor *itemData) {
	ctx := target.ui()

	// The workspace owns a docked pane's geometry; opening it must not
	// move it next to a menu or a settings checkbox.
	if target.Docked {
		target.MarkOpen()
		return
	}
	// Respect explicit zone placement: if a window has a zone, open it at
	// the zone rather than near the anchor.
	if target.zone != nil {
		target.MarkOpen()
		return
	}
	if anchor != nil {
		ctx.placeWindowNear(target, anchor)
	}
	target.MarkOpen()
}

// ToggleNear toggles the window's open state. When opening, it places the
// window near the given anchor item as in MarkOpenNear.
func (target *windowData) ToggleNear(anchor *itemData) {
	if target.Open {
		target.Close()
		return
	}
	// Respect zone placement if set.
	if target.zone != nil {
		target.MarkOpen()
	} else {
		target.MarkOpenNear(anchor)
	}
}

// placeWindowNear computes a good position for win next to anchor and moves it
// there, preferring not to overlap the anchor's parent window if possible.
func (ctx *Context) placeWindowNear(win *windowData, anchor *itemData) {
	if win == nil || anchor == nil {
		return
	}
	// Anchor screen rect
	ar := anchor.DrawRect
	// Candidate positions: right, below, left, above
	gap := float32(8) * ctx.UIScale()
	size := win.GetSize()
	candidates := []point{
		{X: ar.X1 + gap, Y: ar.Y0},          // right
		{X: ar.X0, Y: ar.Y1 + gap},          // below
		{X: ar.X0 - size.X - gap, Y: ar.Y0}, // left
		{X: ar.X0, Y: ar.Y0 - size.Y - gap}, // above
	}

	// Parent window rect to avoid overlapping.
	var parentRect rect
	hasParent := false
	if anchor.ParentWindow != nil {
		parentRect = anchor.ParentWindow.getWinRect()
		hasParent = true
	}

	// Helper to test overlap area after clamping.
	bestIdx := -1
	var bestOverlap float32 = -1
	for i, c := range candidates {
		// Attempt to set position (this clamps to screen)
		win.SetPos(Point(c))
		// Compute resulting rect
		wp := win.getPosition()
		wr := rect{X0: wp.X, Y0: wp.Y, X1: wp.X + size.X, Y1: wp.Y + size.Y}
		var overlap float32
		if hasParent {
			inter := intersectRect(wr, parentRect)
			if inter.X1 > inter.X0 && inter.Y1 > inter.Y0 {
				overlap = (inter.X1 - inter.X0) * (inter.Y1 - inter.Y0)
			} else {
				overlap = 0
			}
		} else {
			overlap = 0
		}
		if overlap == 0 {
			// Perfect candidate: no overlap with source window
			bestIdx = i
			bestOverlap = 0
			break
		}
		if bestIdx == -1 || overlap < bestOverlap {
			bestIdx = i
			bestOverlap = overlap
		}
	}

	// Apply best candidate if we didn't already via SetPos loop
	if bestIdx >= 0 {
		c := candidates[bestIdx]
		win.SetPos(Point(c))
	}
}

// Send a window to the back
func (target *windowData) ToBack() {
	ctx := target.ui()

	for w, win := range ctx.windows {
		if win == target {
			ctx.windows = append(ctx.windows[:w], ctx.windows[w+1:]...)
			idx := 0
			for idx < len(ctx.windows) && ctx.windows[idx].AlwaysDrawFirst {
				idx++
			}
			if target.AlwaysDrawFirst {
				idx = 0
			}
			ctx.windows = append(ctx.windows[:idx], append([]*windowData{target}, ctx.windows[idx:]...)...)
		}
	}
	if ctx.activeWindow == target {
		numWindows := len(ctx.windows)
		if numWindows > 0 {
			ctx.activeWindow = ctx.windows[numWindows-1]
		}
	}
}

// getPosition returns the window's position in screen pixels
// (i.e., scaled by the effective UI scale for this window).
func (win *windowData) getPosition() point {
	return Point{X: win.Position.X * win.scale(), Y: win.Position.Y * win.scale()}
}

// getPosition returns the item's position in screen pixels relative to its
// parent window, honoring the current UI scale.
func (item *itemData) getPosition(win *windowData) point {
	ctx := item.ui()

	s := ctx.uiScale
	if win != nil {
		s = win.scale()
	}
	return Point{X: item.Position.X * s, Y: item.Position.Y * s}
}
