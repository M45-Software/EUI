# EUI refinement plan

Work through these pieces in order. Changes stay inside EUI; goThoom is outside
this work's modification scope. Use current names and APIs without compatibility
aliases. A piece is complete when its behavior, documentation, and relevant
checks are verified, rather than merely when scaffolding exists.

1. **Keyboard navigation — complete.** Tab/Shift+Tab focus order for controls;
   activation; radio groups, sliders, dropdowns, context menus, tab strips;
   visible focus; automatic scrolling; access to each interactive window;
   preservation of editor shortcuts and host keyboard capture. Verify command
   sequences, callback effects, hidden/disabled/inactive-page exclusions, nested
   scrolling at multiple scales, and rendered focus indicators.
2. **Consistent runtime updates — complete.** Setters for values, checked state,
   enabled/visible state, and options; automatic invalidation and focus cleanup;
   explicit policy for programmatic updates versus user-generated events.
3. **Explicit event delivery — complete.** Simple callback defaults; opt-in queues;
   documented and tested ordering and overflow behavior.
4. **Responsive layout — complete.** Consistent minimum, preferred, and maximum
   sizing; spacing and alignment; fill behavior; Studio rearranges gracefully
   when its window narrows.
5. **Interaction testing — complete.** Injectable pointer, keyboard, and time
   inputs; complete user sequences; rendering regressions; CI beyond Linux.
6. **IME composition — complete locally.** Composition and committed text, composition
   display, candidate placement, focus-loss behavior, and platform validation.
7. **Larger-interface measurements — complete locally.** Representative list, document,
   scroll, resize, and theme benchmarks; measure allocations and timing, then use
   results to guide virtualization and cache refinements.
8. **Explicit UI context — complete locally.** Independently owned windows, focus, fonts,
   themes, caches, and cleanup; isolated instances and tests, with a deliberate
   public API.

## Checks for the keyboard piece

```sh
CGO_ENABLED=0 xvfb-run -a go test -tags 'integration test' ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
EUI_KEYBOARD_CAPTURE=/tmp/eui-keyboard.png CGO_ENABLED=0 xvfb-run -a \
  go test -run '^TestRenderKeyboardFocus$' .
```

Verified for piece 1: the tagged regression suite, vet, build, editor/title-cache
rendering regressions, Studio renders, and focus pixel checks at 1×, 1.5×, and
2× scale. An isolated X11 app also passed real-keypress sequences for buttons,
checkboxes, sliders and release events, dropdowns, radio groups, tab pages,
window cycling, and host keyboard capture. The reusable focus pixel regression
is part of CI. Remaining pieces retain their original scope above.

## Checks for runtime updates

Verified for piece 2: the full tagged suite, vet, build, runtime-update pixel
regressions, keyboard focus renders, editor/title-cache/tooltip rendering checks,
and refreshed Studio captures in dark, light, and editor configurations.

Runtime setters now normalize control state, invalidate rendering, reflow changed
geometry, and cancel unavailable controls' interaction state. Programmatic state
updates are silent; dismissing a live dropdown preview restores its selected
appearance through the preview callback. Child-list updates also reflow open
windows. Tests cover numeric bounds, input bindings/history, copied options,
reentrant preview callbacks, inherited disabled state, unavailable tabs, cache
invalidation, and batched child updates. The runtime pixel regression is in CI.

## Checks for event delivery

Verified for piece 3: the full tagged suite, vet, build, and runtime, keyboard,
editor, title-cache, and tooltip pixel regressions. Event-delivery regressions
run in the normal suite and CI.

Handlers default to synchronous callbacks without allocating queues. Bounded
FIFO queues are explicitly enabled, expose a dropped-newest count, and remain
nonblocking without suppressing callbacks. Queue replacement/disable closes the
owned channel while preserving buffered events for draining. Tests cover overflow
and recovery, queue lifecycle, recursion, reconfiguration inside callbacks,
callback/payload snapshots, password masking, silent runtime setters with queues
enabled, and actual pointer/keyboard delivery order. Dropdowns and context menus
share selection delivery and end previews first; follow-up menus survive callback
reentrancy. Sliders and wheels use consistent change-only notifications.

## Checks for responsive layout

Verified for piece 4: full tagged suite, vet, build, and the layout, runtime,
keyboard, editor, title-cache, and tooltip rendering regressions. Layout pixel
checks exercise dark/light palettes at 1x, 1.5x, and 2x scale and verify that
aligned hit targets match rendered controls. The layout pixel regression and
360x640 Studio editor smoke render are in CI.

