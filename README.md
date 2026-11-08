# EUI

**A native Go UI toolkit for Ebitengine. Small setup. Lots of possibilities.**

Build game settings, editors, debug tools, dashboards, and in-game interfaces with
real windows and widgets. Fonts, palettes, and styles are embedded: no asset
bundle, browser, or separate renderer to set up.

![EUI Studio: a working playground with live theme controls, an animated orbital scene, inputs, sliders, and a tabbed notebook](docs/studio.png)

*EUI Studio is a real application built with the library. Every control works.
The canvas is drawn procedurally with Ebitengine; the screenshot comes directly
from the running program.*

## Try the studio

Requires **Go 1.27.1**. EUI targets **Ebitengine 2.10.2**; desktop builds support
`CGO_ENABLED=0` and need the usual display/graphics runtime libraries.
This repository is private; use a GitHub account with access.

```sh
gh repo clone Distortions81/EUI
cd EUI
go run ./examples/studio
```

- Switch among **12 palettes and 6 styles** without restarting.
- Pick a custom accent with the HSV and opacity editor.
- Change particle count, orbit speed, grid, and trails in the live canvas.
- Pause, resume, reset, and rename the scene.
- Open a dialog and select its text; explore the notebook's tabs.
- Edit highlighted Go text in the **Editor** tab, undo/redo, and toggle word wrapping.

Try a different starting look:

```sh
go run ./examples/studio -theme Paper -style Rounded
```

<details>
<summary>See the light theme</summary>

![EUI Studio using the Paper palette and Rounded style](docs/studio-light.png)

</details>

The studio uses a 1360 × 900 logical canvas, scaled to fit the desktop window.
For a minimal application that follows the window's dimensions and display
scale, see [`examples/basic`](examples/basic/main.go).

## Add EUI to your game

Configure authenticated Git access for the private repository, then add it to
an existing Go module:

```sh
GOPRIVATE=github.com/Distortions81/EUI go get github.com/Distortions81/EUI
```

Import the package as `eui`:

```go
import eui "github.com/Distortions81/EUI"
```

Initialize once and compose your UI:

```go
if err := eui.Init(); err != nil {
    return err
}
win := eui.NewWindow()
win.Title = "Tools"
win.AutoSize, win.Movable, win.Closable = true, true, true
win.AddItem(eui.NewColumn(
    eui.NewLabel("Ready to build."),
    eui.NewActionButton("Say hello", func() {
        eui.ShowPopup("Hello", "Your UI is running.", []eui.PopupButton{
            {Text: "Great!"},
        })
    }),
))
win.MarkOpen()
```

Connect EUI to your `ebiten.Game`:

```go
func (g *Game) Update() error {
    return eui.Update()
}

func (g *Game) Draw(screen *ebiten.Image) {
    // Draw your game here, then the UI on top.
    eui.Draw(screen)
}

func (g *Game) Layout(w, h int) (int, int) {
    return eui.Layout(w, h)
}
```

That is the entire integration. See the [complete basic program](examples/basic/main.go)
for the `main` function and window setup.

## What is included

| Area | Features |
| --- | --- |
| Layout | Rows, columns, overlays, wrapped labels, tabs, scrolling, content sizing |
| Windows | Movement, resizing, snapping, search, shadows, cached rendering |
| Controls | Buttons, split buttons, checkboxes, radio buttons, sliders, dropdowns, progress bars |
| Text | Single/multiline editing, undo/redo, search, highlighting, wrapping, cached text windows |
| Dialogs | Message/action popups, color pickers, context menus, tooltips |
| Appearance | Embedded fonts, 12 palettes, 6 styles, live accents, display scaling |
| Rendering | Ebitengine images, reusable render targets, repaint coalescing |

Widget constructors return an item and an event handler when you need detailed
control. The convenience builders (`NewColumn`, `NewRow`, `NewLabel`, and
`NewActionButton`) keep common layouts short. Set `Size` for explicit dimensions,
and use `Fixed` / `Scrollable` for bounded viewports. Set `ConstrainToSize` when
a non-scrollable control must stay within its declared size. `NewWrappedLabel`
reflows paragraphs to a logical width, and `LayoutWindowBody` reserves space for
fixed controls around a scrolling body.

## Editing text

Use `NewInput` for a single line, or `NewTextArea` for a scrolling document:

```go
editor, events := eui.NewTextArea()
editor.Size = eui.Point{X: 480, Y: 260}
editor.Text = "First line\nSecond line"
editor.AcceptTab = true
editor.SetWordWrap(true)
events.Handle = func(event eui.UIEvent) {
    if event.Type == eui.EventInputChanged {
        draft = event.Text
    }
}
win.AddItem(editor)
```

Both support selection replacement, word/line navigation, held-key repeat,
grapheme-aware deletion, and bounded undo/redo history. Use Ctrl+A/X/C/V/Z on
Windows and Linux, or Command on macOS. Password fields do not copy, cut, or
retain undo history. Multiline controls scroll the caret into view; optional
word wrapping changes the display without modifying the source text.

