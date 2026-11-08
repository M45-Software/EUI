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

Studio follows the window dimensions. At desktop widths it uses a dashboard;
medium windows stack the scene and notebook beside appearance controls; narrow
windows use one scrolling column. Controls, editor drafts, history, and selection
survive breakpoint changes. The supported minimum window is 360 × 480.

```sh
go run ./examples/studio -width 480 -height 800
```

<details>
<summary>See the narrow layout</summary>

![Studio in a narrow scrolling window](docs/studio-narrow.png)

</details>

For a minimal application that follows the window dimensions and display scale,
see [`examples/basic`](examples/basic/main.go).

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
ui, err := eui.NewContext()
if err != nil { return err }
defer ui.Dispose()
win := ui.NewWindow()
win.Title = "Tools"
win.AutoSize, win.Movable, win.Closable = true, true, true
win.AddItem(ui.NewColumn(
    ui.NewLabel("Ready to build."),
    ui.NewActionButton("Say hello", func() {
        ui.ShowPopup("Hello", "Your UI is running.", []eui.PopupButton{
            {Text: "Great!"},
        })
    }),
))
win.MarkOpen()
```

Store `ui` on your game, then connect it to `ebiten.Game`:

```go
type Game struct { UI *eui.Context }

func (g *Game) Update() error {
    return g.UI.Update()
}

func (g *Game) Draw(screen *ebiten.Image) {
    // Draw your game here, then the UI on top.
    g.UI.Draw(screen)
}

