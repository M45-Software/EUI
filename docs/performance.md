# Larger-interface measurements

The suite measures retained lists and editable documents with 1,000 and 10,000
rows. It covers bounds and layout traversal, complete scrolling frames, window
resize, palette changes, cached/cold document layout, caret lookup, an edit plus
undo, and wrapping at changing widths. These are repeatable workload samples,
not a claim about maximum interface size or presented frame rate.

## Reproduce

```sh
BENCHTIME=500ms COUNT=5 build-scripts/benchmark-large-ui.sh /tmp/eui-benchmarks
```

Run from any directory, with Go and a display available; the script selects Xvfb
on Linux if DISPLAY is absent. It saves environment details and separate pure
CPU and frame logs. The script defaults CGO_ENABLED to 0. The equivalent commands
are:

```sh
go test -run '^$' -bench '^BenchmarkLargeInterface$' -benchmem -benchtime=500ms -count=5 .
go test -run '^$' -bench '^BenchmarkLargeInterfaceFrames$' -benchmem -benchtime=500ms -count=5 .
go test -run '^$' -bench '^BenchmarkDocumentEdits$' -benchmem -benchtime=500ms -count=5 .
```

On headless Linux, prefix each Go command with `xvfb-run -a`. Run rendering
benchmarks alone: Ebitengine permits one RunGame per process. Linux CI runs a
single-iteration smoke check and keeps its output as an artifact; it enforces
correct operation rather than a timing threshold.

For profiling, select one workload and keep the CPU profile and binary outside
the source tree:

```sh
go test -run '^$' -bench '^BenchmarkLargeInterface$/Document/10000/WarmLayout$' \
  -benchtime=2s -cpuprofile=/tmp/eui-doc.cpu -o /tmp/eui-doc.test .
go tool pprof -top /tmp/eui-doc.test /tmp/eui-doc.cpu
```

## Workload and method

Lists contain distinct status labels with accented text, 24 logical pixels of
row height, a two-pixel gap, fill width, and an 850×600 scrolling viewport.
Documents contain distinct numbered lines with tabs, combining characters,
emoji, and comments, about 1 MiB at 10,000 lines. Built-in fonts need not contain
all those glyphs; the source offsets still exercise Unicode editing. Wrapped
resize varies the viewport between 260 and 419 pixels and asserts that rows
actually wrap. EditAtEnd includes insertion, layout, undo, and restored layout.

Frame cases run Update and Draw inside a live Ebitengine game at 960×720.
They move through 101 scroll stops. All stops are exercised before timing.
ReadPixels runs with the benchmark timer stopped after each frame to flush
queued work and bound command accumulation. Timings therefore measure CPU frame
preparation, potentially including synchronous driver work; they exclude
readback, presentation, and vsync. They cannot establish GPU FPS. Allocation
samples can include Ebitengine command-buffer and cache growth; inspect the raw
samples rather than treating one frame byte count as a fixed budget.

Recorded on 2026-11-08 using Go 1.27.1, Ebitengine 2.10.2, Linux amd64,
CGO_ENABLED=0, and an AMD Ryzen 9 7950X (GOMAXPROCS=32). Xvfb used Mesa 26.1.3
llvmpipe software rendering. Baseline code includes refinement pieces 1–6; both
versions use identical fixtures and warm-up. The four suites ran sequentially.
Pure cases use three 100 ms samples; frame cases use three 300 ms samples.
Shared-machine scheduling, GC, software rendering, and sample duration limit
precision. Values below are medians; rerun on the application's target hardware.

## Recorded results

All rows in this table use 10,000 items or document lines. A cached lookup is a
small operation, whereas an edit or frame includes substantial work; compare
before/after within a row.

