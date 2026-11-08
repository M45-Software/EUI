package eui

import "testing"

func TestNewWindowBackgroundFollowsTheme(t *testing.T) {
	ctx := newTestContext(t)

	originalTheme := ctx.currentTheme
	originalWindows := ctx.windows
	defer func() {
		ctx.currentTheme = originalTheme
		ctx.windows = originalWindows
	}()

	first := *baseTheme
	first.Window.BGColor = NewColor(1, 2, 3, 255)
	first.Window.TitleBGColor = NewColor(4, 5, 6, 255)
	ctx.currentTheme = &first

	win := ctx.NewWindow()
	if win.BGColor != (Color{}) || win.TitleBGColor != (Color{}) {
		t.Fatal("new window copied theme backgrounds into per-window overrides")
	}
	if got := win.backgroundColor(); got != first.Window.BGColor {
		t.Fatalf("background = %v, want %v", got, first.Window.BGColor)
	}
	if got := win.BackgroundColor(); got != first.Window.BGColor {
		t.Fatalf("public background = %v, want %v", got, first.Window.BGColor)
	}

	second := *baseTheme
	second.Window.BGColor = NewColor(7, 8, 9, 255)
	second.Window.TitleBGColor = NewColor(10, 11, 12, 255)
	ctx.windows = []*windowData{win}
	ctx.updateThemeReferences(&first, &second)

	if got := win.backgroundColor(); got != second.Window.BGColor {
		t.Fatalf("updated background = %v, want %v", got, second.Window.BGColor)
	}
	if got := win.titleBackgroundColor(); got != second.Window.TitleBGColor {
		t.Fatalf("updated title background = %v, want %v", got, second.Window.TitleBGColor)
	}
}

func TestWindowBackgroundOverrideSurvivesThemeChange(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.NewWindow()
	override := NewColor(20, 30, 40, 200)
	win.BGColor = override

	replacement := *baseTheme
	replacement.Window.BGColor = NewColor(50, 60, 70, 255)
	win.Theme = &replacement

	if got := win.backgroundColor(); got != override {
		t.Fatalf("background override = %v, want %v", got, override)
	}
}
