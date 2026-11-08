package eui

import "testing"

func TestCloseSearchClearsState(t *testing.T) {
	ctx := newTestContext(t)

	win := &WindowData{ctx: ctx, Searchable: true, searchOpen: true, SearchText: "hi"}
	ctx.activeSearch = win
	var cb string
	win.OnSearch = func(s string) { cb = s }

	win.closeSearch()

	if win.SearchText != "" {
		t.Fatalf("expected empty SearchText, got %q", win.SearchText)
	}
	if cb != "" {
		t.Fatalf("expected OnSearch callback with empty string, got %q", cb)
	}
	if ctx.activeSearch != nil {
		t.Fatalf("expected activeSearch to be nil")
	}
}
