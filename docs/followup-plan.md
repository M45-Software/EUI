# EUI follow-up plan

Implement the selected follow-ups in EUI. goThoom is outside the modification
scope. Accessibility semantics and release/versioning work are excluded.

1. **Hosted CI availability — unavailable externally.** The pushed Test run
   42495405373 never started jobs because GitHub reported an account billing or
   spending-limit problem. The account owner confirmed Actions is unavailable.
   Keep local tests/render checks and Windows/macOS cross-builds operational;
   do not claim those checks restore hosted CI or prove real macOS IME behavior.
   Hosted execution and real Windows/macOS IME sessions remain unverified.
2. **Modal dialogs — implemented and verified locally.** Block other windows'
   pointer, wheel, touch,
   keyboard and window-cycle input; trap focus in the top modal; define default,
   cancel, Escape and close-button behavior; support nested/reentrant dialogs,
   close vetoes, retirement, and restoration of available focus/selection/history.
   Neutral input transitions must prevent held-key/press activation leaking to
   another window. Dialog layering, backdrops and hit targets pass pixel checks
   for AccentDark/Paper at scales 1, 1.5 and 2. Local tagged tests, vet, build,
   editor/Studio render regressions and benchmark smoke checks pass. An isolated
   native Linux X11/IBus session verified pointer blocking, focus trapping, IME
   commit priority, held-key isolation and restoration. Windows/macOS cross-builds
   pass; Wine runs the Windows modal tests and rendering checks. This does not
   establish native Windows/macOS IME coverage.
3. **Form validation — implemented and verified locally.** Required fields,
   finite numeric/integer bounds and custom validators show wrapped inline errors.
   Apply validates all fields before saving an applied snapshot; Cancel restores
   that snapshot and discards errors, preedit and undo/redo. Host model updates
   are explicit after successful Apply. Studio includes a Forms tab with scene
   settings. Replay tests cover typing, paste, undo, identical IME commit/cancel,
   atomic failure/success, hidden invalid fields and context ownership. Pixel
   checks cover error wrapping and Cancel targets in both palettes at three
   scales. The local full suite, vet, build and regression matrix pass; Windows
   tests/pixels pass under Wine and Windows/macOS cross-builds pass. Real native
   Windows/macOS IME coverage remains unavailable as recorded above.
4. **Gamepad navigation — implemented and verified locally.** Opt-in native
   standard-layout input and replay frames support spatial focus, sequential
   shoulder traversal, confirm/cancel, explicit control adjustment, deadzones,
   repeat and stable controller ownership. Handoffs and modals require neutral
   input. Host-rendered prompts and Studio footer hints follow device use. Replay
   tests cover editors/IME, menus, tabs, radios, nested modals, callback reentry,
   controller disconnect, capture and separate contexts. Gamepad focus pixels
   match keyboard focus in both palettes at scales 1, 1.5 and 2. Local full tests,
   vet/build, regression matrix, Windows tests/rendering under Wine and Windows/
   macOS cross-builds pass. No physical controller is exposed in this environment;
   native hardware behavior remains unverified.
5. **Incremental document editing — implemented and verified locally.** A UTF-8
   gap buffer updates local edits and undo/redo without whole-document rune
   allocations. Layout reuses unchanged paragraph measurements and bounded wrap
   partitions, preserving exact visual rows, Unicode/CRLF navigation, selection,
   IME, highlighting and history. Cold/oracle comparisons, large randomized edits,
   pixel checks, the full regression matrix and native X11/IBus sessions pass.
   Windows tests/rendering pass under Wine; Windows/macOS cross-builds pass.
   Fixed edit/resize benchmark samples and source hashes are recorded in
   [performance.md](performance.md). Public immutable text snapshots and complete
   visual-row metadata still do work proportional to document size.

Items 2–5 are implemented and locally verified. Hosted CI availability is
recorded separately; local checks do not establish hosted execution or real
Windows/macOS IME and physical-controller coverage.
