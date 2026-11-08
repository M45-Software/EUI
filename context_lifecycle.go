package eui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"weak"
)

var ErrContextDisposed = errors.New("eui: context is disposed")
var ErrContextReentry = errors.New("eui: recursive Update/Draw on the same context")

func (ctx *Context) mustAlive() {
	if ctx == nil || ctx.disposed {
		panic(ErrContextDisposed)
	}
	if ctx.self != ctx {
		panic("eui: context must be created by NewContext and must not be copied")
	}
}
func (item *itemData) ui() *Context {
	if item == nil {
		return nil
	}
	if item.ctx == nil {
		panic("eui: item has no context; use Context.NewItem or attach it before use")
	}
	item.ctx.mustAlive()
	return item.ctx
}
func (win *windowData) ui() *Context {
	if win == nil {
		return nil
	}
	if win.ctx == nil {
		panic("eui: window has no context; use Context.NewWindow")
	}
	win.ctx.mustAlive()
	return win.ctx
}
func (item *itemData) Context() *Context {
	if item == nil {
		return nil
	}
	return item.ctx
}
func (win *windowData) Context() *Context {
	if win == nil {
		return nil
	}
	return win.ctx
}
func (ctx *Context) IsDisposed() bool { return ctx == nil || ctx.disposed }

// NewItem creates an owned custom control from configuration. Runtime controls
// cannot be copied into a new item; use the widget constructors for those.
func (ctx *Context) NewItem(config ItemData) *ItemData {
	ctx.mustAlive()
	if config.ctx != nil {
		panic("eui: NewItem requires unowned configuration")
	}
	ctx.bindItems([]*itemData{&config})
	return &config
}

// Validate the whole batch before assigning any owners. Ownership remains with
// the context after detaching; moving live controls between contexts is invalid.
func (ctx *Context) bindItems(items []*itemData) {
	ctx.mustAlive()
	if ctx.ownershipVisit == nil {
		ctx.ownershipVisit = map[*itemData]uint8{}
	}
	visited := ctx.ownershipVisit
	defer clear(visited)
	var visit func(*itemData)
	visit = func(item *itemData) {
		if item == nil {
			return
		}
		if item.ctx != nil && item.ctx != ctx {
			panic("eui: control belongs to another context")
		}
		if item.Handler != nil && item.Handler.owner != nil && item.Handler.owner != ctx {
			panic("eui: handler belongs to another context")
		}
		if visited[item] == 1 {
			panic("eui: cyclic control tree")
		}
		if visited[item] == 2 {
			return
		}
		visited[item] = 1
		for _, child := range item.Contents {
			visit(child)
		}
		for _, page := range item.Tabs {
			visit(page)
		}
		visited[item] = 2
	}
	for _, item := range items {
		visit(item)
	}
	for item := range visited {
		item.ctx = ctx
		ctx.registerItem(item)
	}
}
func (ctx *Context) bindTheme() {
	th := ctx.currentTheme
	if th == nil {
		return
	}
	th.Window.ctx = ctx
	for _, item := range []*itemData{&th.Button, &th.Text, &th.Checkbox, &th.Radio, &th.Input, &th.Slider, &th.Dropdown, &th.Progress, &th.Tab} {
		item.ctx = ctx
		ctx.registerItem(item)
	}
}
func (ctx *Context) baseThemeCopy() *Theme {
	th := *baseTheme
	ctx.currentTheme = &th
	ctx.bindTheme()
	return &th
}

// ParseColor resolves a theme color name in this context, or any literal color
// format accepted by Color.UnmarshalJSON. It never consults another context.
func (ctx *Context) ParseColor(data []byte) (Color, error) {
	ctx.mustAlive()
	var value Color
	// Resolution of quoted names shares the theme resolver, including literals.
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		if color, ok := ctx.namedColors[strings.ToLower(strings.TrimSpace(name))]; ok {
			return color, nil
		}
	}
	if err := value.UnmarshalJSON(data); err != nil {
		return Color{}, fmt.Errorf("parse color: %w", err)
	}
	return value, nil
}

