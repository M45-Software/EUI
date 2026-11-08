package eui

import "testing"

func TestNoSurfaceButtonSurvivesStyleChanges(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	ctx.isolateThemeTest(t)
	button, _ := ctx.NewButton()
	button.NoSurface = true
	if err := ctx.LoadStyle("Outline"); err != nil {
		t.Fatal(err)
	}
	if !button.NoSurface {
		t.Fatal("style change restored the button surface")
	}
	if !button.Outlined || button.Border <= 0 {
		t.Fatal("test style does not exercise outlined buttons")
	}
	if itemDrawsOutline(button, button.themeStyle()) {
		t.Fatal("NoSurface button retained its outline")
	}
	button.NoSurface = false
	if !itemDrawsOutline(button, button.themeStyle()) {
		t.Fatal("ordinary outlined button lost its outline")
	}
}
