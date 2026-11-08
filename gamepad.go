package eui

import (
	"math"

	"github.com/Distortions81/EUI/internal/inputkeys"
	"github.com/hajimehoshi/ebiten/v2"
)

// GamepadInput is one connected controller using Ebitengine's standard layout.
// Buttons contains all held buttons. Stick values must be finite; values outside
// [-1, 1] are clamped. Positive X/Y point right/down. Omitted IDs are disconnected.
type GamepadInput struct {
	ID           ebiten.GamepadID
	Buttons      []ebiten.StandardGamepadButton
	LeftX, LeftY float64
}

type gamepadSample struct {
	buttons    [ebiten.StandardGamepadButtonMax + 1]bool
	x, y       float64
	wasNeutral bool
}

func (pad gamepadSample) neutral() bool {
	if math.Abs(pad.x) > .3 || math.Abs(pad.y) > .3 {
		return false
	}
	for _, button := range navigationButtons {
		if pad.buttons[button] {
			return false
		}
	}
	return true
}

func (pad gamepadSample) active() bool {
	if gamepadDirection(pad, 0) != 0 {
		return true
	}
	for _, button := range navigationButtons {
		if pad.buttons[button] {
			return true
		}
	}
	return false
}

var navigationButtons = [...]ebiten.StandardGamepadButton{
	ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonRightRight,
	ebiten.StandardGamepadButtonLeftTop, ebiten.StandardGamepadButtonLeftBottom,
	ebiten.StandardGamepadButtonLeftLeft, ebiten.StandardGamepadButtonLeftRight,
	ebiten.StandardGamepadButtonFrontTopLeft, ebiten.StandardGamepadButtonFrontTopRight,
	ebiten.StandardGamepadButtonCenterRight,
}

type gamepadNavigation struct {
	enabled, owned, awaitNeutral, handled, lastUsed bool
	id                                              ebiten.GamepadID
	buttons                                         [ebiten.StandardGamepadButtonMax + 1]int
	direction, ticks                                int
	editing                                         *itemData
}

// SetGamepadNavigation opts this context into standard-layout gamepad input.
// Disabled by default. Call on the UI goroutine. Switching cancels adjustment
// capture silently and requires neutral input before a held control can act.
func (ctx *Context) SetGamepadNavigation(enabled bool) {
	ctx.mustAlive()
	if ctx.gamepad.enabled == enabled {
		return
	}
	ctx.cancelGamepadAdjustment()
	ctx.gamepad = gamepadNavigation{enabled: enabled, awaitNeutral: true}
}
func (ctx *Context) GamepadNavigation() bool { ctx.mustAlive(); return ctx.gamepad.enabled }

// ActiveGamepad returns the controller that most recently acquired navigation.
// Ownership is retained until disconnection or input routing/configuration reset.
func (ctx *Context) ActiveGamepad() (ebiten.GamepadID, bool) {
	ctx.mustAlive()
	return ctx.gamepad.id, ctx.gamepad.owned
}

// GamepadInputHandled includes held navigation inputs during repeat delays and
// modal neutral transitions. Host capture, disabled input and unfocused windows
// leave gamepad input to the host. Use routed contexts for shared device streams.
func (ctx *Context) GamepadInputHandled() bool { ctx.mustAlive(); return ctx.gamepad.handled }

// NavigationPrompt describes a host-rendered input hint. Labels use physical
// standard-layout positions, so they do not assume an Xbox/PlayStation/Switch brand.
type NavigationPrompt struct{ Action, Label string }

// NavigationPrompts returns a fresh slice for the last used navigation device.
// Gamepads cannot enter text: editable fields retain ordinary keyboard/IME input.
func (ctx *Context) NavigationPrompts() []NavigationPrompt {
	ctx.mustAlive()
	if ctx.gamepad.enabled && ctx.gamepad.lastUsed {
		move := "Move focus"
		confirm := "Confirm"
		if ctx.gamepad.editing != nil {
			move = "Adjust"
			confirm = "Finish adjustment"
		}
		return []NavigationPrompt{{move, "D-pad / left stick"}, {confirm, "Bottom face button"}, {"Back", "Right face button"}, {"Previous / next field", "Shoulder buttons"}, {"Next window", "Menu button"}}
	}
	return []NavigationPrompt{{"Previous / next field", "Shift+Tab / Tab"}, {"Confirm", "Enter / Space"}, {"Back", "Escape"}, {"Next window", "F6"}}
}