Size remains the preferred logical size; resolved geometry has separate state.
Minimum/maximum limits, bounded fill sharing on both axes, inter-child gaps,
inner padding, main/cross alignment, overlays, hidden controls, wrapping, and
scrollbar clearance share placement calculations for measurement, rendering,
and focus reveal. Tests cover overflow minima, reverse-flow overflow and scrollbar
hit targets, runtime changes, style-triggered tab reflow, zero available space,
and LayoutWindowBody's gap/padding reservation.

Studio follows actual window dimensions with wide, medium, and narrow
arrangements inside a scrolling workspace. Breakpoint tests cover 1360, 1140,
960, 760, 480, and 360 pixel widths, preserve editor text/history/focus/caret/
selection, verify tree ownership and focused-editor reachability, and confirm
canvas rasters follow their allocated size. Desktop dark/light/editor, 480x800,
and 360x640 editor captures were rendered and inspected. Updated screenshots
and the responsive API guide are included. Remaining pieces retain their scope.

## Checks for interaction testing

Verified for piece 5: the full tagged Linux suite, vet, build, and all replay,
layout, runtime, keyboard, editor, title-cache, and tooltip rendering regressions.
Studio dark/light/editor, narrow, and compact editor smoke renders also passed.
Replay pixel checks prove that a click flash expires and a caret blinks using
injected time rather than sleeping. Workflow YAML and the Windows/macOS matrix
were parsed and checked.

InputFrame/InputSource feed the same transition tracker as native Ebitengine
polling. Tests drive Update and Draw through presses, movement, release, touch
identity changes, two-finger scrolling, held-key repeat, focus traversal, radio
selection, tab changes, search, window move/resize, middle-button move, divider
capture, menu previews and follow-up menus, text selection/replacement, clipboard
success/failure, undo/redo, password restrictions, and Control/Command/Option/AltGr
rules. Source replacement cancels gestures and resets timing while preserving
focus and selection; invalid frame clocks cannot partially mutate UI state.

Real X11 key/text/pointer injection passed against an isolated native app,
including initial Tab entry and a slider captured outside its native window.
The full Windows root test binary passed under Wine. Windows amd64 and macOS
arm64 test binaries cross-compiled, and the macOS application build passed.
Windows and macOS CI jobs now run the suite, vet, build, and a native Studio
editor smoke render with a retained artifact. Those actual hosted-runner results
await a push; Wine and cross-compilation are not native macOS validation.

Sequence tests exposed and fixed first-Tab entry before window activation, short
slider track clicks, pointer capture outside sliders, touch ownership transitions,
browser wheel idle-budget consumption, and caret clock restart after source
replacement. goThoom remains outside this work's modification scope.

## Checks for IME composition

Verified for piece 6: full tagged Linux suite, vet, build, and IME, interaction,
layout, runtime, keyboard, editor, title-cache, and tooltip pixel regressions.
Studio dark/light/editor/narrow/compact smoke renders passed. Windows amd64 and
macOS arm64 test binaries cross-compiled; the macOS application build passed.
The Windows root suite, including composition regressions, passed under Wine.
Native Windows/macOS language-IME sessions remain unverified, and hosted CI
results await a push. These limitations are not replaced by cross-compilation.

A native X11 application passed ordinary keyboard/text/pointer input without
character duplication. An isolated IBus test engine then exercised real XIM
preedit delivery, CJK commits, Escape cancellation, focus transfer, and Enter
routing without activating the default button. The test engine used temporary
DBus/XDG state and an installed CJK font; no system configuration or font assets
were changed. This verifies the native transport, not every language engine's
candidate-list behavior.

Automated regressions cover separate preedit/model/binding/history, one-step
commit undo/redo, reconversion/caret placement, ordered packets, stale target and
document rejection, reentrant cancellation, password masking and cleared native
context, titlebar search/query limits, app focus loss, keyboard capture,
hide/disable/removal, source changes, and external caret/geometry changes.
Candidate bounds are checked through labels, tabs, nested scrolling, wrapping,
window movement, NoScale windows, combining characters, and scales 1x/1.5x/2x.
Pixel checks verify inline preedit, clause selection/underline/caret, exact
cancellation restoration, commit rendering, masked password preedit, and search
title-cache invalidation. The IME pixel regression is included in Linux CI.

Ebitengine fixes candidate bounds when a session starts. EUI therefore cancels
and reopens sessions after external caret/layout changes. Native surrounding
text excludes password contents; callbacks receive committed text only. Search
uses a stable text viewport and shares the internal editor/composer path.
Application-owned editors retain responsibility for their own composition.
Remaining pieces retain their scope above.

## Checks for larger-interface measurements

