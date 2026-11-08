package eui

import (
	"fmt"
	"math"
	"runtime"
	"time"
	"unicode/utf8"

	"github.com/M45-Software/EUI/internal/inputkeys"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/time/rate"
)

// TouchPoint is one active contact in screen pixels. IDs remain stable until
// that contact lifts; the lowest active ID determines the primary touch.
type TouchPoint struct {
	ID   ebiten.TouchID
	X, Y int
}

// InputFrame describes one Update tick. Keys, MouseButtons, and Touches contain
// all currently held inputs, not just changes. Text and wheel deltas belong only
// to this tick. Time must be nonzero and nondecreasing within an input stream.
// Mac selects Command/Option editing rules; native input sets it from the OS.
// EUI reads the slices during Update and does not retain or mutate them.
type InputFrame struct {
	Time               time.Time
	Keys               []ebiten.Key
	MouseButtons       []ebiten.MouseButton
	PointerX, PointerY int
	Touches            []TouchPoint
	// Gamepads lists connected standard-layout controllers and all held inputs.
	Gamepads       []GamepadInput
	WheelX, WheelY float64
	Text           string
	Mac            bool
	// TextInput is an ordered packet of composition notifications for this tick.
	// It takes precedence over Text; active preedit also suppresses ordinary keys.
	TextInput []TextInputEvent
	// Unfocused cancels composition and suspends keyboard input while retaining
	// control focus for the host's return.
	Unfocused bool
}

// InputSource is sampled exactly once at the start of each Update. A source
// can replay recorded frames or adapt another host's input system. Return a
// fresh complete state each tick, and call Update once per tick as usual.
type InputSource func() InputFrame

// SetInputSource selects a source; nil restores native Ebitengine input. It
// returns the previous source. Call between Update calls on the UI goroutine.
// Switching starts a new stream: held-input history, gestures, menus, click
// flashes, and timing baselines reset without emitting activation/release events.
// Focus, text selection, values, and editor history remain intact.
func (ctx *Context) SetInputSource(source InputSource) InputSource {
	ctx.mustAlive()

	previous := ctx.inputSource
	if ctx.composingItem != nil {
		ctx.composingItem.CancelComposition()
	}
	ctx.cancelNativeTextInput(nil)
	ctx.closeAllDropdowns()
	ctx.CloseContextMenus()
	for _, win := range ctx.windows {
		resetInputTree(win.Contents)
		resetInputTree([]*itemData{win.searchInput})
		win.lastRefresh, win.repaintRequested = time.Time{}, time.Time{}
		win.refreshPending = false
		win.Dirty = true
	}
	for _, item := range []*itemData{ctx.activeItem, ctx.dragFlow, ctx.keyboardSlider} {
		if item != nil {
			item.dragStartInit = false
			item.selecting = false
		}
	}
	ctx.activeItem, ctx.dragFlow, ctx.dragWin, ctx.downWin = nil, nil, nil, nil
	ctx.dragPart = PART_NONE
	ctx.activeTileDivider, ctx.hoveredTileDivider = -1, -1
	ctx.keyboardSlider = nil
	ctx.pointerPressHandled = false
	ctx.ShiftPressed, ctx.CtrlPressed = false, false
	ctx.updateNow = time.Time{}
	ctx.mposOld = Point{}
	ctx.inputSource = source
	ctx.sampledInput = inputState{ctx: ctx}
	ctx.gamepad = gamepadNavigation{enabled: ctx.gamepad.enabled, awaitNeutral: true}
	ctx.wheelLimiter = rate.NewLimiter(rate.Every(125*time.Millisecond), 1)
	return previous
}

func resetInputTree(items []*itemData) {
	for _, item := range items {
		if item == nil {
			continue
		}
		item.clearComposition()
		item.Clicked = time.Time{}
		item.dragStartInit, item.selecting = false, false
		if item.textEdit != nil {
			item.textEdit.lastClick, item.textEdit.caretReset = time.Time{}, time.Time{}
			item.textEdit.clickCount = 0
			item.textEdit.scrollDrag = PART_NONE
			item.textEdit.group = ""
		}
		resetInputTree(item.Contents)
		resetInputTree(item.Tabs)
	}
}

type inputState struct {
	gamepads, nextGamepads map[ebiten.GamepadID]gamepadSample
	ctx                    *Context

	textInput                              []TextInputEvent
	unfocused                              bool
	touchIDs                               [2]ebiten.TouchID
	ready                                  bool
	now                                    time.Time
	keys                                   [ebiten.KeyMax + 1]int
	buttons                                [ebiten.MouseButtonMax + 1]int
	touches, nextTouches                   map[ebiten.TouchID]int
	x, y                                   int
	wheelX, wheelY                         float64
	text                                   string
	mac                                    bool
	primaryDown, primaryJust, primaryEnded bool
	primaryTicks                           int
	primaryTouch                           bool
	primaryID                              ebiten.TouchID
	touchScrolling                         bool
	touchX, touchY                         float64
}

