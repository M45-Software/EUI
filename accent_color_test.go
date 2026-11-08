package eui

import "testing"

// Test that setting the accent color updates the named "accent" color reference.
func TestSetAccentColorUpdatesNamedColor(t *testing.T) {
	ctx := newTestContext(t)

	ctx.namedColors = map[string]Color{}
	ctx.SetAccentColor(NewColor(255, 0, 0, 255))
	c, err := ctx.ParseColor([]byte("\"accent\""))
	if err != nil {
		t.Fatalf("failed to unmarshal accent: %v", err)
	}
	if c != ctx.AccentColor() {
		t.Fatalf("named accent %v does not match accent color %v", c, ctx.AccentColor())
	}
}

// Test that setting the accent color marks all windows dirty so they redraw.
func TestSetAccentColorMarksWindowsDirty(t *testing.T) {
	ctx := newTestContext(t)

	oldWindows := ctx.windows
	oldNamed := ctx.namedColors
	defer func() {
		ctx.windows = oldWindows
		ctx.namedColors = oldNamed
	}()

	win1 := &windowData{ctx: ctx, Open: true}
	win2 := &windowData{ctx: ctx, Open: true}
	ctx.windows = []*windowData{win1, win2}
	ctx.namedColors = map[string]Color{}

	ctx.SetAccentColor(NewColor(0, 0, 255, 255))

	for i, w := range ctx.windows {
		if !w.Dirty {
			t.Fatalf("window %d not marked dirty", i)
		}
	}
}
