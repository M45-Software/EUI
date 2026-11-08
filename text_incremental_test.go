package eui

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func sameEditRows(a, b []editTextLine) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.start != y.start || x.length != y.length || x.display != y.display || x.width != y.width || x.softBreak != y.softBreak || !reflect.DeepEqual(x.bytes, y.bytes) || !reflect.DeepEqual(x.stops, y.stops) {
			return false
		}
	}
	return true
}
func assertIncrementalLayout(t *testing.T, item *ItemData) *editTextLayout {
	t.Helper()
	state := item.editor()
	incremental := item.editLayout()
	state.layout = nil
	cold := item.editLayout()
	state.layout = incremental
	if !sameEditRows(incremental.lines, cold.lines) || !sameEditRows(incremental.logical, cold.logical) || incremental.width != cold.width || incremental.lineHeight != cold.lineHeight {
		t.Fatal("incremental layout differs from cold layout")
	}
	return incremental
}

func TestIncrementalLayoutReusesOnlyUnchangedParagraphs(t *testing.T) {
	ctx := newTestContext(t)
	item := ctx.editingFixture(t, "prefix café\nedit this\nsuffix e\u0301 👩‍💻\n", true)
	item.SetWordWrap(true)
	item.textDrawSize = Point{X: 80, Y: 120}
	before := item.editLayout()
	frozen := fmt.Sprintf("%v", before.lines)
	item.editReplace(before.logical[1].start, before.logical[1].start+4, "changed\n日本", "")
	after := assertIncrementalLayout(t, item)
	if &after.logical[0].bytes[0] != &before.logical[0].bytes[0] || &after.logical[3].bytes[0] != &before.logical[2].bytes[0] {
		t.Fatal("local edit reparsed unchanged prefix/suffix")
	}
	if fmt.Sprintf("%v", before.lines) != frozen {
		t.Fatal("local edit mutated published old layout")
	}
	item.Undo()
	assertIncrementalLayout(t, item)
	item.Redo()
	assertIncrementalLayout(t, item)
	item.FontSize += 3
	assertIncrementalLayout(t, item)
	ctx.uiScale = 1.5
	assertIncrementalLayout(t, item)
	item.UpdateText("entirely new\ntext")
	assertIncrementalLayout(t, item)
	item.Text = "direct\nassignment"
	assertIncrementalLayout(t, item)
}

func TestIncrementalLayoutRandomizedEditsAndWidthChanges(t *testing.T) {
	ctx := newTestContext(t)
	item := ctx.editingFixture(t, strings.Repeat("\tAV fi café e\u0301 👩‍💻 text words\n", 20), true)
	item.SetWordWrap(true)
	rng := rand.New(rand.NewPCG(88, 31))
	inserts := []string{"", "日本", "e\u0301", "👩‍💻", "\n\n", "\tA\nB\t", " same text "}
	for i := 0; i < 150; i++ {
		old := item.editLayout()
		frozen := fmt.Sprintf("%v", old.lines)
		n := item.editor().runeCount
		start, end := rng.IntN(n+1), rng.IntN(n+1)
		if start > end {
			start, end = end, start
		}
		item.editReplace(start, end, inserts[rng.IntN(len(inserts))], "")
		item.textDrawSize = Point{X: float32(30 + rng.IntN(400)), Y: 100}
		assertIncrementalLayout(t, item)
		if fmt.Sprintf("%v", old.lines) != frozen {
			t.Fatalf("edit %d mutated previous geometry", i)
		}
		if i%5 == 0 {
			item.Undo()
			assertIncrementalLayout(t, item)
			item.Redo()
			assertIncrementalLayout(t, item)
		}
	}
}

func TestIncrementalLayoutReusesMiddleBlockAcrossBatchedEdits(t *testing.T) {
	ctx := newTestContext(t)
	item := ctx.editingFixture(t, "zero\none\ntwo\nthree\nfour\nfive\nsix\nseven", true)
	item.SetWordWrap(true)
	before := item.editLayout()
	item.editReplace(before.logical[1].start, before.logical[1].start+3, "insert\nnew", "")
	n := item.editor().runeCount
	item.editReplace(n, n, "\nlast", "")
	after := assertIncrementalLayout(t, item)
	for _, line := range after.logical {
		if line.raw == "four" {
			if &line.bytes[0] != &before.logical[4].bytes[0] {
				t.Fatal("batch edits reparsed unchanged middle block")
			}
			return
		}
	}
	t.Fatal("unchanged middle block disappeared")
}

