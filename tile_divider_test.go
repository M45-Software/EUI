package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTileDividerUsesWideGrabArea(t *testing.T) {
	ctx := newTestContext(t)

	d := TileDivider{
		Orientation: TileDividerVertical,
		Position:    100,
		Start:       20,
		End:         200,
		Thickness:   4,
		HitSize:     16,
	}
	if !ctx.tileDividerHit(d, point{X: 107, Y: 100}) {
		t.Fatal("divider did not include its enlarged grab area")
	}
	if ctx.tileDividerHit(d, point{X: 109, Y: 100}) {
		t.Fatal("divider grab area extended beyond its configured size")
	}
}

func TestDockedWindowHonorsAssignedSizeOnSmallWorkspace(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	oldScale := ctx.UIScale()
	w, h := ctx.ScreenSize()
	t.Cleanup(func() { ctx.SetUIScale(oldScale); ctx.SetScreenSize(w, h) })
	ctx.SetScreenSize(640, 360)
	for _, scale := range []float32{1, 2} {
		ctx.SetUIScale(scale)
		win := ctx.NewWindow()
		win.Resizable = true
		win.SetDocked(true)
		want := Point{X: 32 * scale, Y: 40 * scale}
		win.SetSize(want)
		if win.GetSize() != want {
			t.Fatal("docked window expanded outside its assigned tile")
		}
		win.SetDocked(false)
		win.SetSize(want)
		if win.GetSize() != (Point{X: MinWindowSize * scale, Y: MinWindowSize * scale}) {
			t.Fatal("floating window lost its minimum size")
		}
	}
}

func TestDockedWindowHasNoStandaloneCornerDrag(t *testing.T) {
	ctx := newTestContext(t)

	originalScale := ctx.uiScale
	ctx.uiScale = 1
	t.Cleanup(func() { ctx.uiScale = originalScale })

	win := ctx.NewWindow()
	win.Position = point{X: 20, Y: 20}
	win.Size = point{X: 200, Y: 100}
	win.Movable = true
	win.Resizable = true
	win.Docked = true
	if got := win.getWindowPart(point{X: 20, Y: 20}, true); got != PART_NONE {
		t.Fatalf("docked corner hit = %v, want none", got)
	}
}

func TestDockedWindowRejectsDragAlreadyInProgress(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.NewWindow()
	win.Position = point{X: 20, Y: 20}
	win.Size = point{X: 200, Y: 100}
	win.Movable = true
	win.Resizable = true
	win.Docked = true

	ctx.dragWindowMove(win, point{X: 40, Y: 30})
	ctx.dragWindowResize(win, PART_BOTTOM_RIGHT, point{X: 40, Y: 30})

	if win.Position != (point{X: 20, Y: 20}) || win.Size != (point{X: 200, Y: 100}) {
		t.Fatalf("stale drag changed docked geometry: position=%v size=%v", win.Position, win.Size)
	}
}

func TestDockedWindowDoesNotDrawStandaloneOutline(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.NewWindow()
	win.Outlined = true
	win.Border = 2
	if !win.drawsStandaloneOutline() {
		t.Fatal("ordinary outlined window did not request its outline")
	}
	win.Docked = true
	if win.drawsStandaloneOutline() {
		t.Fatal("docked pane requested an outer window outline")
	}
	win.BorderColor = ColorBlue
	if !win.drawsStandaloneOutline() {
		t.Fatal("explicitly highlighted docked pane did not request its outline")
	}
	if got := win.borderColor(); got != ColorBlue {
		t.Fatalf("explicit border color = %v, want %v", got, ColorBlue)
	}
}

