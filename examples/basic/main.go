// A standalone EUI application: no goThoom resources or settings are needed.
package main

import (
	"image/color"
	"log"

	"github.com/Distortions81/EUI"

	"github.com/hajimehoshi/ebiten/v2"
)

type game struct{ ui *eui.Context }

func (g *game) Update() error { return g.ui.Update() }
func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	g.ui.Draw(screen)
}
func (g *game) Layout(w, h int) (int, int) { return g.ui.Layout(w, h) }

func main() {
	ui, err := eui.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Dispose()
	win := ui.NewWindow()
	win.Title = "EUI example"
	win.AutoSize, win.Movable, win.Closable = true, true, true
	label := ui.NewLabel("A UI with no external assets")
	button := ui.NewActionButton("Open dialog", func() {
		ui.ShowModal("Hello", "This dialog belongs entirely to EUI.", []eui.PopupButton{{Text: "OK", Default: true, Cancel: true}})
	})
	swatch := ui.NewColorSwatch("Accent", ui.AccentColor(), nil, func(c eui.Color) {
		label.Text = "Color selected"
		win.Refresh()
	})
	input, _ := ui.NewInput()
	input.Label = "Name"
	input.Text = "Select and replace this text"
	input.Size.X = 440
	area, _ := ui.NewTextArea()
	area.Label = "Text editing"
	area.Size = eui.Point{X: 440, Y: 220}
	area.AcceptTab = true
	area.Text = "// Try selecting, indenting, and undoing.\nfunc greet() {\n\tprint(\"Hello!\")\n}\n"
	win.AddItem(ui.NewColumn(label, input, area,
		ui.NewLabel("Ctrl/Command+Z: undo. Ctrl+Tab: leave the editor."), button, swatch))
	win.MarkOpen()
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("EUI basic example")
	if err := ebiten.RunGame(&game{ui: ui}); err != nil {
		log.Fatal(err)
	}
}