func (s *inputState) sampleGamepads(frames []GamepadInput) {
	if len(frames) == 0 && s.gamepads == nil {
		return
	}
	if s.gamepads == nil {
		s.gamepads = make(map[ebiten.GamepadID]gamepadSample)
		s.nextGamepads = make(map[ebiten.GamepadID]gamepadSample)
	}
	clear(s.nextGamepads)
	for _, frame := range frames {
		if frame.ID < 0 {
			continue
		}
		if _, exists := s.nextGamepads[frame.ID]; exists {
			continue
		}
		previous, known := s.gamepads[frame.ID]
		pad := gamepadSample{x: max(-1, min(frame.LeftX, 1)), y: max(-1, min(frame.LeftY, 1)), wasNeutral: known && previous.neutral()}
		for _, button := range frame.Buttons {
			if button >= 0 && button <= ebiten.StandardGamepadButtonMax {
				pad.buttons[button] = true
			}
		}
		s.nextGamepads[frame.ID] = pad
	}
	s.gamepads, s.nextGamepads = s.nextGamepads, s.gamepads
}
func (ctx *Context) nativeGamepadInputs() []GamepadInput {
	ctx.nativeGamepadFrames = ctx.nativeGamepadFrames[:0]
	if !ctx.gamepad.enabled || !ctx.inputEnabled {
		return nil
	}
	ctx.nativeGamepadIDs = ebiten.AppendGamepadIDs(ctx.nativeGamepadIDs[:0])
	for _, id := range ctx.nativeGamepadIDs {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		// Reuse per-controller button buffers without retaining caller-owned frames.
		i := len(ctx.nativeGamepadFrames)
		if i >= len(ctx.nativeGamepadButtons) {
			ctx.nativeGamepadButtons = append(ctx.nativeGamepadButtons, nil)
		}
		buttons := ctx.nativeGamepadButtons[i][:0]
		for _, button := range navigationButtons {
			if ebiten.IsStandardGamepadButtonPressed(id, button) {
				buttons = append(buttons, button)
			}
		}
		ctx.nativeGamepadButtons[i] = buttons
		ctx.nativeGamepadFrames = append(ctx.nativeGamepadFrames, GamepadInput{ID: id, Buttons: buttons, LeftX: ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal), LeftY: ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)})
	}
	return ctx.nativeGamepadFrames
}

// Dominant-axis navigation uses .45 to engage and .30 to release. D-pad wins
// over the stick, opposed buttons cancel, and exact diagonals prefer horizontal.
func gamepadDirection(pad gamepadSample, previous int) int {
	x, y := 0, 0
	dpad := false
	for _, button := range []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonLeftLeft, ebiten.StandardGamepadButtonLeftRight, ebiten.StandardGamepadButtonLeftTop, ebiten.StandardGamepadButtonLeftBottom} {
		dpad = dpad || pad.buttons[button]
	}
	if dpad {
		if pad.buttons[ebiten.StandardGamepadButtonLeftLeft] {
			x--
		}
		if pad.buttons[ebiten.StandardGamepadButtonLeftRight] {
			x++
		}
		if pad.buttons[ebiten.StandardGamepadButtonLeftTop] {
			y--
		}
		if pad.buttons[ebiten.StandardGamepadButtonLeftBottom] {
			y++
		}
	} else {
		ax, ay := math.Abs(pad.x), math.Abs(pad.y)
		if max(ax, ay) < .45 {
			switch previous {
			case 1:
				if pad.x < -.30 && ax >= ay {
					return previous
				}
			case 2:
				if pad.x > .30 && ax >= ay {
					return previous
				}
			case 3:
				if pad.y < -.30 && ay > ax {
					return previous
				}
			case 4:
				if pad.y > .30 && ay > ax {
					return previous
				}
			}
			return 0
		}
		if ax >= ay {
			if pad.x < 0 {
				x = -1
			} else {
				x = 1
			}
		} else {
			if pad.y < 0 {
				y = -1
			} else {
				y = 1
			}
		}
	}
	if x < 0 {
		return 1
	}
	if x > 0 {
		return 2
	}
	if y < 0 {
		return 3
	}
	if y > 0 {
		return 4
	}
	return 0
}
func directionKey(direction int) ebiten.Key {
	return [...]ebiten.Key{ebiten.Key(0), ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown}[direction]
}
func gamepadRepeat(ticks int) bool { return ticks == 1 || ticks >= 25 && (ticks-25)%6 == 0 }