func (ctx *Context) nativeInputFrame() InputFrame {
	ctx.nativeKeys = inpututil.AppendPressedKeys(ctx.nativeKeys[:0])
	ctx.nativeButtons = ctx.nativeButtons[:0]
	for button := ebiten.MouseButton0; button <= ebiten.MouseButtonMax; button++ {
		if ebiten.IsMouseButtonPressed(button) {
			ctx.nativeButtons = append(ctx.nativeButtons, button)
		}
	}
	ctx.nativeTouchIDs = ebiten.AppendTouchIDs(ctx.nativeTouchIDs[:0])
	ctx.nativeTouches = ctx.nativeTouches[:0]
	for _, id := range ctx.nativeTouchIDs {
		x, y := ebiten.TouchPosition(id)
		ctx.nativeTouches = append(ctx.nativeTouches, TouchPoint{ID: id, X: x, Y: y})
	}
	x, y := ebiten.CursorPosition()
	wx, wy := ebiten.Wheel()
	ctx.nativeChars = ebiten.AppendInputChars(ctx.nativeChars[:0])
	return InputFrame{Time: time.Now(), Keys: ctx.nativeKeys, MouseButtons: ctx.nativeButtons, PointerX: x, PointerY: y, Touches: ctx.nativeTouches, Gamepads: ctx.nativeGamepadInputs(), WheelX: wx, WheelY: wy, Text: string(ctx.nativeChars), Mac: runtime.GOOS == "darwin", Unfocused: !ebiten.IsFocused()}
}

func (s *inputState) sample(frame InputFrame) error {
	ctx := s.ctx

	if frame.Time.IsZero() {
		return fmt.Errorf("eui: input frame time must be nonzero")
	}
	if math.IsNaN(frame.WheelX) || math.IsNaN(frame.WheelY) || math.IsInf(frame.WheelX, 0) || math.IsInf(frame.WheelY, 0) {
		return fmt.Errorf("eui: input wheel deltas must be finite")
	}
	if s.ready && frame.Time.Before(s.now) {
		return fmt.Errorf("eui: input frame time moved backwards")
	}
	for _, pad := range frame.Gamepads {
		if math.IsNaN(pad.LeftX) || math.IsNaN(pad.LeftY) || math.IsInf(pad.LeftX, 0) || math.IsInf(pad.LeftY, 0) {
			return fmt.Errorf("eui: gamepad axes must be finite")
		}
	}
	for _, event := range frame.TextInput {
		if event.Kind > TextInputEnd || !utf8.ValidString(event.Text) {
			return fmt.Errorf("eui: invalid text input event")
		}
		if r := event.Replacement; r != nil && (r.Start < 0 || r.End < r.Start || r.Caret < -1) {
			return fmt.Errorf("eui: invalid text replacement")
		}
	}

	s.sampleGamepads(frame.Gamepads)
	var keys [ebiten.KeyMax + 1]bool
	for _, key := range frame.Keys {
		if key >= 0 && key <= ebiten.KeyMax {
			keys[key] = true
		}
	}
	for _, group := range [][3]ebiten.Key{{ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight}, {ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight}, {ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight}, {ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight}} {
		keys[group[0]] = keys[group[0]] || keys[group[1]] || keys[group[2]]
	}
	for key, held := range keys {
		if held {
			s.keys[key] = nextInputTick(s.keys[key])
		} else {
			s.keys[key] = 0
		}
	}
	var buttons [ebiten.MouseButtonMax + 1]bool
	for _, button := range frame.MouseButtons {
		if button >= 0 && button <= ebiten.MouseButtonMax {
			buttons[button] = true
		}
	}
	for button, held := range buttons {
		if held {
			s.buttons[button] = nextInputTick(s.buttons[button])
		} else {
			s.buttons[button] = 0
		}
	}
	if s.touches == nil {
		s.touches = make(map[ebiten.TouchID]int)
		s.nextTouches = make(map[ebiten.TouchID]int)
	}
	clear(s.nextTouches)
	var first, second TouchPoint
	count := 0
	for _, touch := range frame.Touches {
		if _, exists := s.nextTouches[touch.ID]; exists {
			continue
		}
		s.nextTouches[touch.ID] = nextInputTick(s.touches[touch.ID])
		if count == 0 || touch.ID < first.ID {
			second, first = first, touch
		} else if count == 1 || touch.ID < second.ID {
			second = touch
		}
		count++
	}
	s.touches, s.nextTouches = s.nextTouches, s.touches
	oldDown, oldTouch, oldID := s.primaryDown, s.primaryTouch, s.primaryID
	s.x, s.y = frame.PointerX, frame.PointerY
	s.primaryDown, s.primaryJust, s.primaryTicks = false, false, 0
	s.primaryTouch, s.primaryID = false, 0
	switch count {
	case 0:
		s.primaryTicks = s.buttons[ebiten.MouseButton0]
		s.primaryDown, s.primaryJust = s.primaryTicks > 0, s.primaryTicks == 1
	case 1:
		s.x, s.y = first.X, first.Y
		s.primaryTouch, s.primaryID = true, first.ID
		s.primaryTicks = s.touches[first.ID]
		s.primaryDown, s.primaryJust = true, s.primaryTicks == 1
	default:
		s.x, s.y = first.X, first.Y
	}
	s.primaryEnded = oldDown && (!s.primaryDown || oldTouch != s.primaryTouch || oldTouch && oldID != s.primaryID)
	s.wheelX, s.wheelY = frame.WheelX, frame.WheelY
	if count >= 2 {
		x, y := (float64(first.X)+float64(second.X))/2, (float64(first.Y)+float64(second.Y))/2
		s.wheelX, s.wheelY = 0, 0
		ids := [2]ebiten.TouchID{first.ID, second.ID}
		if s.touchScrolling && s.touchIDs == ids {
			s.wheelX, s.wheelY = (x-s.touchX)*touchScrollScale, (y-s.touchY)*touchScrollScale
		}
		s.touchIDs = ids
		s.touchX, s.touchY = x, y
	}
	s.touchScrolling = count >= 2
	if ctx != nil && ctx.isWasm && count < 2 && (s.wheelX != 0 || s.wheelY != 0) {
		if ctx.wheelLimiter.AllowN(frame.Time, 1) {
			s.wheelX, s.wheelY = wheelDirection(s.wheelX), wheelDirection(s.wheelY)
		} else {
			s.wheelX, s.wheelY = 0, 0
		}
	}
	s.textInput = append(s.textInput[:0], frame.TextInput...)
	for i := range s.textInput {
		if r := s.textInput[i].Replacement; r != nil {
			copy := *r
			s.textInput[i].Replacement = &copy
		}
	}
	s.unfocused = frame.Unfocused

	s.now, s.text, s.mac, s.ready = frame.Time, frame.Text, frame.Mac, true
	return nil
}