// Dispose releases the context's windows, sessions and cached images. It is
// idempotent and does not invoke application callbacks or dispose app images.
func (ctx *Context) Dispose() {
	if ctx == nil || ctx.disposed {
		return
	}
	if ctx.updateDepth > 0 || ctx.drawDepth > 0 || ctx.nativeTextInput.pumping {
		ctx.disposePending = true
		return
	}
	ctx.disposeNow()
}
func (ctx *Context) disposeNow() {

	ctx.mustAlive()
	ctx.cancelNativeTextInput(nil)
	// Weak registries let detached controls be collected while still retiring
	// queues and caches of any retained handles when their owner is disposed.
	for reference := range ctx.ownedItems {
		item := reference.Value()
		if item == nil {
			continue
		}
		if item.Handler != nil {
			item.Handler.DisableQueue()
		}
		item.Focused, item.Hovered, item.Open = false, false, false
		item.selecting = false
		if item.textEdit != nil {
			item.textEdit = nil
		}
		item.releaseOwnImages()
	}
	for reference := range ctx.ownedWindows {
		win := reference.Value()
		if win == nil {
			continue
		}
		win.Open = false
		win.releaseOwnImages()
	}
	ctx.ownershipVisit = nil
	ctx.ownedItems = nil
	ctx.ownedWindows = nil
	ctx.uiPrimitives.clear()
	ctx.plainUIText.clear()
	for _, image := range ctx.shadowMasks {
		image.Deallocate()
	}
	ctx.uiRenderTargets.Clear()
	if ctx.whiteImage != nil {
		ctx.whiteImage.Deallocate()
		ctx.whiteImage = nil
	}
	ctx.modals = nil
	ctx.windows = nil
	ctx.contextMenus = nil
	ctx.shadowMasks = nil
	ctx.activeWindow = nil
	ctx.focusedItem = nil
	ctx.hoveredItem = nil
	ctx.activeItem = nil
	ctx.selectedTextItem = nil
	ctx.composingItem = nil
	ctx.activeSearch = nil
	ctx.dragWin = nil
	ctx.dragFlow = nil
	ctx.downWin = nil
	ctx.inputWindowOrder = nil
	ctx.dropdownReuse = nil
	ctx.inputSource = nil
	ctx.faceCache = nil
	ctx.boldFaceCache = nil
	ctx.configuredClipboard = nil
	ctx.sampledInput = inputState{}
	ctx.nativeKeys = nil
	ctx.nativeButtons = nil
	ctx.nativeTouchIDs = nil
	ctx.nativeTouches = nil
	ctx.nativeChars = nil
	ctx.gamepad = gamepadNavigation{}
	ctx.nativeGamepadIDs = nil
	ctx.nativeGamepadFrames = nil
	ctx.nativeGamepadButtons = nil
	ctx.GamepadCancelHandler = nil
	ctx.tileDividers = nil
	ctx.mplusFaceSource = nil
	ctx.mplusBoldFaceSource = nil
	ctx.currentTheme = nil
	ctx.currentStyle = nil
	ctx.namedColors = nil
	ctx.WindowStateChanged = nil
	ctx.CapsLockToggleHandler = nil
	ctx.RepaintObserver = nil
	ctx.UnmanagedImageCreated = nil
	ctx.disposed = true
}

func (ctx *Context) requireItem(item *itemData) {
	if item != nil && item.ctx != ctx {
		panic("eui: control belongs to another context")
	}
}
func (ctx *Context) requireWindow(win *windowData) {
	if win != nil && win.ctx != ctx {
		panic("eui: window belongs to another context")
	}
}

func (ctx *Context) registerItem(item *itemData) *itemData {
	if ctx.ownedItems == nil {
		ctx.ownedItems = map[weak.Pointer[itemData]]struct{}{}
	}
	ctx.ownedItems[weak.Make(item)] = struct{}{}
	if len(ctx.ownedItems) >= max(256, ctx.nextItemSweep) {
		for key := range ctx.ownedItems {
			if key.Value() == nil {
				delete(ctx.ownedItems, key)
			}
		}
		ctx.nextItemSweep = max(256, len(ctx.ownedItems)*2)
	}
	if item.Handler != nil {
		if item.Handler.owner != nil && item.Handler.owner != ctx {
			panic("eui: handler belongs to another context")
		}
		item.Handler.owner = ctx
	}
	return item
}
func (ctx *Context) registerWindow(win *windowData) *windowData {
	if ctx.ownedWindows == nil {
		ctx.ownedWindows = map[weak.Pointer[windowData]]struct{}{}
	}
	ctx.ownedWindows[weak.Make(win)] = struct{}{}
	if len(ctx.ownedWindows) >= max(64, ctx.nextWindowSweep) {
		for key := range ctx.ownedWindows {
			if key.Value() == nil {
				delete(ctx.ownedWindows, key)
			}
		}
		ctx.nextWindowSweep = max(64, len(ctx.ownedWindows)*2)
	}
	return win
}

// SetInputEnabled routes input to this instance. Disabled contexts still update
// animations and draw, retaining focus and values. Disabling cancels captures,
// menus and native preedit without activation/release events. Use one enabled
// context for a shared native device stream, or supply routed InputSources.
func (ctx *Context) SetInputEnabled(enabled bool) {
	ctx.mustAlive()
	if ctx.inputEnabled == enabled {
		return
	}
	ctx.SetInputSource(ctx.inputSource)
	ctx.inputEnabled = enabled
}
func (ctx *Context) InputEnabled() bool { ctx.mustAlive(); return ctx.inputEnabled }