func (ctx *Context) cancelGamepadAdjustment() {
	if ctx.gamepad.editing != nil && ctx.keyboardSlider == ctx.gamepad.editing {
		ctx.keyboardSlider = nil
	}
	ctx.gamepad.editing = nil
}
func (ctx *Context) updateGamepadState() {
	g := &ctx.gamepad
	g.handled = false
	if !g.enabled {
		return
	}
	if g.owned {
		if _, connected := ctx.sampledInput.gamepads[g.id]; !connected {
			ctx.cancelGamepadAdjustment()
			*g = gamepadNavigation{enabled: true}
		}
	}
	if !g.owned {
		for id, pad := range ctx.sampledInput.gamepads {
			if pad.active() && (!g.owned || id < g.id) {
				g.id = id
				g.owned = true
			}
		}
		if !g.owned {
			return
		}
		g.awaitNeutral = !ctx.sampledInput.gamepads[g.id].wasNeutral
	}
	pad := ctx.sampledInput.gamepads[g.id]
	for button, down := range pad.buttons {
		if down {
			g.buttons[button] = nextInputTick(g.buttons[button])
		} else {
			g.buttons[button] = 0
		}
	}
	direction := gamepadDirection(pad, g.direction)
	if direction == 0 {
		g.ticks = 0
	} else if direction != g.direction {
		g.ticks = 1
	} else {
		g.ticks = nextInputTick(g.ticks)
	}
	g.direction = direction
	if g.awaitNeutral && pad.neutral() {
		g.awaitNeutral = false
		g.direction, g.ticks = 0, 0
		clear(g.buttons[:])
	}
	// Physical keyboard/pointer input has precedence over simultaneous pad input.
	physical := len(ctx.sampledInput.text) > 0 || len(ctx.sampledInput.textInput) > 0 || ctx.sampledInput.primaryDown || ctx.sampledInput.wheelX != 0 || ctx.sampledInput.wheelY != 0
	for _, ticks := range ctx.sampledInput.keys {
		physical = physical || ticks > 0
	}
	for _, ticks := range ctx.sampledInput.buttons {
		physical = physical || ticks > 0
	}
	if physical || ctx.keyboardInputCaptured || ctx.sampledInput.unfocused || !ctx.inputEnabled {
		ctx.cancelGamepadAdjustment()
		g.awaitNeutral = !pad.neutral()
	}
	if physical || ctx.sampledInput.x != int(ctx.mposOld.X) || ctx.sampledInput.y != int(ctx.mposOld.Y) {
		g.lastUsed = false
	}
	g.handled = !pad.neutral() && !ctx.keyboardInputCaptured && !ctx.sampledInput.unfocused && ctx.inputEnabled && (ctx.modalWindow() != nil || ctx.activeWindow != nil && ctx.activeWindow.Open)
}
func (ctx *Context) gamepadNeutral() bool {
	if !ctx.gamepad.enabled || !ctx.gamepad.owned {
		return true
	}
	pad, ok := ctx.sampledInput.gamepads[ctx.gamepad.id]
	return !ok || pad.neutral()
}
func (ctx *Context) gamepadSliderHeld() bool {
	return ctx.gamepad.enabled && ctx.gamepad.editing == ctx.keyboardSlider && ctx.gamepad.editing != nil && ctx.gamepad.direction != 0 && !ctx.gamepad.awaitNeutral
}

func (ctx *Context) pollGamepad() {
	g := &ctx.gamepad
	epoch := ctx.modalEpoch
	if !g.enabled || !g.owned || g.awaitNeutral || ctx.keyboardInputCaptured || ctx.sampledInput.unfocused || !ctx.inputEnabled {
		return
	}
	// A physical input handoff requires fresh pad intent even when it is neutral.
	for _, ticks := range ctx.sampledInput.keys {
		if ticks > 0 {
			return
		}
	}
	if ctx.sampledInput.primaryDown || len(ctx.sampledInput.text) > 0 || len(ctx.sampledInput.textInput) > 0 {
		return
	}
	pressed := func(button ebiten.StandardGamepadButton) bool { return g.buttons[button] == 1 }
	back := pressed(ebiten.StandardGamepadButtonRightRight)
	confirm := pressed(ebiten.StandardGamepadButtonRightBottom)
	prev, next := pressed(ebiten.StandardGamepadButtonFrontTopLeft), pressed(ebiten.StandardGamepadButtonFrontTopRight)
	window := pressed(ebiten.StandardGamepadButtonCenterRight)
	move := gamepadRepeat(g.ticks)
	if !back && !confirm && !prev && !next && !window && !move {
		return
	}
	g.lastUsed = true
	if target := ctx.textInputTarget(); target != nil && target.IsComposing() {
		if back {
			target.CancelComposition()
		}
		g.awaitNeutral = true
		return
	}
	if g.editing != nil && (ctx.FocusedItem() != g.editing || !focusAvailable(g.editing)) {
		ctx.cancelGamepadAdjustment()
	}
	if back {
		switch {
		case ctx.activeContextMenu() != nil:
			ctx.CloseContextMenus()
		case ctx.hasOpenDropdown():
			ctx.closeAllDropdowns()
		case ctx.activeSearch != nil:
			ctx.activeSearch.closeSearch()
		case g.editing != nil:
			ctx.finishKeyboardSlider()
			g.editing = nil
		case ctx.modalWindow() != nil:
			ctx.cancelModal()
		default:
			if ctx.GamepadCancelHandler != nil {
				ctx.GamepadCancelHandler()
			}
		}
		return
	}
	if ctx.activeSearch != nil {
		return
	}
	if prev || next || window {
		ctx.finishKeyboardSlider()
		g.editing = nil
		if ctx.modalFrameChanged(epoch) {
			return
		}
		if window {
			ctx.focusNextWindow(false)
		} else {
			ctx.tabFocus(prev)
		}
		return
	}
	target := ctx.FocusedItem()
	if menu := ctx.activeContextMenu(); menu != nil {
		target = menu
	}
	if confirm {
		if g.editing != nil {
			ctx.finishKeyboardSlider()
			g.editing = nil
			return
		}
		if target == nil {
			ctx.tabFocus(false)
			return
		}
		if target.ItemType == ITEM_SLIDER || target.ItemType == ITEM_COLORWHEEL || target.ItemType == ITEM_FLOW {
			g.editing = target
			return
		}
		if !ctx.controlKey(target, ebiten.KeyEnter, false, inputkeys.Modifiers{}) && itemAcceptsTextEditing(target) {
			if win := target.ParentWindow; win != nil {
				ctx.activateControl(win.DefaultButton)
			}
		}
		return
	}
	if move {
		key := directionKey(g.direction)
		if target != nil && (target.contextMenu || target.ItemType == ITEM_DROPDOWN && target.Open || g.editing == target) {
			ctx.controlKey(target, key, false, inputkeys.Modifiers{})
			return
		}
		ctx.focusDirection(g.direction)
	}
}