func (g *Game) Layout(w, h int) (int, int) {
    return g.UI.Layout(w, h)
}
```

That is the entire integration. See the [complete basic program](examples/basic/main.go)
for the `main` function and window setup.

## Independent UI instances

Create a `Context` with `eui.NewContext()`. Constructors, input, focus, scale,
fonts, palettes/styles, file-watch paths, menus, callbacks and renderer caches
belong to that instance. There is no selected or default instance. A callback
may update another context without changing the caller's state.

```go
hud, err := eui.NewContext()
if err != nil { return err }
defer hud.Dispose()
settings, err := eui.NewContext()
if err != nil { return err }
defer settings.Dispose()
settings.LoadTheme("Paper")
settings.SetInputEnabled(false) // draw it while routing native input to HUD
```

Use instance constructors such as `ui.NewWindow()` and `ui.NewInput()`. For
custom controls use `ui.NewItem(eui.ItemData{ItemType: eui.ITEM_FLOW})`, or attach
unowned configuration to an owned parent before using context-dependent methods.
A control retains its context after detaching. Mixing controls, windows or owned
handlers from different contexts panics before a tree is changed. Types, enums,
color literals and pure helpers remain in the package.

Route input with `ui.SetInputEnabled` or each instance's `ui.SetInputSource`.
Disabled instances keep focus and values, update animations, and draw. Native
Ebitengine devices and its one OS text-input session are physical shared
resources: enable native input on one instance at a time. Custom sources can
route separate coordinate spaces and input streams to multiple instances.
System clipboard access is also shared by the OS; `ui.SetClipboard` can provide
an isolated adapter. These resource boundaries do not share UI state.

`Dispose()` closes owned event queues, cancels preedit, releases cached images
and pools, and retires all live owned handles, including detached controls.
It is idempotent, emits no application callbacks, and keeps application-owned
images intact. Calls during Update/Draw defer resource release until that call
finishes. A disposed `Update()` returns `eui.ErrContextDisposed`; `Draw()` does
nothing. Other context operations and context-dependent handle methods panic
after disposal. Do not copy a context or use its zero value; use NewContext.
Calls on the same context belong on the game goroutine. Recursive Update on that
same context returns `eui.ErrContextReentry`; independent contexts may be called
from callbacks. See [ownership and embedding](docs/client-integration.md).

![Two independently owned EUI contexts](docs/contexts.png)

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

## Responsive layout

`Size` is a preferred logical size. Layout keeps it intact; `GetSize()` reports
the resolved pixel footprint. Zero dimensions use measured content. Natural
flows and captions can grow for content; `Fixed` / `ConstrainToSize` keep a
specified box, and `MinSize` / `MaxSize` constrain the final footprint, including
field labels. Zero limits mean unbounded; a conflicting minimum takes precedence.

| Setting | Behavior |
| --- | --- |
| `MinSize`, `MaxSize` | Logical lower and upper limits on each dimension |
| `FillWidth`, `FillHeight` | Consume available space within those limits |
| `Gap` | Logical space between visible flow children, with no trailing gap |
| `ContentPadding` | Logical `Insets{Left, Top, Right, Bottom}` inside a flow |
| `MainAlign` | Pack the child group at `LayoutStart`, `LayoutCenter`, or `LayoutEnd` along the flow |
| `CrossAlign` | Align individual children across the flow with the same constants |

In a row, width-filling children share the space left after non-filling children,
gaps, and padding. Columns share height in the same way. Capped shares are
redistributed, and minima are preserved even if the viewport is too small. Across
the flow, fill consumes its usable width or height; centered/end alignment still
applies if a maximum prevents filling it. Overlays fill each child independently.
Position adds an inset and participates in measurement. Hidden children contribute
neither size nor gap. Scrolling areas reserve scrollbar space, and tab content
starts below the measured tab strip.

```go
left := ui.NewColumn(ui.NewLabel("Navigation"))
left.Size.X = 180
left.MaxSize.X = 220
right := ui.NewColumn(ui.NewWrappedLabel("Content follows the viewport.", 400))
right.FillWidth = true
right.Contents[0].FillWidth = true
row := ui.NewRow(left, right)
row.FillWidth = true
row.Gap = 16
row.ContentPadding = eui.Insets{Left: 12, Top: 12, Right: 12, Bottom: 12}
win.AddItem(row)
```

For a scrolling body between fixed rows, use a fixed/filling column and give its
body `FillHeight`, `Fixed`, and `Scrollable`. `LayoutWindowBody` also handles this
pattern and reserves flow gaps and padding. Wrapped labels reflow at their
allocated width, so following rows move with their changed height.

Use `SetSize`, `SetSizeLimits`, `SetFill`, `SetFlowLayout`, and
`SetContentPadding` for runtime changes with automatic reflow. Direct field edits
followed by `win.Refresh()` can batch a structural layout update. Choose row or
column arrangements at application-specific breakpoints, as Studio does, rather
than shrinking every control. `ScrollIntoView()` reveals a control without
changing its focus, caret, or selection.

## Testing interactions and replay

`SetInputSource` accepts a function returning one `InputFrame` per `Update`.
Native Ebitengine input uses the same transition tracking as replayed frames.
Supply all held keys/buttons/contacts, plus only the text and wheel deltas for
that tick. Coordinates are screen pixels; keep the frame clock nonzero and
nondecreasing. Zero/backward times or non-finite wheel deltas return an error
from `Update` before changing UI state. Invalid key/button codes and duplicate
contact IDs are ignored. The slices are consumed without being retained or
modified.

```go
frame := eui.InputFrame{Time: time.Unix(1, 0), PointerX: 100, PointerY: 80}
previous := ui.SetInputSource(func() eui.InputFrame { return frame })
defer ui.SetInputSource(previous)

