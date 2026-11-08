package eui

import (
	"strings"
	"testing"
)

func TestTextEditorFindTextWrapsAndRevealsUnicodeMatches(t *testing.T) {
	ctx := newTestContext(t)

	value := "Élan\n" + strings.Repeat("other line\n", 30) + "élan"
	item := ctx.editingFixture(t, value, true)
	oldSearch := ctx.activeSearch
	t.Cleanup(func() { ctx.activeSearch = oldSearch })
	win := ctx.NewWindow()
	win.Open, win.Searchable = true, true
	win.OpenSearch()
	for _, tc := range []struct {
		start    int
		backward bool
		want     int
	}{{0, false, 0}, {4, false, len([]rune(value)) - 4}, {len([]rune(value)), false, 0}, {-1, true, len([]rune(value)) - 4}} {
		if !item.FindText("ÉLAN", tc.start, tc.backward) || item.SelectStart != tc.want || item.SelectEnd != tc.want+4 {
			t.Fatalf("find %+v: selection %d:%d", tc, item.SelectStart, item.SelectEnd)
		}
		viewport, _ := item.editGeometry()
		item.followEditCaret(viewport)
		if tc.want > 0 && item.editor().scroll.Y <= 0 {
			t.Fatal("distant match was not scrolled into view")
		}
		if ctx.focusedItem != nil || ctx.activeSearch != win || item.Text != value || len(item.editor().undo) != 0 {
			t.Fatal("search stole focus or modified the draft")
		}
	}
	if item.FindText("missing", 0, false) || item.FindText("", 0, false) {
		t.Fatal("empty or absent query matched")
	}
	item.HideText = true
	if item.FindText("Élan", 0, false) {
		t.Fatal("search exposed password text")
	}
}
