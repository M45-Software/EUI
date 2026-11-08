package eui

import "testing"

func TestSetWindowShadows(t *testing.T) {
	ctx := newTestContext(t)

	original := ctx.windowShadows
	t.Cleanup(func() { ctx.SetWindowShadows(original) })

	win := ctx.NewWindow()
	win.Dirty = false
	ctx.windows = append(ctx.windows, win)
	t.Cleanup(func() { ctx.windows = ctx.windows[:len(ctx.windows)-1] })

	ctx.SetWindowShadows(false)
	if ctx.windowShadows {
		t.Fatal("window shadows remained enabled")
	}
	if !win.Dirty {
		t.Fatal("changing window shadows did not invalidate window caches")
	}
}

func TestThemeSwitchUpdatesWindowAndMenuEffects(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("NeonNight"); err != nil {
		t.Fatal(err)
	}
	win := ctx.NewWindow()
	menu, _ := ctx.NewDropdown()
	win.AddItem(menu)
	ctx.windows = []*windowData{win}
	for _, name := range []string{"SeaGlass", "HighContrast", "Arcade"} {
		if err := ctx.LoadTheme(name); err != nil {
			t.Fatal(err)
		}
		if win.ShadowSize != ctx.currentTheme.Window.ShadowSize || win.ShadowColor != ctx.currentTheme.Window.ShadowColor || win.ShadowFalloff != ctx.currentTheme.Window.ShadowFalloff {
			t.Fatalf("%s left stale window effects", name)
		}
		if menu.ShadowSize != ctx.currentTheme.Dropdown.ShadowSize || menu.ShadowColor != ctx.currentTheme.Dropdown.ShadowColor || menu.ShadowFalloff != ctx.currentTheme.Dropdown.ShadowFalloff {
			t.Fatalf("%s left stale menu effects", name)
		}
	}
}
