package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func BenchmarkUIDrawClippedTextList(b *testing.B) {
	ctx := newTestContext(b)

	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		b.Fatal(err)
	}
	win := ctx.NewWindow()
	flow := &itemData{ctx: ctx, ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Size: point{X: 300, Y: 200}}
	for range 1000 {
		flow.Contents = append(flow.Contents, &itemData{ctx: ctx,
			ItemType: ITEM_TEXT, Text: "Player status\nAdditional information", FontSize: 12,
			Position: point{X: 1000},
		})
	}
	screen := ebiten.NewImage(300, 200)
	b.Cleanup(func() { screen.Deallocate() })
	var dropdowns []openDropdown
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		flow.drawFlows(win, flow, point{}, point{}, rect{X1: 300, Y1: 200}, screen, &dropdowns)
	}
}

func BenchmarkUITextListBounds(b *testing.B) {
	ctx := newTestContext(b)

	if err := ctx.EnsureFontSource(goregular.TTF); err != nil {
		b.Fatal(err)
	}
	flow := &itemData{ctx: ctx, ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	for range 1000 {
		flow.Contents = append(flow.Contents, &itemData{ctx: ctx,
			ItemType: ITEM_TEXT, Text: "Player status\nAdditional information", FontSize: 12,
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		flow.bounds(point{})
	}
}