// Press, then release on the next game tick.
frame.MouseButtons = []ebiten.MouseButton{ebiten.MouseButton0}
if err := ui.Update(); err != nil { return err }
frame.Time = frame.Time.Add(time.Second / 60)
frame.MouseButtons = nil
if err := ui.Update(); err != nil { return err }
```

Held-key repeats count `Update` ticks. Clock advancement controls double clicks,
caret blinking, click flashes, repaint scheduling, and browser wheel throttling;
it does not synthesize skipped repeat ticks. `Draw` uses the last replayed time
without polling the source again. Native rendering still uses wall time, as do
profiling, display-scale polling, and file-watch checks. Set `Mac` on a replay
frame to exercise Command/Option shortcuts on another host platform.

A sole touch is the primary pointer. Two or more contacts end primary capture
and scroll with the two lowest IDs. Switching scroll contacts resets the gesture
baseline, and returning to a remaining single contact does not synthesize a new
click. Replacing a primary contact completes its old capture before a fresh
contact can press. `PointerPosition` reads current native device positions or the
latest custom-source sample. UI handlers use the single frame latched by Update,
so host pointer queries do not repoll the UI's source.

Change sources between updates on the UI goroutine. `SetInputSource(nil)` restores
the devices. Switching sources cancels gestures and menus and resets clock/held
input history, preserving focus, selection, values, and text history. Preview
callbacks can run when menus are dismissed, as with other preview cancellation.

`SetClipboard` accepts a `Clipboard` implementation with `ReadText` and
`WriteText` methods. It returns the previous implementation; nil uses the system
clipboard. This lets replay tests exercise copy, failed cut/paste, and undo/redo
without reading or changing the host clipboard.

The normal suite drives complete sequences through `Update` and `Draw`, covering
mouse/touch capture, key repeat, window gestures, dividers, search, radio groups,
tab pages, dropdown previews, reentrant callbacks, text editing, platform
modifiers, and timing. Run it with a display available (Xvfb on Linux):

```sh
CGO_ENABLED=0 xvfb-run -a go test -tags 'integration test' ./...
EUI_INTERACTION_CAPTURE=/tmp/replay.png CGO_ENABLED=0 xvfb-run -a \
  go test -run '^TestRenderInteractionReplay$' .
```

Linux CI includes pixel regressions; Windows and macOS jobs run the suite, vet,
build, and a native Studio editor smoke render, retaining its PNG as an artifact.

## Event delivery

Callbacks are the default. Widget constructors return a handler with no queue;
assign `Handle` for detailed events, or use `NewActionButton` for a click action.
Callbacks run synchronously on the EUI/game goroutine. Keep them short and apply
UI changes on that goroutine.

```go
slider, events := ui.NewSlider()
events.Handle = func(event eui.UIEvent) {
    if event.Type == eui.EventSliderChanged {
        volume = event.Value
    }
}
```

Each interaction updates widget state first, queues a value snapshot if enabled,
calls `Handle`, then calls the control's convenience callback:

| Interaction | Event | Convenience callback after `Handle` |
| --- | --- | --- |
| Button activation | `EventClick` | `Action` |
| Checkbox toggle | `EventCheckboxChanged` | `Action` |
| Radio activation | `EventRadioSelected` | `Action` |
| Changed slider value | `EventSliderChanged` | `Action` |
| End of a changed slider interaction | `EventSliderReleased` | None |
| Dropdown or context-menu choice | `EventDropdownSelected` | `OnSelect(index)` |
| Changed wheel color | `EventColorChanged` | `OnColorChange(color)`, then `Action` |
| Text edit, undo, or redo | `EventInputChanged` | None |

Pointer and keyboard input use this same order. Radio peers are updated before
notification without separate events. Sliders and color wheels emit change
events only when their value changes. Dropdown previews end before selection
notification; opening, previewing, or canceling a menu does not emit a selection.
A nil `OnColorChange` applies the color to the accent. Programmatic setters are
silent, as described under Updating controls.

Callbacks and payloads are captured for each emitted event. Replacing a callback
inside `Handle` affects the next interaction; changing a control does not rewrite
the current event's payload. `UIEvent.Item` is a live handle, so use the payload
fields when retaining an event. Password input events contain masked text.
Recursive interactions run immediately: the nested callbacks complete before the
outer callback returns, while each handler's queue retains emit order. A callback may remove its
control, close its window, or open a follow-up menu.

To consume events later, explicitly enable a bounded queue:

```go
pending := events.EnableQueue(64)
// During your game Update, drain without blocking:
drainEvents:
for {
    select {
    case event, ok := <-pending:
        if !ok {
            break drainEvents
        }
        consume(event)
    default:
        break drainEvents
    }
}
// Check events.DroppedEvents() to detect overflow and resynchronize if needed.
// Call events.DisableQueue() when finished.
```

Queue delivery is FIFO and nonblocking. A full queue drops the newest event and
increments `DroppedEvents()`; `Handle` and convenience callbacks still run.
Choose capacity for the expected burst and drain regularly. `EnableQueue` requires
a positive capacity, closes any previous channel, and resets the dropped count.
`DisableQueue` closes the channel and retains the count. Closed channels can still
be drained; use the receive `ok` result or range over them rather than receiving
forever. `Events()` returns the current receive-only channel, or nil when disabled.
EUI owns channel closure. Configure queues, emit, and read counters on the game
goroutine; a worker may receive payload snapshots but must not access `Item` or
other UI state there. Queue and callback delivery are both enabled when requested;
using both to perform the same side effect will perform it twice.

## Editing text

Use `NewInput` for a single line, or `NewTextArea` for a scrolling document:

```go
editor, events := ui.NewTextArea()
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
Use `FocusedTextInput` to reserve editing keys from game shortcuts.
Application-owned editors manage their own composition and persistence.
See Studio's **Editor** tab for a complete example:

