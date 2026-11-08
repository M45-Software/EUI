package eui

import "testing"

func TestSliderTooltipIndicatorRequiresLabel(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win := NewWindow()
	win.ShowTooltipIndicators = true
	slider, _ := NewSlider()
	slider.Size = Point{X: 180, Y: 24}
	slider.SetTooltip("Adjust the enhancement strength.")
	win.AddItem(slider)
	if got := slider.tooltipIndicatorRect(Point{}, slider.GetSize()); got != (rect{}) {
		t.Fatalf("unlabeled slider has an indicator over its track: %v", got)
	}
	if !shouldDrawTooltip(slider, nil, nil) {
		t.Fatal("unlabeled slider lost its hover help")
	}
	slider.Label = "Enhancement strength"
	if got := slider.tooltipIndicatorRect(Point{}, slider.GetSize()); got.X1 <= got.X0 {
		t.Fatal("labeled slider lost its caption indicator")
	}
}

func TestInputTooltipIndicatorReservesEditingViewport(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	previousScale := uiScale
	defer func() { uiScale = previousScale }()
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		win := NewWindow()
		win.ShowTooltipIndicators = true
		input, _ := NewInput()
		input.Text = "A long field value that scrolls past the marker"
		input.SetTooltip("Rename this scene")
		win.AddItem(input)
		size := Point{X: 300 * scale, Y: 32 * scale}
		info := input.tooltipIndicatorRect(Point{}, size)
		viewport, _ := input.editBaseViewport(Point{}, size, input.editLayout().lineHeight)
		if info.X0 <= viewport.X1 {
			t.Fatalf("scale %v: input marker overlaps edit viewport: marker=%v viewport=%v", scale, info, viewport)
		}
		input.Text = "Short"
		if changed := input.tooltipIndicatorRect(Point{}, size); changed != info {
			t.Fatal("editable marker moved with text")
		}
	}
}
