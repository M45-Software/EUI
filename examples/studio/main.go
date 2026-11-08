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
	ui                                                              *eui.Context
	tick                                                            int
	phase                                                           float64
	speed                                                           float32
	count                                                           int
	playing, grid, trails                                           bool
	sceneName                                                       string
	palette, style                                                  string
	tint                                                            eui.Color
	windows                                                         []*eui.WindowData
	viewport                                                        *eui.WindowData
	page, appearancePanel, scenePanel, controlsPanel, notebookPanel *eui.ItemData
	right, bottom, actions, editorTools                             *eui.ItemData
	imageItem                                                       *eui.ItemData
	layoutWidth, layoutHeight                                       int
	canvas                                                          *ebiten.Image
	activity, progress, nameInput                                   *eui.ItemData
	editor, notebookTabs                                            *eui.ItemData
	initialTab                                                      string
	paletteDD, styleDD, accent                                      *eui.ItemData
	messages                                                        []string
	screenshot                                                      string
	captured                                                        bool
}

func (s *studio) label(caption string, size float32, w, h float32) *eui.ItemData {
	t := s.ui.NewLabel(caption)
	t.FontSize, t.Size = size, eui.Point{X: w, Y: h}
	t.ConstrainToSize = true
	t.Position = eui.Point{}
	t.FillWidth = true
	return t
}
func (s *studio) gap(h float32) *eui.ItemData { return s.label("", 12, 1, h) }
func (s *studio) button(caption string, w float32, fn func()) *eui.ItemData {
	b := s.ui.NewActionButton(caption, fn)
	b.Size = eui.Point{X: w, Y: 32}
	b.Position = eui.Point{}
	b.FillWidth = true
	return b
}
func (s *studio) panel(title string, minimumHeight float32) (*eui.ItemData, *eui.ItemData) {
	root := s.ui.NewColumn()
	root.FillWidth = true
	root.MinSize.Y = minimumHeight
	root.ContentPadding = eui.Insets{Left: 14, Top: 10, Right: 14, Bottom: 14}
	root.Filled, root.Outlined = true, true
	root.Border = 1
	body := s.ui.NewColumn()
	body.FillWidth = true
	root.SetItems([]*eui.ItemData{s.label(title, 10, 0, 26), body})
	return root, body
}

// Retain the controls when breakpoints change so drafts, undo history,
// selections, and callbacks survive resizing.
func (s *studio) reflow(w, h int) {
	if s.page == nil || w == s.layoutWidth && h == s.layoutHeight {
		return
	}
	s.layoutWidth, s.layoutHeight = w, h
	margin, header, footer := float32(32), float32(136), float32(54)
	if w < 760 {
		margin, header, footer = 16, 116, 42
	}
	s.viewport.Size = eui.Point{X: max(1, float32(w)-2*margin), Y: max(1, float32(h)-header-footer)}
	s.viewport.SetPos(eui.Point{X: margin, Y: header})
	s.page.Gap = 24
	if w >= 1140 {
		s.page.FlowType = eui.FLOW_HORIZONTAL
		s.appearancePanel.FillWidth = false
		s.appearancePanel.Size.X = 288
		s.appearancePanel.MaxSize.X = 288
		s.right.Contents = []*eui.ItemData{s.scenePanel, s.bottom}
		s.bottom.FlowType = eui.FLOW_HORIZONTAL
		s.bottom.Contents = []*eui.ItemData{s.controlsPanel, s.notebookPanel}
		s.page.Contents = []*eui.ItemData{s.appearancePanel, s.right}
	} else if w >= 760 {
		s.page.FlowType = eui.FLOW_HORIZONTAL
		s.appearancePanel.FillWidth = false
		s.appearancePanel.Size.X = 260
		s.appearancePanel.MaxSize.X = 260
		s.bottom.Contents = nil
		s.right.Contents = []*eui.ItemData{s.scenePanel, s.controlsPanel, s.notebookPanel}
		s.page.Contents = []*eui.ItemData{s.appearancePanel, s.right}
	} else {
		s.page.FlowType = eui.FLOW_VERTICAL
		s.appearancePanel.FillWidth = true
		s.appearancePanel.Size.X = 0
		s.appearancePanel.MaxSize.X = 0
		s.right.Contents = nil
		s.bottom.Contents = nil
		s.page.Contents = []*eui.ItemData{s.scenePanel, s.controlsPanel, s.notebookPanel, s.appearancePanel}
	}
	// Compact controls change direction rather than shrinking their captions.
	controlWidth := float32(w) - 2*margin - 28
	if w >= 1140 {
		controlWidth = (float32(w)-2*margin-288-48)/2 - 28
	} else if w >= 760 {
		controlWidth = float32(w) - 2*margin - 260 - 24 - 28
	}
	s.actions.FlowType = eui.FLOW_HORIZONTAL
	s.editorTools.FlowType = eui.FLOW_HORIZONTAL
	if controlWidth < 400 {
		s.actions.FlowType = eui.FLOW_VERTICAL
	}
	if controlWidth < 350 {
		s.editorTools.FlowType = eui.FLOW_VERTICAL
	}
	// These nodes remain in the same workspace; rebinding the complete tree
	// avoids treating a breakpoint move as removing and destroying a control.
	var bind func(*eui.ItemData, *eui.ItemData)
	bind = func(item, parent *eui.ItemData) {
		item.Parent, item.ParentWindow = parent, s.viewport
		if item.Theme == nil {
			item.Theme = s.viewport.Theme
		}
		for _, child := range item.Contents {
			bind(child, item)
		}
		for _, tab := range item.Tabs {
			bind(tab, item)
		}
	}
	bind(s.page, nil)
	s.notebookTabs.Size.Y = 280
	if controlWidth < 350 {
		s.notebookTabs.Size.Y = 320
	}
	s.ui.SetScreenSize(w, h)
	s.viewport.Refresh()
	if focused := s.ui.FocusedItem(); focused != nil && focused.ParentWindow == s.viewport {
		focused.ScrollIntoView()
	}
}

