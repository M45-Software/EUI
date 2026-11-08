package eui

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderLargeDocument(t *testing.T) {
	ctx := newTestContext(t)

	path := os.Getenv("EUI_LARGE_CAPTURE")
	if path == "" {
		t.Skip("set EUI_LARGE_CAPTURE to a PNG path; run alone")
	}
	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	previous := ctx.uiScale
	t.Cleanup(func() { ctx.uiScale = previous })
	g := &largeDocumentRenderGame{ctx: ctx, path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type largeDocumentRenderGame struct {
	ctx *Context

	path string
	done bool
	err  error
}

func (*largeDocumentRenderGame) Layout(int, int) (int, int) { return 640, 360 }
func (g *largeDocumentRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *largeDocumentRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.verify(screen)
	}
}
func (g *largeDocumentRenderGame) verify(screen *ebiten.Image) error {
	ctx := g.ctx

	value := benchmarkDocument(10000)
	lines := strings.Split(value, "\n")
	visible := strings.Join(lines[5000:5030], "\n")
	prefix := utf8.RuneCountInString(strings.Join(lines[:5000], "\n")) + 1
	reference := ebiten.NewImage(640, 360)
	defer reference.Deallocate()
	before, after := make([]byte, 640*360*4), make([]byte, 640*360*4)
	for _, scale := range []float32{1, 1.5, 2} {
		ctx.uiScale = scale
		for _, highlighted := range []bool{false, true} {
			large, _ := ctx.NewTextArea()
			small, _ := ctx.NewTextArea()
			large.UpdateText(value)
			small.UpdateText(visible)
			for _, item := range []*ItemData{large, small} {
				item.Size = Point{X: 600 / scale, Y: 320 / scale}
				item.textDrawOrigin = Point{X: 20, Y: 20}
				item.textDrawSize = Point{X: 600, Y: 320}
				if highlighted {
					item.SetTextHighlighter(func(s string) []TextColorSpan { return []TextColorSpan{{0, utf8.RuneCountInString(s), ColorGreen}} })
				}
				item.editLayout()
				item.editor().followCaret = false
			}
			large.editor().scroll.Y = 5000*large.editLayout().lineHeight + .5
			small.editor().scroll.Y = .5
			large.SelectStart, large.SelectEnd = prefix+3, prefix+12
			small.SelectStart, small.SelectEnd = 3, 12
			screen.Clear()
			reference.Clear()
			bounds := rect{X0: 20, Y0: 20, X1: 620, Y1: 340}
			large.drawEditableText(screen, Point{X: 20, Y: 20}, Point{X: 600, Y: 320}, bounds, ColorWhite, ColorBlue)
			small.drawEditableText(reference, Point{X: 20, Y: 20}, Point{X: 600, Y: 320}, bounds, ColorWhite, ColorBlue)
			screen.ReadPixels(before)
			reference.ReadPixels(after)
			// Scrollbar thumbs have different proportions; compare the text viewport.
			viewport := large.editViewport(Point{X: 20, Y: 20}, Point{X: 600, Y: 320})
			for y := int(math.Ceil(float64(viewport.Y0))); y < int(viewport.Y1); y++ {
				a, b := (y*640+int(math.Ceil(float64(viewport.X0))))*4, (y*640+int(viewport.X1))*4
				if !bytes.Equal(before[a:b], after[a:b]) {
					return fmt.Errorf("large-document viewport differs from local excerpt: scale=%v highlighted=%v row=%d", scale, highlighted, y)
				}
			}
		}
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, screen); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
