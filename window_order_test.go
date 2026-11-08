package eui

import "testing"

func TestBringForwardReordersBackgroundWindowsWithinTheirLayer(t *testing.T) {
	ctx := newTestContext(t)

	oldWindows := ctx.windows
	oldActive := ctx.activeWindow
	ctx.windows = nil
	ctx.activeWindow = nil
	t.Cleanup(func() {
		ctx.windows = oldWindows
		ctx.activeWindow = oldActive
	})

	first := ctx.NewWindow()
	first.AlwaysDrawFirst = true
	second := ctx.NewWindow()
	second.AlwaysDrawFirst = true
	utility := ctx.NewWindow()
	first.AddWindow(false)
	second.AddWindow(false)
	utility.AddWindow(false)

	first.BringForward()
	if len(ctx.windows) != 3 || ctx.windows[0] != second || ctx.windows[1] != first || ctx.windows[2] != utility {
		t.Fatalf("window order = %p %p %p", ctx.windows[0], ctx.windows[1], ctx.windows[2])
	}
	if ctx.activeWindow != first {
		t.Fatal("brought-forward background window did not become active")
	}
}
