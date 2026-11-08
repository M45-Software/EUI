# Development

[Project overview](../README.md) · [API reference](api/README.md)

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

Regenerate the Studio images from the real application:

```sh
xvfb-run -a go run ./examples/studio -screenshot docs/studio.png
xvfb-run -a go run ./examples/studio -theme Paper -style Rounded -screenshot docs/studio-light.png
xvfb-run -a go run ./examples/studio -tab Editor -screenshot docs/studio-editor.png
```

The screenshot option writes one frame and exits. No image editor is needed.
More focused theme visual checks are documented in [`themes/README.md`](../themes/README.md).

For testing a local library change in goThoom and switching the client to a
pinned standalone dependency, see [client integration](client-integration.md).

The ongoing toolkit work is tracked in [the refinement plan](refinement-plan.md).

## Measuring larger interfaces

Use `build-scripts/benchmark-large-ui.sh /tmp/eui-benchmarks` to measure 1,000
and 10,000-row lists and documents, including scrolling, resizing, edits,
wrapping, and theme changes. It records timings, allocations, and environment
information. Set `BENCHTIME` and `COUNT` to control sample duration and repeats.
Frame benchmarks run inside Ebitengine; GPU readback is excluded from the timer
and flushes queued work between iterations. Their results describe CPU frame
preparation, rather than presented FPS. See [the measurements and remaining
limits](performance.md) for the workload and recorded before/after results.

Editor lookups cache document rune counts, caret lookup uses the sorted visual
rows, and paints visit only viewport rows. Local edits use a UTF-8 gap buffer
and reuse unchanged paragraphs' measurements and grapheme boundaries. Wrapping
reuses exact line-break partitions across compatible widths. Cold loads and
font changes still parse the full document; publishing `Text` and assembling
visual rows still scale with document size. Plain text uses a
bounded raster cache with reusable backing allocations: at most 8 MiB of active
allocations, 256 entries, and a separate 1 MiB idle reserve. Generic flow lists
still retain and traverse every item. Keep application history bounded; measure
before choosing a purpose-built virtual list for larger datasets.
