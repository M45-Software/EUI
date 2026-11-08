package eui

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone; Ebitengine permits one RunGame per process.
func TestRenderIMEComposition(t *testing.T) {
	ctx := newTestContext(t)

	path := os.Getenv("EUI_IME_CAPTURE")
	if path == "" {
		t.Skip("set EUI_IME_CAPTURE to a PNG path; run alone")
	}
	win := ctx.keyboardFixture(t)
	win.Position, win.Size = Point{X: 20, Y: 20}, Point{X: 600, Y: 400}
	win.Title = "Inline composition"
	item, _ := ctx.NewTextArea()
	item.Position, item.Size = Point{}, Point{X: 500, Y: 150}
	item.Label = "Preedit stays separate from the document"
	item.UpdateText("Before old after")
	password, _ := ctx.NewInput()
	password.Position, password.Size = Point{}, Point{X: 400, Y: 30}
	password.Label = "Password composition is masked"
	password.HideText = true
	password.UpdateText("secret")
	root := ctx.NewColumn(item, password)
	root.Gap = 20
	win.AddItem(root)
	g := &imeRenderGame{ctx: ctx, win: win, item: item, password: password, path: path, frame: InputFrame{Time: time.Unix(1000, 0), PointerX: -100, PointerY: -100}}
	previous := ctx.SetInputSource(func() InputFrame { return g.frame })
	t.Cleanup(func() { ctx.SetInputSource(previous) })
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type imeRenderGame struct {
	ctx *Context

	win            *WindowData
	item, password *ItemData
	frame          InputFrame
	path           string
	done           bool
	err            error
}

func (*imeRenderGame) Layout(int, int) (int, int) { return 640, 480 }
func (g *imeRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *imeRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.render(screen)
	}
}
func (g *imeRenderGame) render(screen *ebiten.Image) error {
	ctx := g.ctx

	step := func(events ...TextInputEvent) []byte {
		g.frame.Time = g.frame.Time.Add(time.Second / 60)
		g.frame.TextInput = events
		if err := ctx.Update(); err != nil {
			g.err = err
		}
		screen.Clear()
		ctx.Draw(screen)
		pixels := make([]byte, 640*480*4)
		screen.ReadPixels(pixels)
		return pixels
	}
	montage := ebiten.NewImage(1280, 960)
	defer montage.Deallocate()
	shot := func(index int) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(index%2*640), float64(index/2*480))
		montage.DrawImage(screen, op)
	}
	step()
	ctx.Focus(g.item)
	g.item.editSelect(7, 10)
	baseline := step()
	shot(0)
	events := 0
	g.item.Handler.Handle = func(UIEvent) { events++ }
	composed := step(TextInputEvent{Kind: TextInputCompose, Text: "café", SelectionStart: 0, SelectionEnd: 4})
	shot(1)
	if bytes.Equal(baseline, composed) || g.item.Text != "Before old after" || events != 0 || g.item.CanUndo() {
		return fmt.Errorf("preedit did not render separately from committed text")
	}
	if restored := step(TextInputEvent{Kind: TextInputCancel}); !bytes.Equal(restored, baseline) {
		return fmt.Errorf("cancelling preedit did not restore original pixels")
	}
	step(TextInputEvent{Kind: TextInputCompose, Text: "café", SelectionEnd: 4})
	committed := step(TextInputEvent{Kind: TextInputCommit, Text: "café"})
	shot(2)
	if bytes.Equal(composed, committed) || g.item.Text != "Before café after" || events != 1 {
		return fmt.Errorf("commit did not remove preedit selection/underline or notify once")
	}
	ctx.Focus(g.password)
	g.password.editMove(6, false)
	passwordBase := step()
	passwordPreedit := step(TextInputEvent{Kind: TextInputCompose, Text: "café", SelectionEnd: 4})
	shot(3)
	if bytes.Equal(passwordBase, passwordPreedit) || g.password.displayEditText() != "**********" || g.password.SecretText != "secret" {
		return fmt.Errorf("password preedit pixels or masking failed")
	}
	step(TextInputEvent{Kind: TextInputCancel})
	g.win.Searchable = true
	g.win.OpenSearch()
	searchBase := step()
	searchPreedit := step(TextInputEvent{Kind: TextInputCompose, Text: "café", SelectionEnd: 4})
	if bytes.Equal(searchBase, searchPreedit) || g.win.SearchText != "" {
		return fmt.Errorf("search preedit did not invalidate title cache independently of query")
	}
	searchCommit := step(TextInputEvent{Kind: TextInputCommit, Text: "café"})
	if bytes.Equal(searchPreedit, searchCommit) || g.win.SearchText != "café" {
		return fmt.Errorf("search commit did not update title pixels/query")
	}
	if g.err != nil {
		return g.err
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, montage); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
