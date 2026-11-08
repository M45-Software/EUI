// EUI Studio is a working UI playground. All controls are real EUI widgets.
package main

import (
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	eui "github.com/Distortions81/EUI"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const width, height = 1360, 900

type studio struct {
	tick                          int
	phase                         float64
	speed                         float32
	count                         int
	playing, grid, trails         bool
	sceneName                     string
	palette, style                string
	tint                          eui.Color
	windows                       []*eui.WindowData
	viewport                      *eui.WindowData
	canvas                        *ebiten.Image
	activity, progress, nameInput *eui.ItemData
	editor, notebookTabs          *eui.ItemData
	initialTab                    string
	paletteDD, styleDD, accent    *eui.ItemData
	messages                      []string
	screenshot                    string
	captured                      bool
}

func label(s string, size float32, w, h float32) *eui.ItemData {
	t := eui.NewLabel(s)
	t.FontSize, t.Size = size, eui.Point{X: w, Y: h}
	t.ConstrainToSize = true
	return t
}
func gap(h float32) *eui.ItemData { return label("", 12, 1, h) }
func button(s string, w float32, fn func()) *eui.ItemData {
	b := eui.NewActionButton(s, fn)
	b.Size = eui.Point{X: w, Y: 32}
	return b
}
func (s *studio) panel(title string, x, y, w, h float32) (*eui.WindowData, *eui.ItemData) {
	win := eui.NewWindow()
	win.Title, win.Size = title, eui.Point{X: w, Y: h}
	win.Closable, win.Resizable, win.Movable = false, false, false
	win.NoScroll, win.ShowTooltipIndicators = true, true
	win.Padding, win.BorderPad = 14, 0
	root := eui.NewColumn()
	root.Size.X = w - 28
	win.AddItem(root)
	win.MarkOpen()
	win.SetPos(eui.Point{X: x, Y: y})
	s.windows = append(s.windows, win)
	return win, root
}
func (s *studio) log(message string) {
	s.messages = append(s.messages, message)
	if len(s.messages) > 5 {
		s.messages = s.messages[len(s.messages)-5:]
	}
	if s.activity != nil {
		s.activity.UpdateText(strings.Join(s.messages, "\n\n"))
	}
}
func (s *studio) theme(name string) {
	if err := eui.LoadTheme(name); err != nil {
		s.log(err.Error())
		return
	}
	s.palette = name
	// Palette and geometry are independent in this playground.
	if err := eui.LoadStyle(s.style); err != nil {
		s.log(err.Error())
	}
	s.tint = eui.AccentColor()
	if s.editor != nil {
		s.editor.RefreshTextHighlighting()
	}
	if s.accent != nil {
		eui.SetColorSwatch(s.accent, s.tint)
	}
	s.log("Palette changed to " + name)
}
func dropdown(options []string, selected string, w float32, apply func(string)) *eui.ItemData {
	item, events := eui.NewDropdown()
	item.Options = options
	item.Selected = max(0, slices.Index(options, selected))
	item.Size = eui.Point{X: w, Y: 32}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(options) {
			apply(options[ev.Index])
		}
	}
	return item
}
func checkbox(title string, checked bool, apply func(bool)) *eui.ItemData {
	item, events := eui.NewCheckbox()
	item.Text = title
	item.Checked = checked
	item.Size = eui.Point{X: 260, Y: 26}
	item.FontSize = 12
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			apply(ev.Checked)
		}
	}
	return item
}
func (s *studio) build() error {
	if err := eui.Init(); err != nil {
		return err
	}
	if err := eui.LoadTheme(s.palette); err != nil {
		return err
	}
	if err := eui.LoadStyle(s.style); err != nil {
		return err
	}
	eui.AutoHiDPI = false
	eui.SetScreenSize(width, height)
	s.tint = eui.AccentColor()
	_, appearance := s.panel("01  /  APPEARANCE", 32, 136, 288, 688)
	appearance.AddItem(label("Make it yours.", 23, 252, 40))
	appearance.AddItem(label("Every control follows your theme.", 11, 252, 26))
	appearance.AddItem(gap(16))
	appearance.AddItem(label("COLOR PALETTE", 10, 252, 22))
	palettes, err := eui.ListThemes()
	if err != nil {
		return err
	}
	s.paletteDD = dropdown(palettes, s.palette, 248, s.theme)
	appearance.AddItem(s.paletteDD)
	appearance.AddItem(gap(12))
	// Quick picks make the palette family immediately discoverable.
	for _, names := range [][]string{{"AccentDark", "Paper"}, {"SeaGlass", "Coffee"}, {"NeonNight", "Arcade"}} {
		row := eui.NewRow()
		for _, name := range names {
			row.AddItem(button(name, 120, func() { s.theme(name); s.paletteDD.Selected = slices.Index(palettes, name) }))
		}
		appearance.AddItem(row)
	}
	appearance.AddItem(gap(18))
	appearance.AddItem(label("WIDGET STYLE", 10, 252, 22))
	styles, err := eui.ListStyles()
	if err != nil {
		return err
	}
	s.styleDD = dropdown(styles, s.style, 248, func(name string) {
		if err := eui.LoadStyle(name); err != nil {
			s.log(err.Error())
			return
		}
		s.style = name
		s.log("Style changed to " + name)
	})
	appearance.AddItem(s.styleDD)
	appearance.AddItem(gap(18))
	appearance.AddItem(label("ACCENT COLOR", 10, 252, 22))
	s.accent = eui.NewColorSwatch("Studio accent", s.tint, nil, func(c eui.Color) { s.tint = c; eui.SetAccentColor(c); s.log("Custom accent applied") })
	s.accent.Size = eui.Point{X: 248, Y: 32}
	appearance.AddItem(s.accent)
	appearance.AddItem(gap(16))
	appearance.AddItem(checkbox("Window shadows", true, eui.SetWindowShadows))
	appearance.AddItem(gap(20))
	appearance.AddItem(label(fmt.Sprintf("%d palettes  /  %d styles", len(palettes), len(styles)), 12, 252, 26))
	appearance.AddItem(label("Embedded. Editable. Yours.", 11, 252, 24))

	var scene *eui.ItemData
	s.viewport, scene = s.panel("02  /  LIVE CANVAS", 344, 136, 984, 350)
	head := eui.NewRow(label("Orbit garden", 22, 610, 36), label("LIVE  /  Ebitengine", 11, 300, 36))
	scene.AddItem(head)
	scene.AddItem(label("Tune the motion below. The UI and scene share the same game loop.", 12, 938, 28))
	imageItem, canvas := eui.NewImageItem(950, 228)
	s.canvas = canvas
	scene.AddItem(imageItem)

	_, controls := s.panel("03  /  SCENE CONTROLS", 344, 510, 478, 314)
	s.nameInput, _ = eui.NewInput()
	s.nameInput.Text = s.sceneName
	s.nameInput.Size = eui.Point{X: 438, Y: 32}
	s.nameInput.FontSize = 12
	s.nameInput.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			s.sceneName = ev.Text
		}
	}
	s.nameInput.SetTooltip("Rename the scene. Try selecting text or using the clipboard.")
	controls.AddItem(s.nameInput)
	slider, events := eui.NewSlider()
	slider.Label = "Orbit speed"
	slider.MinValue = 0
	slider.MaxValue = 2
	slider.Value = s.speed
	slider.Size = eui.Point{X: 438, Y: 32}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			s.speed = ev.Value
		}
	}
	controls.AddItem(slider)
	controls.AddItem(dropdown([]string{"12 particles", "24 particles", "48 particles", "72 particles"}, "48 particles", 438, func(v string) { fmt.Sscanf(v, "%d particles", &s.count); s.log(v + " in motion") }))
	opts := eui.NewRow()
	g := checkbox("Guide grid", s.grid, func(v bool) { s.grid = v })
	g.Size.X = 210
	opts.AddItem(g)
	tr := checkbox("Orbital trails", s.trails, func(v bool) { s.trails = v })
	tr.Size.X = 210
	opts.AddItem(tr)
	controls.AddItem(opts)
	var play *eui.ItemData
	play = button("Pause motion", 140, func() {
		s.playing = !s.playing
		if s.playing {
			play.Text = "Pause motion"
		} else {
			play.Text = "Resume motion"
		}
		play.ParentWindow.Refresh()
	})
	reset := button("Reset orbit", 140, func() { s.phase = 0; s.log("Orbit returned to its starting position") })
	info := button("Scene info", 140, func() {
		eui.ShowPopup("Your scene", fmt.Sprintf("%s\n\n%d particles at %.2fx speed.\n\nThis is a standard EUI dialog. Its text can be selected and copied.", s.sceneName, s.count, s.speed), []eui.PopupButton{{Text: "Back to studio"}})
	})
	controls.AddItem(eui.NewRow(play, reset, info))
	controls.AddItem(gap(6))
	s.progress, _ = eui.NewProgressBar()
	s.progress.Size = eui.Point{X: 438, Y: 12}
	s.progress.MinValue = 0
	s.progress.MaxValue = 100
	controls.AddItem(s.progress)

	_, notebook := s.panel("04  /  NOTEBOOK", 846, 510, 482, 314)
	tabs := eui.NewColumn()
	tabs.Fixed = true
	tabs.Size = eui.Point{X: 448, Y: 244}
	activity := eui.NewColumn()
	activity.Name = "Activity"
	activity.Size = eui.Point{X: 430, Y: 210}
	s.activity = label("", 12, 428, 198)
	s.activity.SelectableText = true
	activity.AddItem(s.activity)
	recipe := eui.NewColumn()
	recipe.Name = "Quick start"
	recipe.Size = eui.Point{X: 430, Y: 210}
	code := label("eui.Init()\n\nwin := eui.NewWindow()\nwin.Title = \"Hello, EUI\"\nwin.AddItem(eui.NewColumn(\n    eui.NewLabel(\"Ready to build.\"),\n))\nwin.MarkOpen()", 12, 428, 200)
	code.SelectableText = true
	recipe.AddItem(code)
	about := eui.NewColumn()
	about.Name = "About"
	about.Size = eui.Point{X: 430, Y: 210}
	about.AddItem(label("Small setup. Lots of possibilities.\n\nWindows, layouts, inputs, themes,\ndialogs, tooltips and color pickers.\n\nBuilt in Go. Rendered by Ebitengine.\nNo game assets. No external fonts.", 13, 428, 200))
	editorTab := eui.NewColumn()
	editorTab.Name = "Editor"
	editorTab.Size = eui.Point{X: 430, Y: 210}
	editorTab.AddItem(eui.NewWrappedLabel("Edit, select, undo, and try wrapping a long line.", 426))
	s.editor, _ = eui.NewTextArea()
	s.editor.Text = "// Your editable playground\npackage main\nfunc main() {\n    message := \"Hello, EUI!\"\n    println(message)\n}"
	s.editor.Size = eui.Point{X: 426, Y: 140}
	s.editor.FontSize = 12
	s.editor.AcceptTab = true
	s.editor.SetTextHighlighter(highlightGo)
	undo := button("Undo", 90, func() { s.editor.Undo() })
	redo := button("Redo", 90, func() { s.editor.Redo() })
	wrap := checkbox("Wrap lines", false, func(value bool) { s.editor.SetWordWrap(value) })
	wrap.Size.X = 140
	editorTab.AddItem(eui.NewRow(eui.NewSplitButton(undo, redo), wrap))
	editorTab.AddItem(s.editor)
	tabs.Tabs = []*eui.ItemData{activity, editorTab, recipe, about}
	s.notebookTabs = tabs
	for i, tab := range tabs.Tabs {
		if tab.Name == s.initialTab {
			tabs.ActiveTab = i
		}
	}
	notebook.AddItem(tabs)
	s.log("EUI Studio is ready")
	s.log("Try a palette, then make it your own")
	s.log("The canvas responds to every adjustment")
	return nil
}
func (s *studio) Update() error {
	if s.captured {
		return ebiten.Termination
	}
	s.tick++
	if s.playing {
		s.phase += float64(s.speed) / 60
	}
	if s.progress != nil && s.tick%4 == 0 {
		s.progress.Value = float32(math.Mod(s.phase*10, 100))
		s.progress.ParentWindow.Refresh()
	}
	return eui.Update()
}
func writeText(dst *ebiten.Image, str string, x, y, size float64, c color.Color, bold bool) {
	src := eui.FontSource()
	if bold {
		src = eui.BoldFontSource()
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, str, &text.GoTextFace{Source: src, Size: size}, op)
}
func (s *studio) Draw(screen *ebiten.Image) {
	bg := color.RGBA{R: 12, G: 17, B: 26, A: 255}
	fg := color.RGBA{R: 232, G: 238, B: 246, A: 255}
	muted := color.RGBA{R: 144, G: 159, B: 177, A: 255}
	if eui.IsLightTheme() {
		bg = color.RGBA{R: 228, G: 233, B: 238, A: 255}
		fg = color.RGBA{R: 26, G: 36, B: 47, A: 255}
		muted = color.RGBA{R: 80, G: 96, B: 110, A: 255}
	}
	screen.Fill(bg)
	accent := s.tint.ToRGBA()
	vector.FillRect(screen, 32, 38, 5, 53, accent, false)
	writeText(screen, "EUI", 50, 28, 46, accent, true)
	writeText(screen, "Studio", 150, 28, 46, fg, true)
	writeText(screen, "A playground for interfaces that feel at home in your game.", 52, 89, 15, muted, false)
	writeText(screen, "THE INTERFACE IS THE EXAMPLE", 1010, 47, 11, muted, true)
	writeText(screen, s.palette+"  /  "+s.style, 1010, 69, 15, fg, false)
	s.drawScene()
	s.viewport.Refresh()
	eui.Draw(screen)
	writeText(screen, "EUI  /  NATIVE GO UI", 34, 850, 11, accent, true)
	writeText(screen, "Change a theme. Move a slider. Build something.", 344, 850, 12, muted, false)
	writeText(screen, fmt.Sprintf("%02d PARTICLES   /   %.2fx SPEED", s.count, s.speed), 1050, 850, 11, muted, false)
	if s.screenshot != "" && s.tick >= 30 {
		f, err := os.Create(s.screenshot)
		if err != nil {
			log.Fatal(err)
		}
		if err = png.Encode(f, screen); err != nil {
			log.Fatal(err)
		}
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
		s.captured = true
	}
}
func (s *studio) drawScene() {
	dst := s.canvas
	dst.Fill(color.RGBA{R: 9, G: 17, B: 28, A: 255})
	if s.grid {
		for x := float32(0); x < 950; x += 32 {
			vector.FillRect(dst, x, 0, 1, 228, color.RGBA{R: 19, G: 32, B: 46, A: 255}, false)
		}
		for y := float32(0); y < 228; y += 32 {
			vector.FillRect(dst, 0, y, 950, 1, color.RGBA{R: 19, G: 32, B: 46, A: 255}, false)
		}
	}
	const cx, cy = 475.0, 114.0
	tint := s.tint.ToRGBA()
	for orbit := 0; orbit < 4; orbit++ {
		rx := float64(140 + orbit*60)
		ry := float64(25 + orbit*19)
		if s.trails {
			var path vector.Path
			for j := 0; j <= 120; j++ {
				a := float64(j) * 2 * math.Pi / 120
				x := float32(cx + rx*math.Cos(a))
				y := float32(cy + ry*math.Sin(a))
				if j == 0 {
					path.MoveTo(x, y)
				} else {
					path.LineTo(x, y)
				}
			}
			vector.StrokePath(dst, &path, &vector.StrokeOptions{Width: 1}, &vector.DrawPathOptions{ColorScale: colorScale(color.RGBA{R: 35, G: 58, B: 78, A: 255}), AntiAlias: true})
		}
	}
	for i := 0; i < s.count; i++ {
		ring := i % 4
		angle := float64(i)*2.39996 + s.phase*(0.35+float64(ring)*0.08)
		x := float32(cx + float64(140+ring*60)*math.Cos(angle))
		y := float32(cy + float64(25+ring*19)*math.Sin(angle))
		c := tint
		if i%3 == 0 {
			c = color.RGBA{R: 146, G: 213, B: 194, A: 255}
		}
		if i%7 == 0 {
			c = color.RGBA{R: 234, G: 186, B: 121, A: 255}
		}
		vector.FillCircle(dst, x, y, 8, color.RGBA{R: c.R / 8, G: c.G / 8, B: c.B / 8, A: 255}, true)
		vector.FillCircle(dst, x, y, 3.2, c, true)
	}
	vector.FillCircle(dst, cx, cy, 20, color.RGBA{R: 22, G: 48, B: 65, A: 255}, true)
	vector.FillCircle(dst, cx, cy, 8, tint, true)
	writeText(dst, s.sceneName, 18, 15, 13, color.RGBA{R: 196, G: 211, B: 225, A: 255}, false)
	writeText(dst, "PROCEDURAL / NO ASSETS", 18, 204, 10, color.RGBA{R: 107, G: 132, B: 154, A: 255}, false)
}
func colorScale(c color.Color) ebiten.ColorScale {
	var scale ebiten.ColorScale
	scale.ScaleWithColor(c)
	return scale
}
func (*studio) Layout(w, h int) (int, int) { return width, height }
func main() {
	screenshot := flag.String("screenshot", "", "write a PNG of the running example and exit")
	palette := flag.String("theme", "AccentDark", "initial palette")
	style := flag.String("style", "Default", "initial widget style")
	initialTab := flag.String("tab", "Activity", "initial notebook tab: Activity, Editor, Quick start, About")
	flag.Parse()
	s := &studio{speed: 0.75, count: 48, playing: true, grid: true, trails: true, sceneName: "Orbit garden / study 001", palette: *palette, style: *style, screenshot: *screenshot, initialTab: *initialTab}
	if err := s.build(); err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowTitle("EUI Studio — a UI playground")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(s); err != nil {
		log.Fatal(err)
	}
}

// Tokenize only this example's Go snippets; the toolkit accepts any highlighter.
func highlightGo(source string) []eui.TextColorSpan {
	colors := eui.CurrentSyntaxColors()
	files := token.NewFileSet()
	file := files.AddFile("playground.go", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, []byte(source), func(token.Position, string) {}, scanner.ScanComments)
	var spans []eui.TextColorSpan
	for {
		pos, tok, literal := lexer.Scan()
		if tok == token.EOF {
			break
		}
		var tint eui.Color
		switch {
		case tok == token.COMMENT:
			tint = colors.Comments
		case tok == token.STRING || tok == token.CHAR:
			tint = colors.Strings
		case tok.IsKeyword():
			tint = colors.Keywords
		case tok == token.INT || tok == token.FLOAT || tok == token.IMAG:
			tint = colors.Numbers
		default:
			continue
		}
		start := file.Offset(pos)
		end := min(len(source), start+len(literal))
		if literal == "" {
			end = min(len(source), start+len(tok.String()))
		}
		spans = append(spans, eui.TextColorSpan{Start: utf8.RuneCountInString(source[:start]), End: utf8.RuneCountInString(source[:end]), Color: tint})
	}
	return spans
}