func (s *studio) refreshPanels() {
	color, outline := s.viewport.Theme.Window.BGColor, s.viewport.Theme.Window.BorderColor
	for _, panel := range []*eui.ItemData{s.appearancePanel, s.scenePanel, s.controlsPanel, s.notebookPanel} {
		if panel != nil {
			panel.Color = color
			panel.OutlineColor = outline
		}
	}
	if s.viewport != nil {
		s.viewport.Refresh()
	}
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
	if err := s.ui.LoadTheme(name); err != nil {
		s.log(err.Error())
		return
	}
	s.palette = name
	// Palette and geometry are independent in this playground.
	if err := s.ui.LoadStyle(s.style); err != nil {
		s.log(err.Error())
	}
	s.tint = s.ui.AccentColor()
	if s.editor != nil {
		s.editor.RefreshTextHighlighting()
	}
	if s.accent != nil {
		eui.SetColorSwatch(s.accent, s.tint)
	}
	s.refreshPanels()
	s.log("Palette changed to " + name)
}
func (s *studio) dropdown(options []string, selected string, w float32, apply func(string)) *eui.ItemData {
	item, events := s.ui.NewDropdown()
	item.SetOptions(options)
	item.SetSelected(max(0, slices.Index(options, selected)))
	item.Size = eui.Point{X: w, Y: 32}
	item.Position = eui.Point{}
	item.FillWidth = true
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(item.Options) {
			apply(item.Options[ev.Index])
		}
	}
	return item
}
func (s *studio) checkbox(title string, checked bool, apply func(bool)) *eui.ItemData {
	item, events := s.ui.NewCheckbox()
	item.Text = title
	item.Checked = checked
	item.Size = eui.Point{X: 260, Y: 26}
	item.FontSize = 12
	item.Position = eui.Point{}
	item.FillWidth = true
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			apply(ev.Checked)
		}
	}
	return item
}
func (s *studio) build() error {
	if s.ui == nil {
		ui, err := eui.NewContext()
		if err != nil {
			return err
		}
		s.ui = ui
	}
	if err := s.ui.LoadTheme(s.palette); err != nil {
		return err
	}
	if err := s.ui.LoadStyle(s.style); err != nil {
		return err
	}
	s.ui.AutoHiDPI = false
	s.ui.SetScreenSize(width, height)
	s.tint = s.ui.AccentColor()
	s.viewport = s.ui.NewWindow()
	s.viewport.TitleHeight = 0
	s.viewport.Title = "EUI Studio"
	s.viewport.Closable, s.viewport.Movable, s.viewport.Resizable = false, false, false
	s.viewport.Docked = true
	s.viewport.Padding, s.viewport.BorderPad = 0, 0
	s.viewport.NoBGColor = true
	s.viewport.ShowTooltipIndicators = true
	s.page = s.ui.NewColumn()
	s.page.FillWidth = true
	s.viewport.AddItem(s.page)
	s.viewport.MarkOpen()
	s.windows = []*eui.WindowData{s.viewport}
	var appearance *eui.ItemData
	s.appearancePanel, appearance = s.panel("01  /  APPEARANCE", 688)
	appearance.AddItem(s.label("Make it yours.", 23, 252, 40))
	appearance.AddItem(s.label("Every control follows your theme.", 11, 252, 26))
	appearance.AddItem(s.gap(16))
	appearance.AddItem(s.label("COLOR PALETTE", 10, 252, 22))
	palettes, err := s.ui.ListThemes()
	if err != nil {
		return err
	}
	s.paletteDD = s.dropdown(palettes, s.palette, 248, s.theme)
	appearance.AddItem(s.paletteDD)
	appearance.AddItem(s.gap(12))
	// Quick picks make the palette family immediately discoverable.
	for _, names := range [][]string{{"AccentDark", "Paper"}, {"SeaGlass", "Coffee"}, {"NeonNight", "Arcade"}} {
		row := s.ui.NewRow()
		row.FillWidth = true
		row.Gap = 4
		for _, name := range names {
			row.AddItem(s.button(name, 120, func() { s.theme(name); s.paletteDD.SetSelected(slices.Index(palettes, name)) }))
		}
		appearance.AddItem(row)
	}
	appearance.AddItem(s.gap(18))
	appearance.AddItem(s.label("WIDGET STYLE", 10, 252, 22))
	styles, err := s.ui.ListStyles()
	if err != nil {
		return err
	}
	s.styleDD = s.dropdown(styles, s.style, 248, func(name string) {
		if err := s.ui.LoadStyle(name); err != nil {
			s.log(err.Error())
			return
		}
		s.style = name
		s.log("Style changed to " + name)
	})
	appearance.AddItem(s.styleDD)
	appearance.AddItem(s.gap(18))
	appearance.AddItem(s.label("ACCENT COLOR", 10, 252, 22))
	s.accent = s.ui.NewColorSwatch("Studio accent", s.tint, nil, func(c eui.Color) { s.tint = c; s.ui.SetAccentColor(c); s.log("Custom accent applied") })
	s.accent.Size = eui.Point{X: 248, Y: 32}
	s.accent.Position = eui.Point{}
	s.accent.FillWidth = true
	appearance.AddItem(s.accent)
	appearance.AddItem(s.gap(16))
	appearance.AddItem(s.checkbox("Window shadows", true, s.ui.SetWindowShadows))
	appearance.AddItem(s.gap(20))
	appearance.AddItem(s.label(fmt.Sprintf("%d palettes  /  %d styles", len(palettes), len(styles)), 12, 252, 26))
	appearance.AddItem(s.label("Embedded. Editable. Yours.", 11, 252, 24))

	var scene *eui.ItemData
	s.scenePanel, scene = s.panel("02  /  LIVE CANVAS", 350)
	head := s.ui.NewRow(s.label("Orbit garden", 22, 610, 36), s.label("LIVE / Ebitengine", 10, 120, 36))
	head.FillWidth = true
	head.Gap = 8
	head.Contents[1].FillWidth = false
	scene.AddItem(head)
	caption := s.ui.NewWrappedLabel("Tune the motion below. The UI and scene share the same game loop.", 938)
	caption.FillWidth = true
	caption.Position = eui.Point{}
	scene.AddItem(caption)
	scene.Gap = 8
	imageItem, canvas := s.ui.NewImageItem(950, 228)
	s.canvas = canvas
	s.imageItem = imageItem
	imageItem.FillWidth = true
	imageItem.Position = eui.Point{}
	scene.AddItem(imageItem)

	var controls *eui.ItemData
	s.controlsPanel, controls = s.panel("03  /  SCENE CONTROLS", 314)
	controls.Gap = 6
	s.nameInput, _ = s.ui.NewInput()
	s.nameInput.Text = s.sceneName
	s.nameInput.Size = eui.Point{X: 438, Y: 32}
	s.nameInput.Position = eui.Point{}
	s.nameInput.FillWidth = true
	s.nameInput.FontSize = 12
	s.nameInput.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			s.sceneName = ev.Text
		}
	}
	s.nameInput.SetTooltip("Rename the scene. Try selecting text or using the clipboard.")
	controls.AddItem(s.nameInput)
	slider, events := s.ui.NewSlider()
	slider.Label = "Orbit speed"
	slider.MinValue = 0
	slider.MaxValue = 2
	slider.Value = s.speed
	slider.Size = eui.Point{X: 438, Y: 32}
	slider.Position = eui.Point{}
	slider.FillWidth = true
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			s.speed = ev.Value
		}
	}
	controls.AddItem(slider)
	controls.AddItem(s.dropdown([]string{"12 particles", "24 particles", "48 particles", "72 particles"}, "48 particles", 438, func(v string) { fmt.Sscanf(v, "%d particles", &s.count); s.log(v + " in motion") }))
	opts := s.ui.NewRow()
	opts.FillWidth = true
	opts.Gap = 8
	g := s.checkbox("Guide grid", s.grid, func(v bool) { s.grid = v })
	g.Size.X = 210
	opts.AddItem(g)
	tr := s.checkbox("Orbital trails", s.trails, func(v bool) { s.trails = v })
	tr.Size.X = 210
	opts.AddItem(tr)
	controls.AddItem(opts)
	var play *eui.ItemData
	play = s.button("Pause motion", 140, func() {
		s.playing = !s.playing
		if s.playing {
			play.UpdateText("Pause motion")
		} else {
			play.UpdateText("Resume motion")
		}
	})
	reset := s.button("Reset orbit", 140, func() { s.phase = 0; s.log("Orbit returned to its starting position") })
	info := s.button("Scene info", 140, func() {
		s.ui.ShowPopup("Your scene", fmt.Sprintf("%s\n\n%d particles at %.2fx speed.\n\nThis is a standard EUI dialog. Its text can be selected and copied.", s.sceneName, s.count, s.speed), []eui.PopupButton{{Text: "Back to studio"}})
	})
	s.actions = s.ui.NewRow(play, reset, info)
	s.actions.FillWidth = true
	s.actions.Gap = 6
	controls.AddItem(s.actions)
	controls.AddItem(s.gap(6))
	s.progress, _ = s.ui.NewProgressBar()
	s.progress.Size = eui.Point{X: 438, Y: 12}
	s.progress.Position = eui.Point{}
	s.progress.FillWidth = true
	s.progress.MinValue = 0
	s.progress.MaxValue = 100
	controls.AddItem(s.progress)

	var notebook *eui.ItemData
	s.notebookPanel, notebook = s.panel("04  /  NOTEBOOK", 314)
	tabs := s.ui.NewColumn()
	tabs.Fixed = true
	tabs.Scrollable = true
	tabs.FillWidth = true
	tabs.Size = eui.Point{X: 448, Y: 244}
	activity := s.ui.NewColumn()
	activity.Name = "Activity"
	activity.Size = eui.Point{X: 430, Y: 210}
	s.activity = s.ui.NewWrappedLabel("", 428)
	s.activity.FillWidth = true
	s.activity.Position = eui.Point{}
	s.activity.MinSize.Y = 198
	s.activity.SelectableText = true
	activity.AddItem(s.activity)
	recipe := s.ui.NewColumn()
	recipe.Name = "Quick start"
	recipe.Size = eui.Point{X: 430, Y: 210}
	code := s.label("ui, err := eui.NewContext()\nif err != nil { return err }\ndefer ui.Dispose()\n\nwin := ui.NewWindow()\nwin.Title = \"Hello, EUI\"\nwin.AddItem(ui.NewColumn(\n    ui.NewLabel(\"Ready to build.\"),\n))\nwin.MarkOpen()", 12, 428, 200)
	code.SetWrappedText(code.Text)
	code.Position = eui.Point{}
	code.SelectableText = true
	recipe.AddItem(code)
	about := s.ui.NewColumn()
	about.Name = "About"
	about.Size = eui.Point{X: 430, Y: 210}
	aboutText := s.ui.NewWrappedLabel("Small setup. Lots of possibilities.\n\nWindows, layouts, inputs, themes,\ndialogs, tooltips and color pickers.\n\nBuilt in Go. Rendered by Ebitengine.\nNo game assets. No external fonts.", 428)
	aboutText.FillWidth = true
	aboutText.Position = eui.Point{}
	about.AddItem(aboutText)
	editorTab := s.ui.NewColumn()
	editorTab.Name = "Editor"
	editorTab.Size = eui.Point{X: 430, Y: 210}
	hint := s.ui.NewWrappedLabel("Edit, select, undo, and try wrapping a long line.", 426)
	hint.FillWidth = true
	hint.Position = eui.Point{}
	editorTab.AddItem(hint)
	editorTab.Gap = 6
	s.editor, _ = s.ui.NewTextArea()
	s.editor.Text = "// Your editable playground\npackage main\nfunc main() {\n    message := \"Hello, EUI!\"\n    println(message)\n}"
	s.editor.Size = eui.Point{X: 426, Y: 140}
	s.editor.FontSize = 12
	s.editor.Position = eui.Point{}
	s.editor.FillWidth = true
	s.editor.AcceptTab = true
	s.editor.SetTextHighlighter(s.highlightGo)
	undo := s.button("Undo", 90, func() { s.editor.Undo() })
	redo := s.button("Redo", 90, func() { s.editor.Redo() })
	wrap := s.checkbox("Wrap lines", false, func(value bool) { s.editor.SetWordWrap(value) })
	wrap.Size.X = 140
	tools := s.ui.NewSplitButton(undo, redo)
	tools.FillWidth = true
	s.editorTools = s.ui.NewRow(tools, wrap)
	s.editorTools.FillWidth = true
	s.editorTools.Gap = 6
	editorTab.AddItem(s.editorTools)
	editorTab.AddItem(s.editor)
	tabs.Tabs = []*eui.ItemData{activity, editorTab, recipe, about}
	s.notebookTabs = tabs
	for i, tab := range tabs.Tabs {
		if tab.Name == s.initialTab {
			tabs.ActiveTab = i
		}
	}
	notebook.AddItem(tabs)
	s.right = s.ui.NewColumn()
	s.right.FillWidth = true
	s.right.Gap = 24
	s.bottom = s.ui.NewRow()
	s.bottom.FillWidth = true
	s.bottom.Gap = 24
	s.reflow(width, height)
	s.refreshPanels()
	s.log("EUI Studio is ready")
	s.log("Try a palette, then make it your own")
	s.log("The canvas responds to every adjustment")
	if s.initialTab == "Editor" {
		s.ui.Focus(s.editor)
	}
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
		s.progress.SetValue(float32(math.Mod(s.phase*10, 100)))
	}
	return s.ui.Update()
}
func (s *studio) writeText(dst *ebiten.Image, str string, x, y, size float64, c color.Color, bold bool) {
	src := s.ui.FontSource()
	if bold {
		src = s.ui.BoldFontSource()
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
	if s.ui.IsLightTheme() {
		bg = color.RGBA{R: 228, G: 233, B: 238, A: 255}
		fg = color.RGBA{R: 26, G: 36, B: 47, A: 255}
		muted = color.RGBA{R: 80, G: 96, B: 110, A: 255}
	}
	screen.Fill(bg)
	accent := s.tint.ToRGBA()
	vector.FillRect(screen, 32, 38, 5, 53, accent, false)
	s.writeText(screen, "EUI", 50, 28, 46, accent, true)
	s.writeText(screen, "Studio", 150, 28, 46, fg, true)
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w >= 760 {
		s.writeText(screen, "A playground for interfaces that feel at home in your game.", 52, 89, 15, muted, false)
		if w >= 1140 {
			s.writeText(screen, "THE INTERFACE IS THE EXAMPLE", float64(w-350), 47, 11, muted, true)
			s.writeText(screen, s.palette+" / "+s.style, float64(w-350), 69, 15, fg, false)
		}
	} else {
		s.writeText(screen, s.palette+" / "+s.style, 32, 89, 13, muted, false)
	}
	canvasSize := s.imageItem.GetSize()
	cw, ch := max(1, int(canvasSize.X)), max(1, int(canvasSize.Y))
	if s.canvas.Bounds().Dx() != cw || s.canvas.Bounds().Dy() != ch {
		previous := s.canvas
		s.canvas = ebiten.NewImage(cw, ch)
		s.imageItem.Image = s.canvas
		previous.Deallocate()
	}
	s.drawScene()
	s.viewport.Refresh()
	s.ui.Draw(screen)
	s.writeText(screen, "EUI / NATIVE GO UI", 24, float64(h-30), 10, accent, true)
	if w >= 760 {
		s.writeText(screen, "Tab selects controls. Enter activates. Scroll for more.", 220, float64(h-30), 11, muted, false)
	}
	if w >= 1140 {
		s.writeText(screen, fmt.Sprintf("%02d PARTICLES / %.2fx SPEED", s.count, s.speed), float64(w-300), float64(h-30), 11, muted, false)
	}
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
	cw, ch := float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())
	sx, sy := float64(cw)/950, float64(ch)/228
	dst.Fill(color.RGBA{R: 9, G: 17, B: 28, A: 255})
	if s.grid {
		for x := float32(0); x < cw; x += 32 {
			vector.FillRect(dst, x, 0, 1, ch, color.RGBA{R: 19, G: 32, B: 46, A: 255}, false)
		}
		for y := float32(0); y < ch; y += 32 {
			vector.FillRect(dst, 0, y, cw, 1, color.RGBA{R: 19, G: 32, B: 46, A: 255}, false)
		}
	}
	cx, cy := float64(cw)/2, float64(ch)/2
	tint := s.tint.ToRGBA()
	for orbit := 0; orbit < 4; orbit++ {
		rx := float64(140+orbit*60) * sx
		ry := float64(25+orbit*19) * sy
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
		x := float32(cx + float64(140+ring*60)*sx*math.Cos(angle))
		y := float32(cy + float64(25+ring*19)*sy*math.Sin(angle))
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
	vector.FillCircle(dst, float32(cx), float32(cy), 20, color.RGBA{R: 22, G: 48, B: 65, A: 255}, true)
	vector.FillCircle(dst, float32(cx), float32(cy), 8, tint, true)
	s.writeText(dst, s.sceneName, 18, 15, 13, color.RGBA{R: 196, G: 211, B: 225, A: 255}, false)
	s.writeText(dst, "PROCEDURAL / NO ASSETS", 18, float64(ch)-24, 10, color.RGBA{R: 107, G: 132, B: 154, A: 255}, false)
}
func colorScale(c color.Color) ebiten.ColorScale {
	var scale ebiten.ColorScale
	scale.ScaleWithColor(c)
	return scale
}
func (s *studio) Layout(w, h int) (int, int) {
	w, h = max(360, w), max(480, h)
	s.reflow(w, h)
	return w, h
}
func main() {
	screenshot := flag.String("screenshot", "", "write a PNG of the running example and exit")
	palette := flag.String("theme", "AccentDark", "initial palette")
	style := flag.String("style", "Default", "initial widget style")
	windowWidth := flag.Int("width", width, "initial window width")
	windowHeight := flag.Int("height", height, "initial window height")
	initialTab := flag.String("tab", "Activity", "initial notebook tab: Activity, Editor, Quick start, About")
	flag.Parse()
	s := &studio{speed: 0.75, count: 48, playing: true, grid: true, trails: true, sceneName: "Orbit garden / study 001", palette: *palette, style: *style, screenshot: *screenshot, initialTab: *initialTab}
	if err := s.build(); err != nil {
		log.Fatal(err)
	}
	defer s.ui.Dispose()
	defer s.canvas.Deallocate()
	ebiten.SetWindowSize(max(360, *windowWidth), max(480, *windowHeight))
	ebiten.SetWindowSizeLimits(360, 480, -1, -1)
	ebiten.SetWindowTitle("EUI Studio — a UI playground")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(s); err != nil {
		log.Fatal(err)
	}
}

// Tokenize only this example's Go snippets; the toolkit accepts any highlighter.
func (s *studio) highlightGo(source string) []eui.TextColorSpan {
	colors := s.ui.CurrentSyntaxColors()
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