```sh
go run ./examples/studio -tab Editor
```

![Studio editor with highlighted Go text and undo/redo controls](docs/studio-editor.png)

## Measuring larger interfaces

Use `build-scripts/benchmark-large-ui.sh /tmp/eui-benchmarks` to measure 1,000
and 10,000-row lists and documents, including scrolling, resizing, edits,
wrapping, and theme changes. It records timings, allocations, and environment
information. Set `BENCHTIME` and `COUNT` to control sample duration and repeats.
Frame benchmarks run inside Ebitengine; GPU readback is excluded from the timer
and flushes queued work between iterations. Their results describe CPU frame
preparation, rather than presented FPS. See [the measurements and remaining
limits](docs/performance.md) for the workload and recorded before/after results.

Editor lookups cache document rune counts, caret lookup uses the sorted visual
rows, and paints visit only viewport rows. Width-only reflow reuses parsed
physical lines; cold document changes still rebuild layout. Plain text uses a
bounded raster cache with reusable backing allocations: at most 8 MiB of active
allocations, 256 entries, and a separate 1 MiB idle reserve. Generic flow lists
still retain and traverse every item. Keep application history bounded; measure
before choosing a purpose-built virtual list for larger datasets.

## IME composition

Internally edited inputs, text areas, passwords, and titlebar search use
Ebitengine's native text-input composer. Preedit is underlined inline, with a
composition selection and caret; it remains separate from `Text`, bindings,
change events, and undo history. A commit replaces the original selection as one
undoable edit and emits one change notification. Password preedit is masked,
existing secrets are omitted from native surrounding-text context, and passwords
retain no undo history. Search callbacks receive committed queries only.

IME-owned keys are processed before shortcuts, navigation, and default buttons.
An Enter that confirms composition does not also submit the form. Ordinary text
is delivered once through the composer. Host shortcuts should continue to respect
`FocusedTextInput()` and `SearchActive()`.

`IsComposing()` reports inline preedit; `CancelComposition()` discards it.
Focus loss, host keyboard capture, hiding/disabling/removing the control, source
replacement, or external changes to its document, selection, wrapping, or geometry
cancel preedit while preserving committed text. The next native session uses the
new caret position. Ebitengine's current public composer API fixes candidate
bounds at session creation, so moving/resizing/scaling or scrolling the control
restarts composition rather than leaving the candidate window at an old position.
`TextInputBounds()` computes the visible caret anchor in logical screen pixels,
including nested layout, labels, wrapping, and scrolling.

For custom sources, supply ordered `InputFrame.TextInput` notifications using
`TextInputCompose`, `TextInputCommit`, `TextInputCancel`, or `TextInputEnd`.
Composition selection and replacement offsets count runes. `TextReplacement`
provides an explicit document range and caret for reconversion; `Target` rejects
notifications intended for a different focused control. Notifications take
precedence over `InputFrame.Text`; active preedit suppresses ordinary keys.
`Passthrough` permits the triggering key after a notification. `TextInputEnd`
stops editing after any preceding commit; `Unfocused` suspends keyboard input and
cancels preedit while retaining control focus for the host's return.

