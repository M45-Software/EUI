# Ownership and embedding

EUI owns reusable controls, editors, layout, themes, and render targets.
Applications own saving, tokenization, history/completion policies, assets, and
which UI receives a physical input stream. goThoom was a source of reusable
behavior; its checkout is outside this refinement's modification scope.

## Own one context per interface

Create each interface with `eui.NewContext()` and retain the returned pointer.
It installs embedded Go fonts and independent default palette/style data. Call
constructors on it, such as `ui.NewWindow()`, `ui.NewColumn()` and `ui.NewInput()`.
Store it on the game or application component that owns the interface. Forward
Update, Draw and Layout to that instance. Dispose it when that owner finishes.

```go
type Game struct { UI *eui.Context }

func NewGame() (*Game, error) {
    ui, err := eui.NewContext()
    if err != nil { return nil, err }
    return &Game{UI: ui}, nil
}
func (g *Game) Update() error { return g.UI.Update() }
func (g *Game) Draw(screen *ebiten.Image) { g.UI.Draw(screen) }
func (g *Game) Layout(w, h int) (int, int) { return g.UI.Layout(w, h) }
```

The owner calls `game.UI.Dispose()` after RunGame returns. Application images,
including those returned by NewImageItem/NewImageFastItem, remain application
resources; release them separately. Context disposal returns cached window/item
images to the owning pool and clears both active text-cache images and idle
reserves. Weak ownership registries cover retained detached and unopened handles
without keeping discarded widgets alive; their stale keys are pruned on growth.

Context constructors and setters operate on the game goroutine. Do not copy a
Context or use a zero value. Update/Draw can be interleaved between independent
instances. Callbacks retain their original owning instance, so configuring another
context inside one does not change where subsequent UI operations execute.

## Configure each instance

Set fonts, regular/bold face caches, user scale, screen dimensions, clipboard
adapter, input source, palette/style names, accent, reload settings, and data-root
paths on that context. Diagnostics and lifecycle callbacks are fields/methods of
the same instance. Contexts contain no selected/default-instance indirection.

`Color.UnmarshalJSON` parses literal HSV/RGBA/hex formats independently of UI
state. `ui.ParseColor` additionally resolves a quoted name from that context's
palette. Palette decoding expands names at Color-typed fields using a local
lookup table; it leaves ordinary strings alone. Contexts can therefore load,
validate, and edit different palettes without consulting one another's colors.
The canonical style name is `Default`.

## Control identity and attachment

Use a widget constructor, or `ui.NewItem(eui.ItemData{...})` for custom
configuration. Attaching unowned configuration to an owned parent adopts all its
children and tab pages. Attach before invoking methods that need fonts, geometry,
focus, or rendering. A detached item keeps its owning context. A tree update
rejects foreign items and owned handlers before changing its child list; it does
not move live UI state between instances. ItemData and WindowData expose a
`Context()` accessor. Copying an existing runtime control is not a cloning API.

Theme values can be configured per window. Images, immutable font sources,
colors, and application data may be deliberately shared; UI state and caches
remain independent. Supplying the same mutable application object to two
contexts is an application sharing decision.

## Route physical input

Ebitengine supplies one OS window/device stream and one native IME session. Use
`SetInputEnabled(false)` on inactive instances; it cancels their captures, menus,
and preedit while retaining focus, values, and history. Disabled contexts still
update animation/time and draw. Re-enabling begins a fresh held-input stream.
One instance should receive native input at a time. A native IME lease prevents
another instance from opening a competing session; disabling, unfocusing, or
disposing its owner releases it. Cancelling another context does not release the
owner's session.

For separate viewports, controllers, or replay, set an InputSource on each
instance and route coordinates/keys explicitly. Sources are sampled once per
Update, including while input is disabled. Frame data is copied/filtered without
mutating the host's slices. The operating-system clipboard is shared unless
separate Clipboard adapters are supplied. Engine glyph/texture atlases also
remain engine resources; each context owns and disposes its own references.

## End the lifetime deliberately

`Dispose()` is idempotent and calls no application handlers, previews, or window
lifecycle callbacks. It closes owned event queues while preserving buffered
messages for draining. New events from retired handlers are suppressed. It
releases preedit, cached images, pools, font caches, watchers' state, and callback
references. Application model values and caller-owned images are not destroyed.

Disposal inside Update/Draw is deferred until that operation finishes, preventing
native-composer or renderer reentry hazards. Update then returns
`eui.ErrContextDisposed`; future Update calls return that error, and Draw does
nothing. Other context-dependent calls panic after disposal. Recursive Update on
the same context returns `eui.ErrContextReentry`. A callback can operate on a
different live context.

## Run checks inside EUI

```sh
CGO_ENABLED=0 xvfb-run -a go test -tags 'integration test' ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
EUI_CONTEXT_CAPTURE=/tmp/eui-contexts.png CGO_ENABLED=0 xvfb-run -a \
  go test -run '^TestRenderIndependentContexts$' .
```

The examples use explicit ownership. Tests create an independent context for
each test rather than resetting package UI globals. Input/ownership tests
interleave two instances; pixel tests render different palettes, fonts, scales
and image modes, dispose one, and verify the other's repaint is unchanged.