func TestWrapPartitionCacheMatchesFreshWrapping(t *testing.T) {
	ctx := newTestContext(t)
	values := []string{"", "W", "👩‍💻", "AV fi words with spaces at different widths", "\talpha\tbeta \t gamma", "e\u0301 e\u0301\u2003日本\u00a0👩‍💻 longwordswithoutspaces", "a  b   c    ", strings.Repeat("AVfi👩‍💻", 8)}
	for _, size := range []float32{9, 12, 21} {
		for _, value := range values {
			item := ctx.editingFixture(t, value, true)
			item.FontSize = size
			layout := item.editLayout()
			line := layout.logical[0]
			line.start = 17
			cache := editLineWrapCache{}
			var saved []editTextLine
			for i := 0; i < 300; i++ {
				width := float32(i%150)*1.13 + .5
				if i >= 150 {
					width = 170.01 - width
				}
				rows := cache.appendRows(nil, line, layout.face, width)
				want := wrapEditLine(line, value, layout.face, width)
				if !sameEditRows(rows, want) {
					t.Fatalf("font=%v width=%v text=%q: cache returned wrong partition", size, width, value)
				}
				if i == 30 {
					saved = rows
				}
				if i == 31 && !sameEditRows(saved, wrapEditLine(line, value, layout.face, float32(30)*1.13+.5)) {
					t.Fatal("reflow mutated cached published rows")
				}
			}
		}
	}
}

func TestIncrementalCompositionAndPasswordBoundaries(t *testing.T) {
	ctx := newTestContext(t)
	_, item, d := ctx.imeEditor(t, "prefix\n日本 e\u0301\nsuffix")
	item.SetWordWrap(true)
	item.editSelect(7, 9)
	assertIncrementalLayout(t, item)
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "👩‍💻 new\nline", SelectionEnd: 4})
	assertIncrementalLayout(t, item)
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "👩‍💻 new\nline"})
	assertIncrementalLayout(t, item)
	item.Undo()
	assertIncrementalLayout(t, item)
	item.Redo()
	assertIncrementalLayout(t, item)
	password, _ := ctx.NewInput()
	password.HideText = true
	password.UpdateText("e\u0301")
	ctx.Focus(password)
	first := password.editLayout()
	password.UpdateText("ab")
	second := assertIncrementalLayout(t, password)
	if reflect.DeepEqual(first.logical[0].stops, second.logical[0].stops) {
		t.Fatal("equal masked text reused old secret grapheme boundaries")
	}
}

func TestLineLocalNavigationMatchesWholeDocumentOracle(t *testing.T) {
	ctx := newTestContext(t)
	for _, value := range []string{"", "a\n\nb\n", "\r\nA\r\n\r\n", "café e\u0301\n👩‍💻\n🇺🇸🇨🇦\r\nlast", "\tA\u200d\tB\n\u0301end"} {
		item := ctx.editingFixture(t, value, true)
		for pos := 0; pos <= item.editor().runeCount; pos++ {
			for _, direction := range []int{-1, 1} {
				if got, want := item.charBoundary(pos, direction), editCharBoundary(value, pos, direction); got != want {
					t.Fatalf("text=%q pos=%d dir=%d got=%d want=%d", value, pos, direction, got, want)
				}
			}
			a, b := item.sourceLineBounds(pos)
			c, d := editLineBounds(value, pos)
			if a != c || b != d {
				t.Fatal("line bounds differ from source oracle")
			}
		}
	}
}

func TestIncrementalIMECallbackResetDoesNotReviveOldSelection(t *testing.T) {
	ctx := newTestContext(t)
	_, item, d := ctx.imeEditor(t, "before")
	var modal *WindowData
	item.Handler.Handle = func(UIEvent) {
		item.UpdateText("before")
		modal = ctx.ShowModal("Next", "", []PopupButton{{Text: "OK"}})
	}
	d.ime(TextInputEvent{Kind: TextInputCompose, Text: "日本", SelectionEnd: 2})
	d.ime(TextInputEvent{Kind: TextInputCommit, Text: "日本"})
	if modal == nil || !modal.Open || ctx.FocusedItem() == item || ctx.selectedTextItem == item {
		t.Fatal("IME callback restored editor selection across a modal boundary")
	}
}
