package eui

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"testing"
)

import "github.com/hajimehoshi/ebiten/v2"

func TestRenderIncrementalDocument(t *testing.T) {
	path := os.Getenv("EUI_INCREMENTAL_CAPTURE")
	if path == "" {
		t.Skip("set EUI_INCREMENTAL_CAPTURE to a PNG path; run alone")
	}
	ctx := newTestContext(t)
	g := &incrementalRenderGame{ctx: ctx, path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type incrementalRenderGame struct {
	ctx  *Context
	path string
	done bool
	err  error
}

func (*incrementalRenderGame) Layout(int, int) (int, int) { return 720, 420 }
func (g *incrementalRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *incrementalRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.check(screen)
	}
}
func (g *incrementalRenderGame) check(screen *ebiten.Image) error {
	ctx := g.ctx
	reference := ebiten.NewImage(720, 420)
	defer reference.Deallocate()
	before, after := make([]byte, 720*420*4), make([]byte, 720*420*4)
	for _, theme := range []string{"AccentDark", "Paper"} {
		for _, scale := range []float32{1, 1.5, 2} {
			if err := ctx.LoadTheme(theme); err != nil {
				return err
			}
			ctx.uiScale = scale
			item, _ := ctx.NewTextArea()
			item.UpdateText(benchmarkDocument(2000))
			item.WordWrap = true
			item.Size = Point{X: 680 / scale, Y: 380 / scale}
			item.textDrawOrigin, item.textDrawSize = Point{X: 20, Y: 20}, Point{X: 680, Y: 380}
			item.DrawRect = rect{X0: 20, Y0: 20, X1: 700, Y1: 400}
			ctx.Focus(item)
			item.SetTextHighlighter(func(value string) []TextColorSpan { return []TextColorSpan{{0, 200000, ColorGreen}} })
			item.editMove(item.editor().runeCount/2, false)
			item.editLayout()
			for step := 0; step < 7; step++ {
				switch step {
				case 0:
					item.editInsert("日本 e\u0301 👩‍💻 inserted\nline\t", "")
				case 1:
					item.Undo()
				case 2:
					item.Redo()
				case 3:
					item.textDrawSize.X = 530
				case 4:
					item.setComposition("日本語", 1, 2)
				case 5:
					item.commitIMEEdit(item.CursorPos, item.CursorPos, "日本語", item.CursorPos+3)
				case 6:
					item.SetWordWrap(false)
				}
				state := item.editor()
				state.followCaret = true
				bounds := rect{X0: 20, Y0: 20, X1: 20 + item.textDrawSize.X, Y1: 400}
				screen.Clear()
				item.drawEditableText(screen, item.textDrawOrigin, item.textDrawSize, bounds, ColorWhite, ColorBlue)
				incremental, scroll := state.layout, state.scroll
				caretOn := state.caretOn
				state.layout = nil
				state.followCaret = false
				state.scroll = scroll
				state.caretOn = caretOn
				reference.Clear()
				item.drawEditableText(reference, item.textDrawOrigin, item.textDrawSize, bounds, ColorWhite, ColorBlue)
				screen.ReadPixels(before)
				reference.ReadPixels(after)
				if !bytes.Equal(before, after) {
					return fmt.Errorf("incremental pixels differ from fresh layout: %s/%gx step %d", theme, scale, step)
				}
				state.layout = incremental
			}
		}
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, screen)
}