Verified for piece 7: 1,000/10,000-row list and document benchmarks cover bounds,
traversal, full scrolling Update/Draw frames, resize, palette changes, cached and
cold layout, end-caret lookup, edit/undo, and actual wrapped resize. Identical
before/after fixtures ran sequentially; raw timing/allocation logs, environment,
method, and limitations are recorded in performance.md and performance-results.
The benchmark script passed a single-iteration smoke run, and CI runs the smoke
plus the deep-scroll pixel regression with retained timing output.

Measured bottlenecks led to cached rune counts, binary caret lookup, viewport-only
editor row iteration, reuse of parsed physical rows on width changes, layout
preallocation, removal of the forward-flow total-size prepass, and bounded
reusable text raster allocations. At 10,000 lines, median cold layout went from
61.0 ms/55.2 MB to 39.5 ms/26.0 MB; deeply scrolling Update/Draw from 28.64 ms to
1.49 ms in the headless software-rendering environment. Generic lists retain
ownership of all rows; full virtualization and incremental document editing
remain measured future options rather than implied capabilities. Cold edits and
full wrapping still exceed a frame budget at that scale; the report states this.

The full tagged Linux suite, vet, build, all prior rendering gates, new large
viewport/cache pixel checks, and Studio dark/light/editor/narrow/compact renders
passed. New regressions verify count refresh through all edit paths, physical and
soft row boundaries, parsed-row reuse/invalidation, and fractional viewport
ranges. Deep-scroll pixels match a local excerpt at 1x/1.5x/2x, with selection
and highlighting. Texture reuse clears stale pixels and enforces allocation
budgets in managed/unmanaged modes. Windows tests and text-cache pixels passed
under Wine; Windows/macOS test binaries and the macOS app cross-compiled.
Actual hosted native CI remains pending a push. The final piece retains its
scope: independent explicit UI contexts and cleanup.

## Checks for explicit UI contexts

Verified for piece 8: NewContext owns windows, focus, input transitions and time,
menus, themes/styles/accent/named colors, editable paths and watcher state, fonts
and face caches, diagnostics, native-composer state, primitive/text/shadow caches,
and render-target pools. There is no default/selected UI instance or state swap.
Constructors and state APIs are context methods; pure geometry/color helpers
remain package functions. The basic example, Studio, README and embedding guide
use explicit ownership. The obsolete extraction-only client check is removed;
goThoom remains outside the modification scope.

Each migrated test now creates its own context. Isolation tests interleave two
streams, focus/preedit/history, theme/font/scale changes, callbacks, menus,
clipboard adapters, named-color validation, editable roots and reload settings.
Foreign controls/handlers/default buttons are rejected before tree/input
mutation. Window-list snapshots cannot replace internal entries. Weak registries
retire retained detached/unopened handles while allowing discarded controls to
be collected; stale registry keys are pruned on growth.

Dispose is idempotent, closes owned event queues without dropping buffered
messages, suppresses retired handlers, and releases caches/pools while keeping
application images intact. Disposal inside Update/Draw is deferred; recursive
Update is rejected. Tests verify cleanup from callbacks and independent continued
operation of another context. Disabled contexts retain focus/history, suspend
input, and cannot overwrite the active OS cursor.

A pixel regression renders independent palettes, fonts, scales and image modes,
disposes one instance, forces the other to repaint, and proves its pixels and
target ownership remain intact. That gate is part of Linux CI. A real X11/IBus
app also passed CJK preedit, commit/cancel, cross-context native-input routing,
default-button suppression, and disposal after RunGame exits. Native devices,
clipboard and Ebitengine's one OS IME session remain physical shared resources;
explicit input routing and an owner-checked session lease cover that boundary.

Direct-control, shadow, pixel-alignment, full-theme-gallery, recycled-target and
colored-emoji cache rendering checks also passed.

The full tagged Linux library/Studio suite, vet/build, every prior refinement's
pixel gate, the large-interface benchmark smoke, and dark/light/editor/narrow/
compact Studio renders passed. Windows root tests and the independent-context
pixel regression passed under Wine. Final Windows amd64/macOS arm64 test binaries
and the macOS app cross-compiled. Actual hosted native jobs still await a push;
real Windows/macOS language-IME sessions retain the documented validation limit.

## Local completion audit

All eight implementation pieces are complete locally. The final tree retains
and reruns their regression coverage rather than relying on earlier passing
versions. Keyboard/focus and interaction sequences, runtime setter/event tests,
responsive Studio/layout checks, IME lifecycle/placement, large-interface
measurements, and multi-context ownership/disposal all have corresponding tests
and documented evidence above. Performance raw samples and environment remain in
performance-results; context ownership and physical-resource limits are in
client-integration.md. Source and filenames use current names without aliases.
Actual hosted-runner results are separate from the completed local checks;
no native macOS runtime result is claimed.
