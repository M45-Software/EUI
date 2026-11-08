package eui

import (
	"bytes"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Windows returns a snapshot of this context's window list.
func (ctx *Context) Windows() []*WindowData {
	ctx.mustAlive()
	return append([]*WindowData(nil), ctx.windows...)
}

// PointerPressWindow returns the window under the current pointer press. It
// remains available for the duration of the press even if that window closes.
func (ctx *Context) PointerPressWindow() *WindowData {
	ctx.mustAlive()
	return ctx.downWin
}

// PointerPressHandled reports whether EUI handled the pointer press this frame.
func (ctx *Context) PointerPressHandled() bool {
	ctx.mustAlive()
	return ctx.pointerPressHandled
}

// PointerCursorClaimed reports whether EUI selected a non-default cursor for
// the current frame. Callers that also manage the system cursor should not
// overwrite it while this is true.
func (ctx *Context) PointerCursorClaimed() bool {
	ctx.mustAlive()
	return ctx.cursorShape != ebiten.CursorShapeDefault
}

// WindowSnapping reports whether window snapping is enabled.
func (ctx *Context) WindowSnapping() bool {
	ctx.mustAlive()
	return ctx.windowSnapping
}

// SetWindowSnapping enables or disables window snapping.
func (ctx *Context) SetWindowSnapping(enabled bool) {
	ctx.mustAlive()

	ctx.windowSnapping = enabled
	if enabled {
		return
	}
	for _, win := range ctx.windows {
		win.snapAnchorActive = false
		win.snapAnchorX = false
		win.snapAnchorY = false
		win.resizeSnapAnchorX = false
		win.resizeSnapAnchorY = false
	}
}

// MiddleClickMove reports whether middle-click window dragging is enabled.
func (ctx *Context) MiddleClickMove() bool {
	ctx.mustAlive()
	return ctx.middleClickMove
}

// SetMiddleClickMove enables or disables dragging windows with the middle mouse button.
func (ctx *Context) SetMiddleClickMove(enabled bool) {
	ctx.mustAlive()
	ctx.middleClickMove = enabled
}

// SetKeyboardInputCaptured pauses EUI keyboard and gamepad navigation actions
// while leaving pointer and window controls active.
func (ctx *Context) SetKeyboardInputCaptured(captured bool) {
	ctx.mustAlive()

	if captured {
		if ctx.composingItem != nil {
			ctx.composingItem.CancelComposition()
		}
		ctx.cancelNativeTextInput(nil)
	}
	ctx.keyboardInputCaptured = captured
}

// SetScreenSize sets the current screen size used for layout calculations.
func (ctx *Context) SetScreenSize(w, h int) {
	ctx.mustAlive()

	ctx.screenWidth = w
	ctx.screenHeight = h
	needDirty := false
	for _, win := range ctx.windows {
		size := win.GetSize()
		resized := false
		if size.X > float32(ctx.screenWidth) {
			if win.NoScale {
				win.Size.X = float32(ctx.screenWidth)
			} else {
				win.Size.X = float32(ctx.screenWidth) / ctx.uiScale
			}
			resized = true
		}
		if size.Y > float32(ctx.screenHeight) {
			if win.NoScale {
				win.Size.Y = float32(ctx.screenHeight)
			} else {
				win.Size.Y = float32(ctx.screenHeight) / ctx.uiScale
			}
			resized = true
		}
		if win.AutoSize {
			win.updateAutoSize()
			win.adjustScrollForResize()
			needDirty = true
		} else if resized {
			win.resizeFlows()
			win.adjustScrollForResize()
			needDirty = true
			if win.OnResize != nil {
				win.OnResize()
			}
		}
		if win.zone != nil {
			win.updateZonePosition()
		}
		win.clampToScreen()
	}
	ctx.updateAllTooltipBounds()
	if needDirty {
		ctx.markAllDirty()
	}

}

// ScreenSize returns the current screen size.
func (ctx *Context) ScreenSize() (int, int) {
	ctx.mustAlive()
	return ctx.screenWidth, ctx.screenHeight
}

// SetFontSource sets the text face source used when rendering text.
func (ctx *Context) SetFontSource(src *text.GoTextFaceSource) {
	ctx.mustAlive()

	ctx.mplusFaceSource = src
	ctx.faceCache = map[float64]*text.GoTextFace{}
}

// SetBoldFontSource sets the bold text face source used when rendering bold text.
func (ctx *Context) SetBoldFontSource(src *text.GoTextFaceSource) {
	ctx.mustAlive()

	ctx.mplusBoldFaceSource = src
	ctx.boldFaceCache = map[float64]*text.GoTextFace{}
}

// FontSource returns the current text face source.
func (ctx *Context) FontSource() *text.GoTextFaceSource {
	ctx.mustAlive()
	return ctx.mplusFaceSource
}

// BoldFontSource returns the current bold text face source.
func (ctx *Context) BoldFontSource() *text.GoTextFaceSource {
	ctx.mustAlive()
	return ctx.mplusBoldFaceSource
}

// EnsureFontSource initializes the font source from ttf data if needed.
func (ctx *Context) EnsureFontSource(ttf []byte) error {
	ctx.mustAlive()

	if ctx.mplusFaceSource != nil {
		return nil
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		return err
	}
	ctx.mplusFaceSource = s
	ctx.faceCache = map[float64]*text.GoTextFace{}
	return nil
}

// EnsureBoldFontSource initializes the bold font source from ttf data if needed.
func (ctx *Context) EnsureBoldFontSource(ttf []byte) error {
	ctx.mustAlive()

	if ctx.mplusBoldFaceSource != nil {
		return nil
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		return err
	}
	ctx.mplusBoldFaceSource = s
	ctx.boldFaceCache = map[float64]*text.GoTextFace{}
	return nil
}

// ShowContextMenu opens a simple context menu at screen-space pixel
// coordinates (x, y) with the provided options. onSelect is called with the
// selected index when an item is clicked. Returns the handle to the menu.
// (context menu APIs are defined in context_menu.go)

// AddItem appends a child item to the parent item.
func (parent *ItemData) AddItem(child *ItemData) { parent.addItemTo(child) }

// AddItem appends a child item to the window.
func (win *WindowData) AddItem(child *ItemData) { win.addItemTo(child) }

// PrependItem prepends a child item to the parent item.
func (parent *ItemData) PrependItem(child *ItemData) { parent.prependItemTo(child) }

// PrependItem prepends a child item to the window.
func (win *WindowData) PrependItem(child *ItemData) { win.prependItemTo(child) }

// RemoveItem removes a child item from the parent item.
func (parent *ItemData) RemoveItem(child *ItemData) { parent.removeItem(child) }

// SetItems replaces a parent's children in one batch and invalidates layout.
func (parent *ItemData) SetItems(children []*ItemData) { parent.setItems(children) }

// RemoveItem removes a child item from the window.
func (win *WindowData) RemoveItem(child *ItemData) { win.removeItem(child) }

// InsertItem inserts a child item into the parent item at the provided index.
func (parent *ItemData) InsertItem(idx int, child *ItemData) { parent.insertItem(idx, child) }

// InsertItem inserts a child item into the window at the provided index.
func (win *WindowData) InsertItem(idx int, child *ItemData) { win.insertItem(idx, child) }

// ReplaceItem replaces the child item at the provided index with the new item.
func (parent *ItemData) ReplaceItem(idx int, child *ItemData) { parent.replaceItem(idx, child) }

// ReplaceItem replaces the child item at the provided index in the window.
func (win *WindowData) ReplaceItem(idx int, child *ItemData) { win.replaceItem(idx, child) }

// UpdateText silently updates text and reflows layout as needed. For inputs it
// replaces the logical value (including masked passwords), synchronizes TextPtr,
// resets undo history, and clamps caret/selection. ReplaceText is an undoable
// editing command and emits EventInputChanged instead.
func (item *ItemData) UpdateText(text string) { item.updateText(text) }

// SetProgressIndeterminate updates the indeterminate state of a progress bar
// and refreshes its parent window's tracking flag.
func SetProgressIndeterminate(pb *ItemData, ind bool) {
	if pb == nil || pb.ItemType != ITEM_PROGRESS {
		return
	}
	if pb.Indeterminate == ind {
		return
	}
	pb.Indeterminate = ind
	if pb.ParentWindow != nil {
		pb.ParentWindow.updateHasIndeterminate()
	}
	pb.markDirty()
}

// ListThemes returns the available palette names.
func (ctx *Context) ListThemes() ([]string, error) {
	ctx.mustAlive()
	return ctx.listThemes()
}

// ListStyles returns the available style theme names.
func (ctx *Context) ListStyles() ([]string, error) {
	ctx.mustAlive()
	return ctx.listStyles()
}

// SetUserDataRoot places editable themes and generated theme documentation
// under the application's persistent user data directory.
func (ctx *Context) SetUserDataRoot(root string) {
	ctx.mustAlive()

	if root == "" {
		ctx.themeDirectory = "themes"
	} else {
		ctx.themeDirectory = filepath.Join(root, "themes")
	}
	ctx.updateThemePath()
	ctx.updateStylePath()
	ctx.ensureThemeDocs()
	ctx.refreshThemeMod()
	ctx.refreshStyleMod()
}

// CurrentThemeName returns the active theme name.
func (ctx *Context) CurrentThemeName() string {
	ctx.mustAlive()
	return ctx.currentThemeName
}

// SetCurrentThemeName updates the active theme name.
func (ctx *Context) SetCurrentThemeName(name string) {
	ctx.mustAlive()

	ctx.currentThemeName = name
	ctx.updateThemePath()
}

// CurrentStyleName returns the active style theme name.
func (ctx *Context) CurrentStyleName() string {
	ctx.mustAlive()
	return ctx.currentStyleName
}

// SetCurrentStyleName updates the active style theme name.
func (ctx *Context) SetCurrentStyleName(name string) {
	ctx.mustAlive()

	ctx.currentStyleName = name
	ctx.updateStylePath()
}

// AccentSaturation returns the current accent color saturation value.
func (ctx *Context) AccentSaturation() float64 {
	ctx.mustAlive()
	return ctx.accentSaturation
}

// AccentColor returns the current accent color.
func (ctx *Context) AccentColor() Color {
	ctx.mustAlive()

	return Color(hsvaToRGBA(ctx.accentHue, ctx.accentSaturation, ctx.accentValue, ctx.accentAlpha))
}

// TextColor returns the active theme's normal text color.
func (ctx *Context) TextColor() Color {
	ctx.mustAlive()

	if ctx.currentTheme != nil {
		return ctx.currentTheme.Text.TextColor
	}
	return baseTheme.Text.TextColor
}

// IsLightTheme reports whether the active theme uses a light window background.
func (ctx *Context) IsLightTheme() bool {
	ctx.mustAlive()

	bg := baseTheme.Window.BGColor
	if ctx.currentTheme != nil {
		bg = ctx.currentTheme.Window.BGColor
	}
	// ITU-R BT.601 luma is sufficient here and avoids classifying saturated
	// backgrounds by their unweighted channel sum.
	luma := 299*int(bg.R) + 587*int(bg.G) + 114*int(bg.B)
	return luma >= 128000
}

// SubtleAlternateRowColor returns a low-contrast opaque row color derived
// from the active window background.
func (ctx *Context) SubtleAlternateRowColor() Color {
	ctx.mustAlive()

	bg := Color{R: 48, G: 48, B: 48, A: 255}
	if ctx.currentTheme != nil {
		bg = ctx.currentTheme.Window.BGColor
	}
	target := 255
	if int(bg.R)+int(bg.G)+int(bg.B) >= 384 {
		target = 0
	}
	blend := func(v uint8) uint8 {
		return uint8((int(v)*97 + target*3) / 100)
	}
	return Color{R: blend(bg.R), G: blend(bg.G), B: blend(bg.B), A: 255}
}

// HasTextSelection reports whether Copy is handled by a selected UI text range.
func (ctx *Context) HasTextSelection() bool {
	ctx.mustAlive()

	return ctx.selectedTextItem != nil && ctx.selectedTextItem.SelectedText() != ""
}

// FocusedTextInput returns the visible, enabled text control with keyboard
// focus, or nil. Hosts can use this to reserve editing keys from game hotkeys.
func (ctx *Context) FocusedTextInput() *ItemData {
	ctx.mustAlive()

	it := ctx.FocusedItem()
	if !itemAcceptsTextEditing(it) {
		return nil
	}
	return it
}

// OpenSearch shows and focuses this window's titlebar search box.
func (win *windowData) OpenSearch() {
	ctx := win.ui()

	if win == nil || !win.Open || !win.Searchable {
		return
	}
	if ctx.focusedItem != nil {
		ctx.ClearFocus(ctx.focusedItem)
	}
	win.searchOpen = true
	ctx.activeSearch = win
	win.ensureSearchInput()
	win.markDirty()
}

// ClearFocus removes focus from the provided item if it is currently focused.
func (ctx *Context) ClearFocus(it *ItemData) {
	ctx.mustAlive()
	ctx.requireItem(it)

	if it != nil && ctx.focusedItem == it {
		it.CancelComposition()
		it.Focused = false
		ctx.focusedItem = nil
		invalidateFocus(it)
		ctx.finishKeyboardSlider()
	}
}

// Focus gives keyboard focus to a control and reveals it in scrolling ancestors.
// Tab strips are focused through their owning flow. Text inputs place the caret
// at the end; multiline editor Tab/indent behavior is preserved.
func (ctx *Context) Focus(it *ItemData) {
	ctx.mustAlive()
	ctx.requireItem(it)

	if !ctx.focusControl(it, true) {
		return
	}
	if it.ParentWindow != nil && ctx.activeWindow != it.ParentWindow {
		it.ParentWindow.BringForward()
	}
	if itemHandlesTextEditing(it) {
		it.editor().group = ""
		it.editMove(len([]rune(it.editText())), false)
	} else if itemAcceptsTextEditing(it) {
		it.CursorPos = len([]rune(it.Text))
	}
}

// ScrollbarWidth returns the pixel width used when rendering scrollbars so
// callers can reserve space and avoid overlap.
func (ctx *Context) ScrollbarWidth() float32 {
	ctx.mustAlive()
	return ctx.currentStyle.BorderPad.Slider * 2
}
