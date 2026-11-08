package eui

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/time/rate"
)

func TestInputSnapshotNormalizesHoldsAliasesAndDoesNotRetainSlices(t *testing.T) {
	var state inputState
	keys := []ebiten.Key{ebiten.KeyShiftLeft, ebiten.KeyControlRight, ebiten.KeyAltLeft, ebiten.KeyMetaRight, ebiten.KeyA, ebiten.KeyA, -1, ebiten.KeyMax + 1}
	buttons := []ebiten.MouseButton{ebiten.MouseButton0, ebiten.MouseButton0, -1, ebiten.MouseButtonMax + 1}
	touches := []TouchPoint{{ID: 5, X: 20, Y: 30}, {ID: 5, X: 100, Y: 100}}
	frame := InputFrame{Time: time.Unix(1, 0), Keys: keys, MouseButtons: buttons, Touches: touches, PointerX: 1, PointerY: 2, Text: "é", Mac: true}
	if err := state.sample(frame); err != nil {
		t.Fatal(err)
	}
	for _, key := range []ebiten.Key{ebiten.KeyShift, ebiten.KeyControl, ebiten.KeyAlt, ebiten.KeyMeta, ebiten.KeyA} {
		if state.keys[key] != 1 {
			t.Fatalf("key %v not normalized once", key)
		}
	}
	if state.x != 20 || state.y != 30 || !state.primaryJust || !state.primaryTouch || state.primaryTicks != 1 {
		t.Fatal("duplicate touch ID changed primary contact")
	}
	keys[0] = ebiten.KeyB
	buttons[0] = ebiten.MouseButton1
	touches[0].X = 99
	if state.keys[ebiten.KeyShiftLeft] != 1 || state.buttons[ebiten.MouseButton0] != 1 || state.x != 20 {
		t.Fatal("snapshot retained caller-owned slice data")
	}
	frame.Time = frame.Time.Add(time.Second)
	frame.Keys = []ebiten.Key{ebiten.KeyShiftRight}
	frame.Touches = []TouchPoint{{ID: 5, X: 40, Y: 50}}
	if err := state.sample(frame); err != nil {
		t.Fatal(err)
	}
	if state.keys[ebiten.KeyShift] != 2 || state.keys[ebiten.KeyShiftLeft] != 0 || state.keys[ebiten.KeyA] != 0 || state.primaryJust || state.primaryTicks != 2 {
		t.Fatal("hold/release transitions or modifier-side switch were incorrect")
	}
	before := state
	bad := frame
	bad.Time = time.Time{}
	if err := state.sample(bad); err == nil {
		t.Fatal("zero clock accepted")
	}
	bad = frame
	bad.Time = bad.Time.Add(-time.Second)
	if err := state.sample(bad); err == nil {
		t.Fatal("backward clock accepted")
	}
	bad = frame
	bad.WheelY = math.NaN()
	if err := state.sample(bad); err == nil {
		t.Fatal("non-finite wheel accepted")
	}
	if !reflect.DeepEqual(state, before) {
		t.Fatal("invalid frame partially updated snapshot")
	}
}

func TestInputTouchOwnershipAndTwoFingerScrolling(t *testing.T) {
	var state inputState
	now := time.Unix(1, 0)
	sample := func(touches ...TouchPoint) {
		t.Helper()
		now = now.Add(time.Second / 60)
		if err := state.sample(InputFrame{Time: now, Touches: touches}); err != nil {
			t.Fatal(err)
		}
	}
	sample(TouchPoint{ID: 7, X: 100, Y: 100})
	if !state.primaryJust {
		t.Fatal("first touch did not press")
	}
	sample(TouchPoint{ID: 9, X: 200, Y: 200}, TouchPoint{ID: 7, X: 100, Y: 100})
	if state.primaryDown || !state.primaryEnded || state.wheelY != 0 || state.x != 100 {
		t.Fatal("second contact failed to end primary capture or caused a scroll jump")
	}
	sample(TouchPoint{ID: 7, X: 100, Y: 60}, TouchPoint{ID: 9, X: 200, Y: 160})
	if math.Abs(state.wheelY-(-2)) > .001 {
		t.Fatalf("two-finger scroll = %g, want -2", state.wheelY)
	}
	sample(TouchPoint{ID: 1, X: 2000, Y: 2000}, TouchPoint{ID: 7, X: 100, Y: 60}, TouchPoint{ID: 9, X: 200, Y: 160})
	if state.wheelX != 0 || state.wheelY != 0 {
		t.Fatal("changing scroll contacts caused an average-position jump")
	}
	sample(TouchPoint{ID: 7, X: 100, Y: 60})
	if state.primaryJust || !state.primaryDown || state.primaryTicks != 5 || state.wheelY != 0 {
		t.Fatal("remaining contact synthesized a fresh click")
	}
	sample()
	sample(TouchPoint{ID: 8, X: 150, Y: 90})
	if !state.primaryJust {
		t.Fatal("new touch after lift did not press")
	}
	sample(TouchPoint{ID: 11, X: 160, Y: 90})
	if !state.primaryEnded || !state.primaryJust {
		t.Fatal("contact replacement did not end old capture")
	}
}

func TestInputBrowserWheelUsesInjectedClockAndIdleDoesNotConsumeBudget(t *testing.T) {
	ctx := newTestContext(t)

	oldWasm, oldLimiter := ctx.isWasm, ctx.wheelLimiter
	ctx.isWasm = true
	ctx.wheelLimiter = rate.NewLimiter(rate.Every(125*time.Millisecond), 1)
	t.Cleanup(func() { ctx.isWasm, ctx.wheelLimiter = oldWasm, oldLimiter })
	state := inputState{ctx: ctx}
	now := time.Unix(10, 0)
	sample := func(delta float64, advance time.Duration) {
		t.Helper()
		now = now.Add(advance)
		if err := state.sample(InputFrame{Time: now, WheelY: delta}); err != nil {
			t.Fatal(err)
		}
	}
	sample(0, 0)
	sample(-100, time.Millisecond)
	if state.wheelY != -3 {
		t.Fatal("idle sample consumed browser wheel rate budget")
	}
	sample(-100, time.Millisecond)
	if state.wheelY != 0 {
		t.Fatal("browser wheel did not throttle burst")
	}
	sample(100, 126*time.Millisecond)
	if state.wheelY != 3 {
		t.Fatal("injected clock did not restore wheel budget")
	}
}