| Workload | Before | After | Before → after allocations/op |
| --- | ---: | ---: | ---: |
| List content bounds | 0.428 ms | 0.274 ms | 0 → 0 |
| List scroll traversal | 0.360 ms | 0.212 ms | 0 → 0 |
| List resize | 2.76 ms | 2.15 ms | 2 → 2 |
| List palette switch | 6.08 ms | 4.82 ms | 911 → 910 |
| Cached document layout | 530,699 ns | 28.7 ns | 0 → 0 |
| Document end-caret lookup | 1,075,186 ns | 44.0 ns | 0 → 0 |
| Cold document layout | 61.0 ms | 39.5 ms | 220,024 → 60,005 |
| Insert, layout, undo, layout | 136.6 ms | 84.2 ms | 440,057 → 120,016 |
| Wrapped document resize | 155.5 ms | 60.6 ms | 660,028 → 70,007 |
| List scrolling Update/Draw | 11.16 ms | 1.88 ms | 798 → 244 |
| Document scrolling Update/Draw | 28.64 ms | 1.49 ms | 156 → 152 |

Cold document layout drops from 55.2 MB to 26.0 MB allocated per operation;
wrapped resize from 125.4 MB to 29.7 MB. These are decimal MB of allocated Go
memory, not retained memory or VRAM. The raw logs include the 1,000-row cases,
byte counts, and variation between samples:

- [Environment](performance-results/environment.txt)
- [CPU before](performance-results/pure-before.txt) and [CPU after](performance-results/pure-after.txt)
- [Frames before](performance-results/frames-before.txt) and [frames after](performance-results/frames-after.txt)

## Changes driven by the measurements

The cached-document profile spent most CPU time recounting runes. Editor state
now retains the rune count until the document changes. Direct assignments,
programmatic setters, edits, undo/redo, and passwords refresh it. Caret lookup
uses binary search over sorted visual rows. Drawing visits the visible row
range rather than scanning the entire document.

Width-only reflows reuse measured physical lines, grapheme stops, and tab/byte
mappings. Parsed data is replaced on document/font changes. Preallocation reduces
cold-layout growth and copying. Wrapping preserves the same source offsets and
caret behavior; it still measures and allocates changed visual rows.

The list profile also showed repeated texture allocation/retirement. Plain text
now reuses cleared backing images through a dedicated bounded pool. The raster
cache allows at most 256 entries and 8 MiB of active backing allocations, plus
1 MiB of idle reusable allocations. Byte accounting includes size-class padding
and the managed-image border, or standalone texture rounding in unmanaged mode;
Ebitengine atlas packing and GPU driver overhead are additional. Start-aligned
forward flows also avoid a redundant total-size traversal.

## Remaining limits and virtualization decision

Generic flow lists remain retained: every item owns state, and layout, pointer
handling, and keyboard traversal can still visit the whole tree. This work does
not recycle arbitrary interactive controls. At the measured 10,000-label scale,
removing redundant traversal and texture churn addressed the observed costs.
A data-driven virtual list is a future option when target-device profiles show
row traversal or retained memory dominating. It needs explicit stable row
identity, focus/selection retention, accessibility, and ownership rules.

The earlier results above predate the incremental editor work below. Cold loads
and font changes still require a full parse. Immutable public `Text`/`TextPtr`
snapshots require copying the changed document, and complete visual-row metadata
still scales with the number of rows. Large paste/replacement, edits far from
the gap, word operations, and application-provided whole-document highlighters
can do document-wide work. Do not infer that a fast scroll frame establishes
edit latency on a different workload or target device.

Validation checks Unicode count invalidation, caret boundaries, reuse without
stale offsets, and fractional viewport clipping. A pixel regression compares a
deeply scrolled 10,000-line editor to its local excerpt at 1×, 1.5×, and 2×,
including highlighting and selection. Cache regressions compare direct/cached
pixels across eviction and reuse and enforce active/idle memory bounds. Timing
numbers are observations, not flaky pass/fail assertions.


## Incremental document editing (2026-11-08)

This comparison starts from the buffer/layout implementation in `29296bf`, with
the same modal/form/gamepad changes present in both trees. It uses the unchanged
`BenchmarkLargeInterface/Document` fixtures and the same machine/Go configuration
as above. Each edit sample performs ten insert/layout/undo/layout cycles; medians
use five samples. Each resize sample performs 160 operations, covering the entire
existing 260–419 pixel width sweep; medians use three samples. Fixed operation
counts ensure both versions visit the same widths. The final comparison ran
without concurrent tests or benchmarks. Figures measure CPU work and allocated
Go memory, rather than retained memory, GPU work, or presented FPS.