// Spatial navigation remains in the active window. Shoulder traversal and menu
// window cycling provide deterministic ways out of rows or adjustment controls.
func (ctx *Context) focusDirection(direction int) bool {
	win := ctx.activeWindow
	if modal := ctx.modalWindow(); modal != nil {
		win = modal
	}
	if win == nil || !win.Open || !ctx.modalAllows(win) {
		return ctx.focusNextWindow(false)
	}
	win.resizeFlows()
	var controls []*itemData
	collectFocusControls(win.Contents, &controls)
	current := ctx.FocusedItem()
	if current == nil || current.ParentWindow != win {
		return ctx.focusNext(win, false)
	}
	geometry := ctx.navigationRects(win)
	origin, ok := geometry[current]
	if !ok {
		return ctx.focusNext(win, false)
	}
	ox, oy := (origin.X0+origin.X1)/2, (origin.Y0+origin.Y1)/2
	best := math.Inf(1)
	var selected *itemData
	for _, candidate := range controls {
		if candidate == current {
			continue
		}
		bounds, ok := geometry[candidate]
		if !ok {
			continue
		}
		dx, dy := float64((bounds.X0+bounds.X1)/2-ox), float64((bounds.Y0+bounds.Y1)/2-oy)
		forward, side := dx, math.Abs(dy)
		switch direction {
		case 1:
			forward = -dx
		case 3:
			forward, side = -dy, math.Abs(dx)
		case 4:
			forward, side = dy, math.Abs(dx)
		}
		if forward <= .5 {
			continue
		}
		score := forward + side*3 + side*side/forward
		if score < best {
			best, selected = score, candidate
		}
	}
	if selected == nil {
		return false
	}
	return ctx.focusControl(selected, true)
}

// Measure each node once; spatial traversal must not re-walk or scroll the
// entire tree for every candidate, including candidates never drawn on screen.
func (ctx *Context) navigationRects(win *windowData) map[*itemData]rect {
	result := make(map[*itemData]rect)
	var visit func(*itemData, point)
	visit = func(item *itemData, origin point) {
		if item == nil || item.Invisible || item.Disabled {
			return
		}
		size := item.GetSize()
		bounds := rect{X1: size.X, Y1: size.Y}
		if len(item.Tabs) > 0 && item.selectableTab(item.ActiveTab) {
			layout, _ := ctx.layoutTabs(item, item.themeStyle())
			bounds = layout[item.ActiveTab]
		}
		if item.ItemType != ITEM_FLOW && item.PinTo != PIN_TOP_LEFT {
			origin = pointSub(item.getPosition(win), win.Scroll)
		}
		result[item] = rectAdd(bounds, origin)
		for child, position := range item.layoutChildren(size) {
			visit(child, pointAdd(origin, pointSub(position, item.Scroll)))
		}
	}
	for _, root := range win.Contents {
		visit(root, pointSub(root.getPosition(win), win.Scroll))
	}
	return result
}
