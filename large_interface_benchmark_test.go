package eui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func (ctx *Context) benchmarkList(tb testing.TB, count int) (*WindowData, *ItemData) {
	tb.Helper()
	win := ctx.keyboardFixture(tb)
	win.Position = Point{X: 10, Y: 10}
	win.Size = Point{X: 900, Y: 680}
	win.NoScroll = true
	rows := make([]*ItemData, count)
	for i := range rows {
		rows[i] = ctx.NewLabel(fmt.Sprintf("%05d  Player café %d — health %d / 100", i, i%127, 60+i%41))
		rows[i].Position = Point{}
		rows[i].Size.Y = 24
		rows[i].FillWidth = true
	}
	list := ctx.NewColumn(rows...)
	list.Fixed, list.Scrollable = true, true
	list.Size = Point{X: 850, Y: 600}
	list.Gap = 2
	win.AddItem(list)
	win.Refresh()
	return win, list
}
func benchmarkDocument(lines int) string {
	var value strings.Builder
	for i := range lines {
		fmt.Fprintf(&value, "\trow_%05d := \"café e\u0301 / 👩‍💻 status %d\" // document text with a longer comment for wrapping\n", i, i%127)
	}
	return value.String()
}
func (ctx *Context) benchmarkEditor(tb testing.TB, lines int) (*WindowData, *ItemData) {
	win := ctx.keyboardFixture(tb)
	win.Position = Point{X: 10, Y: 10}
	win.Size = Point{X: 900, Y: 680}
	item, _ := ctx.NewTextArea()
	item.Position = Point{}
	item.Size = Point{X: 850, Y: 600}
	item.UpdateText(benchmarkDocument(lines))
	win.AddItem(item)
	item.editLayout()
	return win, item
}

// CPU-only layout/navigation measurements. Rendering is measured separately by
// BenchmarkLargeInterfaceFrames inside a running game with bounded GPU queues.
func BenchmarkLargeInterface(b *testing.B) {
	ctx := newTestContext(b)

	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("List/%d", count), func(b *testing.B) {
			b.Run("Bounds", func(b *testing.B) {
				_, list := ctx.benchmarkList(b, count)
				b.ReportAllocs()
				for b.Loop() {
					if list.contentBounds().Y <= 600 {
						b.Fatal("missing overflow")
					}
				}
			})
			b.Run("ScrollTraversal", func(b *testing.B) {
				_, list := ctx.benchmarkList(b, count)
				extent := list.contentBounds().Y - list.GetSize().Y
				tick, visible := 0, 0
				b.ReportAllocs()
				for b.Loop() {
					list.Scroll.Y = float32(tick%101) / 100 * extent
					tick++
					visible = 0
					for child, pos := range list.layoutChildren(list.GetSize()) {
						y := pos.Y - list.Scroll.Y
						if y < 600 && y+child.GetSize().Y > 0 {
							visible++
						}
					}
				}
				if visible < 1 || visible > 30 {
					b.Fatal("invalid visible row count", visible)
				}
			})
			b.Run("Resize", func(b *testing.B) {
				win, list := ctx.benchmarkList(b, count)
				tick := 0
				b.ReportAllocs()
				for b.Loop() {
					tick++
					list.Size.X = 700 + float32(tick%160)
					win.Refresh()
				}
			})
			b.Run("Theme", func(b *testing.B) {
				ctx.benchmarkList(b, count)
				names := []string{"AccentDark", "Paper"}
				tick := 0
				b.Cleanup(func() { _ = ctx.LoadTheme("AccentDark") })
				b.ReportAllocs()
				for b.Loop() {
					if err := ctx.LoadTheme(names[tick%2]); err != nil {
						b.Fatal(err)
					}
					tick++
				}
			})
		})
		b.Run(fmt.Sprintf("Document/%d", count), func(b *testing.B) {
			b.Run("WarmLayout", func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				b.ReportAllocs()
				for b.Loop() {
					item.editLayout()
				}
			})
			b.Run("CaretEnd", func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				item.CursorPos = len([]rune(item.Text))
				b.ReportAllocs()
				for b.Loop() {
					item.editCaret()
				}
			})
			b.Run("ColdLayout", func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				b.ReportAllocs()
				for b.Loop() {
					item.textEdit.layout = nil
					item.editLayout()
				}
			})
			b.Run("EditAtEnd", func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				item.editMove(len([]rune(item.Text)), false)
				b.ReportAllocs()
				for b.Loop() {
					item.editInsert("!", "")
					item.editLayout()
					item.Undo()
					item.editLayout()
				}
			})
			b.Run("WrappedResize", func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				item.SetWordWrap(true)
				item.textDrawSize = Point{X: 260, Y: 600}
				if len(item.editLayout().lines) <= count+1 {
					b.Fatal("fixture did not wrap")
				}
				tick := 0
				b.ReportAllocs()
				for b.Loop() {
					tick++
					item.textDrawSize = Point{X: 260 + float32(tick%160), Y: 600}
					item.editLayout()
				}
			})
		})
	}
}