func TestFixedNonMovableWindowCornerDoesNotBecomeMoveHandle(t *testing.T) {
	ctx := newTestContext(t)

	originalScale := ctx.uiScale
	ctx.uiScale = 1
	t.Cleanup(func() { ctx.uiScale = originalScale })

	win := ctx.NewWindow()
	win.Position = point{X: 20, Y: 20}
	win.Size = point{X: 200, Y: 100}
	win.Movable = false
	win.Resizable = false
	if got := win.getWindowPart(point{X: 20, Y: 20}, true); got != PART_NONE {
		t.Fatalf("fixed corner hit = %v, want none", got)
	}
}

func TestStandaloneWindowCoversWorkspaceDivider(t *testing.T) {
	ctx := newTestContext(t)

	originalWindows := ctx.windows
	originalScale := ctx.uiScale
	ctx.uiScale = 1
	t.Cleanup(func() {
		ctx.windows = originalWindows
		ctx.uiScale = originalScale
	})

	utility := ctx.NewWindow()
	utility.Open = true
	utility.Position = point{X: 80, Y: 40}
	utility.Size = point{X: 80, Y: 120}
	ctx.windows = []*windowData{utility}

	if !ctx.tileDividerCoveredByStandaloneWindow(point{X: 100, Y: 100}) {
		t.Fatal("standalone window did not cover workspace divider input")
	}
	utility.Docked = true
	if ctx.tileDividerCoveredByStandaloneWindow(point{X: 100, Y: 100}) {
		t.Fatal("docked pane incorrectly covered workspace divider input")
	}
}

func TestStandaloneWindowsReceiveInputAboveDockedPanes(t *testing.T) {
	ctx := newTestContext(t)

	originalWindows := ctx.windows
	originalOrder := ctx.inputWindowOrder
	t.Cleanup(func() {
		ctx.windows = originalWindows
		ctx.inputWindowOrder = originalOrder
	})

	utilityBack := ctx.NewWindow()
	utilityFront := ctx.NewWindow()
	dockedBack := ctx.NewWindow()
	dockedFront := ctx.NewWindow()
	dockedBack.Docked = true
	dockedFront.Docked = true
	ctx.windows = []*windowData{utilityBack, dockedBack, utilityFront, dockedFront}

	got := ctx.windowsFrontToBack()
	want := []*windowData{utilityFront, utilityBack, dockedFront, dockedBack}
	if len(got) != len(want) {
		t.Fatalf("input order has %d windows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("input order item %d did not preserve layered z-order", i)
		}
	}
}

func TestPointerCursorClaimedTracksEUIResizeCursor(t *testing.T) {
	ctx := newTestContext(t)

	original := ctx.cursorShape
	t.Cleanup(func() { ctx.cursorShape = original })

	ctx.cursorShape = ebiten.CursorShapeEWResize
	if !ctx.PointerCursorClaimed() {
		t.Fatal("resize cursor was not claimed by EUI")
	}
	ctx.cursorShape = ebiten.CursorShapeDefault
	if ctx.PointerCursorClaimed() {
		t.Fatal("default cursor remained claimed by EUI")
	}
}

func TestDockedWindowReopensWithoutMovingNearAnchor(t *testing.T) {
	ctx := newTestContext(t)

	originalWindows, originalCallback := ctx.windows, ctx.WindowStateChanged
	ctx.WindowStateChanged = nil
	t.Cleanup(func() {
		ctx.windows, ctx.WindowStateChanged = originalWindows, originalCallback
	})
	win := ctx.NewWindow()
	win.Docked = true
	win.Position = point{X: 20, Y: 30}
	win.Size = point{X: 200, Y: 100}
	position, size := win.Position, win.Size
	anchor := &itemData{ctx: ctx, DrawRect: rect{X0: 400, Y0: 400, X1: 450, Y1: 425}}
	win.MarkOpen()
	for i := 0; i < 3; i++ {
		win.Close()
		win.MarkOpenNear(anchor)
		if !win.IsOpen() || !win.Docked || win.Position != position || win.Size != size {
			t.Fatalf("reopening moved or undocked the pane: open=%t docked=%t position=%v size=%v", win.IsOpen(), win.Docked, win.Position, win.Size)
		}
	}
}