```go
frame.TextInput = []eui.TextInputEvent{{
    Kind: eui.TextInputCompose, Text: "にほん", SelectionEnd: 3,
}}
// On a later tick, replace preedit with the chosen text:
frame.TextInput = []eui.TextInputEvent{{Kind: eui.TextInputCommit, Text: "日本"}}
```

Native behavior follows the platform backend: some IMEs draw their own preedit
and report only commits. Install a font covering the entered language with
`SetFontSource`; the bundled Go fonts do not provide full CJK coverage. Linux X11
preedit/commit/cancel/focus transfer have been verified through an isolated IBus
test engine. Windows and macOS adapters compile and have CI coverage, but real
language-IME sessions on those systems still need validation.

## Keyboard navigation

Tab and Shift+Tab move through enabled, visible controls in layout order.
F6 and Shift+F6 cycle through interactive windows in registration order.
Keyboard focus has a contrasting outline and scrolls into view, including in
nested scrolling panels. Hidden controls, disabled subtrees, and inactive tab
pages are excluded.

| Control | Keys |
| --- | --- |
| Button or checkbox | Enter or Space activates/toggles |
| Radio group | One Tab stop; arrow keys move and select within the group |
| Slider | Arrows adjust; Shift takes larger steps; Ctrl/Command takes finer floating-point steps; Page Up/Down, Home/End reach farther or the limits |
| Dropdown or context menu | Up/Down, Home/End navigate; Enter/Space confirms; Escape cancels; Tab dismisses |
| Tab strip | Arrows or Home/End select an enabled page; Tab enters its controls |
| Color wheel | Left/Right change hue; Up/Down saturation; Page Up/Down brightness; Shift takes larger steps |

Editors retain their text-navigation and indentation shortcuts. Single-line
Enter can activate the window's default button. Multiline Enter inserts a newline.

`Focus(item)` focuses and reveals a control; `FocusedItem()` reports general
control focus, while `FocusedTextInput()` reports text editing only.
`SetKeyboardInputCaptured(true)` reserves keyboard handling for the host while
pointer controls remain available. Slider keyboard adjustments emit
`EventSliderChanged` as the value changes and one `EventSliderReleased` when the
adjustment keys are released or focus moves away.

## Updating controls

Runtime setters update the model and invalidate the affected window. Layout
changes reflow automatically; value-only updates use the window's repaint cadence.
Layout changes, focus-sensitive updates, and enabling/disabling repaint without
waiting for a background refresh interval. Direct fields remain convenient during
initial construction; use the setters after attaching controls to a live UI.
For properties without a setter, assign the field and call `win.Refresh()`;
that invalidates layout and item render caches as well as the window cache.

| Setter | Behavior |
| --- | --- |
| `SetValue`, `SetRange` | Slider/progress values are clamped to finite, ordered bounds; integer sliders stay on an in-range integer |
| `SetChecked` | Updates a checkbox/radio; checking a grouped radio unchecks peers in its container/window |
| `SetDisabled` | Disables a subtree while preserving each child's own disabled flag and layout |
| `SetVisible` | Hides/shows a subtree, removing/restoring its layout space |
| `SetOptions` | Copies option strings, normalizes selection, resets menu scrolling, and dismisses an active preview |
| `SetSelected` | Selects a dropdown row; `-1` clears selection; header and invalid indices are ignored |
| `SetActiveTab` | Selects an enabled, visible page and reflows its contents |
| `UpdateText` | Updates labels or input values; input bindings are synchronized, offsets clamped, and undo history reset |
| `SetWrappedText` | Preserves source paragraphs while rewrapping a label |
| `SetLabel`, `SetSize`, `SetTooltip` | Update field labels, logical dimensions, and help indicators with layout invalidation |
| `SetWheelColor`, `SetColorSwatch`, `SetButtonColors` | Repaint color controls without triggering their user callbacks |