| Workload | Lines | Before | After | Allocated MB/op before → after | Allocations/op before → after |
| --- | ---: | ---: | ---: | ---: | ---: |
| Insert, layout, undo, layout | 1,000 | 8.61 ms | 0.137 ms | 5.79 → 0.332 | 12,016 → 14 |
| Insert, layout, undo, layout | 10,000 | 85.06 ms | 1.194 ms | 57.74 → 3.214 | 120,016 → 14 |
| Wrapped resize | 1,000 | 3.48 ms | 0.333 ms | 2.66 → 0.496 | 6,105 → 409 |
| Wrapped resize | 10,000 | 65.86 ms | 4.554 ms | 27.28 → 4.915 | 61,006 → 4,067 |

For the 10,000-line fixtures this is about 71× faster for edit/undo and 14× for
resizing. These ratios describe these fixed workloads and machine, not a general
latency guarantee. Cached layout/caret lookup remains constant-time; cold layout
still rebuilds the document. See the raw samples, environment, and editor source
hashes:

- [Environment and source hashes](performance-results/incremental/environment.txt)
- [Edit/undo before](performance-results/incremental/before-EditAtEnd.txt) and [after](performance-results/incremental/after-EditAtEnd.txt)
- [Resize before](performance-results/incremental/before-WrappedResize.txt) and [after](performance-results/incremental/after-WrappedResize.txt)

Reproduce the fixed samples in each source tree, running one at a time:

```sh
CGO_ENABLED=0 xvfb-run -a go test -run '^$' \
  -bench '^BenchmarkLargeInterface/Document/(1000|10000)/EditAtEnd$' \
  -benchtime=10x -count=5 -benchmem .
CGO_ENABLED=0 xvfb-run -a go test -run '^$' \
  -bench '^BenchmarkLargeInterface/Document/(1000|10000)/WrappedResize$' \
  -benchtime=160x -count=3 -benchmem .
```

The editor now uses a UTF-8 byte gap buffer. Nearby changes update the gap and
rune count without allocating a whole-document rune slice. Undo/redo reconcile
the differing region of immutable snapshots. Large deletions shrink the buffer's
retained capacity. Public values and change events retain their existing
immediate snapshot semantics, so publishing edited text is still O(document
bytes).

Layout reuses unchanged paragraphs' shaped display, rune/byte mappings and
grapheme boundaries, including unchanged middle blocks after multiple edits,
adjusting absolute source offsets in new row metadata.
Published earlier layouts retain their geometry. Source slices are rebound to
the current document, avoiding retention of a different full old snapshot per
unchanged paragraph. New font faces invalidate measured data. Password layouts
continue deriving grapheme stops from the secret, not the display mask.

Each paragraph retains at most two wrapping partitions. A partition is reused
only inside a measured width interval that preserves its exact break points;
other paragraphs rewrap when their breaks change. Complete visual-row counts,
scrollbars, selection and caret lookup remain exact. Reflow preallocates row
metadata from the preceding layout. Warm IME display text is cached until the
preedit or source changes. Character/line navigation uses the paragraph index,
including CRLF and composed Unicode character boundaries.

`BenchmarkDocumentEdits` adds middle typing, middle wrapped typing, a 500-line
paste, real Backspace commands, and warm composition layout at both document
sizes. The benchmark script and CI smoke configuration include these workloads.
Tests compare random UTF-8 gap edits/restores against rune-string oracles, local
edits and resize partitions against cold layout, and source navigation against
the previous whole-document rules. The new incremental pixel regression compares
edits, undo/redo, resize, highlighting and IME rendering with a fresh layout in
both palettes at three scales. An isolated native Linux X11/IBus session verifies
10,000-line composition commit/cancel, undo/redo, Backspace and wrap toggles.
Windows tests and rendering are checked under Wine; Windows/macOS binaries are
cross-built. Hosted CI and real Windows/macOS IME sessions remain unavailable.