// Run alone with -run '^$'. Each iteration is a complete Update/Draw CPU frame;
// GPU readback happens with the timer stopped to flush work between iterations.
// These numbers measure frame preparation, not GPU presentation or frame rate.
func BenchmarkLargeInterfaceFrames(b *testing.B) {
	ctx := newTestContext(b)

	if err := ctx.initFonts(); err != nil {
		b.Fatal(err)
	}
	g := &largeInterfaceBenchmarkGame{ctx: ctx, b: b}
	if err := ebiten.RunGame(g); err != nil {
		b.Fatal(err)
	}
}

type largeInterfaceBenchmarkGame struct {
	ctx *Context

	b    *testing.B
	done bool
}

func (*largeInterfaceBenchmarkGame) Layout(int, int) (int, int) { return 960, 720 }
func (g *largeInterfaceBenchmarkGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *largeInterfaceBenchmarkGame) Draw(screen *ebiten.Image) {
	ctx := g.ctx

	if g.done {
		return
	}
	g.done = true
	for _, count := range []int{1000, 10000} {
		g.b.Run(fmt.Sprintf("ListScroll/%d", count), func(b *testing.B) {
			win, list := ctx.benchmarkList(b, count)
			maxScroll := list.contentBounds().Y - list.GetSize().Y
			ctx.benchmarkFrames(b, screen, func(tick int) { list.Scroll.Y = float32(tick%101) / 100 * maxScroll; win.markDirty() })
		})
		g.b.Run(fmt.Sprintf("DocumentScroll/%d", count), func(b *testing.B) {
			win, item := ctx.benchmarkEditor(b, count)
			item.editor().followCaret = false
			maxScroll := float32(len(item.editLayout().lines))*item.editLayout().lineHeight - 600
			ctx.benchmarkFrames(b, screen, func(tick int) { item.editor().scroll.Y = float32(tick%101) / 100 * maxScroll; win.markDirty() })
		})
	}
}
func (ctx *Context) benchmarkFrames(b *testing.B, screen *ebiten.Image, change func(int)) {
	frame := InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}
	previous := ctx.SetInputSource(func() InputFrame { return frame })
	b.Cleanup(func() { ctx.SetInputSource(previous) })
	pixels := make([]byte, 960*720*4)
	render := func(tick int) {
		frame.Time = frame.Time.Add(time.Second / 60)
		change(tick)
		if err := ctx.Update(); err != nil {
			b.Fatal(err)
		}
		screen.Clear()
		ctx.Draw(screen)
	}
	// Populate glyph atlases and exercise every scroll stop before timing.
	for i := 0; i <= 100; i++ {
		render(i)
		screen.ReadPixels(pixels)
	}
	tick := 0
	b.ReportAllocs()
	for b.Loop() {
		tick++
		render(tick)
		b.StopTimer()
		screen.ReadPixels(pixels)
		b.StartTimer()
	}
}
