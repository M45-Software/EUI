package eui

import "testing"

func TestOverlayChildrenShareOneOrigin(t *testing.T) {
	ctx := newTestContext(t)

	oldScale := ctx.UIScale()
	t.Cleanup(func() { ctx.SetUIScale(oldScale) })
	ctx.SetUIScale(1)

	background, _ := ctx.NewButton()
	background.Size = Point{X: 100, Y: 24}
	background.Position = Point{}
	action, _ := ctx.NewButton()
	action.Size = Point{X: 20, Y: 24}
	action.Position = Point{X: 80}
	overlay := ctx.NewOverlay(background, action)
	overlay.Fixed = true
	overlay.Size = Point{X: 100, Y: 24}

	if overlay.FlowType != FLOW_OVERLAY {
		t.Fatal("NewOverlay did not create overlay flow")
	}
	if got := overlay.contentBounds(); got != (Point{X: 100, Y: 24}) {
		t.Fatalf("overlay bounds = %+v, want children sharing 100x24 rectangle", got)
	}
}
