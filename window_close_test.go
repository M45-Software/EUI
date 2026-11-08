package eui

import "testing"

func TestCloseUpdatesActiveWindow(t *testing.T) {
	ctx := newTestContext(t)

	ctx.windows = nil
	w1 := ctx.NewWindow()
	w2 := ctx.NewWindow()
	w1.MarkOpen()
	w2.MarkOpen()
	ctx.activeWindow = w2
	w2.Close()
	if ctx.activeWindow != w1 {
		t.Fatalf("expected activeWindow to fall back to w1, got %v", ctx.activeWindow)
	}
}

func TestMarkOpenCallsOnOpenOnlyForClosedWindow(t *testing.T) {
	ctx := newTestContext(t)

	ctx.windows = nil
	win := ctx.NewWindow()
	calls := 0
	win.OnOpen = func() { calls++ }

	win.MarkOpen()
	win.MarkOpen()
	if calls != 1 {
		t.Fatalf("OnOpen calls while already open = %d, want 1", calls)
	}

	win.Close()
	win.MarkOpen()
	if calls != 2 {
		t.Fatalf("OnOpen calls after reopening = %d, want 2", calls)
	}
}

func TestBeforeClosePreservesWindowUntilAllowed(t *testing.T) {
	ctx := newTestContext(t)

	oldWindows, oldActive := ctx.windows, ctx.activeWindow
	t.Cleanup(func() { ctx.windows, ctx.activeWindow = oldWindows, oldActive })
	ctx.windows = nil
	win := ctx.NewWindow()
	win.AddWindow(false)
	win.MarkOpen()
	allow, closed := false, 0
	win.BeforeClose = func() bool { return allow }
	win.OnClose = func() { closed++ }
	win.Close()
	if !win.Open || ctx.activeWindow != win || closed != 0 {
		t.Fatal("vetoed close changed window state")
	}
	allow = true
	win.Close()
	if win.Open || ctx.activeWindow == win || closed != 1 {
		t.Fatal("allowed close did not complete")
	}
}