`ReplaceText` applies a transformation as one undoable edit. `Undo`, `Redo`,
`CanUndo`, and `CanRedo` support toolbar actions. `FindText` selects and reveals
a literal match. `SetTextHighlighter` accepts rune-indexed `TextColorSpan` ranges;
`CurrentSyntaxColors` supplies the active palette's editor colors. Call
`RefreshTextHighlighting` when your highlighter's external inputs change.
`SetTextColorSwatches` adds clickable color previews over source ranges.

For application-owned chat history and completion, set `ExternalTextEditing`.
Use `FocusedTextInput` to reserve editing keys from game shortcuts. IME
composition UI and file persistence are supplied by the embedding application.
See Studio's **Editor** tab for a complete example:

```sh
go run ./examples/studio -tab Editor
```

![Studio editor with highlighted Go text and undo/redo controls](docs/studio-editor.png)

## Updating controls

Use `UpdateText` for label changes: it repaints and reflows the containing
window. Direct field assignments remain available; call `win.Refresh()` after
changing layout or appearance. `SetWrappedText` preserves original paragraphs
when updating a wrapped label. Call `SetWordWrap` to change editor wrapping.

`BeforeClose` may return false to keep a window open while resolving unsaved
changes. Closing clears its input state and releases cached render targets,
including tab contents. `RemoveWindow` retires it; detached items release their
cached targets and can be attached again. Images supplied by the application
remain the application's responsibility.

## Themes included

**Palettes:** AccentDark, AccentLight, Arcade, Candy, Coffee, Dusk, Forest,
HighContrast, Midnight, NeonNight, Paper, SeaGlass.

**Styles:** Borderless, Default, Flat, HighContrast, Outline, Rounded.

Example JSON templates are included alongside the built-ins. All original
palettes and styles from goThoom are preserved in [`themes`](themes/README.md).
Colors and widget geometry are independent:

```go
if err := eui.LoadTheme("SeaGlass"); err != nil {
    return err
}
if err := eui.LoadStyle("Rounded"); err != nil {
    return err
}
```

Call `SetUserDataRoot` to choose where editable themes live; it creates example
files there. Enable `AutoReload` while designing a theme. EUI does not change the
process working directory. `Init` loads embedded Go regular/bold fonts while
preserving custom fonts installed earlier with `SetFontSource` and
`SetBoldFontSource`.

## A few useful boundaries

- EUI currently has **one global UI context**. Mutate UI state on the game thread.
- `Close` hides ordinary windows for reuse; `RemoveWindow` retires them. Popup and
  color-picker windows remove themselves on close.
- Popups are floating dialogs; they do not block interaction with other windows.
- `NewTextWindow` supplies a scrolling text view and optional editable display.
  Use `NewInput` for a standalone input control. Application text-window editing,
  URL actions, and annotation policies are supplied by your own callbacks.
- Most dimensions use logical UI units. `eui.Layout` handles display scaling;
  custom framebuffer layouts should set EUI's screen size and scale explicitly.

## Develop and verify

On Debian/Ubuntu, install graphics runtime and headless test dependencies:

```sh
sudo apt-get install libgl1 libx11-6 libxcursor1 libxi6 libxrandr2 libxxf86vm1 xvfb
```

With a desktop display:

```sh
CGO_ENABLED=0 go test -tags 'integration test' ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
```

On headless Linux, prefix test or example commands with `xvfb-run -a`.
GitHub Actions runs the tagged tests, vet, build, Studio screenshots, and the
text-editor rendering regression. Render tests that invoke Ebitengine run in
separate processes. To check editor pixels locally:

```sh
EUI_TEXT_EDIT_CAPTURE=/tmp/eui-editor.png xvfb-run -a go test -run '^TestRenderTextEditing$' .
```

Regenerate the README images from the real application:

```sh
xvfb-run -a go run ./examples/studio -screenshot docs/studio.png
xvfb-run -a go run ./examples/studio -theme Paper -style Rounded -screenshot docs/studio-light.png
xvfb-run -a go run ./examples/studio -tab Editor -screenshot docs/studio-editor.png
```

The screenshot option writes one frame and exits. No image editor is needed.
More focused theme visual checks are documented in [`themes/README.md`](themes/README.md).

For testing a local library change in goThoom and switching the client to a
pinned standalone dependency, see [client integration](docs/client-integration.md).

## Origin and license

EUI was extracted from [goThoom](https://github.com/Distortions81/goThoom), where it
was developed for a real desktop game client. This repository is independent;
The standalone library also includes the reusable editor, layout, and rendering
work developed in that client. Client integration is checked against a local
module replacement before pinning a published EUI revision.

[MIT licensed](LICENSE). No Clan Lord game assets are included.
