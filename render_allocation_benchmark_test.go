package eui

import "testing"

func BenchmarkUIStyleResolution(b *testing.B) {
	ctx := newTestContext(b)

	for _, mode := range []string{"Default", "CustomColors", "Disabled"} {
		b.Run(mode, func(b *testing.B) {
			item, _ := ctx.NewButton()
			if mode == "CustomColors" {
				item.SetButtonColors(ColorDarkRed, ColorRed)
			}
			var caption Color
			b.ReportAllocs()
			for b.Loop() {
				style := item.themeStyle()
				if mode == "Disabled" {
					style = disabledStyle(style)
				}
				caption = style.TextColor
			}
			if caption != item.themeStyle().TextColor && mode != "Disabled" {
				b.Fatal("unexpected caption color")
			}
		})
	}
}

func BenchmarkUITabLayout(b *testing.B) {
	ctx := newTestContext(b)

	if err := ctx.initFonts(); err != nil {
		b.Fatal(err)
	}
	flow := &itemData{ctx: ctx, ItemType: ITEM_FLOW, Fixed: true, Size: point{X: 420, Y: 240}}
	for _, name := range []string{"Display", "Text", "Audio", "Controls", "Performance", "Files", "Network", "Advanced"} {
		flow.Tabs = append(flow.Tabs, &itemData{ctx: ctx, Name: name})
	}
	var tabRects [16]rect
	ctx.layoutTabsInto(tabRects[:0], flow, flow.themeStyle())
	var size point
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, size = ctx.layoutTabsInto(tabRects[:0], flow, flow.themeStyle())
	}
	if size.X <= 0 || size.Y <= 0 {
		b.Fatal("empty tab layout")
	}
}
