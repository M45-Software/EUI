//go:build integration
// +build integration

package eui

import "testing"

func TestSnapToCorner(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 100
	ctx.screenHeight = 100
	ctx.uiScale = 1

	winSize := point{X: 10, Y: 10}
	offset := CornerSnapThreshold - 1

	tests := []struct {
		pos point
		h   HZone
		v   VZone
	}{
		{point{offset, offset}, HZoneLeft, VZoneTop},
		{point{float32(ctx.screenWidth) - winSize.X - offset, offset}, HZoneRight, VZoneTop},
		{point{offset, float32(ctx.screenHeight) - winSize.Y - offset}, HZoneLeft, VZoneBottom},
		{point{float32(ctx.screenWidth) - winSize.X - offset, float32(ctx.screenHeight) - winSize.Y - offset}, HZoneRight, VZoneBottom},
	}

	for _, tt := range tests {
		win := &windowData{ctx: ctx, Position: tt.pos, Size: winSize}
		ctx.snapToCorner(win)
		if win.zone == nil {
			t.Fatalf("zone not set for pos %+v", tt.pos)
		}
		if win.zone.h != tt.h || win.zone.v != tt.v {
			t.Fatalf("pos %+v snapped to (%v,%v); want (%v,%v)", tt.pos, win.zone.h, win.zone.v, tt.h, tt.v)
		}
	}
}

// TestSnapToCornerScaled verifies corner snapping when a uiScale other than 1 is in use.
func TestSnapToCornerScaled(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 200
	ctx.screenHeight = 200
	ctx.uiScale = 2

	winSize := point{X: 10, Y: 10}
	offset := CornerSnapThreshold - 1

	sw := float32(ctx.screenWidth) / ctx.uiScale
	sh := float32(ctx.screenHeight) / ctx.uiScale

	tests := []struct {
		pos point
		h   HZone
		v   VZone
	}{
		{point{offset, offset}, HZoneLeft, VZoneTop},
		{point{sw - winSize.X - offset, offset}, HZoneRight, VZoneTop},
		{point{offset, sh - winSize.Y - offset}, HZoneLeft, VZoneBottom},
		{point{sw - winSize.X - offset, sh - winSize.Y - offset}, HZoneRight, VZoneBottom},
	}

	for _, tt := range tests {
		win := &windowData{ctx: ctx, Position: tt.pos, Size: winSize}
		ctx.snapToCorner(win)
		if win.zone == nil {
			t.Fatalf("zone not set for pos %+v", tt.pos)
		}
		if win.zone.h != tt.h || win.zone.v != tt.v {
			t.Fatalf("pos %+v snapped to (%v,%v); want (%v,%v)", tt.pos, win.zone.h, win.zone.v, tt.h, tt.v)
		}
	}
}

func TestSnapToWindow(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 200
	ctx.screenHeight = 200
	ctx.uiScale = 1

	base := &windowData{ctx: ctx, Position: point{50, 50}, Size: point{30, 30}, Open: true}
	winSize := point{10, 10}
	offset := CornerSnapThreshold - 1

	tests := []struct {
		name   string
		pos    point
		expect point
	}{
		{"left", point{base.Position.X - winSize.X - offset, base.Position.Y}, point{base.Position.X - winSize.X, base.Position.Y}},
		{"right", point{base.Position.X + base.Size.X + offset, base.Position.Y}, point{base.Position.X + base.Size.X, base.Position.Y}},
		{"top", point{base.Position.X, base.Position.Y - winSize.Y - offset}, point{base.Position.X, base.Position.Y - winSize.Y}},
		{"bottom", point{base.Position.X, base.Position.Y + base.Size.Y + offset}, point{base.Position.X, base.Position.Y + base.Size.Y}},
	}

	for _, tt := range tests {
		win := &windowData{ctx: ctx, Position: tt.pos, Size: winSize, Open: true}
		ctx.windows = []*windowData{base, win}
		ctx.snapToWindow(win)
		if win.Position != tt.expect {
			t.Fatalf("%s: pos %+v snapped to %+v; want %+v", tt.name, tt.pos, win.Position, tt.expect)
		}
	}
	ctx.windows = nil
}

func TestSnapResizeToWindow(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 200
	ctx.screenHeight = 200
	ctx.uiScale = 1

	base := &windowData{ctx: ctx, Position: point{120, 50}, Size: point{20, 20}, Open: true}
	win := &windowData{ctx: ctx, Position: point{50, 50}, Size: point{68, 20}, Open: true, Resizable: true}
	ctx.windows = []*windowData{base, win}

	ctx.snapResize(win, PART_RIGHT)

	expectedWidth := base.Position.X - win.Position.X
	if win.Size.X != expectedWidth {
		t.Fatalf("width snapped to %v; want %v", win.Size.X, expectedWidth)
	}
	ctx.windows = nil
}

func TestSnapResizeToScreen(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 200
	ctx.screenHeight = 200
	ctx.uiScale = 1

	win := &windowData{ctx: ctx, Position: point{50, 50}, Size: point{20, 141}, Open: true, Resizable: true}

	ctx.snapResize(win, PART_BOTTOM)

	expectedHeight := float32(ctx.screenHeight) - win.Position.Y
	if win.Size.Y != expectedHeight {
		t.Fatalf("height snapped to %v; want %v", win.Size.Y, expectedHeight)
	}
}

func TestSnapResizeToScreenScaled(t *testing.T) {
	ctx := newTestContext(t)

	ctx.screenWidth = 200
	ctx.screenHeight = 200
	ctx.uiScale = 2

	win := &windowData{ctx: ctx, Position: point{50, 50}, Size: point{20, 45}, Open: true, Resizable: true}

	ctx.snapResize(win, PART_BOTTOM)

	expectedHeight := float32(ctx.screenHeight)/ctx.uiScale - win.Position.Y
	if win.Size.Y != expectedHeight {
		t.Fatalf("height snapped to %v; want %v", win.Size.Y, expectedHeight)
	}
}
