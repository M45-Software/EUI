package eui

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLargeDocumentCountsStayCurrentAcrossAllEditPaths(t *testing.T) {
	ctx := newTestContext(t)

	item := ctx.editingFixture(t, strings.Repeat("café e\u0301 👩‍💻\n", 1000), true)
	check := func() {
		t.Helper()
		n := utf8.RuneCountInString(item.editText())
		item.CursorPos, item.SelectStart, item.SelectEnd = math.MaxInt, math.MaxInt, math.MaxInt
		state := item.editor()
		if state.runeCount != n || item.CursorPos != n || item.SelectStart != n || item.SelectEnd != n {
			t.Fatalf("stale document count: %d want %d", state.runeCount, n)
		}
	}
	check()
	item.editInsert("日本", "")
	check()
	item.Undo()
	check()
	item.Redo()
	check()
	item.UpdateText("éé")
	check()
	// Same byte length, different rune count: count is keyed by document content.
	item.Text = "abcd"
	check()
	item.HideText = true
	item.SecretText = "e\u0301👩‍💻"
	item.Text = "******"
	check()
	item.UpdateText("new secret")
	check()
}
func TestDocumentReflowReusesLogicalRowsWithoutStaleOffsets(t *testing.T) {
	ctx := newTestContext(t)

	value := strings.Repeat("\talpha e\u0301 👩‍💻 beta gamma delta\n", 100)
	item := ctx.editingFixture(t, value, true)
	baseline := item.editLayout()
	item.SetWordWrap(true)
	item.textDrawSize = Point{X: 90, Y: 120}
	narrow := item.editLayout()
	if &baseline.logical[0] != &narrow.logical[0] || len(narrow.lines) <= len(baseline.lines) {
		t.Fatal("width-only reflow did not reuse parsed document rows")
	}
	// A cached old layout must remain usable after reflow, without alias mutation.
	if baseline.lines[0].softBreak || baseline.lines[0].display != baseline.logical[0].display {
		t.Fatal("reflow mutated the previous unwrapped layout")
	}
	item.textDrawSize.X = 300
	wide := item.editLayout()
	if &wide.logical[0] != &baseline.logical[0] || len(wide.lines) >= len(narrow.lines) {
		t.Fatal("wider reflow failed to reuse data or reduce visual rows")
	}
	for _, layout := range []*editTextLayout{narrow, wide} {
		runes := []rune(value)
		for _, row := range layout.lines {
			raw := string(runes[row.start : row.start+row.length])
			if len(graphemeStops(raw)) != len(row.stops) {
				t.Fatal("reflow split a grapheme")
			}
		}
	}
	item.FontSize += 3
	changedFont := item.editLayout()
	if &changedFont.logical[0] == &wide.logical[0] {
		t.Fatal("font change reused old measured rows")
	}
	item.Text = "changed\ntext"
	changedText := item.editLayout()
	if changedText.value != item.Text || len(changedText.logical) != 2 {
		t.Fatal("direct document change reused stale rows")
	}
	item.SetWordWrap(false)
	if len(item.editLayout().lines) != 2 {
		t.Fatal("unwrapping failed")
	}
}
func TestDocumentCaretAtEveryPhysicalAndSoftBoundary(t *testing.T) {
	ctx := newTestContext(t)

	item := ctx.editingFixture(t, "\n\talpha e\u0301 👩‍💻 beta gamma\n\nlast\n", true)
	item.SetWordWrap(true)
	item.textDrawSize = Point{X: 80, Y: 100}
	layout := item.editLayout()
	for i, line := range layout.lines {
		for _, upstream := range []bool{false, true} {
			pos := line.start + line.length
			want := i
			if line.softBreak && !upstream {
				want++
			}
			got, _ := layout.caretAt(pos, upstream)
			if got != want {
				t.Fatalf("row %d end upstream=%v: got %d want %d", i, upstream, got, want)
			}
		}
	}
	if row, _ := layout.caret(-1); row != 0 {
		t.Fatal("negative caret did not clamp")
	}
	if row, _ := layout.caret(math.MaxInt); row != len(layout.lines)-1 {
		t.Fatal("caret past end did not clamp")
	}
}
func TestDocumentVisibleRowsAtFractionalScrollAndClipEdges(t *testing.T) {
	ctx := newTestContext(t)

	item := ctx.editingFixture(t, strings.Repeat("row\n", 10000), true)
	layout := item.editLayout()
	h := layout.lineHeight
	cases := []struct {
		origin, top, bottom float32
		first, last         int
	}{
		{0, 0, h, 0, 1}, {0, h, h * 2, 1, 2},
		{-h*5000 - h/2, 0, h * 3, 5000, 5004},
		{h * 2, 0, h, 0, 0}, {-h * 20000, 0, h, 10001, 10001},
	}
	for _, tc := range cases {
		first, last := layout.visibleLines(rect{Y0: tc.top, Y1: tc.bottom}, tc.origin)
		if first != tc.first || last != tc.last {
			t.Fatalf("visible rows: %d:%d want %d:%d", first, last, tc.first, tc.last)
		}
	}
}