```go
speed.SetRange(0, 2)
speed.SetValue(1.25)
enabled.SetChecked(true)
choices.SetOptions([]string{"Low", "Medium", "High"})
choices.SetSelected(1)
advanced.SetVisible(showAdvanced)
settings.SetDisabled(busy)
```

Programmatic setters do **not** emit `UIEvent`, `Action`, `OnSelect`, or
`OnColorChange`. This lets a user-event handler synchronize related controls
without feeding changes back into itself. Dismissing an open dropdown preview
may call `OnHover` once with the previous selection to restore its appearance.
An unchanged scalar value does not schedule a repaint. `SetOptions` still owns a copy
of an identical input slice. Option images retain their row indices; shortening
options truncates excess image references without disposing application images.

`ReplaceText`, `Undo`, and `Redo` are editing commands: they retain their
`EventInputChanged` behavior. Use `UpdateText` to replace a model value silently,
including in a disabled input. Password updates take the logical secret and
keep the displayed text masked. `SetWordWrap` changes presentation while retaining
the draft, selection, and history. For flows, `Fixed`, `Scrollable`, and
`ConstrainToSize` continue to govern how an explicit `SetSize` is resolved.

Hiding or disabling controls cancels their focus, selection, and drag state and
closes their menus. It does not resume those interactions when shown or enabled.
Changing page availability selects another available page, or leaves `ActiveTab`
at `-1` when none exists. Hidden progress bars do not keep a window repainting.
Dirty item render caches are discarded on the next paint; application-owned
`Image` values are retained. Child updates (`AddItem`, `RemoveItem`, `InsertItem`,
`ReplaceItem`, and batched `SetItems`) also reflow a live window automatically.
Row/column builders attach their children in one batch.

`BeforeClose` may return false to keep a window open while resolving unsaved
changes. Closing clears its input state and releases cached render targets,
including tab contents. `RemoveWindow` retires it; detached items release their
cached targets and can be attached again. Window lifecycle callbacks keep their
existing behavior.

## Themes included

**Palettes:** AccentDark, AccentLight, Arcade, Candy, Coffee, Dusk, Forest,
HighContrast, Midnight, NeonNight, Paper, SeaGlass.

**Styles:** Borderless, Default, Flat, HighContrast, Outline, Rounded.

Example JSON templates are included alongside the built-ins. All original
palettes and styles from goThoom are preserved in [`themes`](themes/README.md).
Colors and widget geometry are independent:

```go
if err := ui.LoadTheme("SeaGlass"); err != nil {
    return err
}
if err := ui.LoadStyle("Rounded"); err != nil {
    return err
}
```

Call `SetUserDataRoot` to choose where editable themes live; it creates example
files there. Enable `AutoReload` while designing a theme. EUI does not change the
process working directory. `NewContext` loads embedded Go regular/bold fonts. Choose custom fonts on that
instance with `ui.SetFontSource` and `ui.SetBoldFontSource`.

## A few useful boundaries

- Each `Context` owns its UI state. Use it on the Ebitengine/game goroutine and dispose it when finished.
- `Close` hides ordinary windows for reuse; `RemoveWindow` retires them. Popup and
  color-picker windows remove themselves on close.
- Popups are floating dialogs; they do not block interaction with other windows.
- `NewTextWindow` supplies a scrolling text view and optional editable display.
  Use `NewInput` for a standalone input control. Application text-window editing,
  URL actions, and annotation policies are supplied by your own callbacks.
- Most dimensions use logical UI units. `ui.Layout` handles display scaling;
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
GitHub Actions runs the tagged tests, vet, build, Studio screenshots, and
text-editor and keyboard-focus rendering regressions. Render tests that invoke Ebitengine run in
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

The ongoing toolkit work is tracked in [the refinement plan](docs/refinement-plan.md).

## Origin and license

EUI was extracted from [goThoom](https://github.com/Distortions81/goThoom), where it
was developed for a real desktop game client. This repository is independent;
The standalone library also includes the reusable editor, layout, and rendering
work developed in that client. Client integration is checked against a local
module replacement before pinning a published EUI revision.

[MIT licensed](LICENSE). No Clan Lord game assets are included.