func nextInputTick(previous int) int {
	if previous < int(^uint(0)>>1) {
		return previous + 1
	}
	return previous
}
func wheelDirection(value float64) float64 {
	if value > 0 {
		return 3
	}
	if value < 0 {
		return -3
	}
	return 0
}

func (ctx *Context) sampleInput() error {
	frame := InputFrame{}
	if ctx.inputSource != nil {
		frame = ctx.inputSource()
	} else {
		frame = ctx.nativeInputFrame()
	}
	if !ctx.inputEnabled {
		frame.Keys = nil
		frame.MouseButtons = nil
		frame.Touches = nil
		frame.Gamepads = nil
		frame.Text = ""
		frame.TextInput = nil
		frame.WheelX, frame.WheelY = 0, 0
		frame.PointerX, frame.PointerY = -1<<30, -1<<30
		frame.Unfocused = true
	}
	return ctx.sampledInput.sample(frame)
}

func (ctx *Context) uiNow() time.Time {
	if ctx.inputSource != nil {
		if ctx.sampledInput.ready {
			return ctx.sampledInput.now
		}
		return time.Unix(0, 0)
	}
	return time.Now()
}
func (ctx *Context) keyPressed(key ebiten.Key) bool {
	return key >= 0 && key <= ebiten.KeyMax && ctx.sampledInput.keys[key] > 0
}
func (ctx *Context) keyJustPressed(key ebiten.Key) bool {
	return key >= 0 && key <= ebiten.KeyMax && ctx.sampledInput.keys[key] == 1
}
func (ctx *Context) keyPressDuration(key ebiten.Key) int {
	if key < 0 || key > ebiten.KeyMax {
		return 0
	}
	return ctx.sampledInput.keys[key]
}
func (ctx *Context) mousePressed(button ebiten.MouseButton) bool {
	return button >= 0 && button <= ebiten.MouseButtonMax && ctx.sampledInput.buttons[button] > 0
}
func (ctx *Context) mouseJustPressed(button ebiten.MouseButton) bool {
	return button >= 0 && button <= ebiten.MouseButtonMax && ctx.sampledInput.buttons[button] == 1
}
func (ctx *Context) mousePressDuration(button ebiten.MouseButton) int {
	if button < 0 || button > ebiten.MouseButtonMax {
		return 0
	}
	return ctx.sampledInput.buttons[button]
}
func (ctx *Context) inputModifiers() inputkeys.Modifiers {
	return inputkeys.Modifiers{Control: ctx.keyPressed(ebiten.KeyControl), Alt: ctx.keyPressed(ebiten.KeyAlt), Meta: ctx.keyPressed(ebiten.KeyMeta), Mac: ctx.sampledInput.mac}
}
